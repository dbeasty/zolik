package rules

import "testing"

func TestValidateSet_WildRatio(t *testing.T) {
	_, err := validateSet([]string{"7H", "JOKER1", "JOKER2"}, 3)
	if err == nil {
		t.Fatalf("expected too many wilds error")
	}
}

func TestValidateSet_DuplicateSuitRejected(t *testing.T) {
	// Even with two physical decks in play, a single set may not reuse a suit.
	_, err := validateSet([]string{"5D", "5C", "5C"}, 3)
	if err == nil {
		t.Fatalf("expected error for duplicate suit within a set")
	}
}

func TestValidateSet_DistinctSuitsAllowed(t *testing.T) {
	mv, err := validateSet([]string{"5D", "5C", "5H"}, 3)
	if err != nil {
		t.Fatalf("expected valid set, got %v", err)
	}
	if mv.Type != MeldSet {
		t.Fatalf("expected set")
	}
}

func TestValidateSet_AceIsNotWild(t *testing.T) {
	// Two natural queens plus an ace is not "three queens" — an ace is a
	// real rank, not a stand-in for a third queen. Only jokers are wild
	// in a set.
	_, err := validateSet([]string{"QH", "QD", "AC"}, 3)
	if err == nil {
		t.Fatalf("expected error: an ace cannot substitute for a third queen in a set")
	}
}

func TestValidateSet_JokerStillWild(t *testing.T) {
	mv, err := validateSet([]string{"QH", "QD", "JOKER1"}, 3)
	if err != nil {
		t.Fatalf("expected joker to complete the set, got %v", err)
	}
	if mv.WildCount != 1 {
		t.Fatalf("expected WildCount 1, got %d", mv.WildCount)
	}
}

func TestOrderMeldForDisplay_RunSortsAscending(t *testing.T) {
	cards := []string{"6H", "8H", "7H"}
	mv, err := validateRun(cards, 3)
	if err != nil {
		t.Fatalf("expected valid run, got %v", err)
	}
	got := OrderMeldForDisplay(cards, mv)
	want := []string{"6H", "7H", "8H"}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected %v, got %v", want, got)
		}
	}
}

func TestValidateMeldAction_StoresRunInSortedOrder(t *testing.T) {
	p := "p1"
	st := baseActiveState(2, p) // Game 2: one set, one run — a run contributes.
	st.Hands[p] = []string{"6H", "9H", "8H", "7H", "2S"}

	st, meldID, _, err := ValidateMeldAction(st, p, []string{"6H", "9H", "8H", "7H"})
	if err != nil {
		t.Fatalf("expected valid meld, got %v", err)
	}
	_ = meldID
	got := st.Melds[p][0]
	want := []string{"6H", "7H", "8H", "9H"}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected run stored in sorted order %v, got %v", want, got)
		}
	}
}

func TestOrderMeldForDisplay_SetSortsBySuit(t *testing.T) {
	cards := []string{"QS", "QD", "QH"}
	mv, err := validateSet(cards, 3)
	if err != nil {
		t.Fatalf("expected valid set, got %v", err)
	}
	got := OrderMeldForDisplay(cards, mv)
	want := []string{"QD", "QH", "QS"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected %v, got %v", want, got)
		}
	}
}

func TestValidateRun_AdjacentWildsRejected(t *testing.T) {
	// Ranks 4 and 7 with two consecutive gap fillers (both wild) => invalid.
	_, err := validateRun([]string{"4H", "7H", "JOKER1", "JOKER2"}, 4)
	if err == nil {
		t.Fatalf("expected error for invalid run")
	}
	if err == nil {
		t.Fatalf("expected error for invalid run")
	}
}

func TestValidateRun_AceHighAllowed(t *testing.T) {
	// Q-K-A in suit + wildcard filling J => J-Q-K-A (len 4)
	mv, err := validateRun([]string{"QH", "KH", "AH", "JOKER1"}, 4)
	if err != nil {
		t.Fatalf("expected valid run, got %v", err)
	}
	if mv.Type != MeldRun {
		t.Fatalf("expected run")
	}
}

func TestValidateRun_AceBridgeRejected(t *testing.T) {
	// K-A-2 bridge is invalid (cannot wrap).
	_, err := validateRun([]string{"KH", "AH", "2H", "3H"}, 4)
	if err == nil {
		t.Fatalf("expected ace bridge rejection")
	}
}

