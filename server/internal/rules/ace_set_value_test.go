package rules

import "testing"

// An ace scores as the real, highest card in the game everywhere except one
// place: the bottom of a run, where the low-ace convention makes it 1.
//
// Every natural ace used to score 1. That made a set of three aces worth 3
// points — less than three 2s at 6 — and Q-K-A worth 21 instead of 35, which
// put every meld floor the lobby offers (35/50/70) out of reach for a hand
// built on aces.
func TestAceScoresAsARealAceExceptAtARunsBottom(t *testing.T) {
	cfg := ResolveConfig(ProfileZolikClassic)

	for _, tc := range []struct {
		name  string
		cards []string
		want  int
		typ   MeldType
	}{
		// Sets: every ace is a real ace.
		{"three aces", []string{"AC", "AH", "AS"}, 3 * AceMeldValue, MeldSet},
		{"four aces", []string{"AC", "AD", "AH", "AS"}, 4 * AceMeldValue, MeldSet},
		{"a joker in an ace set contributes nothing", []string{"AC", "AH", "JOKER1"}, 2 * AceMeldValue, MeldSet},
		{"three 2s, for contrast", []string{"2C", "2D", "2S"}, 6, MeldSet},
		{"three queens, unchanged", []string{"QC", "QD", "QS"}, 30, MeldSet},

		// Runs: the top ace is real, the bottom ace is the positional 1.
		{"Q-K-A tops out with a real ace", []string{"QC", "KC", "AC"}, 10 + 10 + AceMeldValue, MeldRun},
		{"J-Q-K-A likewise", []string{"JC", "QC", "KC", "AC"}, 10 + 10 + 10 + AceMeldValue, MeldRun},
		{"A-2-3 bottoms out at 1", []string{"AD", "2D", "3D"}, AceRunLowValue + 2 + 3, MeldRun},
		{"A-2-3-4", []string{"AD", "2D", "3D", "4D"}, AceRunLowValue + 2 + 3 + 4, MeldRun},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mv, err := ValidateMeld(tc.cards, cfg)
			if err != nil {
				t.Fatalf("ValidateMeld(%v): %v", tc.cards, err)
			}
			if mv.Type != tc.typ {
				t.Fatalf("type = %s, want %s", mv.Type, tc.typ)
			}
			if mv.NaturalValue != tc.want {
				t.Errorf("naturalValue = %d, want %d", mv.NaturalValue, tc.want)
			}
		})
	}

	if AceMeldValue <= PenaltyPoints("2C", true) {
		t.Errorf("a real ace (%d) must outscore a 2 (%d)", AceMeldValue, PenaltyPoints("2C", true))
	}
	if AceRunLowValue >= AceMeldValue {
		t.Errorf("a run's low ace (%d) must be worth less than a real ace (%d)",
			AceRunLowValue, AceMeldValue)
	}
}

// runNaturalValue is asked directly for the one combination validateRun can
// reach but no realistic hand does — both ends taken by an ace — so the two
// values are pinned as additive rather than one overwriting the other.
func TestRunNaturalValueScoresEachAceEndOnce(t *testing.T) {
	cards := []string{"AD", "2D", "3D", "KD", "AD"}
	naturals := 2 + 3 + 10
	if got, want := runNaturalValue(cards, true, true), naturals+AceRunLowValue+AceMeldValue; got != want {
		t.Errorf("both ends = %d, want %d", got, want)
	}
	if got, want := runNaturalValue(cards, true, false), naturals+AceRunLowValue; got != want {
		t.Errorf("low end only = %d, want %d", got, want)
	}
	if got, want := runNaturalValue(cards, false, true), naturals+AceMeldValue; got != want {
		t.Errorf("high end only = %d, want %d", got, want)
	}
	// An ace not used at either end is a wild filler, worth nothing.
	if got, want := runNaturalValue(cards, false, false), naturals; got != want {
		t.Errorf("neither end = %d, want %d", got, want)
	}
}

// The live readout for a selection that is not yet a valid meld guesses which
// way an ace is heading, so a player assembling one does not watch a figure
// that is wrong by an order of magnitude until the moment it validates.
func TestLooseValueGuessesWhereAnAceIsHeading(t *testing.T) {
	for _, tc := range []struct {
		name  string
		cards []string
		want  int
	}{
		{"two aces can only be a set", []string{"AC", "AH"}, 2 * AceMeldValue},
		{"an ace with its own king is heading for Q-K-A", []string{"KC", "AC"}, 10 + AceMeldValue},
		{"an ace with a king of another suit is not", []string{"KD", "AC"}, 10 + AceRunLowValue},
		{"an ace with a low card reads as the bottom", []string{"AC", "2C"}, AceRunLowValue + 2},
		{"no aces at all is unaffected", []string{"QC", "5D"}, 15},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := looseNaturalValue(tc.cards); got != tc.want {
				t.Errorf("looseNaturalValue(%v) = %d, want %d", tc.cards, got, tc.want)
			}
		})
	}
}
