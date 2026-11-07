package app

import (
	"slices"
	"testing"
)

// The games a store build hides from a player under 18, and labels 18+ for
// everyone else. Pinned here because the flag is declared module by module:
// a new chip game that forgets it would reach a child's game list, and a
// game that sets it by mistake would vanish from one.
func TestSimulatedGamblingModules(t *testing.T) {
	var got []string
	for _, m := range (&App{}).hostedModules() {
		if m.Descriptor().SimulatedGambling {
			got = append(got, m.Descriptor().ID)
		}
	}
	slices.Sort(got)
	want := []string{"blackjack", "ferbl", "holdem", "okobere", "sietemedio"}
	if !slices.Equal(got, want) {
		t.Fatalf("simulated-gambling modules = %v, want %v", got, want)
	}
}
