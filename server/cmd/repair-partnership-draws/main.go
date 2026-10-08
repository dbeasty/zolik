// Command repair-partnership-draws stores partnership wins that were recorded
// as draws as the wins they were, and rebuilds the lifetime records they fed.
//
// Until the scoreboard learned about sides, any match with more than one
// winner was recorded as a draw. A Canasta partnership always has two winners,
// so every partnership win went down as a draw for both partners, and a player
// who won every match had a record of none. The fix stopped new records going
// wrong; this repairs the ones already written. See stats.RepairPartnershipDraws.
//
// Run it with the server stopped when the store is kdb, which is single-process.
// It is safe to run more than once.
//
//	go run ./cmd/repair-partnership-draws -kdb /path/to/kdb-data
//	go run ./cmd/repair-partnership-draws -mongo-uri mongodb://... -mongo-db zolik
//
// Pass -dry to count what would change without writing anything.
//
// Sides come from each module's own Seated.Sides, asked with the seats in the
// order the record kept them. A record keeps its variation but not its table
// options, so a module whose sides depended on an option would be asked without
// it; Canasta, the only game with sides, decides them from the seat count alone.
package main

import (
	"context"
	"flag"
	"log"

	"zolik/server/internal/blackjack"
	"zolik/server/internal/canasta"
	"zolik/server/internal/db"
	"zolik/server/internal/ginrummy"
	"zolik/server/internal/holdem"
	"zolik/server/internal/lastcard"
	"zolik/server/internal/marias"
	"zolik/server/internal/module"
	"zolik/server/internal/okobere"
	"zolik/server/internal/prsi"
	"zolik/server/internal/rummytiles"
	"zolik/server/internal/sedma"
	"zolik/server/internal/snaps"
	"zolik/server/internal/stats"
	"zolik/server/internal/zolikmod"
)

func main() {
	kdbPath := flag.String("kdb", "", "KDB_PATH of an embedded store")
	mongoURI := flag.String("mongo-uri", "", "Mongo URI, instead of -kdb")
	mongoDB := flag.String("mongo-db", "zolik", "Mongo database name")
	dry := flag.Bool("dry", false, "count what would change, write nothing")
	flag.Parse()

	modules := module.NewRegistry(zolikmod.New(), prsi.New(), canasta.New(), holdem.New(),
		ginrummy.New(), rummytiles.New(), blackjack.New(), marias.New(), lastcard.New(), okobere.New(), sedma.New(), snaps.New())
	sidesOf := func(m stats.MatchResult) [][]string {
		mod := modules.Get(m.ModuleID)
		if mod == nil {
			return nil
		}
		seats := stats.SeatOrder(m)
		refs := make([]module.PlayerRef, len(seats))
		for i, id := range seats {
			refs[i] = module.PlayerRef{ID: id}
		}
		return module.SidesOf(mod, module.MatchConfig{Variation: m.Variation}, refs)
	}

	ctx := context.Background()
	var (
		report stats.DrawRepairReport
		err    error
	)
	switch {
	case *kdbPath != "":
		k, openErr := db.OpenKDB(*kdbPath)
		if openErr != nil {
			log.Fatalf("opening %s (is the server still running?): %v", *kdbPath, openErr)
		}
		report, err = stats.RepairPartnershipDraws(ctx, stats.NewKDBRepository(k), sidesOf, *dry)
		if closeErr := k.Close(ctx); closeErr != nil && err == nil {
			err = closeErr
		}
	case *mongoURI != "":
		m, connErr := db.Connect(ctx, db.Config{URI: *mongoURI, DB: *mongoDB})
		if connErr != nil {
			log.Fatalf("connecting: %v", connErr)
		}
		report, err = stats.RepairPartnershipDraws(ctx, stats.NewRepository(m), sidesOf, *dry)
	default:
		log.Fatal("one of -kdb or -mongo-uri is required")
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
