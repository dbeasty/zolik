package rules

import "testing"

func TestDetermineGameWinner_LowestTotal(t *testing.T) {
	st := GameState{
		TurnOrder:   []string{"a", "b"},
		TotalScores: map[string]int{"a": 120, "b": 95},
		GameScores:  map[string][]int{"a": {10, 20}, "b": {5, 15}},
	}
	w, draw := DetermineGameWinner(st)
	if draw || w != "b" {
		t.Fatalf("expected b to win, got winner=%q draw=%v", w, draw)
	}
}

func TestDetermineGameWinner_TiebreakFewestRoundsWon(t *testing.T) {
	st := GameState{
		TurnOrder:   []string{"a", "b"},
		TotalScores: map[string]int{"a": 100, "b": 100},
		GameScores: map[string][]int{
			"a": {10, 20, 30, 40}, // won rounds 0,1,2,3 at min? simplify: a wins more rounds
			"b": {50, 50, 50, 50},
		},
	}
	// Give a more round wins
	st.GameScores = map[string][]int{
		"a": {5, 5, 5, 5},
		"b": {10, 10, 10, 10},
	}
	w, draw := DetermineGameWinner(st)
	if draw || w != "b" {
		t.Fatalf("expected b (fewer rounds won) to win tiebreak, got winner=%q draw=%v", w, draw)
	}
}

func TestDetermineGameWinner_DrawWhenStillTied(t *testing.T) {
	st := GameState{
		TurnOrder:   []string{"a", "b"},
		TotalScores: map[string]int{"a": 50, "b": 50},
		GameScores: map[string][]int{
			"a": {10, 20},
			"b": {10, 20},
		},
	}
	w, draw := DetermineGameWinner(st)
	if !draw || w != "" {
		t.Fatalf("expected draw, got winner=%q draw=%v", w, draw)
	}
}

func TestApplyAction_EmitsGameEndedEvent(t *testing.T) {
	p := "p1"
	st := baseActiveState(7, p)
	st.Hands[p] = []string{"5H", "6H", "7H", "8H"}
	st.Melds[p] = [][]string{
		{"5D", "6D", "7D", "8D"},
		{"5C", "6C", "7C", "8C"},
	}
	st.MeldMeta[p] = []MeldInfo{
		{MeldID: "m1", Type: MeldRun, OwnerID: p},
		{MeldID: "m2", Type: MeldRun, OwnerID: p},
	}
	st.RoundReqMet[p] = false
	st.TotalScores = map[string]int{p: 0}
	st.GameScores = map[string][]int{p: {}}

	outcome, err := ApplyAction(st, p, Action{Type: ActionLayMeld, Cards: []string{"5H", "6H", "7H", "8H"}})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	foundDeal, foundGame := false, false
	for _, ev := range outcome.Events {
		if ev.Type == "deal_ended" {
			foundDeal = true
		}
		if ev.Type == "game_ended" {
			foundGame = true
		}
	}
	if !foundDeal || !foundGame {
		t.Fatalf("expected deal_ended and game_ended events, got %v", outcome.Events)
	}
}

func TestValidateMeldAction_CannotEmptyHandBeforeGame7(t *testing.T) {
	p := "p1"
	st := baseActiveState(1, p)
	st.Hands[p] = []string{"7H", "7D", "7C"}
	st.InitialMeldMinimum = 0
	_, _, _, err := ValidateMeldAction(st, p, []string{"7H", "7D", "7C"})
	if err == nil {
		t.Fatalf("expected error when melding entire hand in game 1")
	}
}
