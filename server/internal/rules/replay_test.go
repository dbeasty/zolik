package rules

import "testing"

func freshReplayTestState() GameState {
	return GameState{
		Status:      StatusActive,
		GameNumber:  1,
		Phase:       PhaseDraw,
		TurnOrder:   []string{"p1", "p2"},
		CurrentTurn: "p1",
		DrawPile:    []string{"9C", "8D", "7H", "6S", "5C"},
		DiscardPile: []string{"2H"},
		Hands: map[string][]string{
			"p1": {"3H", "4H"},
			"p2": {"3S", "4S"},
		},
		RoundReqMet: map[string]bool{"p1": false, "p2": false},
		DeckSeed:    42,
		GameScores:  map[string][]int{"p1": {}, "p2": {}},
		TotalScores: map[string]int{"p1": 0, "p2": 0},
	}
}

func TestReplayActions_ReproducesSequentialApply(t *testing.T) {
	actions := []LoggedAction{
		{PlayerID: "p1", Action: Action{Type: ActionDrawCard, DrawFrom: DrawFromDeck}},
		{PlayerID: "p1", Action: Action{Type: ActionDiscard, Card: "3H"}},
		{PlayerID: "p2", Action: Action{Type: ActionDrawCard, DrawFrom: DrawFromDeck}},
	}

	// Sequential ApplyAction, one call at a time, is the ground truth. Each
	// chain gets its own freshly-built state — GameState's map fields
	// (Hands, RoundReqMet, ...) aren't deep-copied by struct assignment, so
	// two chains sharing one built state would corrupt each other exactly
	// like two takeback replays would if they ever shared a live map
	// instead of one freshly deserialized from storage.
	want := freshReplayTestState()
	for _, la := range actions {
		outcome, err := ApplyAction(want, la.PlayerID, la.Action)
		if err != nil {
			t.Fatalf("sequential apply failed: %v", err)
		}
		want = outcome.State
	}

	got, err := ReplayActions(freshReplayTestState(), actions)
	if err != nil {
		t.Fatalf("ReplayActions failed: %v", err)
	}

	if got.CurrentTurn != want.CurrentTurn {
		t.Fatalf("CurrentTurn mismatch: got %q want %q", got.CurrentTurn, want.CurrentTurn)
	}
	if len(got.DrawPile) != len(want.DrawPile) {
		t.Fatalf("DrawPile length mismatch: got %d want %d", len(got.DrawPile), len(want.DrawPile))
	}
	for pid := range want.Hands {
		if len(got.Hands[pid]) != len(want.Hands[pid]) {
			t.Fatalf("Hands[%s] mismatch: got %v want %v", pid, got.Hands[pid], want.Hands[pid])
		}
		for i := range want.Hands[pid] {
			if got.Hands[pid][i] != want.Hands[pid][i] {
				t.Fatalf("Hands[%s][%d] mismatch: got %v want %v", pid, i, got.Hands[pid], want.Hands[pid])
			}
		}
	}
	if got.DiscardPile[len(got.DiscardPile)-1] != want.DiscardPile[len(want.DiscardPile)-1] {
		t.Fatalf("top of discard mismatch: got %v want %v", got.DiscardPile, want.DiscardPile)
	}
}

func TestReplayActions_StopsAtFirstRejectedAction(t *testing.T) {
	initial := GameState{
		Status:      StatusActive,
		GameNumber:  1,
		Phase:       PhaseDraw,
		TurnOrder:   []string{"p1", "p2"},
		CurrentTurn: "p1",
		DrawPile:    []string{"9C"},
		DiscardPile: []string{"2H"},
		Hands:       map[string][]string{"p1": {"3H"}, "p2": {"3S"}},
		RoundReqMet: map[string]bool{"p1": false, "p2": false},
	}

	actions := []LoggedAction{
		// p2 acting out of turn must be rejected.
		{PlayerID: "p2", Action: Action{Type: ActionDrawCard, DrawFrom: DrawFromDeck}},
	}

	if _, err := ReplayActions(initial, actions); err == nil {
		t.Fatalf("expected replay to fail on an out-of-turn action")
	}
}
