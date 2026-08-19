package rules

import "testing"

func TestValidateSet_WildRatio(t *testing.T) {
	_, err := validateSet([]string{"7H", "JOKER1", "JOKER2"})
	if err == nil {
		t.Fatalf("expected too many wilds error")
	}
}

func TestValidateRun_AdjacentWildsRejected(t *testing.T) {
	// Ranks 4 and 7 with two consecutive gap fillers (both wild) => invalid.
	_, err := validateRun([]string{"4H", "7H", "JOKER1", "JOKER2"})
	if err == nil {
		t.Fatalf("expected error for invalid run")
	}
	if err == nil {
		t.Fatalf("expected error for invalid run")
	}
}

func TestValidateRun_AceHighAllowed(t *testing.T) {
	// Q-K-A in suit + wildcard filling J => J-Q-K-A (len 4)
	mv, err := validateRun([]string{"QH", "KH", "AH", "JOKER1"})
	if err != nil {
		t.Fatalf("expected valid run, got %v", err)
	}
	if mv.Type != MeldRun {
		t.Fatalf("expected run")
	}
}

func TestValidateRun_AceBridgeRejected(t *testing.T) {
	// K-A-2 bridge is invalid (cannot wrap).
	_, err := validateRun([]string{"KH", "AH", "2H", "3H"})
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
	ns, _, _, err := ValidateDraw(st, "p1", DrawFromDeck)
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
	_, _, _, err := ValidateDraw(st, "p1", DrawFromDeck)
	if err == nil {
		t.Fatalf("expected error")
	}
	re, ok := err.(RulesError)
	if !ok || re.Code != ErrNoCardsLeft {
		t.Fatalf("expected ErrNoCardsLeft got %#v", err)
	}
}

func TestEndRound_ScoringAndAdvance(t *testing.T) {
	st := GameState{
		Status:      StatusActive,
		Round:       1,
		Phase:       PhaseDiscard,
		TurnOrder:   []string{"p1", "p2"},
		Hands:       map[string][]string{"p1": {"KH"}, "p2": {}},
		RoundReqMet: map[string]bool{"p1": true, "p2": false},
		DeckSeed:    123,
		RoundScores: map[string][]int{"p1": {}, "p2": {}},
		TotalScores: map[string]int{"p1": 0, "p2": 0},
	}

	next, err := EndRound(st, "p2")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if next.Round != 2 {
		t.Fatalf("expected round=2 got %d", next.Round)
	}
	if next.Status != StatusActive {
		t.Fatalf("expected active status")
	}
	// p2 went out, so p2 score = 0; p1 has KH remaining => 10 points.
	if len(next.RoundScores["p1"]) != 1 || next.RoundScores["p1"][0] != 10 {
		t.Fatalf("expected p1 round score 10 got %#v", next.RoundScores["p1"])
	}
	if len(next.RoundScores["p2"]) != 1 || next.RoundScores["p2"][0] != 0 {
		t.Fatalf("expected p2 round score 0 got %#v", next.RoundScores["p2"])
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
	_, _, _, err := ValidateDraw(st, "p1", DrawFromDiscard)
	if err == nil {
		t.Fatalf("expected discard draw to be locked before round 3")
	}
	re, ok := err.(RulesError)
	if !ok || re.Code != ErrDiscardLocked {
		t.Fatalf("expected ErrDiscardLocked got %#v", err)
	}
	// Drawing from the deck must still work while discard is locked.
	if _, _, _, err := ValidateDraw(st, "p1", DrawFromDeck); err != nil {
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
	if _, _, _, err := ValidateDraw(st, "p1", DrawFromDiscard); err != nil {
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
	if _, _, _, err := ValidateDraw(st, "p1", DrawFromDiscard); err != nil {
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

func TestValidateDraw_DeckLockedBeforeMinRound(t *testing.T) {
	st := GameState{
		Status:           StatusActive,
		Phase:            PhaseDraw,
		Round:            2,
		DrawPile:         []string{"2H"},
		DiscardPile:      []string{"7H"},
		Hands:            map[string][]string{"p1": {}},
		CurrentTurn:      "p1",
		DeckDrawMinRound: 3,
	}
	_, _, _, err := ValidateDraw(st, "p1", DrawFromDeck)
	if err == nil {
		t.Fatalf("expected deck draw to be locked before round 3")
	}
	re, ok := err.(RulesError)
	if !ok || re.Code != ErrDeckLocked {
		t.Fatalf("expected ErrDeckLocked got %#v", err)
	}
	// Discard draw must still work while the deck is locked.
	if _, _, _, err := ValidateDraw(st, "p1", DrawFromDiscard); err != nil {
		t.Fatalf("expected discard draw to succeed, got %v", err)
	}
}

func TestValidateDraw_DeckAllowedAtMinRound(t *testing.T) {
	st := GameState{
		Status:           StatusActive,
		Phase:            PhaseDraw,
		Round:            3,
		DrawPile:         []string{"2H"},
		DiscardPile:      []string{"7H"},
		Hands:            map[string][]string{"p1": {}},
		CurrentTurn:      "p1",
		DeckDrawMinRound: 3,
	}
	if _, _, _, err := ValidateDraw(st, "p1", DrawFromDeck); err != nil {
		t.Fatalf("expected deck draw to be allowed at round 3, got %v", err)
	}
}

// If both locks are configured to overlap on the same round, neither draw
// source would otherwise be legal — a genuine deadlock (the turn could never
// advance past PhaseDraw). Whichever source is actually needed must still
// work.
func TestValidateDraw_BothLocksOverlap_DeckStillWorksWhenDiscardEmpty(t *testing.T) {
	st := GameState{
		Status:              StatusActive,
		Phase:               PhaseDraw,
		Round:               1,
		DrawPile:            []string{"2H"},
		DiscardPile:         []string{}, // empty regardless of the lock
		Hands:               map[string][]string{"p1": {}},
		CurrentTurn:         "p1",
		DiscardDrawMinRound: 3,
		DeckDrawMinRound:    3,
	}
	if _, _, _, err := ValidateDraw(st, "p1", DrawFromDeck); err != nil {
		t.Fatalf("expected deck draw to work as a fallback when discard is unavailable, got %v", err)
	}
}

func TestValidateDraw_BothLocksOverlap_DiscardStillWorksWhenNonEmpty(t *testing.T) {
	st := GameState{
		Status:              StatusActive,
		Phase:               PhaseDraw,
		Round:               1,
		DrawPile:            []string{"2H"},
		DiscardPile:         []string{"7H"},
		Hands:               map[string][]string{"p1": {}},
		CurrentTurn:         "p1",
		DiscardDrawMinRound: 3,
		DeckDrawMinRound:    3,
	}
	if _, _, _, err := ValidateDraw(st, "p1", DrawFromDiscard); err != nil {
		t.Fatalf("expected discard draw to work even though the deck is also locked, got %v", err)
	}
}

