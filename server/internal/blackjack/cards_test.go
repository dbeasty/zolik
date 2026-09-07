package blackjack

import "testing"

func TestTotal_CountsAnAceAsElevenWhileItFits(t *testing.T) {
	for _, tc := range []struct {
		name  string
		cards []string
		total int
		soft  bool
	}{
		{"a plain hard hand", []string{"9H", "7D"}, 16, false},
		{"an ace and a six is soft seventeen", []string{"AH", "6D"}, 17, true},
		{"the same hand drawn out is hard thirteen", []string{"AH", "6D", "6C"}, 13, false},
		{"only one ace is ever raised", []string{"AH", "AD"}, 12, true},
		{"three aces and a nine", []string{"AH", "AD", "AC", "9S"}, 12, false},
		{"court cards are ten", []string{"KH", "QD"}, 20, false},
		{"a natural", []string{"AS", "TD"}, 21, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			total, soft := Total(tc.cards)
			if total != tc.total || soft != tc.soft {
				t.Errorf("Total(%v) = %d soft=%v, want %d soft=%v", tc.cards, total, soft, tc.total, tc.soft)
			}
		})
	}
}

func TestIsNatural_IsTwentyOneOnTwoCardsAndNothingElse(t *testing.T) {
	if !isNatural([]string{"AS", "KD"}) {
		t.Error("an ace and a king is a natural")
	}
	if isNatural([]string{"7S", "7D", "7C"}) {
		t.Error("twenty-one drawn to is not a natural")
	}
}

func TestSamePairValue_SplitsByValueNotRank(t *testing.T) {
	if !samePairValue("KH", "TD") {
		t.Error("a king and a ten are both ten-value cards")
	}
	if samePairValue("AH", "TD") {
		t.Error("an ace is not a ten")
	}
}

func TestBuildShoe_IsTheRightSizeAndShuffledPerSeed(t *testing.T) {
	six := buildShoe(6, 1)
	if len(six) != 6*52 {
		t.Fatalf("a six-deck shoe holds %d cards", len(six))
	}
	counts := map[string]int{}
	for _, c := range six {
		counts[c]++
	}
	if got := counts["AS"]; got != 6 {
		t.Errorf("the six-deck shoe holds %d aces of spades, want 6", got)
	}
	if sameOrder(six, buildShoe(6, 2)) {
		t.Error("two seeds shuffled the same shoe")
	}
	if !sameOrder(six, buildShoe(6, 1)) {
		t.Error("the same seed did not reproduce the shoe")
	}
}

func sameOrder(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
