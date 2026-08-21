package rules

import "testing"

func baseActiveState(gameNumber int, playerID string) GameState {
	return GameState{
		Status:             StatusActive,
		GameNumber:         gameNumber,
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
		GameScores:         map[string][]int{playerID: {}, "p2": {}},
		TotalScores:        map[string]int{playerID: 0, "p2": 0},
	}
}

func TestGame1_RequiresTwoSets(t *testing.T) {
	p := "p1"
	st := baseActiveState(1, p)
	st.Hands[p] = []string{"7H", "7D", "7C", "8H", "8D", "8C", "2S"}

	st, _, _, err := ValidateMeldAction(st, p, []string{"7H", "7D", "7C"})
	if err != nil {
		t.Fatalf("first set: %v", err)
	}
	if st.RoundReqMet[p] {
		t.Fatalf("round req should not be met after one set in round 1")
	}

	st, _, _, err = ValidateMeldAction(st, p, []string{"8H", "8D", "8C"})
	if err != nil {
		t.Fatalf("second set: %v", err)
	}
	if !st.RoundReqMet[p] {
		t.Fatalf("round req should be met after two sets in round 1")
	}
}

func TestGame2_RequiresSetAndRun(t *testing.T) {
	p := "p1"
	st := baseActiveState(2, p)
	st.Hands[p] = []string{"7H", "7D", "7C", "5H", "6H", "7H2", "8H"}

	// Use distinct cards for run (duplicate 7H would break set logic)
	st.Hands[p] = []string{"7H", "7D", "7C", "5S", "6S", "7S", "8S", "9S"}

	st, _, _, err := ValidateMeldAction(st, p, []string{"7H", "7D", "7C"})
	if err != nil {
		t.Fatalf("set: %v", err)
	}
	if st.RoundReqMet[p] {
		t.Fatalf("should not meet round 2 after set only")
	}

	st, _, _, err = ValidateMeldAction(st, p, []string{"5S", "6S", "7S", "8S"})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !st.RoundReqMet[p] {
		t.Fatalf("should meet round 2 after set+run")
	}
}

func TestGame3_RequiresTwoRuns(t *testing.T) {
	p := "p1"
	st := baseActiveState(3, p)
	st.Hands[p] = []string{"5H", "6H", "7H", "8H", "5D", "6D", "7D", "8D", "2S"}

	st, _, _, err := ValidateMeldAction(st, p, []string{"5H", "6H", "7H", "8H"})
	if err != nil {
		t.Fatalf("run1: %v", err)
	}
	if st.RoundReqMet[p] {
		t.Fatalf("round 3 needs two runs")
	}

	st, _, _, err = ValidateMeldAction(st, p, []string{"5D", "6D", "7D", "8D"})
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
	st.Hands[p] = []string{"2H", "2D", "2C", "3H", "3D", "3C", "9S"}

	// Two low sets (2+2+2=6 each = 12 total) — below 35. Both melds must
	// still be layable (a shape-complete but under-value combination isn't
	// rejected outright — see ValidateMeldAction) but the player must not be
	// marked down since the sum never clears the floor.
	st, _, _, err := ValidateMeldAction(st, p, []string{"2H", "2D", "2C"})
	if err != nil {
		t.Fatalf("first meld: %v", err)
	}
	st, _, _, err = ValidateMeldAction(st, p, []string{"3H", "3D", "3C"})
	if err != nil {
		t.Fatalf("second meld: %v", err)
	}
	if st.RoundReqMet[p] {
		t.Fatalf("player should not be down: melds total 12 points, below the 35 floor")
	}
}

// TestInitialMeldMinimum_TwoCleanRunsCombineTowardFloor reproduces a
// reported bug: zolik_classic's "down" shape requirement is
// just "at least one clean run" (Sets:0, Runs:0, RequireCleanRun:true), so a
// single clean run alone already satisfies the shape check. Laying a second
// clean run must still add its value toward the floor instead of each meld
// being judged alone and rejected in isolation (e.g. Q-K-A = 21 points,
// 8-9-10 = 27 points — neither clears 35 alone, but together they total 48).
func TestInitialMeldMinimum_TwoCleanRunsCombineTowardFloor(t *testing.T) {
	p := "p1"
	st := baseActiveState(1, p)
	st.Rules = ProfileZolikClassic
	st.InitialMeldMinimum = 35
	st.Hands[p] = []string{"QH", "KH", "AH", "8H", "9H", "TH", "9S"}

	st, _, _, err := ValidateMeldAction(st, p, []string{"8H", "9H", "TH"})
	if err != nil {
		t.Fatalf("first run (8-9-10, 27 points): %v", err)
	}
	if st.RoundReqMet[p] {
		t.Fatalf("27 points alone should not be enough to go down")
	}

	st, _, _, err = ValidateMeldAction(st, p, []string{"QH", "KH", "AH"})
	if err != nil {
		t.Fatalf("second run (Q-K-A, 21 points): %v", err)
	}
	if !st.RoundReqMet[p] {
		t.Fatalf("27+21=48 points across both runs should clear the 35 floor")
	}
}

