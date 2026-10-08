// Command gamebench compares two bots at one game, with the cards taken out of
// the answer (internal/learn's duplicate bench: every seed played twice, the
// contenders swapped between the seats).
//
//	go run ./cmd/gamebench -game holdem -a hard -b medium
//	go run ./cmd/gamebench -game canasta -seats 4 -a hard -b hard
//	go run ./cmd/gamebench -game zolik -variation continental -a hard -b easy
//	go run ./cmd/gamebench -game holdem -a net:models/ckpt.bin -b hard -seeds 500
//	go run ./cmd/gamebench -game zolik -seats 4 -a-seats 3 -a net:final.bin -b hard
//	go run ./cmd/gamebench -game marias -seats 3 -a hard-1000k -b hard
//
// A contender is a skill (easy, medium, hard), a style the game supplies, or
// net:<path>[@temperature] for a trained model. The result is A's advantage
// per match in the game's outcome unit — big blinds for Hold'em, points for
// Canasta, penalty points kept out of hand for Žolíky — with its standard
// error. It exits 1 on any illegal move or stall, which is a bug at every
// strength.
//
// -a-seats puts A in that many seats and B in the rest (three copies of a
// model with one hard bot, say), each seed played once per rotation of the
// table; the default alternates the seats between them.
package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"zolik/server/internal/learn"
	"zolik/server/internal/module"

	_ "zolik/server/internal/canasta"
	_ "zolik/server/internal/holdem"
	_ "zolik/server/internal/lastcard"
	_ "zolik/server/internal/marias"
	_ "zolik/server/internal/sedma"
	_ "zolik/server/internal/zolikmod"
)

func main() {
	game := flag.String("game", "holdem", "game to bench")
	seats := flag.Int("seats", 2, "seats at the table")
	variation := flag.String("variation", "", "variation (the game's default when empty)")
	a := flag.String("a", "hard", "first contender")
	b := flag.String("b", "medium", "second contender")
	aSeats := flag.Int("a-seats", 0, "seats A plays, B the rest (0: alternate)")
	first := flag.Int64("first", 1_000_000, "first seed; keep held-out seeds away from training ones")
	seeds := flag.Int("seeds", 100, "seeds to play, each in both seatings")
	budget := flag.Int("actions", 50_000, "action budget per match")
	opts := flag.String("opts", "", "table options over the game's own, e.g. startingStack=300,bigBlind=20")
	flag.Parse()

	g, err := learn.Lookup(*game)
	if err != nil {
		fail(err)
	}
	ca, err := learn.ParseContender(g, *a)
	if err != nil {
		fail(err)
	}
	cb, err := learn.ParseContender(g, *b)
	if err != nil {
		fail(err)
	}
	bench := g
	if *opts != "" {
		over, err := parseOpts(*opts)
		if err != nil {
			fail(err)
		}
		bench = withOptions{g, over}
	}
	start := time.Now()
	r, err := learn.BenchSeated(bench, *seats, *aSeats, *variation, ca, cb, *first, *seeds, *budget)
	if err != nil {
		fail(err)
	}
	seating := ""
	if *aSeats > 0 {
		seating = fmt.Sprintf(" (A in %d)", *aSeats)
	}
	if *opts != "" {
		seating += " [" + *opts + "]"
	}
	fmt.Printf("%s %d seats%s %s: %s vs %s: %v  [%s]\n", *game, *seats, seating, *variation, *a, *b, r, time.Since(start).Round(time.Millisecond))
	if r.Significant() {
		fmt.Printf("  %s is ahead by more than two standard errors\n", *a)
	}
	if r.Illegal+r.Stalls > 0 {
		os.Exit(1)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "gamebench:", err)
	os.Exit(2)
}

// withOptions benches a game at its own table with some options replaced.
type withOptions struct {
	learn.Benchable
	over module.Options
}

func (w withOptions) Config(seats int, variation string) module.MatchConfig {
	cfg := w.Benchable.Config(seats, variation)
	opts := module.Options{}
	for k, v := range cfg.Options {
		opts[k] = v
	}
	for k, v := range w.over {
		opts[k] = v
	}
	cfg.Options = opts
	return cfg
}

func parseOpts(spec string) (module.Options, error) {
	out := module.Options{}
	for _, kv := range strings.Split(spec, ",") {
		k, v, ok := strings.Cut(kv, "=")
		n, err := strconv.Atoi(v)
		if !ok || err != nil {
			return nil, fmt.Errorf("bad option %q: want name=integer", kv)
		}
		out[k] = n
	}
	return out, nil
}
