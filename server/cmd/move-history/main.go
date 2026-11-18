// Command move-history moves every finished match one player identity played
// onto an account, and rebuilds both lifetime records.
//
// Signing in claims a device's guest history on its own. What it cannot mend
// is one person with two accounts: until registration refused an address that
// already had an account, somebody who typed their address on the username
// screens could make a second one, play on it, and see their real record stand
// still. Name the second account (or a guest id) and the account it belongs to.
//
// Run it with the server stopped when the store is kdb, which is single-process.
// It is safe to run more than once.
//
//	go run ./cmd/move-history -kdb /path/to/kdb-data -from user:<hex> -into <userHex> -name <username>
//	go run ./cmd/move-history -kdb /path/to/kdb-data -from guest:<guestId> -into <userHex> -name <username>
//
// Pass -dry to count what would move without writing anything.
package main

import (
	"context"
	"flag"
	"log"
	"strings"

	"zolik/server/internal/db"
	"zolik/server/internal/stats"
)

func main() {
	kdbPath := flag.String("kdb", "", "KDB_PATH of an embedded store")
	mongoURI := flag.String("mongo-uri", "", "Mongo URI, instead of -kdb")
	mongoDB := flag.String("mongo-db", "zolik", "Mongo database name")
	from := flag.String("from", "", "user:<hex> or guest:<id> whose matches move")
	into := flag.String("into", "", "hex id of the account they move to")
	name := flag.String("name", "", "that account's username")
	dry := flag.Bool("dry", false, "count what would move, write nothing")
	flag.Parse()

	k, id, ok := strings.Cut(*from, ":")
	kind := stats.SubjectKind(k)
	if !ok || id == "" || (kind != stats.SubjectUser && kind != stats.SubjectGuest) {
		log.Fatal("-from must be user:<hex> or guest:<id>")
	}
	if *into == "" || *name == "" {
		log.Fatal("-into and -name are required")
	}
	src := stats.Subject{Kind: kind, ID: id}
	dst := stats.Subject{Kind: stats.SubjectUser, ID: *into, Name: *name}

	ctx := context.Background()
	var (
		repo    stats.Repository
		closeFn = func() error { return nil }
	)
	switch {
	case *kdbPath != "":
		k, err := db.OpenKDB(*kdbPath)
		if err != nil {
			log.Fatalf("opening %s (is the server still running?): %v", *kdbPath, err)
		}
		repo = stats.NewKDBRepository(k)
		closeFn = func() error { return k.Close(ctx) }
	case *mongoURI != "":
		m, err := db.Connect(ctx, db.Config{URI: *mongoURI, DB: *mongoDB})
		if err != nil {
			log.Fatalf("connecting: %v", err)
		}
		repo = stats.NewRepository(m)
	default:
		log.Fatal("one of -kdb or -mongo-uri is required")
	}

	var (
		n   int
		err error
	)
	if *dry {
		n, err = repo.CountMatchesForSubject(ctx, src.Key())
	} else {
		n, err = stats.NewClaimer(repo).MoveHistory(ctx, src, dst)
	}
	if closeErr := closeFn(); closeErr != nil && err == nil {
		err = closeErr
	}
	if err != nil {
		log.Fatalf("stopped after %d matches: %v", n, err)
	}
	if *dry {
		log.Printf("dry run: %d matches recorded for %s would move to %s", n, src.Key(), dst.Key())
		return
	}
	log.Printf("done: moved %d matches from %s to %s", n, src.Key(), dst.Key())
}
