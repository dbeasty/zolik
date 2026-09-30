// Command migrate-namespaces moves an embedded store's matches out of the two
// shared namespaces they used to live in and into a namespace per match — the
// layout internal/match/repository_kdb.go now reads and writes.
//
// Old layout: every match envelope in "matches", keyed by its hex id, and
// every move and snapshot in "match_log", keyed "<hex>/m/<seq>" and
// "<hex>/s/<seq>". New layout: the envelope under "match" in "m/<hex>", the
// moves under "m/<seq>" and the snapshots under "s/<seq>" in the same place,
// with a copy of the envelope left behind in "matches" — which stops being the
// matches and becomes this node's local index of them, for the scans (join
// code, a player's games, the abandonment sweep) that no single match's
// namespace can answer.
//
// This exists because a namespace is KDB's unit of replication and
// authorization. A match in a shared namespace can only be synced to a device
// along with every other match in it; a match in a namespace of its own can be
// synced to exactly the devices of the people sitting at it.
//
// Run it with the server stopped. It opens the store the same way the server
// does, so a running server's directory lock makes this fail loudly at startup
// rather than racing it.
//
// It is safe to run more than once and safe to interrupt: each match moves in
// one cross-namespace transaction, and a match already in its own namespace is
// skipped. A store the server has played on since the deploy is handled too —
// writing an envelope already puts a copy in the match's own namespace, so a
// match can have arrived here with its envelope moved and its moves not.
//
// Every match is read back out of its new namespace, record by record, before
// this reports success, and the old log is checked to no longer hold what was
// moved.
//
//	go run ./cmd/migrate-namespaces -path /path/to/kdb-data
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
	path := flag.String("path", "", "KDB_PATH of the embedded store (required)")
	dry := flag.Bool("dry", false, "count what would move, write nothing")
	flag.Parse()

	if *path == "" {
		log.Fatal("-path is required")
	}

	ctx := context.Background()
	k, err := db.OpenKDB(*path)
	if err != nil {
		log.Fatalf("opening %s (is the server still running?): %v", *path, err)
	}
	report, err := match.MigrateNamespacesKDB(ctx, k, *dry)
	if closeErr := k.Close(ctx); closeErr != nil && err == nil {
		err = closeErr
	}
	if err != nil {
		log.Fatalf("migration stopped: %v (so far: %s)", err, report)
	}
	if *dry {
		log.Printf("dry run: %d matches would move (%d records); %d are already in a namespace of "+
			"their own; %d orphaned records would be left alone",
			report.Matches, report.Records, report.Skipped, report.Orphans)
		return
	}
	log.Printf("done: %s", report)
}
