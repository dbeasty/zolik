package rules

import "testing"

// An ace melded in a set is a real ace, not a run endpoint, and scores as one.
//
// It used to score 1 — the low-ace convention — applied to sets as well as
// runs. That made a set of three aces worth 3 points, less than three 2s at
// 6, and put every meld floor the lobby offers (35/50/70) out of reach for a
// hand built on aces.
func TestAceInASetScoresAsARealAce(t *testing.T) {
	cfg := ResolveConfig(ProfileZolikClassic)

	for _, tc := range []struct {
		name  string
		cards []string
		want  int
	}{
		{"three aces", []string{"AC", "AH", "AS"}, 3 * AceMeldValueInSet},
		{"four aces", []string{"AC", "AD", "AH", "AS"}, 4 * AceMeldValueInSet},
		{"three 2s, for contrast", []string{"2C", "2D", "2S"}, 6},
		{"three queens, unchanged", []string{"QC", "QD", "QS"}, 30},
		{"a joker still contributes nothing", []string{"AC", "AH", "JOKER1"}, 2 * AceMeldValueInSet},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mv, err := ValidateMeld(tc.cards, cfg)
			if err != nil {
				t.Fatalf("ValidateMeld(%v): %v", tc.cards, err)
			}
			if mv.Type != MeldSet {
				t.Fatalf("type = %s, want set", mv.Type)
			}
			if mv.NaturalValue != tc.want {
				t.Errorf("naturalValue = %d, want %d", mv.NaturalValue, tc.want)
			}
		})
	}

	if AceMeldValueInSet <= PenaltyPoints("2C", true) {
		t.Errorf("an ace in a set (%d) must outscore a 2 (%d)",
			AceMeldValueInSet, PenaltyPoints("2C", true))
	}
}

// A run's ace keeps the low-ace convention: it occupies rank 1 or 14 of its
// suit and is worth 1 there, which is what architecture.md documents and what
// this change deliberately leaves alone.
func TestAceInARunKeepsItsEndpointValue(t *testing.T) {
	cfg := ResolveConfig(ProfileZolikClassic)

	// A-2-3: the ace sits at rank 1.
	low, err := ValidateMeld([]string{"AD", "2D", "3D"}, cfg)
	if err != nil {
		t.Fatalf("A-2-3: %v", err)
	}
	if low.Type != MeldRun || low.NaturalValue != 1+2+3 {
		t.Errorf("A-2-3 naturalValue = %d, want %d", low.NaturalValue, 1+2+3)
	}

	// Q-K-A: the ace sits at rank 14, still 1 under the current convention.
	high, err := ValidateMeld([]string{"QC", "KC", "AC"}, cfg)
	if err != nil {
		t.Fatalf("Q-K-A: %v", err)
	}
	if high.Type != MeldRun || high.NaturalValue != 10+10+1 {
		t.Errorf("Q-K-A naturalValue = %d, want %d", high.NaturalValue, 10+10+1)
	}
}

// The live readout for a selection that is not yet a valid meld guesses which
// way an ace is heading, so a player assembling a set of aces sees roughly
// what it will be worth rather than a figure an order of magnitude too small.
func TestLooseValueGuessesAnAceSet(t *testing.T) {
	if got, want := looseNaturalValue([]string{"AC", "AH"}), 2*AceMeldValueInSet; got != want {
		t.Errorf("two aces = %d, want %d", got, want)
	}
	// A lone ace among other ranks still reads as a run endpoint.
	if got, want := looseNaturalValue([]string{"AC", "KC"}), 1+10; got != want {
		t.Errorf("ace with a king = %d, want %d", got, want)
	}
}
