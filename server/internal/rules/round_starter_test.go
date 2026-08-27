package rules

import "testing"

// starterTestConfig is a fixed-length match config that lets a test drive a
// deal to completion without tripping the contract-floor gating other
// suites cover elsewhere: no set/run requirement, no clean-run requirement,
// and a match long enough that the deal under test never happens to be the
// match's last one.
func starterTestConfig(mode DealStarterMode) RulesConfig {
	// Based on ProfileContinental rather than built from a zero value: an
	// all-zero RulesConfig reads as "unset" to effectiveRules (MinRunSize==0
	// is its tell) and falls back to ProfileContinental wholesale, silently
	// discarding every override below it.
	cfg := ProfileContinental
	cfg.Profile = "custom"
	cfg.StaticContract = ContractRequirement{}
	cfg.FixedDealCount = 100
	cfg.MatchEndMode = MatchEndAfterDeals
	cfg.DealStarter = mode
	return cfg
}

// starterTestState is a three-seat table ready for its deal to end: the
// scored deal's own leader is p1, and p3 — not p1's neighbour — is the one
// about to go out, so a rotate pick and a winner pick land on two different
// players and a test can tell which rule actually ran.
func starterTestState(cfg RulesConfig) GameState {
	turnOrder := []string{"p1", "p2", "p3"}
	hands := map[string][]string{}
	roundReqMet := map[string]bool{}
	scores := map[string][]int{}
	totals := map[string]int{}
	for _, pid := range turnOrder {
		hands[pid] = []string{"2H", "3H"}
		roundReqMet[pid] = false
		scores[pid] = []int{}
		totals[pid] = 0
	}
	hands["p3"] = []string{"9S"}
	roundReqMet["p3"] = true

	return GameState{
		Status:        StatusActive,
		Rules:         cfg,
		GameNumber:    1,
		Phase:         PhaseMeld,
		CurrentTurn:   "p3",
		TurnOrder:     turnOrder,
		DealStarterID: "p1",
		Round:         1,
		Hands:         hands,
		Melds:         map[string][][]string{},
		MeldMeta:      map[string][]MeldInfo{},
		RoundReqMet:   roundReqMet,
		DrawPile:      []string{"2C"},
		DiscardPile:   []string{},
		DeckSeed:      42,
		GameScores:    scores,
		TotalScores:   totals,
	}
}

// TestNextDealStarter_Rotate picks the seat after whoever led the deal that
// just ended, regardless of who won it.
func TestNextDealStarter_Rotate(t *testing.T) {
	st := starterTestState(starterTestConfig(DealStarterRotate))
	if got := NextDealStarter(st, st.Rules, "p3"); got != "p2" {
		t.Fatalf("rotate should hand the lead to p2 (after p1), got %q", got)
	}
}

// TestNextDealStarter_RotateWraps checks the rotation wraps back to the
// start of the table rather than running off the end of TurnOrder.
func TestNextDealStarter_RotateWraps(t *testing.T) {
	st := starterTestState(starterTestConfig(DealStarterRotate))
	st.DealStarterID = "p3"
	if got := NextDealStarter(st, st.Rules, "p3"); got != "p1" {
		t.Fatalf("rotate should wrap from p3 back to p1, got %q", got)
	}
}

// TestNextDealStarter_Winner reproduces the original behaviour under the
// opt-in house rule: whoever went out leads next, not the rotation.
func TestNextDealStarter_Winner(t *testing.T) {
	st := starterTestState(starterTestConfig(DealStarterWinner))
	if got := NextDealStarter(st, st.Rules, "p3"); got != "p3" {
		t.Fatalf("winner mode should hand the lead to the winner p3, got %q", got)
	}
}

// TestEndGameWithEvents_Rotate drives a real go-out through ApplyAction and
// checks the deal that gets dealt next actually opens on the rotated seat,
// not on the player who just won it.
func TestEndGameWithEvents_Rotate(t *testing.T) {
	st := starterTestState(starterTestConfig(DealStarterRotate))

	outcome, err := ApplyAction(st, "p3", Action{Type: ActionDiscard, Card: "9S"})
	if err != nil {
		t.Fatalf("discard: %v", err)
	}
	ns := outcome.State
	if ns.DealStarterID != "p2" {
		t.Fatalf("next deal should be led by p2 (after p1), got DealStarterID=%q", ns.DealStarterID)
	}
	if ns.CurrentTurn != "p2" {
		t.Fatalf("next deal's turn should open on p2, got %q", ns.CurrentTurn)
	}
}

// TestEndGameWithEvents_PauseResumesOnRotatedSeat checks the paused path
// agrees with the straight-through one: the intermission remembers the
// rotated seat, not the winner, and hands it to CurrentTurn on resume.
func TestEndGameWithEvents_PauseResumesOnRotatedSeat(t *testing.T) {
	cfg := starterTestConfig(DealStarterRotate)
	cfg.PauseBetweenDeals = true
	st := starterTestState(cfg)

	outcome, err := ApplyAction(st, "p3", Action{Type: ActionDiscard, Card: "9S"})
	if err != nil {
		t.Fatalf("discard: %v", err)
	}
	paused := outcome.State
	if paused.Phase != PhaseIntermission {
		t.Fatalf("expected the table to pause between deals, got phase %q", paused.Phase)
	}
	if paused.PendingDealStarter != "p2" {
		t.Fatalf("paused table should remember p2 as the next starter, got %q", paused.PendingDealStarter)
	}

	resumed, err := ResumeAfterIntermission(paused)
	if err != nil {
		t.Fatalf("ResumeAfterIntermission: %v", err)
	}
	if resumed.CurrentTurn != "p2" {
		t.Fatalf("resumed deal should open on p2, got %q", resumed.CurrentTurn)
	}
	if resumed.DealStarterID != "p2" {
		t.Fatalf("resumed deal's DealStarterID should be p2, got %q", resumed.DealStarterID)
	}
}
