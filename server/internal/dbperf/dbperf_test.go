// Package dbperf benchmarks the repository layer on both storage engines,
// side by side, over the operations the server actually leans on: the match
// load→apply→CAS-store cycle every gameplay action runs, the session lookup
// every authenticated request runs, and the scan-shaped stats queries.
//
// Run with:
//
//	go test ./internal/dbperf -bench . -benchtime 1x   # smoke
//	go test ./internal/dbperf -bench . -benchmem       # real numbers
//
// The KDB benchmarks always run (the engine is embedded, on disk in a temp
// dir). The Mongo benchmarks need the dev compose stack's MongoDB
// (127.0.0.1:27018, or ZOLIK_TEST_MONGO_URI) and skip without it — the same
// convention the integration tests use.
package dbperf

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"zolik/server/internal/auth"
	"zolik/server/internal/db"
	"zolik/server/internal/match"
	"zolik/server/internal/models"
	"zolik/server/internal/stats"
)

type backend struct {
	name     string
	match    match.Repository
	sessions auth.SessionRepository
	stats    stats.Repository
}

func testMongoURI() string {
	if v := strings.TrimSpace(os.Getenv("ZOLIK_TEST_MONGO_URI")); v != "" {
		return v
	}
	return "mongodb://127.0.0.1:27018"
}

func kdbBackend(b *testing.B) backend {
	b.Helper()
	k, err := db.OpenKDB(b.TempDir())
	if err != nil {
		b.Fatalf("opening kdb: %v", err)
	}
	b.Cleanup(func() { _ = k.Close(context.Background()) })
	return backend{
		name:     db.EngineKDB,
		match:    match.NewKDBRepository(k),
		sessions: auth.NewKDBSessionRepository(k),
		stats:    stats.NewKDBRepository(k),
	}
}

func mongoBackend(b *testing.B) (backend, bool) {
	b.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	client, err := mongo.Connect(options.Client().ApplyURI(testMongoURI()))
	if err != nil {
		return backend{}, false
	}
	if err := client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(context.Background())
		return backend{}, false
	}
	m := &db.Mongo{Client: client, DB: client.Database(fmt.Sprintf("zolik_perf_%d", time.Now().UnixNano()))}
	if err := m.EnsureIndexes(ctx); err != nil {
		b.Fatalf("ensuring indexes: %v", err)
	}
	b.Cleanup(func() {
		dropCtx, dropCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer dropCancel()
		_ = m.DB.Drop(dropCtx)
		_ = client.Disconnect(dropCtx)
	})
	return backend{
		name:     db.EngineMongo,
		match:    match.NewRepository(m),
		sessions: auth.NewSessionRepository(m),
		stats:    stats.NewRepository(m),
	}, true
}

// perEngine runs one benchmark body under a sub-benchmark per engine, so the
// output reads as a direct comparison:
//
//	BenchmarkMatchUpdateWithVersion/kdb-10     ...
//	BenchmarkMatchUpdateWithVersion/mongo-10   ...
func perEngine(b *testing.B, run func(b *testing.B, be backend)) {
	b.Run(db.EngineKDB, func(b *testing.B) {
		run(b, kdbBackend(b))
	})
	b.Run(db.EngineMongo, func(b *testing.B) {
		be, ok := mongoBackend(b)
		if !ok {
			b.Skipf("no reachable mongo at %s (set ZOLIK_TEST_MONGO_URI, or start the dev compose stack)", testMongoURI())
		}
		run(b, be)
	})
}

func testMatch(i int) models.Match {
	return models.Match{
		ModuleID: "prsi",
		Status:   "active",
		JoinCode: fmt.Sprintf("PB%04d", i),
		Players: []models.Player{
			{ID: "p1", Name: "Host"},
			{ID: "p2", Name: "Bot", IsAI: true, AIDifficulty: "normal"},
		},
		TurnOrder: []string{"p1", "p2"},
		HostID:    "p1",
		// A realistic mid-game state blob, not an empty document: the cost
		// being measured is moving a real match through storage.
		State: []byte(`{"drawPile":[1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16],` +
			`"discard":[17,18,19],"hands":{"p1":[20,21,22,23,24],"p2":[25,26,27,28,29]},` +
			`"turn":"p1","direction":1,"pendingDraw":0}`),
		Seed: 42,
	}
}