func TestEnsureDrawPile_ReshuffleAndCount(t *testing.T) {
	st := GameState{
		Status:         StatusActive,
		Phase:          PhaseDraw,
		DrawPile:       nil,
		DiscardPile:    []string{"7H", "8H"},
		ReshuffleCount: 0,
		Hands:          map[string][]string{"p1": {}},
		CurrentTurn:    "p1",
	}
	ns, _, _, err := ValidateDraw(st, "p1", DrawFromDeck, "")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if ns.ReshuffleCount != 1 {
		t.Fatalf("expected reshuffleCount=1 got %d", ns.ReshuffleCount)
	}
	if len(ns.DiscardPile) != 0 {
		t.Fatalf("expected discard emptied")
	}
}

func TestEnsureDrawPile_EmptyBothErrors(t *testing.T) {
	st := GameState{
		Status:      StatusActive,
		Phase:       PhaseDraw,
		DrawPile:    nil,
		DiscardPile: nil,
		Hands:       map[string][]string{"p1": {}},
		CurrentTurn: "p1",
	}
	_, _, _, err := ValidateDraw(st, "p1", DrawFromDeck, "")
	if err == nil {
		t.Fatalf("expected error")
	}
	re, ok := err.(RulesError)
	if !ok || re.Code != ErrNoCardsLeft {
		t.Fatalf("expected ErrNoCardsLeft got %#v", err)
	}
}

func TestEndGame_ScoringAndAdvance(t *testing.T) {
	st := GameState{
		Status:      StatusActive,
		GameNumber:  1,
		Phase:       PhaseDiscard,
		TurnOrder:   []string{"p1", "p2"},
		Hands:       map[string][]string{"p1": {"KH"}, "p2": {}},
		RoundReqMet: map[string]bool{"p1": true, "p2": false},
		DeckSeed:    123,
		GameScores:  map[string][]int{"p1": {}, "p2": {}},
		TotalScores: map[string]int{"p1": 0, "p2": 0},
	}

	next, err := EndGame(st, "p2")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if next.GameNumber != 2 {
		t.Fatalf("expected game=2 got %d", next.GameNumber)
	}
	if next.Status != StatusActive {
		t.Fatalf("expected active status")
	}
	// p2 went out, so p2 score = 0; p1 has KH remaining => 10 points.
	if len(next.GameScores["p1"]) != 1 || next.GameScores["p1"][0] != 10 {
		t.Fatalf("expected p1 game score 10 got %#v", next.GameScores["p1"])
	}
	if len(next.GameScores["p2"]) != 1 || next.GameScores["p2"][0] != 0 {
		t.Fatalf("expected p2 game score 0 got %#v", next.GameScores["p2"])
	}
	if next.TotalScores["p1"] != 10 {
		t.Fatalf("expected total score 10 got %d", next.TotalScores["p1"])
	}
}

func TestValidateDraw_DiscardLockedBeforeMinRound(t *testing.T) {
	st := GameState{
		Status:              StatusActive,
		Phase:               PhaseDraw,
		Round:               2,
		DrawPile:            []string{"2H"},
		DiscardPile:         []string{"7H"},
		Hands:               map[string][]string{"p1": {}},
		CurrentTurn:         "p1",
		DiscardDrawMinRound: 3,
	}
	_, _, _, err := ValidateDraw(st, "p1", DrawFromDiscard, "")
	if err == nil {
		t.Fatalf("expected discard draw to be locked before round 3")
	}
	re, ok := err.(RulesError)
	if !ok || re.Code != ErrDiscardLocked {
		t.Fatalf("expected ErrDiscardLocked got %#v", err)
	}
	// Drawing from the deck must still work while discard is locked.
	if _, _, _, err := ValidateDraw(st, "p1", DrawFromDeck, ""); err != nil {
		t.Fatalf("expected deck draw to succeed, got %v", err)
	}
}

func TestValidateDraw_DiscardAllowedAtMinRound(t *testing.T) {
	st := GameState{
		Status:              StatusActive,
		Phase:               PhaseDraw,
		Round:               3,
		DrawPile:            []string{"2H"},
		DiscardPile:         []string{"7H"},
		Hands:               map[string][]string{"p1": {}},
		CurrentTurn:         "p1",
		DiscardDrawMinRound: 3,
	}
	if _, _, _, err := ValidateDraw(st, "p1", DrawFromDiscard, ""); err != nil {
		t.Fatalf("expected discard draw to be allowed at round 3, got %v", err)
	}
}

