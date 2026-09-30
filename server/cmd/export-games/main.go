// Command export-games writes the decisions people made in finished matches
// as training records for the learning bots: one JSON line per decision, as
// internal/learnexport describes them.
//
// It reads, and only reads. Nothing it does writes to the store it is pointed
// at: Mongo is only queried (no index is ensured, no document touched), and a
// KDB store is copied first and the copy opened, because opening a KDB data
// root is not a read — the engine recovers its log, and the sweeper it starts
// deletes expired sessions.
//
// Where it reads from is always said, never assumed. There is no default
// database: the store comes from the flags, or from the same environment
// variables the server reads (FEATURE_FLAG_DB_ENGINE, MONGO_URI, MONGO_DB,
// KDB_PATH), and a .env file is not loaded — a developer's .env is exactly
// where a production URI would be sitting. A Mongo URI naming any host but this
// one is refused unless -allow-remote says it was meant.
//
//	go run ./cmd/export-games -game holdem -salt "$SALT" -kdb ./data/kdb -out holdem.jsonl.gz
//	go run ./cmd/export-games -game canasta -salt "$SALT" -mongo-uri mongodb://localhost:27017 \
//	    -since 2026-06-01 -until 2026-09-30 -out canasta.jsonl
//
// Run it with the server stopped, or against a copy of its data: a KDB root
// copied while the server is writing it can be a torn copy.
package main

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"zolik/server/internal/db"
	"zolik/server/internal/learn"
	"zolik/server/internal/learnexport"
	"zolik/server/internal/match"
	"zolik/server/internal/models"
	"zolik/server/internal/rules"

	_ "zolik/server/internal/canasta"
	_ "zolik/server/internal/holdem"
	_ "zolik/server/internal/zolikmod"
)

func main() {
	log.SetFlags(0)
	log.SetPrefix("export-games: ")
	if err := run(context.Background(), os.Args[1:], os.Stderr); err != nil {
		log.Fatal(err)
	}
}

// run is the whole command, with its arguments and its summary's destination
// passed in, so a test can drive it against a store it built.
func run(ctx context.Context, args []string, summary io.Writer) error {
	fl := flag.NewFlagSet("export-games", flag.ContinueOnError)
	game := fl.String("game", "", "game to export: holdem or canasta (required)")
	since := fl.String("since", "", "first day, YYYY-MM-DD (UTC, by when the match ended); empty is no bound")
	until := fl.String("until", "", "last day, YYYY-MM-DD, inclusive; empty is no bound")
	limit := fl.Int("limit", 0, "export at most this many matches; 0 is all")
	out := fl.String("out", "", "output file, JSON lines; gzip when it ends in .gz; - for stdout (required)")
	salt := fl.String("salt", "", "salt for the match and seat hashes (required; keep it with the data, never in it)")
	includeBots := fl.Bool("include-bots", false, "export bot decisions too, marked bot:true")
	includeAbandoned := fl.Bool("include-abandoned", false, "export abandoned matches as well as completed ones")
	includeActive := fl.Bool("include-active", false, "export matches still in progress as well (their moves so far); for reviewing a game being played, not for training")

	engine := fl.String("engine", os.Getenv("FEATURE_FLAG_DB_ENGINE"), "mongo or kdb (default $FEATURE_FLAG_DB_ENGINE; else whichever of -kdb/-mongo-uri is set)")
	mongoURI := fl.String("mongo-uri", os.Getenv("MONGO_URI"), "Mongo URI (default $MONGO_URI)")
	mongoDB := fl.String("mongo-db", envOr("MONGO_DB", "zolik"), "Mongo database (default $MONGO_DB, else zolik)")
	kdbPath := fl.String("kdb", os.Getenv("KDB_PATH"), "KDB data root (default $KDB_PATH); copied, never opened in place")
	allowRemote := fl.Bool("allow-remote", false, "allow a Mongo URI that names a host other than this machine")
	if err := fl.Parse(args); err != nil {
		return err
	}

	g, err := learn.LookupGame(*game)
	if err != nil {
		return err
	}
	if *salt == "" {
		return errors.New("-salt is required: the hashes are only as private as the salt is secret")
	}
	if *out == "" {
		return errors.New("-out is required")
	}
	filter := match.FinishedFilter{ModuleID: g.Name(), Limit: *limit}
	if *includeAbandoned || *includeActive {
		filter.Statuses = []string{"completed"}
		if *includeAbandoned {
			filter.Statuses = append(filter.Statuses, string(rules.StatusAbandoned))
		}
		if *includeActive {
			// Suspended is still in progress: somebody's socket closed and the
			// reaper has not yet called it abandoned.
			filter.Statuses = append(filter.Statuses, "active", "suspended")
		}
	}
	if filter.EndedFrom, err = day(*since, 0); err != nil {
		return fmt.Errorf("-since: %w", err)
	}
	if filter.EndedBefore, err = day(*until, 1); err != nil {
		return fmt.Errorf("-until: %w", err)
	}

	repo, where, closeStore, err := openStore(ctx, *engine, *mongoURI, *mongoDB, *kdbPath, *allowRemote)
	if err != nil {
		return err
	}
	defer closeStore()
	log.Printf("reading %s matches from %s", g.Name(), where)

	w, finish, err := create(*out)
	if err != nil {
		return err
	}

	var total learnexport.Stats
	failed := 0
	enc := json.NewEncoder(w)
	opt := learnexport.Options{Salt: *salt, IncludeBots: *includeBots}
	err = repo.EachFinished(ctx, filter, func(m models.Match) error {
		moves, err := repo.Moves(ctx, m.ID, 0, -1)
		if err != nil {
			return fmt.Errorf("moves of %s: %w", learnexport.Hash(*salt, "match", m.ID.Hex()), err)
		}
		snapshot := func(seq int) (models.JSONDoc, error) { return repo.Snapshot(ctx, m.ID, seq) }
		recs, st, err := learnexport.Export(g, m, moves, snapshot, opt)
		if err != nil {
			// One match that will not fold is one match lost, not the run:
			// logged by its hash, so the operator can find it with the salt.
			failed++
			log.Printf("skipping match %s: %v", learnexport.Hash(*salt, "match", m.ID.Hex()), err)
			return nil
		}
		for i := range recs {
			if err := enc.Encode(&recs[i]); err != nil {
				return err
			}
		}
		total.Add(st)
		return nil
	})
	if cerr := finish(); err == nil {
		err = cerr
	}
	if err != nil {
		return fmt.Errorf("stopped: %w (written so far: %d records)", err, total.Records)
	}
	summarize(summary, total, failed)
	return nil
}

