package ai

import (
	"testing"

	"zolik/server/internal/rules"
)

// visibleFrom is the same projection the zolikmod bot builds: everything the
// agent is allowed to see, straight off the engine state.
func visibleFrom(gs rules.GameState) VisibleState {
	return VisibleState{
		GameNumber:    gs.GameNumber,
		Round:         gs.Round,
		Phase:         string(gs.Phase),
		CurrentTurn:   gs.CurrentTurn,
		DiscardPile:   gs.DiscardPile,
		Melds:         gs.Melds,
		MeldMeta:      gs.MeldMeta,
		RoundReqMet:   gs.RoundReqMet,
		Rules:         rules.ResolveConfig(gs.Rules),
		PendingJokers: gs.JokersReclaimedPendingMeld,
	}
}

// A joker the agent owes the table is placed before any other shed — even
// when an ordinary lay-off is also on the table.
func TestHeuristicAgent_PaysTheJokerDebtFirst(t *testing.T) {
	agent := NewHeuristicAgent("medium")
	gs := rules.GameState{
		Status:      rules.StatusActive,
		Rules:       rules.ProfileZolikClassic,
		GameNumber:  1,
		Round:       2,
		Phase:       rules.PhaseMeld,
		CurrentTurn: "ai",
		TurnOrder:   []string{"ai", "p2"},
		Hands:       map[string][]string{"ai": {"5C", "JOKER1", "KS"}, "p2": {"2D"}},
		Melds: map[string][][]string{
			"p2": {{"2C", "3C", "4C"}, {"9D", "9H", "9S"}},
		},
		MeldMeta: map[string][]rules.MeldInfo{"p2": {
			{MeldID: "m1", Type: rules.MeldRun, OwnerID: "p2"},
			{MeldID: "m2", Type: rules.MeldSet, OwnerID: "p2"},
		}},
		RoundReqMet:                map[string]bool{"ai": true, "p2": true},
		JokersReclaimedPendingMeld: []string{"JOKER1"},
	}

	action := agent.ChooseAction(visibleFrom(gs), gs.Hands["ai"])
	if action.Type != rules.ActionLayOff || action.Card != "JOKER1" {
		t.Fatalf("expected the pending joker laid off first, got %+v", action)
	}
	if _, err := rules.ApplyAction(gs, "ai", action); err != nil {
		t.Fatalf("the engine refused the agent's joker placement: %v", err)
	}
}

// The whole loop through the real engine: the agent's lay-off buys the joker
// back (the engine converts the exact-place drop to a swap), the next choice
// pays the debt, and the turn then ends in a discard the engine accepts —
// never a wedge.
func TestHeuristicAgent_ReclaimThenReplayThenDiscard(t *testing.T) {
	agent := NewHeuristicAgent("medium")
	gs := rules.GameState{
		Status:      rules.StatusActive,
		Rules:       rules.ProfileZolikClassic,
		GameNumber:  1,
		Round:       2,
		Phase:       rules.PhaseMeld,
		CurrentTurn: "ai",
		TurnOrder:   []string{"ai", "p2"},
		Hands:       map[string][]string{"ai": {"7C", "KS", "KD"}, "p2": {"2D", "2H"}},
		Melds: map[string][][]string{
			"p2": {{"5C", "6C", "JOKER1", "8C"}},
		},
		MeldMeta: map[string][]rules.MeldInfo{"p2": {
			{MeldID: "m1", Type: rules.MeldRun, OwnerID: "p2", WildCount: 1},
		}},
		RoundReqMet: map[string]bool{"ai": true, "p2": true},
		DiscardPile: []string{"9C"},
		GameScores:  map[string][]int{},
		TotalScores: map[string]int{},
	}

	for step := 0; step < 8; step++ {
		action := agent.ChooseAction(visibleFrom(gs), gs.Hands["ai"])
		out, err := rules.ApplyAction(gs, "ai", action)
		if err != nil {
			t.Fatalf("step %d: the engine refused the agent's %s: %v", step, action.Type, err)
		}
		gs = out.State
		if gs.CurrentTurn != "ai" {
			// The turn ended in an accepted discard.
			if n := len(gs.JokersReclaimedPendingMeld); n != 0 {
				t.Fatalf("the turn ended with a joker still owed: %v", gs.JokersReclaimedPendingMeld)
			}
			return
		}
	}
	t.Fatalf("the agent never ended its turn; hand %v, owed %v",
		gs.Hands["ai"], gs.JokersReclaimedPendingMeld)
}
