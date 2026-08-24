// Command migrate-games copies `games` documents into `matches`.
//
// The first of the two leftovers `extensibility-plan.md` §3.x recorded: the
// module runtime shipped alongside the Žolíky path rather than replacing it,
// and the rummy documents stayed where they were. This moves them.
//
// It is additive and idempotent. Nothing is deleted or altered in `games`, and
// a match already carrying `migratedFrom` is skipped — so it is safe to run,
// inspect the result, and run again.
//
//	MONGO_URI=... MONGO_DB=... go run ./cmd/migrate-games
//
// Pass -dry to count what would move without writing anything.
//
// What makes this safe to do at all is that there are not two rummy engines:
// `game.Manager` and `zolikmod` both apply moves with `rules.ApplyAction` on a
// `rules.GameState`, so a migrated game cannot start playing by different
// rules. `internal/game/module_equivalence_test.go` pins the conversion
// itself — same offers, same legality, same result from the same move.
package main

import (
	"context"
	"flag"
	"log"
	"time"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/app"
	"zolik/server/internal/db"
	"zolik/server/internal/legacy"
)

func main() {
	dry := flag.Bool("dry", false, "count what would be migrated, write nothing")
	flag.Parse()

	_ = godotenv.Load()
	cfg := app.LoadConfig()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	m, err := db.Connect(ctx, db.Config{URI: cfg.MongoURI, DB: cfg.MongoDB})
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer m.Client.Disconnect(context.Background())

	if *dry {
		games, err := m.DB.Collection("games").CountDocuments(ctx, bson.M{})
		if err != nil {
			log.Fatalf("count games: %v", err)
		}
		done, err := m.DB.Collection("matches").CountDocuments(ctx,
			bson.M{"migratedFrom": bson.M{"$exists": true}})
		if err != nil {
			log.Fatalf("count migrated: %v", err)
		}
		log.Printf("dry run: %d games, %d already migrated, %d to go", games, done, games-done)
		return
	}

	migrated, skipped, err := legacy.MigrateGames(ctx, m.DB)
	if err != nil {
		log.Fatalf("migrate (after %d migrated, %d skipped): %v", migrated, skipped, err)
	}
	log.Printf("migrated %d games, skipped %d already done", migrated, skipped)
}
