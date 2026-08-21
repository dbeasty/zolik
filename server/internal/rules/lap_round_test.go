package rules

import "testing"

// discardOnce simulates one player's turn ending in a discard, using a
// throwaway hand card so hands never empty (go-out is not under test here).
func discardOnce(t *testing.T, st GameState, playerID string) GameState {
	t.Helper()
	st.CurrentTurn = playerID
	st.Phase = PhaseMeld
	ns, _, err := ValidateDiscard(st, playerID, st.Hands[playerID][0])
	if err != nil {
		t.Fatalf("discard by %s: unexpected err: %v", playerID, err)
	}
	return ns
}

func lapTestState(turnOrder []string, dealStarter string) GameState {
	hands := map[string][]string{}
	for _, pid := range turnOrder {
		hands[pid] = []string{"2H", "3H", "4H", "5H", "6H", "7H", "8H", "9H"}
	}
	return GameState{
		Status:        StatusActive,
		GameNumber:    1,
		Phase:         PhaseMeld,
		TurnOrder:     turnOrder,
		DealStarterID: dealStarter,
		CurrentTurn:   dealStarter,
		Round:         1,
		Hands:         hands,
		DiscardPile:   []string{},
		RoundReqMet:   map[string]bool{},
	}
}

func TestLapRound_IncrementsOnReturnToDealStarter_TwoPlayers(t *testing.T) {
	st := lapTestState([]string{"p1", "p2"}, "p1")

	st = discardOnce(t, st, "p1")
	if st.Round != 1 {
		t.Fatalf("after p1's turn: expected round=1 got %d", st.Round)
	}
	st = discardOnce(t, st, "p2")
	if st.Round != 2 {
		t.Fatalf("after p2's turn (wraps to p1): expected round=2 got %d", st.Round)
	}
	st = discardOnce(t, st, "p1")
	if st.Round != 2 {
		t.Fatalf("after p1's turn: expected round=2 got %d", st.Round)
	}
	st = discardOnce(t, st, "p2")
	if st.Round != 3 {
		t.Fatalf("after p2's turn (wraps to p1 again): expected round=3 got %d", st.Round)
	}
}

func TestLapRound_IncrementsOnReturnToDealStarter_ThreePlayers(t *testing.T) {
	order := []string{"p1", "p2", "p3"}
	st := lapTestState(order, "p1")

	wantRoundAfter := []int{1, 1, 2, 2, 2, 3}
	turns := []string{"p1", "p2", "p3", "p1", "p2", "p3"}
	for i, pid := range turns {
		st = discardOnce(t, st, pid)
		if st.Round != wantRoundAfter[i] {
			t.Fatalf("turn %d (%s): expected round=%d got %d", i, pid, wantRoundAfter[i], st.Round)
		}
	}
}

func TestLapRound_DealStarterNotFirstInTurnOrder(t *testing.T) {
	// DealStarterID need not be TurnOrder[0] — e.g. the previous deal's
	// winner starts the next deal regardless of seating order.
	order := []string{"p1", "p2", "p3"}
	st := lapTestState(order, "p2")
	st.CurrentTurn = "p2"

	st = discardOnce(t, st, "p2")
	if st.Round != 1 {
		t.Fatalf("after p2's turn: expected round=1 got %d", st.Round)
	}
	st = discardOnce(t, st, "p3")
	if st.Round != 1 {
		t.Fatalf("after p3's turn: expected round=1 got %d", st.Round)
	}
	st = discardOnce(t, st, "p1")
	if st.Round != 2 {
		t.Fatalf("after p1's turn (wraps to p2, the deal starter): expected round=2 got %d", st.Round)
	}
}

func TestLapRound_ResetsAndStartsFreshEachDeal(t *testing.T) {
	st := lapTestState([]string{"p1", "p2"}, "p1")
	st = discardOnce(t, st, "p1")
	st = discardOnce(t, st, "p2") // Round now 2

	next, err := StartNextGame(st, "p2")
	if err != nil {
		t.Fatalf("StartNextGame: unexpected err: %v", err)
	}
	if next.Round != 1 {
		t.Fatalf("expected new deal to reset Round to 1, got %d", next.Round)
	}
	if next.DealStarterID != "p2" {
		t.Fatalf("expected DealStarterID=p2 got %q", next.DealStarterID)
	}
}

func TestDiscardDrawMinRound_GatesOnLapRoundNotGameNumber(t *testing.T) {
	// Regression guard: the discard-pile lock must key off the lap Round,
	// not GameNumber, even deep into a late GameNumber.
	st := lapTestState([]string{"p1", "p2"}, "p1")
	st.GameNumber = 6
	st.Round = 1
	st.Rules = ProfileContinental // locks discard pickup until lap round 3
	st.Phase = PhaseDraw
	st.DrawPile = []string{"2C"}
	st.DiscardPile = []string{"7H"}

	_, _, _, err := ValidateDraw(st, "p1", DrawFromDiscard, "")
	re, ok := err.(RulesError)
	if !ok || re.Code != ErrDiscardLocked {
		t.Fatalf("expected ErrDiscardLocked while lap Round < DiscardDrawMinRound, got %#v", err)
	}
}