func TestValidateDraw_DiscardUnrestrictedByDefault(t *testing.T) {
	st := GameState{
		Status:      StatusActive,
		Phase:       PhaseDraw,
		Round:       1,
		DrawPile:    []string{"2H"},
		DiscardPile: []string{"7H"},
		Hands:       map[string][]string{"p1": {}},
		CurrentTurn: "p1",
		// DiscardDrawMinRound left at its zero value.
	}
	if _, _, _, err := ValidateDraw(st, "p1", DrawFromDiscard, ""); err != nil {
		t.Fatalf("expected discard draw to be unrestricted by default, got %v", err)
	}
}

func TestValidateDiscard_BlocksIncompleteInitialMeld(t *testing.T) {
	st := GameState{
		Status:            StatusActive,
		Phase:             PhaseMeld,
		Round:             1,
		CurrentTurn:       "p1",
		Hands:             map[string][]string{"p1": {"9S"}},
		DiscardPile:       []string{},
		RoundReqMet:       map[string]bool{"p1": false},
		MeldsLaidThisTurn: 1,
	}
	_, _, err := ValidateDiscard(st, "p1", "9S")
	if err == nil {
		t.Fatalf("expected discard to be blocked with an incomplete initial meld")
	}
	re, ok := err.(RulesError)
	if !ok || re.Code != ErrIncompleteInitialMeld {
		t.Fatalf("expected ErrIncompleteInitialMeld, got %#v", err)
	}
}

func TestValidateDiscard_AllowedWithNoMeldsLaidThisTurn(t *testing.T) {
	st := GameState{
		Status:            StatusActive,
		Phase:             PhaseMeld,
		Round:             1,
		CurrentTurn:       "p1",
		TurnOrder:         []string{"p1", "p2"},
		Hands:             map[string][]string{"p1": {"9S", "4D"}, "p2": {}},
		DiscardPile:       []string{},
		RoundReqMet:       map[string]bool{"p1": false, "p2": false},
		MeldsLaidThisTurn: 0,
	}
	if _, _, err := ValidateDiscard(st, "p1", "9S"); err != nil {
		t.Fatalf("expected discard with no melds laid this turn to succeed, got %v", err)
	}
}

func TestValidateDiscard_AllowedOnceRoundReqMet(t *testing.T) {
	st := GameState{
		Status:            StatusActive,
		Phase:             PhaseMeld,
		Round:             1,
		CurrentTurn:       "p1",
		TurnOrder:         []string{"p1", "p2"},
		Hands:             map[string][]string{"p1": {"9S"}, "p2": {}},
		DiscardPile:       []string{},
		RoundReqMet:       map[string]bool{"p1": true, "p2": false},
		MeldsLaidThisTurn: 2,
	}
	if _, _, err := ValidateDiscard(st, "p1", "9S"); err != nil {
		t.Fatalf("expected discard to succeed once round requirement is met, got %v", err)
	}
}

func TestValidateDraw_DiscardPickupSetsPendingMeldBeforeGoingDown(t *testing.T) {
	st := GameState{
		Status:      StatusActive,
		Phase:       PhaseDraw,
		Round:       3,
		DrawPile:    []string{"2H"},
		DiscardPile: []string{"9S"},
		Hands:       map[string][]string{"p1": {}},
		CurrentTurn: "p1",
		RoundReqMet: map[string]bool{"p1": false},
	}
	ns, card, _, err := ValidateDraw(st, "p1", DrawFromDiscard, "")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if card != "9S" || ns.DiscardDrawnCardPendingMeld != "9S" {
		t.Fatalf("expected pending-meld obligation on 9S, got card=%q pending=%q", card, ns.DiscardDrawnCardPendingMeld)
	}
}

func TestValidateDraw_DiscardPickupNoObligationOnceDown(t *testing.T) {
	st := GameState{
		Status:      StatusActive,
		Phase:       PhaseDraw,
		Round:       3,
		DrawPile:    []string{"2H"},
		DiscardPile: []string{"9S"},
		Hands:       map[string][]string{"p1": {}},
		CurrentTurn: "p1",
		RoundReqMet: map[string]bool{"p1": true},
	}
	ns, _, _, err := ValidateDraw(st, "p1", DrawFromDiscard, "")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if ns.DiscardDrawnCardPendingMeld != "" {
		t.Fatalf("expected no pending obligation once already down, got %q", ns.DiscardDrawnCardPendingMeld)
	}
}

