package ui

import (
	"fmt"
	"strings"

	"zolik/client-tui/api"
)

// dealHeaderLabel describes the current deal for the header bar.
//
// Both facts come from the ruleset the server resolved for this game, never
// from the profile *name*: a fixed-length match (FixedDealCount > 0) counts
// its deals and names this deal's contract, while a score-limited one just
// re-deals until someone crosses the target, so naming a deal count there
// would be wrong.
//
// This used to hardcode "Game %d of 7" behind a `rulesProfile ==
// "continental"` check, with its own copy of the seven-deal contract table
// transcribed from server/internal/rules/profiles.go — two things that had
// to be kept in sync with the server by hand, and a third profile could not
// be described at all.
func dealHeaderLabel(rules api.ResolvedRules, contract api.Contract, game int) string {
	if rules.FixedDealCount <= 0 {
		return fmt.Sprintf("Deal %d", game)
	}
	of := fmt.Sprintf("Game %d of %d", game, rules.FixedDealCount)
	if label := contractLabel(contract); label != "" {
		return of + ": " + label
	}
	return of
}

func selectedCards(hand []string, sel map[int]bool) []string {
	var out []string
	for i, c := range hand {
		if sel[i] {
			out = append(out, c)
		}
	}
	return out
}

func selectedIndexes(sel map[int]bool) []int {
	var out []int
	for i, on := range sel {
		if on {
			out = append(out, i)
		}
	}
	return out
}

func playerName(players []struct {
	ID   string
	Name string
}, id string) string {
	for _, p := range players {
		if p.ID == id {
			return p.Name
		}
	}
	return id
}

func formatScores(totals map[string]int, players []playerRef) string {
	var parts []string
	for _, p := range players {
		sc := totals[p.ID]
		parts = append(parts, fmt.Sprintf("%s: %d", p.Name, sc))
	}
	return strings.Join(parts, "  │  ")
}

type playerRef struct {
	ID   string
	Name string
	IsAI bool
}