// day parses a YYYY-MM-DD as the start of that day in UTC, plus offset days;
// an empty string is the zero time, which the filter reads as no bound.
func day(s string, offset int) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return time.Time{}, err
	}
	return t.AddDate(0, 0, offset), nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// openStore opens the repository the flags name, read-only by construction,
// and says what it opened for the log.
func openStore(ctx context.Context, engine, mongoURI, mongoDB, kdbPath string, allowRemote bool) (
	repo match.Repository, where string, closeFn func(), err error) {
	switch strings.ToLower(strings.TrimSpace(engine)) {
	case "":
		switch {
		case kdbPath != "" && mongoURI != "":
			return nil, "", nil, errors.New("both a KDB path and a Mongo URI are set; say which with -engine")
		case kdbPath != "":
			engine = db.EngineKDB
		case mongoURI != "":
			engine = db.EngineMongo
		default:
			return nil, "", nil, errors.New("no store: pass -kdb or -mongo-uri (or set KDB_PATH or MONGO_URI)")
		}
	case db.EngineKDB, db.EngineMongo:
		engine = strings.ToLower(strings.TrimSpace(engine))
	default:
		return nil, "", nil, fmt.Errorf("-engine %q: want mongo or kdb", engine)
	}

	if engine == db.EngineMongo {
		if mongoURI == "" {
			return nil, "", nil, errors.New("-engine mongo needs -mongo-uri (or MONGO_URI)")
		}
		host, err := checkMongoHost(mongoURI, allowRemote)
		if err != nil {
			return nil, "", nil, err
		}
		m, err := db.Connect(ctx, db.Config{URI: mongoURI, DB: mongoDB})
		if err != nil {
			return nil, "", nil, fmt.Errorf("connecting to %s: %w", host, err)
		}
		return match.NewRepository(m), fmt.Sprintf("mongo %s/%s", host, mongoDB),
			func() { _ = m.Close(context.Background()) }, nil
	}

	if kdbPath == "" {
		return nil, "", nil, errors.New("-engine kdb needs -kdb (or KDB_PATH)")
	}
	if fi, err := os.Stat(kdbPath); err != nil || !fi.IsDir() {
		return nil, "", nil, fmt.Errorf("-kdb %s: not a directory", kdbPath)
	}
	tmp, err := os.MkdirTemp("", "export-games-kdb-")
	if err != nil {
		return nil, "", nil, err
	}
	copyRoot := filepath.Join(tmp, "kdb")
	log.Printf("copying %s to %s, so the original is never opened", kdbPath, copyRoot)
	if err := copyTree(kdbPath, copyRoot); err != nil {
		_ = os.RemoveAll(tmp)
		return nil, "", nil, fmt.Errorf("copying %s: %w", kdbPath, err)
	}
	k, err := db.OpenKDB(copyRoot)
	if err != nil {
		_ = os.RemoveAll(tmp)
		return nil, "", nil, fmt.Errorf("opening the copy of %s: %w", kdbPath, err)
	}
	return match.NewKDBRepository(k), "a copy of kdb " + kdbPath, func() {
		_ = k.Close(context.Background())
		_ = os.RemoveAll(tmp)
	}, nil
}

