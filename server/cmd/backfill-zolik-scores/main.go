// Command backfill-zolik-scores repairs the recorded score of every Žolíky
// match, and fills in the deal history those records never carried.
//
// Žolíky's scoreboard used to rank players by the cards they were holding at
// the instant the match ended — the leftovers of one deal out of seven, not the
// match score. The runtime records the module's own standings as the permanent
// result, so that number became the player's score in `match_results`, and
// every lifetime figure derived from it (ScoreSum, BestScore, WorstScore,
// AvgScore, and the pairwise head-to-head totals) was built on it.
//
// The fix changed the scoreboard. It did not change what is already written,
// and the two are not on the same scale — a hand penalty is roughly 0..200 and
// a Continental total 0..500+ — so summing old rows with new ones produces a
// figure that means nothing. Hence this.
//
//	MONGO_URI=... MONGO_DB=... go run ./cmd/backfill-zolik-scores
//
// Pass -dry to report what would change without writing.
//
// It works by re-asking the module. The match document still holds the state
// the match ended in, so the corrected standings are the ones the fixed code
// produces from it — not an arithmetic patch applied here, which would be a
// second implementation of how rummy is scored. The same pass fills in the
// round history, for the same reason and from the same source: the engine has
// kept per-deal scores all along.
//
// A match whose document has expired cannot be repaired, and is counted and
// named rather than quietly left. Such a row keeps a score on the old scale
// permanently, which is worth knowing about rather than discovering later.
//
// Safe to run twice: it recomputes from the state each time and writes the same
// answer.
package main

import (
	"context"
	"flag"
	"log"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/app"
	"zolik/server/internal/db"
	"zolik/server/internal/models"
	"zolik/server/internal/module"
	"zolik/server/internal/stats"
	"zolik/server/internal/zolikmod"
)

func main() {
	dry := flag.Bool("dry", false, "report what would change, write nothing")
	flag.Parse()

	_ = godotenv.Load()
	cfg := app.LoadConfig()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	m, err := db.Connect(ctx, db.Config{URI: cfg.MongoURI, DB: cfg.MongoDB})
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer m.Client.Disconnect(context.Background())

	mod := zolikmod.New()
	results := m.DB.Collection("match_results")
	matches := m.DB.Collection("matches")

	cur, err := results.Find(ctx, bson.M{"moduleId": "zolik"})
	if err != nil {
		log.Fatalf("find results: %v", err)
	}
	defer cur.Close(ctx)

	var (
		seen, fixed, unchanged, orphaned int
		touched                          = map[string]stats.Subject{}
	)

	for cur.Next(ctx) {
		var rec stats.MatchResult
		if err := cur.Decode(&rec); err != nil {
			log.Fatalf("decode result: %v", err)
		}
		seen++

		var match models.Match
		err := matches.FindOne(ctx, bson.M{"_id": rec.MatchID}).Decode(&match)
		if err != nil {
			orphaned++
			log.Printf("match=%s: document is gone; its score stays on the old scale", rec.MatchID.Hex())
			continue
		}
		if len(match.State) == 0 {
			orphaned++
			log.Printf("match=%s: no state to recompute from", rec.MatchID.Hex())
			continue
		}

		// The module's own answer, from the state the match ended in.
		sb := stats.BuildScoreboard(match, module.OutcomeOf(mod, match.State))
		if len(sb.Standings) == 0 {
			orphaned++
			log.Printf("match=%s: the module produced no standings", rec.MatchID.Hex())
			continue
		}

		if sameScores(rec.Participants, sb.Standings) && len(rec.Rounds) == len(sb.Rounds) {
			unchanged++
			continue
		}
		fixed++
		log.Printf("match=%s: %s -> %s, %d rounds recorded",
			rec.MatchID.Hex(), scoreLine(rec.Participants), scoreLine(sb.Standings), len(sb.Rounds))

		for _, s := range sb.Standings {
			if s.Subject.Durable() {
				touched[s.Subject.Key()] = s.Subject
			}
		}
		if *dry {
			continue
		}
		_, err = results.UpdateOne(ctx, bson.M{"_id": rec.ID}, bson.M{"$set": bson.M{
			"participants":  sb.Standings,
			"rounds":        sb.Rounds,
			"roundLabelKey": sb.RoundLabelKey,
		}})
		if err != nil {
			log.Fatalf("match=%s: update failed: %v", rec.MatchID.Hex(), err)
		}
	}
	if err := cur.Err(); err != nil {
		log.Fatalf("iterate: %v", err)
	}

	log.Printf("%d Žolíky records: %d corrected, %d already right, %d unrepairable",
		seen, fixed, unchanged, orphaned)

	if *dry || len(touched) == 0 {
		if *dry && len(touched) > 0 {
			log.Printf("dry run: %d lifetime records would be rebuilt", len(touched))
		}
		return
	}

	// The aggregates are derived, so they are rebuilt rather than patched —
	// replayed oldest-first from the records, which is the same path a guest's
	// claimed history takes and the one this repair exists to feed.
	claimer := stats.NewClaimer(stats.NewRepository(m))
	rebuilt := 0
	for _, subj := range touched {
		if err := claimer.RebuildPlayerStats(ctx, subj); err != nil {
			log.Printf("subject=%s: rebuild failed: %v", subj.Key(), err)
			continue
		}
		rebuilt++
	}
	log.Printf("rebuilt %d lifetime records", rebuilt)
}

// sameScores reports the recorded placings already agreeing with the module's.
func sameScores(was, now []stats.Standing) bool {
	if len(was) != len(now) {
		return false
	}
	byID := map[string]stats.Standing{}
	for _, s := range was {
		byID[s.PlayerID] = s
	}
	for _, s := range now {
		old, ok := byID[s.PlayerID]
		if !ok || old.Score != s.Score || old.Rank != s.Rank || old.Won != s.Won {
			return false
		}
	}
	return true
}

func scoreLine(ss []stats.Standing) string {
	out := ""
	for i, s := range ss {
		if i > 0 {
			out += " "
		}
		out += s.Name + ":" + strconv.Itoa(s.Score)
	}
	return "[" + out + "]"
}
