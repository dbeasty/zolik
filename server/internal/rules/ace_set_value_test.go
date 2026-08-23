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
		{"a joker in an ace set is an ace", []string{"AC", "AH", "JOKER1"}, 3 * AceMeldValue, MeldSet},
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

// A run is priced by its resolved rank window, so the two ace slots are asked
// for directly here — including the one combination validateRun can reach but
// no realistic hand does, both ends taken by an ace — to pin them as additive
// rather than one overwriting the other.
func TestRunValueScoresEachAceEndOnce(t *testing.T) {
	if got, want := runRankValue(1), AceRunLowValue; got != want {
		t.Errorf("rank 1 = %d, want %d", got, want)
	}
	if got, want := runRankValue(14), AceMeldValue; got != want {
		t.Errorf("rank 14 = %d, want %d", got, want)
	}
	// A-2-3 … K-A, the full 14-slot window: both ace ends, counted once each.
	full := make([]int, 0, 14)
	for r := 1; r <= 14; r++ {
		full = append(full, r)
	}
	want := AceRunLowValue + (2 + 3 + 4 + 5 + 6 + 7 + 8 + 9) + 4*10 + AceMeldValue
	if got := runValue(full); got != want {
		t.Errorf("A-through-A = %d, want %d", got, want)
	}
}

// A wild is worth the card it stands in for, so a run's value depends only on
// which ranks it spans — never on how many of those ranks are jokers.
func TestRunValueIgnoresWhetherASlotIsWild(t *testing.T) {
	cfg := ResolveConfig(ProfileZolikClassic)
	for _, tc := range []struct {
		name  string
		cards []string
	}{
		// Pinned at both ends by the 10 and the king, so there is only one
		// window and the joker's slot cannot shift under the comparison.
		{"all natural", []string{"TC", "JC", "QC", "KC"}},
		{"joker as the jack", []string{"TC", "JOKER1", "QC", "KC"}},
		{"joker as the queen", []string{"TC", "JC", "JOKER1", "KC"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mv, err := ValidateMeld(tc.cards, cfg)
			if err != nil {
				t.Fatalf("ValidateMeld(%v): %v", tc.cards, err)
			}
			if mv.NaturalValue != 40 {
				t.Errorf("naturalValue = %d, want 40", mv.NaturalValue)
			}
		})
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