// checkMongoHost refuses a URI that reaches past this machine unless the
// caller said it meant to. An export is run by hand, from a shell whose
// environment may have been set up for something else entirely; a MONGO_URI
// left pointing at production is the mistake this is here to stop.
func checkMongoHost(uri string, allowRemote bool) (string, error) {
	scheme, rest, ok := strings.Cut(uri, "://")
	if !ok || (scheme != "mongodb" && scheme != "mongodb+srv") {
		return "", errors.New("-mongo-uri: want mongodb:// or mongodb+srv://")
	}
	// The host list, without credentials, path or options. Parsed by hand
	// because a seed list of several hosts is not a URL net/url accepts.
	if i := strings.IndexAny(rest, "/?"); i >= 0 {
		rest = rest[:i]
	}
	if i := strings.LastIndex(rest, "@"); i >= 0 {
		rest = rest[i+1:]
	}
	shown := rest
	if allowRemote {
		return shown, nil
	}
	if scheme == "mongodb+srv" {
		return "", fmt.Errorf("-mongo-uri names %s by DNS seed list, which is never this machine; pass -allow-remote if that is really meant", shown)
	}
	for _, h := range strings.Split(rest, ",") {
		name := h
		if hh, _, err := net.SplitHostPort(h); err == nil {
			name = hh
		}
		name = strings.Trim(name, "[]")
		if unescaped, err := url.PathUnescape(name); err == nil {
			name = unescaped // a socket path is %-encoded
		}
		if !isLocal(name) {
			return "", fmt.Errorf("-mongo-uri names %s, which is not this machine; pass -allow-remote if that is really meant", shown)
		}
	}
	return shown, nil
}

func isLocal(host string) bool {
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".sock") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// copyTree copies a directory, regular files and directories only.
func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		switch {
		case d.IsDir():
			return os.MkdirAll(target, 0o755)
		case !d.Type().IsRegular():
			return nil
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		o, err := os.Create(target)
		if err != nil {
			return err
		}
		if _, err := io.Copy(o, in); err != nil {
			o.Close()
			return err
		}
		return o.Close()
	})
}

// create opens the output, gzipped when the name says so, and returns the
// function that flushes and closes it.
func create(path string) (io.Writer, func() error, error) {
	var f *os.File
	if path == "-" {
		f = os.Stdout
	} else {
		var err error
		if f, err = os.Create(path); err != nil {
			return nil, nil, err
		}
	}
	buf := bufio.NewWriterSize(f, 1<<20)
	closeFile := func() error {
		if f == os.Stdout {
			return nil
		}
		return f.Close()
	}
	if !strings.HasSuffix(path, ".gz") {
		return buf, func() error {
			err := buf.Flush()
			if cerr := closeFile(); err == nil {
				err = cerr
			}
			return err
		}, nil
	}
	gz := gzip.NewWriter(buf)
	return gz, func() error {
		err := gz.Close()
		if ferr := buf.Flush(); err == nil {
			err = ferr
		}
		if cerr := closeFile(); err == nil {
			err = cerr
		}
		return err
	}, nil
}

func summarize(w io.Writer, st learnexport.Stats, failed int) {
	pct := func(n, d int) float64 {
		if d == 0 {
			return 0
		}
		return 100 * float64(n) / float64(d)
	}
	decisions := st.Human + st.Bot
	fmt.Fprintf(w, "matches read:       %d (%d truncated by a move the module now refuses, %d skipped)\n", st.Matches, st.Truncated, failed)
	fmt.Fprintf(w, "human decisions:    %d\n", st.Human)
	if st.Bot > 0 {
		fmt.Fprintf(w, "bot decisions:      %d\n", st.Bot)
	}
	fmt.Fprintf(w, "matched:            %d (%.1f%%), %d to the nearest size, %d with one candidate\n",
		st.Matched, pct(st.Matched, decisions), st.Approximate, st.Forced)
	if st.PlanStarts > 0 {
		fmt.Fprintf(w, "multi-step plans:   %d chosen, %d made whole, %d moves within one\n", st.PlanStarts, st.PlansWhole, st.WithinPlan)
	}
	unmatched := 0
	for _, n := range st.Unmatched {
		unmatched += n
	}
	fmt.Fprintf(w, "unmatched:          %d\n", unmatched)
	for _, k := range sortedKeys(st.Unmatched) {
		fmt.Fprintf(w, "  %-28s %d\n", k, st.Unmatched[k])
	}
	for _, k := range sortedKeys(st.Dropped) {
		fmt.Fprintf(w, "dropped %-20s %d\n", k, st.Dropped[k])
	}
	fmt.Fprintf(w, "by verb:\n")
	verbs := make([]string, 0, len(st.ByVerb))
	for v := range st.ByVerb {
		verbs = append(verbs, v)
	}
	sort.Strings(verbs)
	for _, v := range verbs {
		s := st.ByVerb[v]
		fmt.Fprintf(w, "  %-14s %6d/%-6d %5.1f%%\n", v, s.Matched, s.Decisions, pct(s.Matched, s.Decisions))
	}
	fmt.Fprintf(w, "records written:    %d\n", st.Records)
}

func sortedKeys(m map[string]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
