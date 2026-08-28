package stats

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/db"
)

func newKDBRepo(t *testing.T) Repository {
	t.Helper()
	k, err := db.OpenKDB(t.TempDir())
	if err != nil {
		t.Fatalf("opening kdb: %v", err)
	}
	t.Cleanup(func() { _ = k.Close(context.Background()) })
	return NewKDBRepository(k)
}

func kdbRecord(matchID bson.ObjectID, key string, completedAt time.Time) MatchResult {
	return MatchResult{
		MatchID:     matchID,
		ModuleID:    "prsi",
		StartedAt:   completedAt.Add(-10 * time.Minute),
		CompletedAt: completedAt,
		SubjectKeys: []string{key},
		Participants: []Standing{{
			PlayerID: "p1",
			Subject:  Subject{Kind: SubjectUser, ID: key},
		}},
		RecordedAt: completedAt,
	}
}

func TestKDBInsertMatchIsIdempotent(t *testing.T) {
	r := newKDBRepo(t)
	ctx := context.Background()

	matchID := bson.NewObjectID()
	rec := kdbRecord(matchID, "user:a", time.Now().UTC())
	if _, err := r.InsertMatch(ctx, rec); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	// A retried or concurrently-observed completion must be a no-op signal,
	// not a second record double-counting everyone's lifetime stats.
	if _, err := r.InsertMatch(ctx, rec); !errors.Is(err, ErrAlreadyRecorded) {
		t.Fatalf("second insert: got %v, want ErrAlreadyRecorded", err)
	}
	n, err := r.CountMatchesForSubject(ctx, "user:a")
	if err != nil || n != 1 {
		t.Fatalf("count = %d (%v), want 1", n, err)
	}

	found, err := r.FindMatchByMatchID(ctx, matchID)
	if err != nil || found.MatchID != matchID {
		t.Fatalf("find by matchId: %v", err)
	}
	if _, err := r.FindMatchByMatchID(ctx, bson.NewObjectID()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("find unknown: got %v, want ErrNotFound", err)
	}
}

func TestKDBMatchHistoryOrderingAndPaging(t *testing.T) {
	r := newKDBRepo(t)
	ctx := context.Background()

	base := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		rec := kdbRecord(bson.NewObjectID(), "user:pager", base.Add(time.Duration(i)*time.Hour))
		if _, err := r.InsertMatch(ctx, rec); err != nil {
			t.Fatalf("insert %d: %v", i, err)
		}
	}

	page, err := r.ListMatchesForSubject(ctx, "user:pager", time.Time{}, 3)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(page) != 3 {
		t.Fatalf("page size = %d, want 3", len(page))
	}
	for i := 1; i < len(page); i++ {
		if page[i].CompletedAt.After(page[i-1].CompletedAt) {
			t.Fatalf("history not newest-first: %v then %v", page[i-1].CompletedAt, page[i].CompletedAt)
		}
	}

	rest, err := r.ListMatchesForSubject(ctx, "user:pager", page[len(page)-1].CompletedAt, 10)
	if err != nil {
		t.Fatalf("second page: %v", err)
	}
	if len(rest) != 2 {
		t.Fatalf("second page size = %d, want 2", len(rest))
	}

	var walked []time.Time
	err = r.EachMatchForSubject(ctx, "user:pager", func(m MatchResult) error {
		walked = append(walked, m.CompletedAt)
		return nil
	})
	if err != nil {
		t.Fatalf("each: %v", err)
	}
	if len(walked) != 5 {
		t.Fatalf("walked %d records, want 5", len(walked))
	}
	for i := 1; i < len(walked); i++ {
		if walked[i].Before(walked[i-1]) {
			t.Fatalf("walk not oldest-first at %d", i)
		}
	}
}

func TestKDBPlayerStatsUpsertAndLeaderboard(t *testing.T) {
	r := newKDBRepo(t)
	ctx := context.Background()

	if _, err := r.FindPlayerStats(ctx, "user:new"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("fresh subject: got %v, want ErrNotFound", err)
	}

	for _, row := range []PlayerStats{
		{SubjectKey: "user:top", Subject: Subject{Kind: SubjectUser, ID: "top"}, Overall: Tally{Matches: 10, Wins: 8}},
		{SubjectKey: "user:mid", Subject: Subject{Kind: SubjectUser, ID: "mid"}, Overall: Tally{Matches: 10, Wins: 5}},
		{SubjectKey: "ai:hard", Subject: Subject{Kind: SubjectAI, ID: "hard"}, Overall: Tally{Matches: 100, Wins: 90}},
	} {
		if err := r.UpsertPlayerStats(ctx, row); err != nil {
			t.Fatalf("upsert %s: %v", row.SubjectKey, err)
		}
	}

	// Upsert must replace, not accumulate.
	if err := r.UpsertPlayerStats(ctx, PlayerStats{
		SubjectKey: "user:top", Subject: Subject{Kind: SubjectUser, ID: "top"},
		Overall: Tally{Matches: 11, Wins: 9},
	}); err != nil {
		t.Fatalf("re-upsert: %v", err)
	}
	got, err := r.FindPlayerStats(ctx, "user:top")
	if err != nil || got.Overall.Matches != 11 {
		t.Fatalf("after re-upsert: %+v (%v)", got.Overall, err)
	}

	many, err := r.FindManyPlayerStats(ctx, []string{"user:top", "user:mid", "user:absent"})
	if err != nil {
		t.Fatalf("find many: %v", err)
	}
	if len(many) != 2 {
		t.Fatalf("find many returned %d rows, want 2 (absent subject silently missing)", len(many))
	}

	rows, err := r.Leaderboard(ctx, LeaderboardQuery{})
	if err != nil {
		t.Fatalf("leaderboard: %v", err)
	}
	// Bots must not appear on the default (user) leaderboard, and the
	// highest-wins user ranks first.
	if len(rows) != 2 {
		t.Fatalf("leaderboard rows = %d, want 2", len(rows))
	}
	if rows[0].Subject.ID != "top" || rows[0].Rank != 1 {
		t.Fatalf("rank 1 = %+v, want user:top", rows[0])
	}
}

func TestKDBReplaceMatchAttributionOnlyMovesIdentity(t *testing.T) {
	r := newKDBRepo(t)
	ctx := context.Background()

	rec, err := r.InsertMatch(ctx, kdbRecord(bson.NewObjectID(), "guest:g1", time.Now().UTC()))
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	claimed := rec
	claimed.SubjectKeys = []string{"user:claimer"}
	claimed.Participants[0].Subject = Subject{Kind: SubjectUser, ID: "claimer"}
	// Attempt to smuggle a score edit through the attribution path.
	claimed.ModuleID = "hacked"
	if err := r.ReplaceMatchAttribution(ctx, claimed); err != nil {
		t.Fatalf("replace attribution: %v", err)
	}

	got, err := r.FindMatchByMatchID(ctx, rec.MatchID)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got.SubjectKeys[0] != "user:claimer" {
		t.Fatalf("attribution did not move: %v", got.SubjectKeys)
	}
	if got.ModuleID != "prsi" {
		t.Fatalf("attribution path edited a non-identity field: moduleId=%s", got.ModuleID)
	}
}
