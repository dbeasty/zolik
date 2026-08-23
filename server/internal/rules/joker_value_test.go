package rules

import "testing"

// A wild is worth the card it stands in for.
//
// Wilds used to count 0 toward the initial-meld floor, which made a joker the
// one card whose position on the table changed nothing. Reported from a live
// game: an AI laid Q♠-K♠-JOKER — the joker behind the king, an ace-high run
// worth 35 against a 35-point floor — and the engine both rewrote it as
// J♠-Q♠-K♠ and scored it 20.
func TestJokerScoresAsTheCardItReplaces(t *testing.T) {
	cfg := ResolveConfig(ProfileZolikClassic)

	for _, tc := range []struct {
		name  string
		cards []string
		want  int
	}{
		// The reported hand. The joker takes rank 14, the real ace's slot,
		// and is worth AceMeldValue there like any other ace at a run's top.
		{"joker behind the king is an ace", []string{"QS", "KS", "JOKER2"}, 10 + 10 + AceMeldValue},
		// The same joker one rank lower is worth one rank less.
		{"joker in front of the queen is a jack", []string{"JOKER2", "QS", "KS", "AS"}, 10 + 10 + 10 + AceMeldValue},
		// A gap in the middle: the joker is the 8 it stands for, not 0.
		{"joker fills a gap as that gap's card", []string{"7D", "JOKER2", "9D"}, 7 + 8 + 9},
		// Sets: every card is the same rank, so the joker is too.
		{"joker in a set of sevens is a seven", []string{"7H", "7D", "JOKER1"}, 3 * 7},
		{"joker in a set of aces is an ace", []string{"AC", "AH", "JOKER1"}, 3 * AceMeldValue},
		{"two jokers in a set of kings", []string{"KC", "KH", "JOKER1", "JOKER2"}, 4 * 10},
		// The low ace is the one slot worth less than the card above it, and
		// a joker standing there inherits that too. It takes two jokers to
		// force the window down there: with one, 2-3-4 at 9 outscores A-2-3
		// at 6 and wins the tie-break, and with two, sliding up to 2-3-4-5
		// would sit them adjacent at ranks 4 and 5.
		{"joker as the low ace is worth AceRunLowValue", []string{"JOKER1", "2S", "3S", "JOKER2"}, AceRunLowValue + 2 + 3 + 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mv, err := ValidateMeld(tc.cards, cfg)
			if err != nil {
				t.Fatalf("ValidateMeld(%v): %v", tc.cards, err)
			}
			if mv.NaturalValue != tc.want {
				t.Errorf("value = %d, want %d", mv.NaturalValue, tc.want)
			}
		})
	}
}

// The whole point of the reported bug: the meld clears the floor now, and the
// readout the player watches while assembling it says so.
func TestJokerBehindTheKingClearsA35PointFloor(t *testing.T) {
	p := "p1"
	st := classicState(p)
	st.Rules.InitialMeldMinimum = 35
	st.Hands[p] = []string{"QS", "KS", "JOKER2", "2C"}

	preview := PreviewMeld(st, p, []string{"QS", "KS", "JOKER2"})
	if !preview.Valid {
		t.Fatalf("expected a valid meld, got %s", preview.WhyNot)
	}
	if preview.NaturalValue != 35 {
		t.Fatalf("preview value = %d, want 35", preview.NaturalValue)
	}
	if !preview.MeetsMinimum {
		t.Fatal("a 35-point meld must clear a 35-point floor")
	}

	st, _, _, err := ValidateMeldAction(st, p, []string{"QS", "KS", "JOKER2"})
	if err != nil {
		t.Fatalf("laying it: %v", err)
	}
	if got := PlayerInitialMeldNaturalValue(st, p); got != 35 {
		t.Fatalf("laid value = %d, want 35", got)
	}
	// And the joker is stored where the player put it — behind the king.
	if got := st.Melds[p][0]; got[len(got)-1] != "JOKER2" {
		t.Fatalf("meld stored as %v, want the joker last, at the ace's slot", got)
	}
}
