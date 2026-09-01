package rules

import "testing"

// The take-and-replay rule: a joker taken off the table this turn must be
// played into a meld again before the turn can end. These tests pin the debt's
// whole life cycle — taken on by swap_joker and by a lay-off that frees a
// joker, paid off by lay_off/lay_meld, restored by every undo tier, and
// enforced (or not, per JokerReclaimMustPlay) at the discard.

// reclaimState is a two-player table where "me" is down and karel's run
// 5C-6C-JOKER1-8C is on the table, so holding 7C buys the joker back.
func reclaimState(hand []string) GameState {
	return GameState{
		Status:      StatusActive,
		Rules:       continentalNoFloor(),
		GameNumber:  1,
		Round:       1,
		Phase:       PhaseMeld,
		CurrentTurn: "me",
		TurnOrder:   []string{"me", "karel"},
		Hands:       map[string][]string{"me": hand, "karel": {"2D", "2H"}},
		Melds: map[string][][]string{
			"karel": {{"5C", "6C", "JOKER1", "8C"}, {"9D", "9H", "9S"}},
		},
		MeldMeta: map[string][]MeldInfo{"karel": {
			{MeldID: "m1", Type: MeldRun, OwnerID: "karel", WildCount: 1},
			{MeldID: "m2", Type: MeldSet, OwnerID: "karel"},
		}},
		RoundReqMet: map[string]bool{"me": true, "karel": true},
		GameScores:  map[string][]int{},
		TotalScores: map[string]int{},
	}
}

func TestSwapJokerTakesOnThePlayItThisTurnDebt(t *testing.T) {
	st := reclaimState([]string{"7C", "KS"})
	ns, err := ValidateSwapJoker(st, "me", "m1", "7C")
	if err != nil {
		t.Fatalf("swap: %v", err)
	}
	if got := ns.JokersReclaimedPendingMeld; len(got) != 1 || got[0] != "JOKER1" {
		t.Fatalf("expected the swapped joker on the books, got %v", got)
	}

	// The debt blocks the discard that would end the turn.
	_, _, err = ValidateDiscard(cloneState(ns), "me", "KS", nil)
	re, ok := err.(RulesError)
	if !ok || re.Code != ErrReclaimedJokerNotMelded {
		t.Fatalf("expected RECLAIMED_JOKER_NOT_MELDED, got %#v", err)
	}

	// Laying the joker off pays the debt, and the discard goes through.
	ns2, err := ValidateLayOff(cloneState(ns), "me", "m2", []string{"JOKER1"}, "")
	if err != nil {
		t.Fatalf("laying the joker off: %v", err)
	}
	if len(ns2.JokersReclaimedPendingMeld) != 0 {
		t.Fatalf("expected the debt paid, got %v", ns2.JokersReclaimedPendingMeld)
	}
	ns3, _, err := ValidateDiscard(ns2, "me", "KS", nil)
	if err != nil {
		t.Fatalf("discard after paying the debt: %v", err)
	}
	if ns3.JokersReclaimedPendingMeld != nil {
		t.Fatalf("expected a clean slate after the turn ended, got %v", ns3.JokersReclaimedPendingMeld)
	}
}

func TestLayOffReclaimTakesOnTheDebtAndItsUndoReturnsIt(t *testing.T) {
	st := reclaimState([]string{"7C", "4C", "KS"})
	// 4C+7C onto the run: 7C takes the joker's place, 4C extends the front.
	ns, err := ValidateLayOff(st, "me", "m1", []string{"4C", "7C"}, "")
	if err != nil {
		t.Fatalf("lay off: %v", err)
	}
	if got := ns.JokersReclaimedPendingMeld; len(got) != 1 || got[0] != "JOKER1" {
		t.Fatalf("expected the freed joker on the books, got %v", got)
	}
	if _, _, err := ValidateDiscard(cloneState(ns), "me", "KS", nil); err == nil {
		t.Fatal("expected the discard refused while the joker debt stands")
	}

	// Undoing the lay-off puts the joker back in the meld and wipes the debt.
	undone, err := ValidateUndoLayOff(ns, "me")
	if err != nil {
		t.Fatalf("undo lay off: %v", err)
	}
	if len(undone.JokersReclaimedPendingMeld) != 0 {
		t.Fatalf("expected the debt gone after undo, got %v", undone.JokersReclaimedPendingMeld)
	}
	if _, _, err := ValidateDiscard(undone, "me", "KS", nil); err != nil {
		t.Fatalf("discard after undoing the take: %v", err)
	}
}

func TestLayMeldSpendsTheReclaimedJokerAndItsUndoRestoresTheDebt(t *testing.T) {
	st := reclaimState([]string{"7C", "QS", "QD", "KS"})
	ns, err := ValidateSwapJoker(st, "me", "m1", "7C")
	if err != nil {
		t.Fatalf("swap: %v", err)
	}

	ns2, _, _, err := ValidateMeldAction(ns, "me", []string{"QS", "QD", "JOKER1"})
	if err != nil {
		t.Fatalf("melding with the reclaimed joker: %v", err)
	}
	if len(ns2.JokersReclaimedPendingMeld) != 0 {
		t.Fatalf("expected the meld to pay the debt, got %v", ns2.JokersReclaimedPendingMeld)
	}
	if _, _, err := ValidateDiscard(cloneState(ns2), "me", "KS", nil); err != nil {
		t.Fatalf("discard after paying the debt: %v", err)
	}

	// Undoing that meld puts the joker back in hand — owing again.
	undone, err := ValidateUndoLayMeld(ns2, "me")
	if err != nil {
		t.Fatalf("undo lay meld: %v", err)
	}
	if got := undone.JokersReclaimedPendingMeld; len(got) != 1 || got[0] != "JOKER1" {
		t.Fatalf("expected the debt restored by the undo, got %v", got)
	}
	if _, _, err := ValidateDiscard(undone, "me", "KS", nil); err == nil {
		t.Fatal("expected the discard refused again after the undo restored the debt")
	}
}

