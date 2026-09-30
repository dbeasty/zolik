// Command migrate-match-log moves matches stored before moves and boards
// left the match document onto the layout the server now reads: one record
// per move, a snapshot of the board, and an envelope without either.
//
// Run it with the server stopped. It is safe to run more than once and safe
// to interrupt; see match.MigrateKDB.
//
//	go run ./cmd/migrate-match-log -kdb /path/to/kdb-data
//	go run ./cmd/migrate-match-log -mongo-uri mongodb://... -mongo-db zolik
//
// Pass -dry to count what would move without writing anything.
package main

import (
	"context"
	"flag"
	"log"

	"zolik/server/internal/db"
	"zolik/server/internal/match"
)

func main() {
	kdbPath := flag.String("kdb", "", "KDB_PATH of an embedded store")
	mongoURI := flag.String("mongo-uri", "", "Mongo URI, instead of -kdb")
	mongoDB := flag.String("mongo-db", "zolik", "Mongo database name")
	dry := flag.Bool("dry", false, "count what would migrate, write nothing")
	flag.Parse()

	ctx := context.Background()
	var (
		report match.MigrationReport
		err    error
	)
	switch {
	case *kdbPath != "":
		k, openErr := db.OpenKDB(*kdbPath)
		if openErr != nil {
			log.Fatalf("opening %s (is the server still running?): %v", *kdbPath, openErr)
		}
		report, err = match.MigrateKDB(ctx, k, *dry)
		if closeErr := k.Close(ctx); closeErr != nil && err == nil {
			err = closeErr
		}
	case *mongoURI != "":
		m, connErr := db.Connect(ctx, db.Config{URI: *mongoURI, DB: *mongoDB})
		if connErr != nil {
			log.Fatalf("connecting: %v", connErr)
		}
		report, err = match.MigrateMongo(ctx, m, *dry)
	default:
		log.Fatal("one of -kdb or -mongo-uri is required")
	}
	if err != nil {
		log.Fatalf("migration stopped: %v (so far: %s)", err, report)
	}
	if *dry {
		log.Printf("dry run: %d matches would move (%d moves); %d are already in the new layout",
			report.Matches, report.Moves, report.Skipped)
		return
	}
	log.Printf("done: %s", report)
}
