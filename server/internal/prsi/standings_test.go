package prsi

import (
	"testing"
)

// What the scoreboard prints, as opposed to what it ranks on.
//
// Prší ranks on cards left with the sign flipped, because RankByScore is
// higher-is-better and here fewer cards is better. That is a runtime
// convenience and no player's idea of a score, so it has to be undone before
// the number reaches a screen — which for a long time it was not: every seat
// strip on every live Prší board read "-4 cards left", and the results table
// agreed with it.
func TestStandingsPrintCardsLeftAsAPlayerWouldSayIt(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.Hands = map[string][]string{"p1": {"9D", "KS", "7C"}, "p2": {"AD"}}
	})

	out, err := New().Standings(raw)
	if err != nil {
		t.Fatalf("Standings: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("got %d standings, want 2", len(out))
	}

	for _, s := range out {
		want := map[string]int{"p1": 3, "p2": 1}[s.PlayerID]
		if s.Shown == nil {
			t.Fatalf("%s has no Shown, so the board prints Score (%d) — the negated one",
				s.PlayerID, s.Score)
		}
		if *s.Shown != want {
			t.Errorf("%s shown = %d, want %d", s.PlayerID, *s.Shown, want)
		}
		// And the ranking number is untouched: the runtime records and orders
		// on it, and flipping it here would reverse who is winning.
		if s.Score != -want {
			t.Errorf("%s score = %d, want %d", s.PlayerID, s.Score, -want)
		}
	}

	// Fewest cards still wins, which is the thing Shown must not disturb.
	if out[0].PlayerID != "p2" || !out[0].Won {
		t.Errorf("leader = %s (won=%v), want p2 winning with one card left",
			out[0].PlayerID, out[0].Won)
	}
}

// The winner's own row is the one case where the old wording was accidentally
// right — zero negates to zero — so it is worth pinning that the fix did not
// turn it into something else.
func TestStandingsShowZeroForAPlayerWhoIsOut(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.Hands = map[string][]string{"p1": {}, "p2": {"AD", "KS"}}
	})

	out, err := New().Standings(raw)
	if err != nil {
		t.Fatalf("Standings: %v", err)
	}
	if out[0].PlayerID != "p1" || *out[0].Shown != 0 {
		t.Errorf("leader = %s with shown %d, want p1 with 0", out[0].PlayerID, *out[0].Shown)
	}
}