func TestUndoTurnWipesTheJokerDebtWithEverythingElse(t *testing.T) {
	st := reclaimState([]string{"7C", "KS"})
	st.TurnMeldSnapshot = snapshotTurnMeld(st, "me")
	ns, err := ValidateSwapJoker(st, "me", "m1", "7C")
	if err != nil {
		t.Fatalf("swap: %v", err)
	}
	undone, err := ValidateUndoTurn(ns, "me")
	if err != nil {
		t.Fatalf("undo turn: %v", err)
	}
	if len(undone.JokersReclaimedPendingMeld) != 0 {
		t.Fatalf("expected no debt after undoing the turn, got %v", undone.JokersReclaimedPendingMeld)
	}
	if got := undone.Melds["karel"][0]; len(got) != 4 || got[2] != "JOKER1" {
		t.Fatalf("expected the joker back in karel's run, got %v", got)
	}
	if _, _, err := ValidateDiscard(undone, "me", "KS", nil); err != nil {
		t.Fatalf("discard after undoing the turn: %v", err)
	}
}

func TestReclaimDebtIsNotEnforcedWhenTheHouseRuleIsOff(t *testing.T) {
	st := reclaimState([]string{"7C", "KS"})
	st.Rules.JokerReclaimMustPlay = false
	ns, err := ValidateSwapJoker(st, "me", "m1", "7C")
	if err != nil {
		t.Fatalf("swap: %v", err)
	}
	// Still tracked — the client shows where the joker came from — but the
	// discard is free to end the turn with it in hand.
	if len(ns.JokersReclaimedPendingMeld) != 1 {
		t.Fatalf("expected the take still tracked, got %v", ns.JokersReclaimedPendingMeld)
	}
	ns2, _, err := ValidateDiscard(ns, "me", "KS", nil)
	if err != nil {
		t.Fatalf("discard with the rule off: %v", err)
	}
	if ns2.JokersReclaimedPendingMeld != nil {
		t.Fatalf("expected the ledger cleared when the turn ended, got %v", ns2.JokersReclaimedPendingMeld)
	}
}

func TestGoingOutDischargesTheJokerDebt(t *testing.T) {
	// The swap leaves "me" holding only the reclaimed joker: the discard that
	// empties an already-down hand ends the deal, the same carve-out the
	// joker-discard ban makes, so it is not held hostage to the debt.
	st := reclaimState([]string{"7C"})
	ns, err := ValidateSwapJoker(st, "me", "m1", "7C")
	if err != nil {
		t.Fatalf("swap: %v", err)
	}
	ns2, goOut, err := ValidateDiscard(ns, "me", "JOKER1", nil)
	if err != nil {
		t.Fatalf("the going-out discard: %v", err)
	}
	if !goOut {
		t.Fatal("expected the discard to go out")
	}
	if len(ns2.Hands["me"]) != 0 {
		t.Fatalf("expected an empty hand, got %v", ns2.Hands["me"])
	}
}

func TestSwapJokerStaysATableMoveForPlayersNotDown(t *testing.T) {
	// A player who has not come out cannot fish a joker out of an opponent's
	// meld — the swap is refused with the same explanation lay_off gives.
	st := reclaimState([]string{"7C", "KS"})
	st.RoundReqMet["me"] = false
	_, err := ValidateSwapJoker(st, "me", "m1", "7C")
	re, ok := err.(RulesError)
	if !ok || re.Code != ErrRoundReqNotMet {
		t.Fatalf("expected ROUND_REQ_NOT_MET, got %#v", err)
	}
	// And the caller's state must be untouched by the refusal.
	if got := st.Melds["karel"][0]; len(got) != 4 || got[2] != "JOKER1" {
		t.Fatalf("a refused swap disturbed the table: %v", got)
	}
	if len(st.Hands["me"]) != 2 {
		t.Fatalf("a refused swap disturbed the hand: %v", st.Hands["me"])
	}
}

func TestSingleCardDragReclaimAlsoTakesOnTheDebt(t *testing.T) {
	// The drag-and-drop shortcut routes a single natural through the engine
	// as a lay_off and converts it to a swap — the debt must follow that
	// path too, or the rule would be dodgeable by dragging.
	st := reclaimState([]string{"7C", "KS"})
	out, err := ApplyAction(st, "me", Action{Type: ActionLayOff, MeldID: "m1", Card: "7C"})
	if err != nil {
		t.Fatalf("drag lay-off: %v", err)
	}
	if got := out.State.JokersReclaimedPendingMeld; len(got) != 1 || got[0] != "JOKER1" {
		t.Fatalf("expected the debt taken on via the drag shortcut, got %v", got)
	}
	if len(out.Events) == 0 || out.Events[0].Type != "joker_swapped" {
		t.Fatalf("expected a joker_swapped event, got %v", out.Events)
	}
}
