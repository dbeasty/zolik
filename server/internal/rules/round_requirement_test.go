package rules

import "testing"

func baseActiveState(round int, playerID string) GameState {
	return GameState{
		Status:             StatusActive,
		Round:              round,
		Phase:              PhaseMeld,
		CurrentTurn:        playerID,
		TurnOrder:          []string{playerID, "p2"},
		Hands:              map[string][]string{playerID: {}, "p2": {}},
		Melds:              map[string][][]string{},
		MeldMeta:           map[string][]MeldInfo{},
		RoundReqMet:        map[string]bool{playerID: false, "p2": false},
		InitialMeldMinimum: 0,
		DrawPile:           []string{"2C"},
		DiscardPile:        []string{},
		DeckSeed:           42,
		RoundScores:        map[string][]int{playerID: {}, "p2": {}},
		TotalScores:        map[string]int{playerID: 0, "p2": 0},
	}
}

func TestRound1_RequiresTwoSets(t *testing.T) {
	p := "p1"
	st := baseActiveState(1, p)
	st.Hands[p] = []string{"7H", "7D", "7C", "8H", "8D", "8C"}

	st, _, err := ValidateMeldAction(st, p, []string{"7H", "7D", "7C"})
	if err != nil {
		t.Fatalf("first set: %v", err)
	}
	if st.RoundReqMet[p] {
		t.Fatalf("round req should not be met after one set in round 1")
	}

	st, _, err = ValidateMeldAction(st, p, []string{"8H", "8D", "8C"})
	if err != nil {
		t.Fatalf("second set: %v", err)
	}
	if !st.RoundReqMet[p] {
		t.Fatalf("round req should be met after two sets in round 1")
	}
}

func TestRound2_RequiresSetAndRun(t *testing.T) {
	p := "p1"
	st := baseActiveState(2, p)
	st.Hands[p] = []string{"7H", "7D", "7C", "5H", "6H", "7H2", "8H"}

	// Use distinct cards for run (duplicate 7H would break set logic)
	st.Hands[p] = []string{"7H", "7D", "7C", "5S", "6S", "7S", "8S", "9S"}

	st, _, err := ValidateMeldAction(st, p, []string{"7H", "7D", "7C"})
	if err != nil {
		t.Fatalf("set: %v", err)
	}
	if st.RoundReqMet[p] {
		t.Fatalf("should not meet round 2 after set only")
	}

	st, _, err = ValidateMeldAction(st, p, []string{"5S", "6S", "7S", "8S"})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !st.RoundReqMet[p] {
		t.Fatalf("should meet round 2 after set+run")
	}
}

func TestRound3_RequiresTwoRuns(t *testing.T) {
	p := "p1"
	st := baseActiveState(3, p)
	st.Hands[p] = []string{"5H", "6H", "7H", "8H", "5D", "6D", "7D", "8D"}

	st, _, err := ValidateMeldAction(st, p, []string{"5H", "6H", "7H", "8H"})
	if err != nil {
		t.Fatalf("run1: %v", err)
	}
	if st.RoundReqMet[p] {
		t.Fatalf("round 3 needs two runs")
	}

	st, _, err = ValidateMeldAction(st, p, []string{"5D", "6D", "7D", "8D"})
	if err != nil {
		t.Fatalf("run2: %v", err)
	}
	if !st.RoundReqMet[p] {
		t.Fatalf("round 3 should be met")
	}
}

func TestInitialMeldMinimum_SumAcrossMelds(t *testing.T) {
	p := "p1"
	st := baseActiveState(1, p)
	st.InitialMeldMinimum = 35
	st.Hands[p] = []string{"2H", "2D", "2C", "3H", "3D", "3C"}

	// Two low sets (2+2+2=6 each = 12 total) — below 35.
	st, _, err := ValidateMeldAction(st, p, []string{"2H", "2D", "2C"})
	if err != nil {
		t.Fatalf("first meld: %v", err)
	}
	st, _, err = ValidateMeldAction(st, p, []string{"3H", "3D", "3C"})
	if err == nil {
		t.Fatalf("expected meld below minimum when total natural < 35")
	}
	re, ok := err.(RulesError)
	if !ok || re.Code != ErrMeldBelowMinimum {
		t.Fatalf("expected ErrMeldBelowMinimum got %v", err)
	}
}

