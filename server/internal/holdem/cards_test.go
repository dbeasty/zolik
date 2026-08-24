package holdem

import "testing"

// TestHandCategories pins what each hand is called. The evaluator is brute
// force over five-card subsets, so what needs testing is the scoring of five
// cards, not the search.
func TestHandCategories(t *testing.T) {
	cases := []struct {
		name  string
		cards []string
		want  int
	}{
		{"straight flush", []string{"9H", "TH", "JH", "QH", "KH"}, straightFlush},
		{"steel wheel", []string{"AH", "2H", "3H", "4H", "5H"}, straightFlush},
		{"four of a kind", []string{"9H", "9C", "9D", "9S", "KH"}, quads},
		{"full house", []string{"9H", "9C", "9D", "KS", "KH"}, fullHouse},
		{"flush", []string{"2H", "5H", "9H", "JH", "KH"}, flush},
		{"straight", []string{"9H", "TC", "JD", "QS", "KH"}, straight},
		{"the wheel", []string{"AH", "2C", "3D", "4S", "5H"}, straight},
		{"three of a kind", []string{"9H", "9C", "9D", "2S", "KH"}, trips},
		{"two pair", []string{"9H", "9C", "KD", "KS", "2H"}, twoPair},
		{"one pair", []string{"9H", "9C", "3D", "KS", "2H"}, pair},
		{"high card", []string{"9H", "7C", "3D", "KS", "2H"}, highCard},
		// The trap every evaluator falls into: an ace-high "straight" that
		// wraps round the top is not a straight at all.
		{"a wrap is not a straight", []string{"QH", "KC", "AD", "2S", "3H"}, highCard},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := evaluateFive(tc.cards).Category; got != tc.want {
				t.Errorf("evaluateFive(%v) category = %d, want %d", tc.cards, got, tc.want)
			}
		})
	}
}

// TestHandComparisons covers the orderings that actually come up in a hand,
// including the ones decided by a kicker rather than a category.
func TestHandComparisons(t *testing.T) {
	cases := []struct {
		name string
		a, b []string
		want int // 1 = a wins, -1 = b wins, 0 = split
	}{
		{
			name: "a flush beats a straight",
			a:    []string{"2H", "5H", "9H", "JH", "KH"},
			b:    []string{"9H", "TC", "JD", "QS", "KH"},
			want: 1,
		},
		{
			name: "the higher straight wins",
			a:    []string{"9H", "TC", "JD", "QS", "KH"},
			b:    []string{"8H", "9C", "TD", "JS", "QH"},
			want: 1,
		},
		{
			name: "the wheel is the lowest straight",
			a:    []string{"AH", "2C", "3D", "4S", "5H"},
			b:    []string{"2H", "3C", "4D", "5S", "6H"},
			want: -1,
		},
		{
			name: "same pair, higher kicker wins",
			a:    []string{"9H", "9C", "AD", "5S", "2H"},
			b:    []string{"9H", "9C", "KD", "5S", "2H"},
			want: 1,
		},
		{
			name: "trips beat two pair",
			a:    []string{"9H", "9C", "9D", "2S", "3H"},
			b:    []string{"AH", "AC", "KD", "KS", "3H"},
			want: 1,
		},
		{
			name: "the higher full house wins on the trips, not the pair",
			a:    []string{"9H", "9C", "9D", "2S", "2H"},
			b:    []string{"8H", "8C", "8D", "AS", "AH"},
			want: 1,
		},
		{
			name: "identical hands split",
			a:    []string{"9H", "9C", "AD", "5S", "2H"},
			b:    []string{"9D", "9S", "AC", "5H", "2D"},
			want: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := evaluateFive(tc.a).Compare(evaluateFive(tc.b))
			if got != tc.want {
				t.Errorf("compare = %d, want %d", got, tc.want)
			}
			// Comparison must be antisymmetric, or a showdown could award a
			// pot twice.
			if back := evaluateFive(tc.b).Compare(evaluateFive(tc.a)); back != -got {
				t.Errorf("reversed compare = %d, want %d", back, -got)
			}
		})
	}
}

// TestBestOfSeven checks the search, not the scoring: the point of seven cards
// is that the best five are not the first five.
func TestBestOfSeven(t *testing.T) {
	// Hole 2H 7C with a board that makes a king-high flush out of the board
	// plus one hole card... except neither hole card is a heart beyond the 2H.
	best := Best([]string{"2H", "7C", "5H", "9H", "JH", "KH", "3S"})
	if best.Category != flush {
		t.Fatalf("category = %d, want a flush", best.Category)
	}
	for _, c := range best.Cards {
		if suitOf(c) != "H" {
			t.Errorf("flush contains %q, which is not a heart", c)
		}
	}

	// A board pairing the player's hole card into trips, where the naive
	// "first five" would score two pair.
	trip := Best([]string{"9H", "9C", "2S", "9D", "KH", "KC", "3D"})
	if trip.Category != fullHouse {
		t.Errorf("category = %d, want a full house", trip.Category)
	}
}

func TestDeckIsOneFullDeck(t *testing.T) {
	deck := buildDeck()
	if len(deck) != 52 {
		t.Fatalf("deck has %d cards, want 52", len(deck))
	}
	seen := map[string]bool{}
	for _, c := range deck {
		if seen[c] {
			t.Errorf("duplicate card %q", c)
		}
		seen[c] = true
	}
}
