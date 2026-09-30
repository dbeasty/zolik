package rules

import "testing"

// The ledger is only worth showing if it is the score: every tally has to
// price to exactly what ScoreDeal charged, and every card has to land in
// exactly one category.
func TestTallyHand_PricesToThePenalty(t *testing.T) {
	cases := []struct {
		name  string
		hand  []string
		melds [][]string
		want  HandTally
	}{
		{
			name: "one of everything",
			hand: []string{"JOKER1", "AH", "KS", "TD", "7C"},
			want: HandTally{
				Cards: 5, Points: 50 + 25 + 10 + 10 + 7,
				Jokers: 1, JokerPoints: 50,
				Aces: 1, AcePoints: 25,
				Faces: 2, FacePoints: 20,
				Pips: 1, PipPoints: 7,
			},
		},
		{
			name: "an ace beside its two counts low",
			hand: []string{"AS", "2S", "KH"},
			want: HandTally{
				Cards: 3, Points: 13,
				AcesLow: 1, AceLowPoints: 1,
				Faces: 1, FacePoints: 10,
				Pips: 1, PipPoints: 2,
			},
		},
		{
			name:  "an ace that extends a table run counts low",
			hand:  []string{"AH", "AD"},
			melds: [][]string{{"TH", "JH", "QH", "KH"}},
			want: HandTally{
				Cards: 2, Points: 26,
				Aces: 1, AcePoints: 25,
				AcesLow: 1, AceLowPoints: 1,
			},
		},
		{name: "empty", hand: nil, want: HandTally{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := TallyHand(c.hand, c.melds, ProfileContinental)
			if got != c.want {
				t.Fatalf("tally = %+v, want %+v", got, c.want)
			}
			if pen := HandPenaltyTotalWithMelds(c.hand, c.melds, ProfileContinental); got.Points != pen {
				t.Fatalf("tally prices to %d, penalty is %d", got.Points, pen)
			}
			n := got.Jokers + got.Aces + got.AcesLow + got.Faces + got.Pips
			p := got.JokerPoints + got.AcePoints + got.AceLowPoints + got.FacePoints + got.PipPoints
			if n != got.Cards || p != got.Points {
				t.Fatalf("categories hold %d cards / %d points, hand is %d / %d", n, p, got.Cards, got.Points)
			}
		})
	}
}

func TestScoreDeal_RecordsTheHandsBehindTheScores(t *testing.T) {
	st := GameState{
		Status:      StatusActive,
		GameNumber:  1,
		Phase:       PhaseDiscard,
		TurnOrder:   []string{"p1", "p2", "p3"},
		Hands:       map[string][]string{"p1": {"KH", "JOKER1"}, "p2": {}, "p3": {"3C"}},
		RoundReqMet: map[string]bool{},
		GameScores:  map[string][]int{"p1": {}, "p2": {}, "p3": {}},
		TotalScores: map[string]int{},
	}
	next, _, err := ScoreDeal(st, "p2")
	if err != nil {
		t.Fatalf("ScoreDeal: %v", err)
	}
	if len(next.DealHands) != 1 {
		t.Fatalf("DealHands = %v, want one deal", next.DealHands)
	}
	got := next.DealHands[0]
	if _, ok := got["p2"]; ok {
		t.Errorf("the deal's winner has a hand recorded: %+v", got["p2"])
	}
	for _, pid := range []string{"p1", "p3"} {
		if got[pid].Points != next.GameScores[pid][0] {
			t.Errorf("%s: hand priced at %d, scored %d", pid, got[pid].Points, next.GameScores[pid][0])
		}
	}
	if got["p1"].Jokers != 1 || got["p1"].Faces != 1 {
		t.Errorf("p1 tally = %+v", got["p1"])
	}
}

// A match already under way when the ledger shipped has scores with no hands
// behind them. The first deal it records has to land on its own index, not on
// deal one.
func TestScoreDeal_PadsHandsForDealsScoredBeforeTheLedger(t *testing.T) {
	st := GameState{
		Status:      StatusActive,
		GameNumber:  3,
		Phase:       PhaseDiscard,
		TurnOrder:   []string{"p1", "p2"},
		Hands:       map[string][]string{"p1": {"9D"}, "p2": {}},
		RoundReqMet: map[string]bool{},
		GameScores:  map[string][]int{"p1": {4, 0}, "p2": {0, 12}},
		TotalScores: map[string]int{"p1": 4, "p2": 12},
	}
	next, _, err := ScoreDeal(st, "p2")
	if err != nil {
		t.Fatalf("ScoreDeal: %v", err)
	}
	if len(next.DealHands) != 3 || next.DealHands[0] != nil || next.DealHands[1] != nil {
		t.Fatalf("DealHands = %v, want two empty deals then this one", next.DealHands)
	}
	if next.DealHands[2]["p1"].Points != 9 {
		t.Fatalf("deal three = %+v", next.DealHands[2])
	}
}