// BenchmarkMatchInsert is match creation: one document write.
func BenchmarkMatchInsert(b *testing.B) {
	perEngine(b, func(b *testing.B, be backend) {
		ctx := context.Background()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, err := be.match.Insert(ctx, testMatch(i)); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkMatchActionCycle is the hot gameplay path: every action a player
// takes is load → apply → version-checked store.
func BenchmarkMatchActionCycle(b *testing.B) {
	perEngine(b, func(b *testing.B, be backend) {
		ctx := context.Background()
		m, err := be.match.Insert(ctx, testMatch(0))
		if err != nil {
			b.Fatal(err)
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			cur, err := be.match.FindByID(ctx, m.ID)
			if err != nil {
				b.Fatal(err)
			}
			if err := be.match.UpdateWithVersion(ctx, m.ID, cur.Version, cur); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkMatchResolveByJoinCode is the join-a-table lookup, against a
// realistic number of live matches. Mongo answers from an index; KDB scans —
// this benchmark is what says whether that difference matters at this scale.
func BenchmarkMatchResolveByJoinCode(b *testing.B) {
	const liveMatches = 100
	perEngine(b, func(b *testing.B, be backend) {
		ctx := context.Background()
		for i := 0; i < liveMatches; i++ {
			if _, err := be.match.Insert(ctx, testMatch(i)); err != nil {
				b.Fatal(err)
			}
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			code := fmt.Sprintf("PB%04d", i%liveMatches)
			if _, err := be.match.FindByJoinCode(ctx, code); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkSessionFindByToken is the per-request auth path, against a
// realistic session table.
func BenchmarkSessionFindByToken(b *testing.B) {
	const liveSessions = 500
	perEngine(b, func(b *testing.B, be backend) {
		ctx := context.Background()
		for i := 0; i < liveSessions; i++ {
			s := models.Session{
				Token:     fmt.Sprintf("bench-token-%04d", i),
				GuestName: "Bench",
				CreatedAt: time.Now().UTC(),
				ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
			}
			if err := be.sessions.CreateSession(ctx, s); err != nil {
				b.Fatal(err)
			}
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			token := fmt.Sprintf("bench-token-%04d", i%liveSessions)
			if _, err := be.sessions.FindByToken(ctx, token); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkSessionCreate is sign-in: one unique-checked insert.
func BenchmarkSessionCreate(b *testing.B) {
	perEngine(b, func(b *testing.B, be backend) {
		ctx := context.Background()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			s := models.Session{
				Token:     fmt.Sprintf("create-token-%08d", i),
				GuestName: "Bench",
				CreatedAt: time.Now().UTC(),
				ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
			}
			if err := be.sessions.CreateSession(ctx, s); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkStatsUpsert is the end-of-match write path for one subject.
func BenchmarkStatsUpsert(b *testing.B) {
	perEngine(b, func(b *testing.B, be backend) {
		ctx := context.Background()
		ps := stats.PlayerStats{
			SubjectKey: "user:bench",
			Subject:    stats.Subject{Kind: stats.SubjectUser, ID: "bench"},
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ps.Overall.Matches = i
			if err := be.stats.UpsertPlayerStats(ctx, ps); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkLeaderboard ranks a populated player table.
func BenchmarkLeaderboard(b *testing.B) {
	const players = 200
	perEngine(b, func(b *testing.B, be backend) {
		ctx := context.Background()
		for i := 0; i < players; i++ {
			ps := stats.PlayerStats{
				SubjectKey: fmt.Sprintf("user:p%03d", i),
				Subject:    stats.Subject{Kind: stats.SubjectUser, ID: fmt.Sprintf("p%03d", i)},
			}
			ps.Overall.Matches = 10 + i
			ps.Overall.Wins = i % 10
			if err := be.stats.UpsertPlayerStats(ctx, ps); err != nil {
				b.Fatal(err)
			}
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			rows, err := be.stats.Leaderboard(ctx, stats.LeaderboardQuery{Limit: 20})
			if err != nil {
				b.Fatal(err)
			}
			if len(rows) == 0 {
				b.Fatal("empty leaderboard")
			}
		}
	})
}

// BenchmarkMatchHistory pages one subject's history out of a populated
// match_results table.
func BenchmarkMatchHistory(b *testing.B) {
	const records = 300
	perEngine(b, func(b *testing.B, be backend) {
		ctx := context.Background()
		base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		for i := 0; i < records; i++ {
			subject := fmt.Sprintf("user:h%02d", i%10)
			rec := stats.MatchResult{
				MatchID:     bson.NewObjectID(),
				ModuleID:    "prsi",
				StartedAt:   base.Add(time.Duration(i) * time.Hour),
				CompletedAt: base.Add(time.Duration(i)*time.Hour + 30*time.Minute),
				SubjectKeys: []string{subject},
				Participants: []stats.Standing{{
					PlayerID: "p1",
					Subject:  stats.Subject{Kind: stats.SubjectUser, ID: subject},
				}},
				RecordedAt: base.Add(time.Duration(i) * time.Hour),
			}
			if _, err := be.stats.InsertMatch(ctx, rec); err != nil {
				b.Fatal(err)
			}
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			subject := fmt.Sprintf("user:h%02d", i%10)
			page, err := be.stats.ListMatchesForSubject(ctx, subject, time.Time{}, 25)
			if err != nil {
				b.Fatal(err)
			}
			if len(page) == 0 {
				b.Fatal("empty history page")
			}
		}
	})
}
