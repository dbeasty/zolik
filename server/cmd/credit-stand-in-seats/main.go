// Command credit-stand-in-seats gives named seats of finished matches their
// result back, where the match was marked as partly played by a stand-in bot
// that never actually played for them.
//
// Until a stand-in was noted only when it played a move, putting one in was
// enough to mark the seat: a stand-in that never moved, or only pressed
// Continue at the pause between rounds, cost its player the whole match's
// result, and the results screen said a bot had played part of the game for
// them. The match log does not record which moves a stand-in made, so these
// cannot be found by reading the store; the seats are named by somebody who
// knows, the player who played them. See stats.CreditStandInSeats.
//
// Run it with the server stopped when the store is kdb, which is single-process.
// It is safe to run more than once.
//
//	go run ./cmd/credit-stand-in-seats -kdb /path/to/kdb-data -seat <matchId>:<playerId> [-seat ...]
//	go run ./cmd/credit-stand-in-seats -mongo-uri mongodb://... -mongo-db zolik -seat ...
//
// Pass -dry to count what would change without writing anything.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/db"
	"zolik/server/internal/match"
	"zolik/server/internal/stats"
)

type seatFlags []stats.StandInSeat

func (s *seatFlags) String() string { return fmt.Sprint(len(*s), " seats") }

func (s *seatFlags) Set(v string) error {
	matchID, playerID, ok := strings.Cut(v, ":")
	if !ok || playerID == "" {
		return fmt.Errorf("%q: want <matchId>:<playerId>", v)
	}
	id, err := bson.ObjectIDFromHex(matchID)
	if err != nil {
		return fmt.Errorf("%q: %q is not a match id", v, matchID)
	}
	*s = append(*s, stats.StandInSeat{MatchID: id, PlayerID: playerID})
	return nil
}

func main() {
	kdbPath := flag.String("kdb", "", "KDB_PATH of an embedded store")
	mongoURI := flag.String("mongo-uri", "", "Mongo URI, instead of -kdb")
	mongoDB := flag.String("mongo-db", "zolik", "Mongo database name")
	dry := flag.Bool("dry", false, "count what would change, write nothing")
	var seats seatFlags
	flag.Var(&seats, "seat", "<matchId>:<playerId> to credit; repeat for more")
	flag.Parse()
	if len(seats) == 0 {
		log.Fatal("name at least one -seat <matchId>:<playerId>")
	}

	ctx := context.Background()
	var (
		statsRepo stats.Repository
		matches   match.Repository
		closeFn   = func() error { return nil }
	)
	switch {
	case *kdbPath != "":
		k, err := db.OpenKDB(*kdbPath)
		if err != nil {
			log.Fatalf("opening %s (is the server still running?): %v", *kdbPath, err)
		}
		statsRepo, matches = stats.NewKDBRepository(k), match.NewKDBRepository(k)
		closeFn = func() error { return k.Close(ctx) }
	case *mongoURI != "":
		m, err := db.Connect(ctx, db.Config{URI: *mongoURI, DB: *mongoDB})
		if err != nil {
			log.Fatalf("connecting: %v", err)
		}
		statsRepo, matches = stats.NewRepository(m), match.NewRepository(m)
	default:
		log.Fatal("one of -kdb or -mongo-uri is required")
	}

	report, err := stats.CreditStandInSeats(ctx, statsRepo, seats, *dry)
	if err == nil && !*dry {
		err = clearStoodIn(ctx, matches, seats)
	}
	if closeErr := closeFn(); closeErr != nil && err == nil {
		err = closeErr
	}
	if err != nil {
		log.Fatalf("repair stopped: %v (so far: %s)", err, report)
	}
	if *dry {
		log.Printf("dry run: %s", report)
		return
	}
	log.Printf("done: %s", report)
}

// clearStoodIn takes the seats off the match's own StoodIn, which is what the
// results screen reads when it says a bot played part of the game for them.
func clearStoodIn(ctx context.Context, repo match.Repository, seats []stats.StandInSeat) error {
	for _, s := range seats {
		m, err := repo.FindByID(ctx, s.MatchID)
		if db.IsNotFound(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("match %s: %w", s.MatchID.Hex(), err)
		}
		kept := make([]string, 0, len(m.StoodIn))
		for _, id := range m.StoodIn {
			if id != s.PlayerID {
				kept = append(kept, id)
			}
		}
		if len(kept) == len(m.StoodIn) {
			continue
		}
		next := m
		next.StoodIn = kept
		if len(kept) == 0 {
			next.StoodIn = nil
		}
		if err := repo.UpdateWithVersion(ctx, m.ID, m.Version, next); err != nil {
			return fmt.Errorf("match %s: %w", s.MatchID.Hex(), err)
		}
	}
	return nil
}
