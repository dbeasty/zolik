package rules

import "testing"

// The defect these cover, seen in a live server log: a player already down
// took the top of the discard pile and handed the same card straight back on
// the same turn. Nothing changed — hand, pile and deck all ended the turn as
// they started it — so the table could pass one card round in a circle
// forever while the deck still held most of its cards. Log excerpt, one
// human and one agent trading a QH:
//
//	22:36:12 player=ad4f… discard card="QH"
//	22:36:13 ai decision: phase=draw chosen={draw_card DrawFrom:discard}
//	22:36:15 ai decision: phase=meld hand=[4S 8C 2S 2C 9H QH] chosen={discard Card:QH}
//
// ValidateDiscard's existing DiscardDrawnCardPendingMeld gate does not catch
// this: that obligation is deliberately empty for a player who is already
// down, because their pickup is otherwise unrestricted.

func downStateHolding(hand []string) GameState {
	return GameState{
		Status:      StatusActive,
		Phase:       PhaseMeld,
		Round:       3,
		GameNumber:  1,
		CurrentTurn: "p1",
		TurnOrder:   []string{"p1", "p2"},
		Hands:       map[string][]string{"p1": hand, "p2": {"3C"}},
		Melds:       map[string][][]string{},
		MeldMeta:    map[string][]MeldInfo{},
		DiscardPile: []string{},
		RoundReqMet: map[string]bool{"p1": true, "p2": true},
	}
}

func TestValidateDraw_DiscardPickupMarksTakenCardEvenWhenDown(t *testing.T) {
	st := GameState{
		Status:      StatusActive,
		Phase:       PhaseDraw,
		Round:       3,
		DrawPile:    []string{"2H"},
		DiscardPile: []string{"9S"},
		Hands:       map[string][]string{"p1": {"4D"}},
		CurrentTurn: "p1",
		RoundReqMet: map[string]bool{"p1": true},
	}
	ns, _, _, err := ValidateDraw(st, "p1", DrawFromDiscard, "")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	// The obligation stays empty (a down player owes nothing to an initial
	// meld) but the taken card is still recorded, which is the whole point:
	// the two fields answer different questions.
	if ns.DiscardDrawnCardPendingMeld != "" {
		t.Fatalf("expected no pending obligation once down, got %q", ns.DiscardDrawnCardPendingMeld)
	}
	if ns.DiscardTakenCard != "9S" {
		t.Fatalf("expected the pickup recorded as taken, got %q", ns.DiscardTakenCard)
	}
}

func TestValidateDiscard_RefusesTheCardJustTakenFromThePile(t *testing.T) {
	st := downStateHolding([]string{"4S", "3S", "QH"})
	st.DiscardTakenCard = "QH"

	_, _, err := ValidateDiscard(st, "p1", "QH", nil)
	if err == nil {
		t.Fatalf("expected the just-taken card to be refused as this turn's discard")
	}
	re, ok := err.(RulesError)
	if !ok || re.Code != ErrDiscardTakenCard {
		t.Fatalf("expected ErrDiscardTakenCard, got %#v", err)
	}

	// Everything else in hand is still fair game — the ban removes one
	// choice, it does not end the turn.
	if _, _, err := ValidateDiscard(st, "p1", "4S", nil); err != nil {
		t.Fatalf("expected a different card to discard cleanly, got %v", err)
	}
}

func TestValidateDiscard_TakenCardAllowedWhenItIsTheOnlyLegalDiscard(t *testing.T) {
	// Laid off down to just the card they took. Refusing it here would wedge
	// the player — and the whole table behind them — with no legal move.
	st := downStateHolding([]string{"QH"})
	st.DiscardTakenCard = "QH"

	ns, goOut, err := ValidateDiscard(st, "p1", "QH", nil)
	if err != nil {
		t.Fatalf("expected the last card in hand to be discardable, got %v", err)
	}
	if !goOut {
		t.Fatalf("expected discarding the last card to go out")
	}
	if len(ns.Hands["p1"]) != 0 {
		t.Fatalf("expected an empty hand, got %v", ns.Hands["p1"])
	}
}

func TestValidateDiscard_TakenCardAllowedWhenEverythingElseIsAJoker(t *testing.T) {
	// Jokers can't be discarded under Continental, so a hand of [taken card,
	// joker] has exactly one legal discard and the ban must yield to it.
	st := downStateHolding([]string{"QH", "JOKER1"})
	st.DiscardTakenCard = "QH"
	st.Rules = ProfileContinental

	if _, _, err := ValidateDiscard(st, "p1", "QH", nil); err != nil {
		t.Fatalf("expected the only discardable card to be allowed, got %v", err)
	}
}

func TestValidateDiscard_TakenCardMarkerRetiresWithTheTurn(t *testing.T) {
	st := downStateHolding([]string{"4S", "3S", "QH"})
	st.DiscardTakenCard = "QH"

	ns, _, err := ValidateDiscard(st, "p1", "4S", nil)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if ns.DiscardTakenCard != "" {
		t.Fatalf("expected the marker cleared once the turn ended, got %q", ns.DiscardTakenCard)
	}
}

