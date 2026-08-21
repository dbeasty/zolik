package game

import (
	"testing"

	"zolik/server/internal/rules"
)

// TestToRulesAction_DrawCardCarriesTargetCard is the regression guard for a
// dropped field: toRulesAction built a draw action without Card, so
// ValidateDraw's targetCard parameter was unreachable from any client and
// zolik_classic's "take any card from the pile" rule was dead on the wire.
func TestToRulesAction_DrawCardCarriesTargetCard(t *testing.T) {
	got, err := toRulesAction(WSIncoming{Type: "draw_card", From: "discard", Card: "5H"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got.Type != rules.ActionDrawCard || got.DrawFrom != rules.DrawFromDiscard {
		t.Fatalf("unexpected action: %+v", got)
	}
	if got.Card != "5H" {
		t.Fatalf("target card dropped in translation: want 5H, got %q", got.Card)
	}
}

// TestToRulesAction_DrawCardWithoutTargetMeansTopCard keeps the default
// intact: an omitted card still means "the top of the pile".
func TestToRulesAction_DrawCardWithoutTargetMeansTopCard(t *testing.T) {
	got, err := toRulesAction(WSIncoming{Type: "draw_card", From: "discard"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got.Card != "" {
		t.Fatalf("want an empty target (top card), got %q", got.Card)
	}
}

// TestDeepDiscardPickup_ReachableFromTheWire drives the whole translation
// through the engine, proving the rule works end to end and not just that one
// struct field survives: a client asking for a card buried in the pile gets
// it plus everything stacked above it.
func TestDeepDiscardPickup_ReachableFromTheWire(t *testing.T) {
	st := rules.GameState{
		Status:      rules.StatusActive,
		Rules:       rules.ProfileZolikClassic, // DiscardPickupAnyFromPile
		GameNumber:  1,
		Round:       1,
		Phase:       rules.PhaseDraw,
		CurrentTurn: "p1",
		TurnOrder:   []string{"p1", "p2"},
		Hands:       map[string][]string{"p1": {"2C"}, "p2": {}},
		DrawPile:    []string{"KD"},
		DiscardPile: []string{"3S", "5H", "9C", "QD"}, // QD is the top card
		RoundReqMet: map[string]bool{"p1": true, "p2": false},
	}

	action, err := toRulesAction(WSIncoming{Type: "draw_card", From: "discard", Card: "5H"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	outcome, err := rules.ApplyAction(st, "p1", action)
	if err != nil {
		t.Fatalf("expected the deep pickup to be legal under zolik_classic, got %v", err)
	}

	// 5H and everything above it (9C, QD) come to hand; 3S stays on the pile.
	want := []string{"2C", "5H", "9C", "QD"}
	if got := outcome.State.Hands["p1"]; !equalCards(got, want) {
		t.Fatalf("hand after deep pickup: want %v, got %v", want, got)
	}
	if got := outcome.State.DiscardPile; !equalCards(got, []string{"3S"}) {
		t.Fatalf("discard pile after deep pickup: want [3S], got %v", got)
	}
}

// TestTopOnlyProfileIgnoresTargetCard confirms the newly-reachable parameter
// cannot be used to bypass a profile that only allows the top card.
func TestTopOnlyProfileIgnoresTargetCard(t *testing.T) {
	st := rules.GameState{
		Status:      rules.StatusActive,
		Rules:       rules.ProfileContinental, // DiscardPickupTopOnly
		GameNumber:  1,
		Round:       3, // past Continental's pickup lock
		Phase:       rules.PhaseDraw,
		CurrentTurn: "p1",
		TurnOrder:   []string{"p1", "p2"},
		Hands:       map[string][]string{"p1": {"2C"}, "p2": {}},
		DrawPile:    []string{"KD"},
		DiscardPile: []string{"3S", "5H", "9C"},
		RoundReqMet: map[string]bool{"p1": true, "p2": false},
	}

	action, _ := toRulesAction(WSIncoming{Type: "draw_card", From: "discard", Card: "3S"})
	outcome, err := rules.ApplyAction(st, "p1", action)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	// The request named the bottom card; a top-only profile still hands over
	// exactly the top one.
	if got := outcome.State.Hands["p1"]; !equalCards(got, []string{"2C", "9C"}) {
		t.Fatalf("top-only profile must ignore the target card: got hand %v", got)
	}
}

// TestRulesActionToWSIncoming_RoundTripsTargetCard covers the AI's path back
// out to the same wire format, so an agent that names a pile card keeps it.
func TestRulesActionToWSIncoming_RoundTripsTargetCard(t *testing.T) {
	in := rulesActionToWSIncoming(rules.Action{
		Type:     rules.ActionDrawCard,
		DrawFrom: rules.DrawFromDiscard,
		Card:     "7D",
	})
	if in.Card != "7D" {
		t.Fatalf("target card dropped on the way out: got %q", in.Card)
	}
	back, err := toRulesAction(in)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if back.Card != "7D" || back.DrawFrom != rules.DrawFromDiscard {
		t.Fatalf("round trip lost the target: %+v", back)
	}
}

func equalCards(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
