package rules

import (
	"reflect"
	"testing"
)

// GameState's maps and slices are reference types, so a caller's state used
// to alias the engine's: every Validate* assigns into state.Hands[playerID]
// and friends, reaching back through to mutate the caller's copy in place.
// The server's discard log read game.Hands after calling ApplyAction to
// report the hand before it, and so always printed the hand *after* — every
// "handBefore=… handAfter=…" line in production had the two sides identical.
// The same aliasing also let a rejected action leave its half-applied edits
// on a state the caller was told nothing had happened to.

func stateForIsolationTest() GameState {
	return GameState{
		Status:      StatusActive,
		Phase:       PhaseMeld,
		Round:       3,
		GameNumber:  1,
		CurrentTurn: "p1",
		TurnOrder:   []string{"p1", "p2"},
		Hands: map[string][]string{
			"p1": {"7S", "6S", "5H", "QH"},
			"p2": {"4S", "8C"},
		},
		Melds:       map[string][][]string{},
		MeldMeta:    map[string][]MeldInfo{},
		DrawPile:    []string{"2H", "3H"},
		DiscardPile: []string{"3D"},
		RoundReqMet: map[string]bool{"p1": true, "p2": true},
	}
}

func TestApplyAction_LeavesTheCallersStateAlone(t *testing.T) {
	st := stateForIsolationTest()
	handBefore := append([]string(nil), st.Hands["p1"]...)
	pileBefore := append([]string(nil), st.DiscardPile...)

	out, err := ApplyAction(st, "p1", Action{Type: ActionDiscard, Card: "QH"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	if !reflect.DeepEqual(st.Hands["p1"], handBefore) {
		t.Fatalf("caller's hand mutated: %v (want %v)", st.Hands["p1"], handBefore)
	}
	if !reflect.DeepEqual(st.DiscardPile, pileBefore) {
		t.Fatalf("caller's discard pile mutated: %v (want %v)", st.DiscardPile, pileBefore)
	}
	// And the returned state really did move on, so this isn't passing by
	// virtue of nothing having happened.
	if got := out.State.Hands["p1"]; len(got) != 3 {
		t.Fatalf("expected the discard applied to the returned state, got %v", got)
	}
}

func TestApplyAction_RejectedActionLeavesNoHalfAppliedEdits(t *testing.T) {
	// A discard that would empty the hand of a player who hasn't met the
	// round requirement is refused — but only after ValidateDiscard has
	// already taken the card out of the hand to see the emptiness.
	st := stateForIsolationTest()
	st.Hands["p1"] = []string{"QH"}
	st.RoundReqMet["p1"] = false
	handBefore := append([]string(nil), st.Hands["p1"]...)

	if _, err := ApplyAction(st, "p1", Action{Type: ActionDiscard, Card: "QH"}); err == nil {
		t.Fatalf("expected the go-out to be refused")
	}
	if !reflect.DeepEqual(st.Hands["p1"], handBefore) {
		t.Fatalf("rejected action still mutated the caller's hand: %v (want %v)", st.Hands["p1"], handBefore)
	}
}