func TestValidateDraw_DeckDrawClearsTheTakenCardMarker(t *testing.T) {
	st := downStateHolding([]string{"4S"})
	st.Phase = PhaseDraw
	st.DrawPile = []string{"2H"}
	st.DiscardTakenCard = "QH"

	ns, _, _, err := ValidateDraw(st, "p1", DrawFromDeck, "")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if ns.DiscardTakenCard != "" {
		t.Fatalf("expected a deck draw to clear the marker, got %q", ns.DiscardTakenCard)
	}
}

func TestValidateMeldAction_PlayingTheTakenCardReleasesItsTwin(t *testing.T) {
	// Two decks are in play, so a hand can hold two QH. Once the taken one
	// has gone to the table the marker must clear, or the player is left
	// unable to discard the QH they were holding all along.
	st := downStateHolding([]string{"QH", "QH", "QD", "QC", "4S"})
	st.DiscardTakenCard = "QH"

	ns, _, _, err := ValidateMeldAction(st, "p1", []string{"QH", "QD", "QC"})
	if err != nil {
		t.Fatalf("unexpected err laying the meld: %v", err)
	}
	if ns.DiscardTakenCard != "" {
		t.Fatalf("expected the marker spent by the meld, got %q", ns.DiscardTakenCard)
	}
	if _, _, err := ValidateDiscard(ns, "p1", "QH", nil); err != nil {
		t.Fatalf("expected the remaining QH to be discardable, got %v", err)
	}
}

func TestValidateUndoLayOff_RestoresTheTakenCardMarker(t *testing.T) {
	// Undoing the lay-off puts the taken card back in hand; if the marker
	// did not come back with it, lay-off-then-undo would be a way to launder
	// the card straight onto the pile.
	st := downStateHolding([]string{"7H", "4S"})
	st.DiscardTakenCard = "7H"
	st.Melds = map[string][][]string{"p2": {{"7D", "7C", "7S"}}}
	st.MeldMeta = map[string][]MeldInfo{"p2": {{MeldID: "meld_1", Type: MeldSet, OwnerID: "p2"}}}

	afterLayOff, err := ValidateLayOff(st, "p1", "meld_1", []string{"7H"}, "")
	if err != nil {
		t.Fatalf("unexpected err laying off: %v", err)
	}
	if afterLayOff.DiscardTakenCard != "" {
		t.Fatalf("expected the marker spent by the lay-off, got %q", afterLayOff.DiscardTakenCard)
	}

	restored, err := ValidateUndoLayOff(afterLayOff, "p1")
	if err != nil {
		t.Fatalf("unexpected err undoing the lay-off: %v", err)
	}
	if restored.DiscardTakenCard != "7H" {
		t.Fatalf("expected the marker restored with the card, got %q", restored.DiscardTakenCard)
	}
	if _, _, err := ValidateDiscard(restored, "p1", "7H", nil); err == nil {
		t.Fatalf("expected the restored marker to still block the discard")
	}
}

func TestValidateUndoDrawDiscard_ClearsTheTakenCardMarker(t *testing.T) {
	// The pickup is being taken back, so there is nothing to hold the player
	// to any more.
	st := downStateHolding([]string{"4S", "QH"})
	st.DiscardTakenCard = "QH"
	st.DiscardDrawnCards = []string{"QH"}

	ns, err := ValidateUndoDrawDiscard(st, "p1")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if ns.DiscardTakenCard != "" {
		t.Fatalf("expected the marker cleared by the undo, got %q", ns.DiscardTakenCard)
	}
}

// TestDiscardPingPong_IsNoLongerReachable drives the real engine through the
// exact sequence from the log: p1 discards QH, p2 (already down) draws it
// back off the pile, and then tries to discard it again.
func TestDiscardPingPong_IsNoLongerReachable(t *testing.T) {
	st := GameState{
		Status:      StatusActive,
		Phase:       PhaseMeld,
		Round:       3,
		GameNumber:  1,
		CurrentTurn: "p1",
		TurnOrder:   []string{"p1", "p2"},
		Hands: map[string][]string{
			"p1": {"7S", "6S", "5H", "QH"},
			"p2": {"4S", "8C", "2S", "2C", "9H"},
		},
		Melds:       map[string][][]string{},
		MeldMeta:    map[string][]MeldInfo{},
		DrawPile:    []string{"2H", "3H"},
		DiscardPile: []string{},
		RoundReqMet: map[string]bool{"p1": true, "p2": true},
	}

	out, err := ApplyAction(st, "p1", Action{Type: ActionDiscard, Card: "QH"})
	if err != nil {
		t.Fatalf("p1 discard failed: %v", err)
	}
	st = out.State
	if got := st.DiscardPile; len(got) != 1 || got[0] != "QH" {
		t.Fatalf("expected QH on top of the pile, got %v", got)
	}

	out, err = ApplyAction(st, "p2", Action{Type: ActionDrawCard, DrawFrom: DrawFromDiscard})
	if err != nil {
		t.Fatalf("p2 pickup failed: %v", err)
	}
	st = out.State

	if _, err := ApplyAction(st, "p2", Action{Type: ActionDiscard, Card: "QH"}); err == nil {
		t.Fatalf("expected the engine to refuse handing QH straight back")
	}

	// p2 is not stuck: any other card ends the turn normally.
	if _, err := ApplyAction(st, "p2", Action{Type: ActionDiscard, Card: "9H"}); err != nil {
		t.Fatalf("expected p2 to discard something else cleanly, got %v", err)
	}
}
