// Command gamebench compares two bots at one game, with the cards taken out of
// the answer (internal/learn's duplicate bench: every seed played twice, the
// contenders swapped between the seats).
//
//	go run ./cmd/gamebench -game holdem -a hard -b medium
//	go run ./cmd/gamebench -game canasta -seats 4 -a hard -b hard
//	go run ./cmd/gamebench -game holdem -a net:models/ckpt.bin -b hard -seeds 500
//
// A contender is a skill (easy, medium, hard), a style the game supplies, or
// net:<path>[@temperature] for a trained model. The result is A's advantage
// per match in the game's outcome unit — big blinds for Hold'em, points for
// Canasta — with its standard error. It exits 1 on any illegal move or stall,
// which is a bug at every strength.
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
)

func main() {
	game := flag.String("game", "holdem", "game to bench")
	seats := flag.Int("seats", 2, "seats at the table")
	variation := flag.String("variation", "", "variation (the game's default when empty)")
	a := flag.String("a", "hard", "first contender")
	b := flag.String("b", "medium", "second contender")
	first := flag.Int64("first", 1_000_000, "first seed; keep held-out seeds away from training ones")
	seeds := flag.Int("seeds", 100, "seeds to play, each in both seatings")
	budget := flag.Int("actions", 50_000, "action budget per match")
	flag.Parse()

	g, err := learn.Lookup(*game)
	if err != nil {
		fail(err)
	}
	ca, err := contender(g, *a)
	if err != nil {
		fail(err)
	}
	cb, err := contender(g, *b)
	if err != nil {
		fail(err)
	}
	start := time.Now()
	r, err := learn.Bench(g, *seats, *variation, ca, cb, *first, *seeds, *budget)
	if err != nil {
		fail(err)
	}
	fmt.Printf("%s %d seats %s: %s vs %s: %v  [%s]\n", *game, *seats, *variation, *a, *b, r, time.Since(start).Round(time.Millisecond))
	if r.Significant() {
		fmt.Printf("  %s is ahead by more than two standard errors\n", *a)
	}
	if r.Illegal+r.Stalls > 0 {
		os.Exit(1)
	}
}

func contender(g learn.Benchable, spec string) (learn.Contender, error) {
	for _, s := range module.Skills {
		if string(s) == spec {
			return learn.Contender{Name: spec, Bot: g.Heuristic(), Skill: s}, nil
		}
	}
	if st, ok := g.(learn.Styled); ok {
		if bot, ok := st.Styles()[spec]; ok {
			return learn.Contender{Name: spec, Bot: bot}, nil
		}
	}
	if path, ok := strings.CutPrefix(spec, "net:"); ok {
		lg, ok := g.(learn.Game)
		if !ok {
			return learn.Contender{}, fmt.Errorf("%s cannot be played by a network yet", g.Name())
		}
		temp := 0.0
		if i := strings.LastIndex(path, "@"); i >= 0 {
			t, err := strconv.ParseFloat(path[i+1:], 64)
			if err != nil {
				return learn.Contender{}, err
			}
			path, temp = path[:i], t
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return learn.Contender{}, err
		}
		n, err := learn.LoadNet(raw)
		if err != nil {
			return learn.Contender{}, err
		}
		if n.StateDim != lg.StateDim() || n.CandDim != lg.CandDim() {
			return learn.Contender{}, fmt.Errorf("%s was trained for another encoder", path)
		}
		return learn.Contender{Name: spec, Bot: learn.NetBot{Game: lg, Net: n, Fallback: g.Heuristic(), Temperature: temp}, Skill: module.SkillHard}, nil
	}
	return learn.Contender{}, fmt.Errorf("unknown contender %q", spec)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "gamebench:", err)
	os.Exit(2)
}