func TestInitialMeldMinimum_PassesWhenTotalEnough(t *testing.T) {
	p := "p1"
	st := baseActiveState(1, p)
	st.InitialMeldMinimum = 35
	// K=10 each => 30 + Q set would work: use 7,8,9,T,J sets... simpler: three tens + one ten in second set
	st.Hands[p] = []string{"KH", "KD", "KC", "QH", "QD", "QC", "2S"}

	st, _, _, err := ValidateMeldAction(st, p, []string{"KH", "KD", "KC"})
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	st, _, _, err = ValidateMeldAction(st, p, []string{"QH", "QD", "QC"})
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

	_, goOut, err := ValidateDiscard(st, p, "KH", nil)
	if err == nil && goOut {
		t.Fatalf("expected cannot go out without round req")
	}
	if err == nil {
		t.Fatalf("expected error discarding last card without round req")
	}
}

func TestGame7_GoOutViaMeldWithoutDiscard(t *testing.T) {
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

	outcome, err := ApplyAction(st, p, Action{Type: ActionLayMeld, Cards: []string{"5H", "6H", "7H", "8H"}})
	if err != nil {
		t.Fatalf("lay meld: %v", err)
	}
	next := outcome.State
	if !next.RoundReqMet[p] {
		t.Fatalf("expected game req met")
	}
	if len(next.Hands[p]) != 0 {
		t.Fatalf("expected empty hand after going out")
	}
	if next.Status != StatusCompleted {
		t.Fatalf("expected match completed after game 7 go-out, got game=%d status=%s", next.GameNumber, next.Status)
	}
	if next.WinnerID == "" && !next.IsDraw {
		t.Fatalf("expected winner or draw after game end")
	}
	// The meld that actually emptied the hand must still show up in the
	// action log, not just its deal_ended/game_ended consequence — see
	// endGameWithEvents callers in engine.go.
	if len(outcome.Events) == 0 || outcome.Events[0].Type != "meld_played" {
		t.Fatalf("expected meld_played to be the first emitted event, got %#v", outcome.Events)
	}
	sawDealEnded := false
	for _, e := range outcome.Events[1:] {
		if e.Type == "meld_played" {
			t.Fatalf("meld_played should only be emitted once, got %#v", outcome.Events)
		}
		if e.Type == "deal_ended" {
			sawDealEnded = true
		}
	}
	if !sawDealEnded {
		t.Fatalf("expected deal_ended after meld_played, got %#v", outcome.Events)
	}
}

// TestGoOut_ViaDiscard_EmitsDiscardEventBeforeDealEnded guards against a
// regression where the winning discard's own event was dropped from the
// action log: ApplyAction built a "player_discarded" event, then on the
// goOut path returned endGameWithEvents' fresh events slice instead of
// prepending to it, silently losing the discard event (state was still
// mutated correctly — only the log/broadcast lost the action).
func TestGoOut_ViaDiscard_EmitsDiscardEventBeforeDealEnded(t *testing.T) {
	p := "p1"
	st := baseActiveState(1, p)
	st.Phase = PhaseMeld
	st.Hands[p] = []string{"9S"}
	st.RoundReqMet[p] = true

	outcome, err := ApplyAction(st, p, Action{Type: ActionDiscard, Card: "9S"})
	if err != nil {
		t.Fatalf("discard: %v", err)
	}
	if len(outcome.Events) == 0 || outcome.Events[0].Type != "player_discarded" {
		t.Fatalf("expected player_discarded to be the first emitted event, got %#v", outcome.Events)
	}
	if outcome.Events[0].Data["card"] != "9S" {
		t.Fatalf("expected discarded card 9S in event data, got %#v", outcome.Events[0].Data)
	}
	sawDealEnded := false
	for _, e := range outcome.Events[1:] {
		if e.Type == "deal_ended" {
			sawDealEnded = true
		}
	}
	if !sawDealEnded {
		t.Fatalf("expected deal_ended after player_discarded, got %#v", outcome.Events)
	}
}

// TestGoOut_ViaLayOff_EmitsLayoffEventBeforeDealEnded is the ActionLayOff
// analogue of TestGoOut_ViaDiscard_EmitsDiscardEventBeforeDealEnded — same
// dropped-event bug, different action type (game 7's discard-free go-out).
func TestGoOut_ViaLayOff_EmitsLayoffEventBeforeDealEnded(t *testing.T) {
	p := "p1"
	st := baseActiveState(7, p)
	st.Hands[p] = []string{"9H"}
	st.RoundReqMet[p] = true
	st.Melds[p] = [][]string{{"5H", "6H", "7H", "8H"}}
	st.MeldMeta[p] = []MeldInfo{{MeldID: "m1", Type: MeldRun, OwnerID: p}}

	outcome, err := ApplyAction(st, p, Action{Type: ActionLayOff, MeldID: "m1", Card: "9H"})
	if err != nil {
		t.Fatalf("lay off: %v", err)
	}
	if len(outcome.Events) == 0 || outcome.Events[0].Type != "card_laid_off" {
		t.Fatalf("expected card_laid_off to be the first emitted event, got %#v", outcome.Events)
	}
	sawDealEnded := false
	for _, e := range outcome.Events[1:] {
		if e.Type == "deal_ended" {
			sawDealEnded = true
		}
	}
	if !sawDealEnded {
		t.Fatalf("expected deal_ended after card_laid_off, got %#v", outcome.Events)
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

func TestRoundRequirementFor_AllSevenGames(t *testing.T) {
	cases := []struct {
		game       int
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
		req := ProfileContinental.ContractFor(tc.game)
		if req.Sets != tc.sets || req.Runs != tc.runs {
			t.Fatalf("game %d: got sets=%d runs=%d want %d/%d", tc.game, req.Sets, req.Runs, tc.sets, tc.runs)
		}
	}
}
