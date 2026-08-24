package canasta

import "testing"

// TestCardValues pins the table every other number in the game is built from.
func TestCardValues(t *testing.T) {
	cases := map[string]int{
		"JOKER1": 50, "JOKER2": 50,
		"2H": 20, "2S": 20,
		"AH": 20, "AS": 20,
		"KH": 10, "QD": 10, "JC": 10, "TS": 10, "9H": 10, "8D": 10,
		"7C": 5, "6S": 5, "5H": 5, "4D": 5,
		"3C": 5, "3S": 5, // black threes are worth five like any low card
		"3H": redThreeValue, "3D": redThreeValue,
	}
	for card, want := range cases {
		if got := cardValue(card); got != want {
			t.Errorf("cardValue(%q) = %d, want %d", card, got, want)
		}
	}
}

func TestCardPredicates(t *testing.T) {
	cases := []struct {
		card                       string
		wild, redThree, blackThree bool
	}{
		{card: "JOKER1", wild: true},
		{card: "JOKER2", wild: true},
		{card: "2H", wild: true},
		{card: "2S", wild: true},
		{card: "3H", redThree: true},
		{card: "3D", redThree: true},
		{card: "3C", blackThree: true},
		{card: "3S", blackThree: true},
		{card: "AS"},
		{card: "TD"},
	}
	for _, tc := range cases {
		if got := isWild(tc.card); got != tc.wild {
			t.Errorf("isWild(%q) = %v, want %v", tc.card, got, tc.wild)
		}
		if got := isRedThree(tc.card); got != tc.redThree {
			t.Errorf("isRedThree(%q) = %v, want %v", tc.card, got, tc.redThree)
		}
		if got := isBlackThree(tc.card); got != tc.blackThree {
			t.Errorf("isBlackThree(%q) = %v, want %v", tc.card, got, tc.blackThree)
		}
	}

	// Jokers collapse to one rank: two differently named jokers must never be
	// treated as two different ranks by the meld validator.
	if rankOf("JOKER1") != rankOf("JOKER2") {
		t.Error("the two joker names should share a rank")
	}
}

func TestDeckIsTwoDecksAndFourJokers(t *testing.T) {
	deck := buildDeck()
	if len(deck) != 108 {
		t.Fatalf("deck has %d cards, want 108", len(deck))
	}
	counts := map[string]int{}
	for _, c := range deck {
		counts[rankOf(c)]++
	}
	if counts[rankJoker] != 4 {
		t.Errorf("%d jokers, want 4", counts[rankJoker])
	}
	for _, r := range ranks {
		if counts[r] != 8 {
			t.Errorf("rank %q appears %d times, want 8", r, counts[r])
		}
	}
	// Four red threes and four black ones, which is what makes the all-four
	// bonus reachable at all.
	red, black := 0, 0
	for _, c := range deck {
		if isRedThree(c) {
			red++
		}
		if isBlackThree(c) {
			black++
		}
	}
	if red != 4 || black != 4 {
		t.Errorf("%d red and %d black threes, want 4 and 4", red, black)
	}
}

// TestCanastaBonuses covers the distinction the whole game is named for.
func TestCanastaBonuses(t *testing.T) {
	natural := Meld{Rank: "K", Cards: []string{"KH", "KD", "KS", "KC", "KH", "KD", "KS"}}
	mixed := Meld{Rank: "K", Cards: []string{"KH", "KD", "KS", "KC", "KH", "2S", "JOKER1"}}
	short := Meld{Rank: "K", Cards: []string{"KH", "KD", "KS"}}

	if !natural.isCanasta() || !natural.isNatural() {
		t.Error("seven wild-free kings should be a natural canasta")
	}
	if !mixed.isCanasta() || mixed.isNatural() {
		t.Error("seven kings with wilds should be a mixed canasta")
	}
	if short.isCanasta() {
		t.Error("three kings are not a canasta")
	}

	tm := &Team{Melds: []Meld{natural, mixed, short}}
	want := naturalCanastaBonus + mixedCanastaBonus
	if got := canastaScore(tm); got != want {
		t.Errorf("canastaScore = %d, want %d", got, want)
	}
	if got := tm.canastas(); got != 2 {
		t.Errorf("canastas = %d, want 2", got)
	}
}

