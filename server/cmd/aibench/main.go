// Command aibench measures how strong the bots actually are.
//
// The point of it is that "we made the AI smarter" is not a claim anybody can
// check by reading a diff. This plays every strength against every other, over
// a fixed seed sweep, across every ruleset a lobby can produce, and prints the
// table. CI asserts the ordering (internal/ai/sim's monotonicity test); this is
// for looking at the numbers while tuning, where a win rate is much less
// interesting than *why* — how fast each profile gets down, how much it is left
// holding, how often it goes out at all.
//
// It drives the same simulator the tests use, which drives the same engine, the
// same ledger and the same visible-state projection the server does. A number
// here is a number about the bot people play against.
//
//	go run ./cmd/aibench                 # every pairing, every ruleset
//	go run ./cmd/aibench -seeds 200      # a longer sweep
//	go run ./cmd/aibench -rules classic  # one ruleset
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"zolik/server/internal/ai/sim"
	"zolik/server/internal/module"
	"zolik/server/internal/rules"
)

func main() {
	seeds := flag.Int("seeds", 60, "how many deals per pairing")
	budget := flag.Int("actions", 4000, "action budget per deal")
	only := flag.String("rules", "", "run only rulesets whose name contains this")
	flag.Parse()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	failures := 0

	for _, rs := range rulesets() {
		if *only != "" && !strings.Contains(rs.name, *only) {
			continue
		}
		fmt.Fprintf(w, "\n== %s ==\n", rs.name)
		fmt.Fprintln(w, "pairing\twins\tdeals\twin%\tmean pts\trejects\tstalls\tstranded")
		for i, a := range module.Skills {
			for _, b := range module.Skills[i+1:] {
				ta, tb := sim.HeadToHead(rs.cfg, a, b, *seeds, *budget)
				row(w, fmt.Sprintf("%s vs %s", a, b), ta)
				row(w, fmt.Sprintf("%s vs %s", b, a), tb)
				// Anything above zero in these three columns is a bug at any
				// strength, so say so loudly rather than leaving it in a
				// column somebody has to notice.
				failures += ta.Rejections + tb.Rejections + ta.Stalls + tb.Stalls +
					ta.StrandedLays + tb.StrandedLays
				if tb.Wins > ta.Wins && b.Rank() < a.Rank() {
					fmt.Fprintf(w, "  !! %s beat %s — the ladder is upside down here\n", b, a)
				}
			}
		}
	}
	_ = w.Flush()
	if failures > 0 {
		fmt.Fprintf(os.Stderr, "\n%d illegal move(s), stall(s) or stranded lay(s) — these are bugs, not weaknesses\n", failures)
		os.Exit(1)
	}
}

func row(w *tabwriter.Writer, name string, t sim.Tally) {
	fmt.Fprintf(w, "%s\t%d\t%d\t%.0f%%\t%.1f\t%d\t%d\t%d\n",
		name, t.Wins, t.Deals, 100*t.WinRate(), t.MeanPoints(),
		t.Rejections, t.Stalls, t.StrandedLays)
}

// rulesets is every configuration worth sweeping: both shipped profiles, plus
// the house-rule combination that produced the stranded-lay bug in a real
// game. A gain under one ruleset can be a loss under another, so a benchmark
// that only ran the default would be measuring the wrong thing.
func rulesets() []struct {
	name string
	cfg  rules.RulesConfig
} {
	floored := rules.ProfileZolikClassic
	floored.InitialMeldMinimum = 35
	floored.DiscardDrawMinRound = 3
	return []struct {
		name string
		cfg  rules.RulesConfig
	}{
		{"zolik_classic", rules.ProfileZolikClassic},
		{"zolik_classic+floor35", floored},
		{"continental", rules.ProfileContinental},
	}
}