func TestInitialMeldMinimum_PassesWhenTotalEnough(t *testing.T) {
	p := "p1"
	st := baseActiveState(1, p)
	st.InitialMeldMinimum = 35
	// K=10 each => 30 + Q set would work: use 7,8,9,T,J sets... simpler: three tens + one ten in second set
	st.Hands[p] = []string{"KH", "KD", "KC", "QH", "QD", "QC"}

	st, _, err := ValidateMeldAction(st, p, []string{"KH", "KD", "KC"})
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	st, _, err = ValidateMeldAction(st, p, []string{"QH", "QD", "QC"})
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if !st.RoundReqMet[p] {
		t.Fatalf("expected round req met")
	}
}

func TestGoOut_BlockedWithoutRoundReq(t *testing.T) {
	p := "p1"
	st := baseActiveState(1, p)
	st.Phase = PhaseMeld
	st.Hands[p] = []string{"KH"}
	st.RoundReqMet[p] = false

	_, goOut, err := ValidateDiscard(st, p, "KH")
	if err == nil && goOut {
		t.Fatalf("expected cannot go out without round req")
	}
	if err == nil {
		t.Fatalf("expected error discarding last card without round req")
	}
}

func TestRound7_GoOutViaMeldWithoutDiscard(t *testing.T) {
	p := "p1"
	st := baseActiveState(7, p)
	st.Hands[p] = []string{"5H", "6H", "7H", "8H"}
	// Already has two runs on table
	st.Melds[p] = [][]string{
		{"5D", "6D", "7D", "8D"},
		{"5C", "6C", "7C", "8C"},
	}
	st.MeldMeta[p] = []MeldInfo{
		{MeldID: "m1", Type: MeldRun, OwnerID: p},
		{MeldID: "m2", Type: MeldRun, OwnerID: p},
	}
	st.RoundReqMet[p] = false

	next, err := ApplyAction(st, p, Action{Type: ActionLayMeld, Cards: []string{"5H", "6H", "7H", "8H"}})
	if err != nil {
		t.Fatalf("lay meld: %v", err)
	}
	if !next.RoundReqMet[p] {
		t.Fatalf("expected round req met")
	}
	if len(next.Hands[p]) != 0 {
		t.Fatalf("expected empty hand after going out")
	}
	if next.Status != StatusCompleted {
		t.Fatalf("expected game completed after round 7 go-out, got round=%d status=%s", next.Round, next.Status)
	}
}

func TestReshuffle_ShufflesDiscardIntoDraw(t *testing.T) {
	st := GameState{
		Status:         StatusActive,
		Phase:          PhaseDraw,
		DrawPile:       nil,
		DiscardPile:    []string{"7H", "8H", "9H"},
		ReshuffleCount: 0,
		DeckSeed:       99,
		Hands:          map[string][]string{"p1": {}},
		CurrentTurn:    "p1",
	}
	ns, err := ensureDrawPile(st)
	if err != nil {
		t.Fatalf("reshuffle: %v", err)
	}
	if len(ns.DiscardPile) != 0 {
		t.Fatalf("discard should be empty")
	}
	if len(ns.DrawPile) != 3 {
		t.Fatalf("expected 3 cards in draw pile")
	}
	// Order should be shuffled (deterministic for seed); not guaranteed != original order
	// but with seed 99 it should differ from 7H,8H,9H order sometimes - just check all cards present
	counts := map[string]int{}
	for _, c := range ns.DrawPile {
		counts[c]++
	}
	for _, c := range []string{"7H", "8H", "9H"} {
		if counts[c] != 1 {
			t.Fatalf("missing card %s in reshuffled pile", c)
		}
	}
}

func TestRoundRequirementFor_AllSevenRounds(t *testing.T) {
	cases := []struct {
		round      int
		sets, runs int
	}{
		{1, 2, 0},
		{2, 1, 1},
		{3, 0, 2},
		{4, 3, 0},
		{5, 2, 1},
		{6, 1, 2},
		{7, 0, 3},
	}
	for _, tc := range cases {
		req := RoundRequirementFor(tc.round)
		if req.Sets != tc.sets || req.Runs != tc.runs {
			t.Fatalf("round %d: got sets=%d runs=%d want %d/%d", tc.round, req.Sets, req.Runs, tc.sets, tc.runs)
		}
	}
}