// TestScoreDeal walks one deal's arithmetic end to end, because every part of
// it is a place an off-by-one would quietly cost somebody a match.
func TestScoreDeal(t *testing.T) {
	s := &GameState{
		Players:   []string{"p1", "p2"},
		TurnOrder: []string{"p1", "p2"},
		TeamOf:    map[string]int{"p1": 0, "p2": 1},
		Teams: []Team{
			{
				ID:      0,
				Players: []string{"p1"},
				Melds: []Meld{
					// A natural canasta of kings: 70 in cards, 500 bonus.
					{Rank: "K", Cards: []string{"KH", "KD", "KS", "KC", "KH", "KD", "KS"}},
					// Three fives: 15 in cards, no bonus.
					{Rank: "5", Cards: []string{"5H", "5D", "5S"}},
				},
				RedThrees: []string{"3H"},
			},
			{
				ID:      1,
				Players: []string{"p2"},
				// No canasta, so its red threes count against it.
				Melds:     []Meld{{Rank: "Q", Cards: []string{"QH", "QD", "QS"}}},
				RedThrees: []string{"3D", "3H"},
			},
		},
		Hands: map[string][]string{
			"p1": {},                       // went out
			"p2": {"AS", "2C", "8H", "4D"}, // 20 + 20 + 10 + 5 = 55 against
		},
	}

	res := scoreDeal(s, "p1", false, false)

	// Team 0: 70 + 15 melded, 500 canasta, 100 red three, 100 going out, none
	// left in hand.
	want0 := 85 + naturalCanastaBonus + redThreeValue + goingOutBonus
	if res.Teams[0].Total != want0 {
		t.Errorf("team 0 scored %d, want %d (%+v)", res.Teams[0].Total, want0, res.Teams[0])
	}
	// Team 1: 30 melded, no canasta so two red threes are -200, 55 in hand.
	want1 := 30 - 200 - 55
	if res.Teams[1].Total != want1 {
		t.Errorf("team 1 scored %d, want %d (%+v)", res.Teams[1].Total, want1, res.Teams[1])
	}
	if s.Teams[0].Score != want0 || s.Teams[1].Score != want1 {
		t.Errorf("running totals are %d/%d, want %d/%d",
			s.Teams[0].Score, s.Teams[1].Score, want0, want1)
	}
	if res.Teams[0].Running != want0 {
		t.Errorf("the result should carry the running total, got %d", res.Teams[0].Running)
	}
}

// TestScoreDealCountsAPartnersHandToo — going out clears your own hand, not
// your partner's, and forgetting that would make going out early strictly
// better than it is.
func TestScoreDealCountsAPartnersHandToo(t *testing.T) {
	s := &GameState{
		Players:   []string{"p1", "p2", "p3", "p4"},
		TurnOrder: []string{"p1", "p2", "p3", "p4"},
		TeamOf:    map[string]int{"p1": 0, "p3": 0, "p2": 1, "p4": 1},
		Teams: []Team{
			{ID: 0, Players: []string{"p1", "p3"},
				Melds: []Meld{{Rank: "K", Cards: []string{"KH", "KD", "KS", "KC", "KH", "KD", "KS"}}}},
			{ID: 1, Players: []string{"p2", "p4"}},
		},
		Hands: map[string][]string{
			"p1": {},           // went out
			"p3": {"AS", "AH"}, // 40 still against the partnership
			"p2": {}, "p4": {},
		},
	}
	res := scoreDeal(s, "p1", false, false)
	if res.Teams[0].InHand != 40 {
		t.Errorf("the partner's hand counted %d against the team, want 40", res.Teams[0].InHand)
	}
}

// TestMatchWinner covers the tie, which is the only case with a real decision
// in it: a target reached by both partnerships at once is settled on the higher
// total, and an exact tie plays on rather than being broken arbitrarily.
func TestMatchWinner(t *testing.T) {
	cases := []struct {
		name   string
		scores []int
		target int
		want   int
	}{
		{name: "nobody there yet", scores: []int{400, 300}, target: 500, want: -1},
		{name: "one over the line", scores: []int{520, 300}, target: 500, want: 0},
		{name: "both over, higher wins", scores: []int{520, 610}, target: 500, want: 1},
		{name: "an exact tie plays on", scores: []int{600, 600}, target: 500, want: -1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &GameState{TargetScore: tc.target}
			for i, sc := range tc.scores {
				s.Teams = append(s.Teams, Team{ID: i, Score: sc})
			}
			if got := matchWinner(s); got != tc.want {
				t.Errorf("matchWinner = %d, want %d", got, tc.want)
			}
		})
	}
}

// TestReachableValueIsAchievable is the property the initial-meld rule depends
// on: it must never promise more than a hand can actually lay, or a player can
// be talked into a lay that strands them below the floor with no way back.
func TestReachableValueIsAchievable(t *testing.T) {
	cases := []struct {
		name string
		hand []string
		want int
	}{
		{
			name: "three aces",
			hand: []string{"AH", "AD", "AS", "8C"},
			want: 60,
		},
		{
			name: "a pair plus a wild melds; the singleton does not",
			hand: []string{"AH", "AD", "2C", "9S"},
			want: 60, // 20 + 20 + 20
		},
		{
			name: "a lone pair with no wild melds nothing",
			hand: []string{"AH", "AD", "9S", "8C"},
			want: 0,
		},
		{
			name: "one wild cannot open two pairs at once",
			hand: []string{"AH", "AD", "KH", "KD", "2C"},
			want: 60, // the aces take the wild; the kings are left short
		},
		{
			name: "threes are worth nothing toward opening",
			hand: []string{"3C", "3S", "3H", "9S"},
			want: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := reachableValue(tc.hand, &Team{}); got != tc.want {
				t.Errorf("reachableValue(%v) = %d, want %d", tc.hand, got, tc.want)
			}
		})
	}
}