func TestValidateDiscard_BlocksWhenPickedUpDiscardCardStillUnmelded(t *testing.T) {
	st := GameState{
		Status:                      StatusActive,
		Phase:                       PhaseMeld,
		Round:                       3,
		CurrentTurn:                 "p1",
		Hands:                       map[string][]string{"p1": {"9S", "4D"}},
		DiscardPile:                 []string{},
		RoundReqMet:                 map[string]bool{"p1": false},
		DiscardDrawnCardPendingMeld: "9S",
	}
	// Trying to discard a *different* card while 9S (the discard pickup)
	// remains unmelded must be rejected.
	_, _, err := ValidateDiscard(st, "p1", "4D")
	if err == nil {
		t.Fatalf("expected discard to be blocked while the picked-up card is unmelded")
	}
	re, ok := err.(RulesError)
	if !ok || re.Code != ErrDiscardCardNotMelded {
		t.Fatalf("expected ErrDiscardCardNotMelded, got %#v", err)
	}
}

func TestValidateMeldAction_UsingPendingDiscardCardClearsObligation(t *testing.T) {
	st := GameState{
		Status:                      StatusActive,
		Phase:                       PhaseMeld,
		GameNumber:                  1,
		CurrentTurn:                 "p1",
		TurnOrder:                   []string{"p1", "p2"},
		Hands:                       map[string][]string{"p1": {"9S", "9D", "9C", "KH", "KD", "KC"}, "p2": {}},
		Melds:                       map[string][][]string{},
		MeldMeta:                    map[string][]MeldInfo{},
		RoundReqMet:                 map[string]bool{"p1": false, "p2": false},
		InitialMeldMinimum:          0,
		DiscardDrawnCardPendingMeld: "9S",
	}
	ns, _, _, err := ValidateMeldAction(st, "p1", []string{"9S", "9D", "9C"})
	if err != nil {
		t.Fatalf("unexpected err laying the meld containing the pending card: %v", err)
	}
	if ns.DiscardDrawnCardPendingMeld != "" {
		t.Fatalf("expected pending obligation cleared after melding 9S, got %q", ns.DiscardDrawnCardPendingMeld)
	}
}

func TestValidateLayOff_BlockedBeforeOwnRoundReqMet(t *testing.T) {
	st := GameState{
		Status:      StatusActive,
		Phase:       PhaseMeld,
		Round:       1,
		CurrentTurn: "p1",
		Hands:       map[string][]string{"p1": {"7H"}},
		Melds:       map[string][][]string{"p2": {{"7D", "7C", "7S"}}},
		MeldMeta:    map[string][]MeldInfo{"p2": {{MeldID: "meld_1", Type: MeldSet, OwnerID: "p2"}}},
		RoundReqMet: map[string]bool{"p1": false, "p2": true},
	}
	_, err := ValidateLayOff(st, "p1", "meld_1", "7H")
	if err == nil {
		t.Fatalf("expected lay-off to be blocked before p1 has met their own round requirement")
	}
	re, ok := err.(RulesError)
	if !ok || re.Code != ErrRoundReqNotMet {
		t.Fatalf("expected ErrRoundReqNotMet, got %#v", err)
	}
}

func TestValidateLayOff_AllowedOnceOwnRoundReqMet(t *testing.T) {
	st := GameState{
		Status:      StatusActive,
		Phase:       PhaseMeld,
		Round:       1,
		CurrentTurn: "p1",
		Hands:       map[string][]string{"p1": {"7H", "4D"}},
		Melds:       map[string][][]string{"p2": {{"7D", "7C", "7S"}}},
		MeldMeta:    map[string][]MeldInfo{"p2": {{MeldID: "meld_1", Type: MeldSet, OwnerID: "p2"}}},
		RoundReqMet: map[string]bool{"p1": true, "p2": true},
	}
	if _, err := ValidateLayOff(st, "p1", "meld_1", "7H"); err != nil {
		t.Fatalf("expected lay-off to succeed once p1 has met their own round requirement, got %v", err)
	}
}
