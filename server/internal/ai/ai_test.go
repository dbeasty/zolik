package ai

import (
	"testing"

	"zolik/server/internal/rules"
)

func TestHeuristicAgent_DrawActionAllowed(t *testing.T) {
	agent := NewHeuristicAgent("medium")
	aiID := "ai1"

	state := rules.GameState{
		Status:      rules.StatusActive,
		Phase:       rules.PhaseDraw,
		Round:       1,
		CurrentTurn: aiID,
		TurnOrder:   []string{aiID, "p2"},
		Hands:       map[string][]string{aiID: {}, "p2": {}},
		DrawPile:    []string{"2H"},
		DiscardPile: []string{"7H"},
		RoundReqMet: map[string]bool{aiID: false, "p2": false},
	}

	visible := VisibleState{
		Round:       state.Round,
		Phase:       string(state.Phase),
		CurrentTurn: state.CurrentTurn,
		DiscardPile: state.DiscardPile,
		RoundReqMet: state.RoundReqMet,
	}

	action := agent.ChooseAction(visible, state.Hands[aiID])
	_, err := rules.ApplyAction(state, aiID, action)
	if err != nil {
		t.Fatalf("expected draw action to be valid, got err: %v", err)
	}
}

func TestHeuristicAgent_MeldLayOff_MeldFoundAndValid(t *testing.T) {
	agent := NewHeuristicAgent("medium")
	aiID := "ai1"

	state := rules.GameState{
		Status:      rules.StatusActive,
		Phase:       rules.PhaseMeld,
		Round:       1,
		CurrentTurn: aiID,
		TurnOrder:   []string{aiID, "p2"},
		Hands:       map[string][]string{aiID: {"7H", "7D", "7C", "2H"}, "p2": {}},
		DrawPile:    []string{"2H"},
		DiscardPile: []string{},
		RoundReqMet: map[string]bool{aiID: false, "p2": false},
		// disable initial meld minimum in this unit test
		InitialMeldMinimum: 0,
	}

	visible := VisibleState{
		Round:       state.Round,
		Phase:       string(state.Phase),
		CurrentTurn: state.CurrentTurn,
		RoundReqMet: state.RoundReqMet,
	}

	action := agent.ChooseAction(visible, state.Hands[aiID])
	outcome, err := rules.ApplyAction(state, aiID, action)
	if err != nil {
		t.Fatalf("expected lay_meld to be valid, got err: %v", err)
	}
	if len(outcome.State.Melds[aiID]) != 1 {
		t.Fatalf("expected exactly one meld laid")
	}
}

func TestHeuristicAgent_DiscardAllowedWhenRoundReqMet(t *testing.T) {
	agent := NewHeuristicAgent("medium")
	aiID := "ai1"

	state := rules.GameState{
		Status:      rules.StatusActive,
		Phase:       rules.PhaseMeld,
		Round:       1,
		CurrentTurn: aiID,
		TurnOrder:   []string{aiID},
		Hands:       map[string][]string{aiID: {"KH"}},
		DrawPile:    []string{"2H"},
		DiscardPile: []string{},
		RoundReqMet: map[string]bool{aiID: true},
		InitialMeldMinimum: 0,
		DeckSeed:          1,
		RoundScores:      map[string][]int{aiID: {}},
		TotalScores:      map[string]int{aiID: 0},
	}

	visible := VisibleState{
		Round:       state.Round,
		Phase:       string(state.Phase),
		CurrentTurn: state.CurrentTurn,
		RoundReqMet: state.RoundReqMet,
	}

	action := agent.ChooseAction(visible, state.Hands[aiID])
	_, err := rules.ApplyAction(state, aiID, action)
	if err != nil {
		t.Fatalf("expected discard action to be valid, got err: %v", err)
	}
}

// Regression test: previously, findContributingMeld would choose a meld that
// completes the round requirement even when its natural value fell below
// InitialMeldMinimum. The server rejects that meld with MELD_BELOW_MINIMUM,
// and since the agent deterministically picks the same candidate every time,
// the AI turn loop would retry the identical losing move until it exhausted
// its step budget, permanently stalling the game. The agent must instead
// fall back to a valid action (discard) whose application succeeds.
func TestHeuristicAgent_MeldBelowMinimum_FallsBackToDiscard(t *testing.T) {
	agent := NewHeuristicAgent("medium")
	aiID := "ai1"

	state := rules.GameState{
		Status:      rules.StatusActive,
		Phase:       rules.PhaseMeld,
		Round:       1,
		CurrentTurn: aiID,
		TurnOrder:   []string{aiID, "p2"},
		Hands:       map[string][]string{aiID: {"2H", "2D", "2C", "9S"}, "p2": {}},
		DrawPile:    []string{"5H"},
		DiscardPile: []string{},
		Melds:       map[string][][]string{aiID: {{"3H", "3D", "3C"}}},
		MeldMeta: map[string][]rules.MeldInfo{
			aiID: {{MeldID: "meld_1", Type: rules.MeldSet, OwnerID: aiID}},
		},
		RoundReqMet:        map[string]bool{aiID: false, "p2": false},
		InitialMeldMinimum: 35, // 3H+3D+3C (9) + 2H+2D+2C (6) = 15, below minimum
		DeckSeed:           1,
		RoundScores:        map[string][]int{aiID: {}},
		TotalScores:        map[string]int{aiID: 0},
	}

	visible := VisibleState{
		Round:              state.Round,
		Phase:              string(state.Phase),
		CurrentTurn:        state.CurrentTurn,
		Melds:              state.Melds,
		MeldMeta:           state.MeldMeta,
		RoundReqMet:        state.RoundReqMet,
		InitialMeldMinimum: state.InitialMeldMinimum,
	}

	action := agent.ChooseAction(visible, state.Hands[aiID])
	if action.Type == rules.ActionLayMeld {
		t.Fatalf("expected agent to avoid the below-minimum meld, got lay_meld action: %+v", action)
	}
	if _, err := rules.ApplyAction(state, aiID, action); err != nil {
		t.Fatalf("expected fallback action %+v to be valid, got err: %v", action, err)
	}
}

func TestHeuristicAgent_OfferAcceptMayBeValid(t *testing.T) {
	agent := NewHeuristicAgent("medium")
	aiID := "ai1"

	state := rules.GameState{
		Status:      rules.StatusActive,
		Phase:       rules.PhaseOffer,
		Round:       1,
		CurrentTurn: "p2", // currentTurn not required for accept offer in our rules
		TurnOrder:   []string{"p2", aiID},
		Hands:       map[string][]string{aiID: {"7H", "7D", "7C", "2H"}, "p2": {}},
		DrawPile:    []string{"2H"},
		DiscardPile: []string{},
		RoundReqMet: map[string]bool{aiID: false, "p2": false},
		Offer:        &rules.DiscardOffer{Card: "7S", OfferedTo: aiID},
		InitialMeldMinimum: 0,
		DeckSeed:          1,
		RoundScores:      map[string][]int{aiID: {}, "p2": {}},
		TotalScores:      map[string]int{aiID: 0, "p2": 0},
	}

	visible := VisibleState{
		Round:       state.Round,
		Phase:       string(state.Phase),
		CurrentTurn: state.CurrentTurn,
		RoundReqMet: state.RoundReqMet,
		Offer:       state.Offer,
	}

	action := agent.ChooseAction(visible, state.Hands[aiID])
	_, err := rules.ApplyAction(state, aiID, action)
	if err != nil {
		t.Fatalf("expected offer action to be valid, got err: %v", err)
	}
}

