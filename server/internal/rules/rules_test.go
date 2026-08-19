package rules

import "testing"

func TestValidateSet_WildRatio(t *testing.T) {
	_, err := validateSet([]string{"7H", "JOKER1", "JOKER2"})
	if err == nil {
		t.Fatalf("expected too many wilds error")
	}
}

func TestValidateSet_DuplicateSuitRejected(t *testing.T) {
	// Even with two physical decks in play, a single set may not reuse a suit.
	_, err := validateSet([]string{"5D", "5C", "5C"})
	if err == nil {
		t.Fatalf("expected error for duplicate suit within a set")
	}
}

func TestValidateSet_DistinctSuitsAllowed(t *testing.T) {
	mv, err := validateSet([]string{"5D", "5C", "5H"})
	if err != nil {
		t.Fatalf("expected valid set, got %v", err)
	}
	if mv.Type != MeldSet {
		t.Fatalf("expected set")
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

