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

// Regression test: when DiscardDrawMinRound locks the discard pile for the
// current round, the agent must not pick draw-from-discard (the server
// would reject it with DISCARD_LOCKED, and the old heuristic always
// preferred discard whenever the pile was non-empty regardless of the lock).
func TestHeuristicAgent_DrawAction_RespectsDiscardLock(t *testing.T) {
	agent := NewHeuristicAgent("medium")
	aiID := "ai1"

	state := rules.GameState{
		Status:      rules.StatusActive,
		Phase:       rules.PhaseDraw,
		Round:       2,
		CurrentTurn: aiID,
		TurnOrder:   []string{aiID, "p2"},
		Hands:       map[string][]string{aiID: {}, "p2": {}},
		DrawPile:    []string{"2H"},
		DiscardPile: []string{"7H"},
		RoundReqMet: map[string]bool{aiID: false, "p2": false},
		Rules:       rules.ProfileContinental, // locks pickup until lap round 3
	}

	visible := VisibleState{
		Round:       state.Round,
		Phase:       string(state.Phase),
		CurrentTurn: state.CurrentTurn,
		DiscardPile: state.DiscardPile,
		RoundReqMet: state.RoundReqMet,
		Rules:       state.Rules,
	}

	action := agent.ChooseAction(visible, state.Hands[aiID])
	if action.DrawFrom == rules.DrawFromDiscard {
		t.Fatalf("expected agent to avoid the locked discard pile, got action: %+v", action)
	}
	if _, err := rules.ApplyAction(state, aiID, action); err != nil {
		t.Fatalf("expected fallback draw action %+v to be valid, got err: %v", action, err)
	}
}

func TestHeuristicAgent_MeldLayOff_MeldFoundAndValid(t *testing.T) {
	agent := NewHeuristicAgent("medium")
	aiID := "ai1"

	// Round 1 needs 2 sets; the hand holds material for both, so the agent
	// should find a full plan and lay the first meld of it. (A hand with
	// only one set's worth of cards would no longer meld at all — see
	// TestHeuristicAgent_WontStartMeldItCannotFinish.)
	state := rules.GameState{
		Status:      rules.StatusActive,
		Phase:       rules.PhaseMeld,
		GameNumber:  1,
		CurrentTurn: aiID,
		TurnOrder:   []string{aiID, "p2"},
		Hands:       map[string][]string{aiID: {"7H", "7D", "7C", "2H", "2D", "2C", "9S"}, "p2": {}},
		DrawPile:    []string{"2H"},
		DiscardPile: []string{},
		RoundReqMet: map[string]bool{aiID: false, "p2": false},
		// no meld-value floor in this unit test
		Rules: continentalNoFloor(),
	}

	visible := VisibleState{
		GameNumber:  state.GameNumber,
		Phase:       string(state.Phase),
		CurrentTurn: state.CurrentTurn,
		RoundReqMet: state.RoundReqMet,
		Rules:       state.Rules,
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

// Regression test for the "Rita melded with only three twos" report: round 1
// needs 2 sets, but the hand only holds material for 1. The agent must not
// start a meld it can't finish this turn (the server won't let it discard
// afterward until it does) — it should discard instead and wait for a hand
// that can complete the full requirement in one turn.
func TestHeuristicAgent_WontStartMeldItCannotFinish(t *testing.T) {
	agent := NewHeuristicAgent("medium")
	aiID := "ai1"

	state := rules.GameState{
		Status:      rules.StatusActive,
		Phase:       rules.PhaseMeld,
		GameNumber:  1,
		CurrentTurn: aiID,
		TurnOrder:   []string{aiID, "p2"},
		Hands:       map[string][]string{aiID: {"2H", "2D", "2C", "9S", "5H"}, "p2": {}},
		DrawPile:    []string{"3H"},
		DiscardPile: []string{},
		RoundReqMet: map[string]bool{aiID: false, "p2": false},
		Rules:       rules.ProfileContinental, // 35-point meld floor
		GameScores:  map[string][]int{aiID: {}},
		TotalScores: map[string]int{aiID: 0},
	}

	visible := VisibleState{
		GameNumber:  state.GameNumber,
		Phase:       string(state.Phase),
		CurrentTurn: state.CurrentTurn,
		RoundReqMet: state.RoundReqMet,
		Rules:       state.Rules,
	}

	action := agent.ChooseAction(visible, state.Hands[aiID])
	if action.Type == rules.ActionLayMeld {
		t.Fatalf("expected agent to avoid starting an unfinishable meld, got: %+v", action)
	}
	if action.Type != rules.ActionDiscard {
		t.Fatalf("expected discard, got: %+v", action)
	}
}

func TestHeuristicAgent_DiscardAllowedWhenRoundReqMet(t *testing.T) {
	agent := NewHeuristicAgent("medium")
	aiID := "ai1"

	state := rules.GameState{
		Status:      rules.StatusActive,
		Phase:       rules.PhaseMeld,
		GameNumber:  1,
		CurrentTurn: aiID,
		TurnOrder:   []string{aiID},
		Hands:       map[string][]string{aiID: {"KH"}},
		DrawPile:    []string{"2H"},
		DiscardPile: []string{},
		RoundReqMet: map[string]bool{aiID: true},
		Rules:       continentalNoFloor(),
		DeckSeed:    1,
		GameScores:  map[string][]int{aiID: {}},
		TotalScores: map[string]int{aiID: 0},
	}

	visible := VisibleState{
		GameNumber:  state.GameNumber,
		Phase:       string(state.Phase),
		CurrentTurn: state.CurrentTurn,
		RoundReqMet: state.RoundReqMet,
		Rules:       state.Rules,
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
		GameNumber:  1,
		CurrentTurn: aiID,
		TurnOrder:   []string{aiID, "p2"},
		Hands:       map[string][]string{aiID: {"2H", "2D", "2C", "9S"}, "p2": {}},
		DrawPile:    []string{"5H"},
		DiscardPile: []string{},
		Melds:       map[string][][]string{aiID: {{"3H", "3D", "3C"}}},
		MeldMeta: map[string][]rules.MeldInfo{
			aiID: {{MeldID: "meld_1", Type: rules.MeldSet, OwnerID: aiID}},
		},
		RoundReqMet: map[string]bool{aiID: false, "p2": false},
		// 35-point floor: 3H+3D+3C (9) + 2H+2D+2C (6) = 15, below minimum
		Rules:       rules.ProfileContinental,
		DeckSeed:    1,
		GameScores:  map[string][]int{aiID: {}},
		TotalScores: map[string]int{aiID: 0},
	}

	visible := VisibleState{
		GameNumber:  state.GameNumber,
		Phase:       string(state.Phase),
		CurrentTurn: state.CurrentTurn,
		Melds:       state.Melds,
		MeldMeta:    state.MeldMeta,
		RoundReqMet: state.RoundReqMet,
		Rules:       state.Rules,
	}

	action := agent.ChooseAction(visible, state.Hands[aiID])
	if action.Type == rules.ActionLayMeld {
		t.Fatalf("expected agent to avoid the below-minimum meld, got lay_meld action: %+v", action)
	}
	if _, err := rules.ApplyAction(state, aiID, action); err != nil {
		t.Fatalf("expected fallback action %+v to be valid, got err: %v", action, err)
	}
}

// Before going down, picking up the discard obligates melding that exact
// card this turn. The agent should only take it when a full initial-meld
// plan using it actually exists in the resulting hand.
func TestHeuristicAgent_TakesDiscardWhenItCompletesTheInitialMeld(t *testing.T) {
	agent := NewHeuristicAgent("medium")
	aiID := "ai1"

	visible := VisibleState{
		GameNumber:  1, // needs 2 sets
		Phase:       string(rules.PhaseDraw),
		CurrentTurn: aiID,
		DiscardPile: []string{"9C"},
		RoundReqMet: map[string]bool{aiID: false},
		// Continental's deal-1 contract (two sets) with no meld-value floor
		// and no discard-pickup round gate, so the draw choice is the only
		// thing under test.
		Rules: openContinental(),
	}
	// Includes a spare card (2C) so melding both sets still leaves a card to
	// discard afterward (required before round 7).
	hand := []string{"9S", "9D", "5H", "5D", "5C", "2C"}

	action := agent.ChooseAction(visible, hand)
	if action.Type != rules.ActionDrawCard || action.DrawFrom != rules.DrawFromDiscard {
		t.Fatalf("expected agent to take the discard that completes its initial meld, got: %+v", action)
	}
}

func TestHeuristicAgent_SkipsDiscardWhenItWontCompleteTheInitialMeld(t *testing.T) {
	agent := NewHeuristicAgent("medium")
	aiID := "ai1"

	visible := VisibleState{
		GameNumber:  1, // needs 2 sets
		Phase:       string(rules.PhaseDraw),
		CurrentTurn: aiID,
		DiscardPile: []string{"KC"},
		RoundReqMet: map[string]bool{aiID: false},
		// Continental's deal-1 contract (two sets) with no meld-value floor
		// and no discard-pickup round gate, so the draw choice is the only
		// thing under test.
		Rules: openContinental(),
	}
	hand := []string{"9S", "9D", "2H", "3H", "4H"}

	action := agent.ChooseAction(visible, hand)
	if action.Type != rules.ActionDrawCard || action.DrawFrom != rules.DrawFromDeck {
		t.Fatalf("expected agent to prefer the deck when the discard card can't be melded this turn, got: %+v", action)
	}
}

// A joker on a clean run is a legal lay-off (see
// TestZolikClassic_LayOffMayDirtyACleanRun), and the agent takes it: the run
// that put it down keeps counting whether or not it stays clean, so declining
// would just be holding the deal's heaviest penalty card for nothing.
func TestFindLayOffTakesAJokerOntoACleanRun(t *testing.T) {
	cfg := rules.ProfileZolikClassic
	melds := map[string][][]string{"p1": {{"5C", "6C", "7C", "8C"}}}
	meta := map[string][]rules.MeldInfo{
		"p1": {{MeldID: "meld_1", Type: rules.MeldRun, OwnerID: "p1", WildCount: 0}},
	}

	if _, card, ok := findLayOff(meta, melds, []string{"JOKER1", "2S"}, cfg, 1); !ok || card != "JOKER1" {
		t.Fatalf("expected the joker to extend the run, got %q ok=%v", card, ok)
	}

	// A natural extension is still taken.
	_, card, ok := findLayOff(meta, melds, []string{"9C", "2S"}, cfg, 1)
	if !ok || card != "9C" {
		t.Fatalf("expected 9C to extend the run, got %q ok=%v", card, ok)
	}
}

func TestPickSmartDiscard_EasyStillDiscardsBlindly(t *testing.T) {
	cfg := rules.ProfileZolikClassic
	visible := VisibleState{
		Rules: cfg,
		Melds: map[string][][]string{"opponent": {{"5C", "6C", "7C", "8C"}}},
	}
	// 9C extends the opponent's run and is also the higher-penalty card, so
	// even the safety-blind "easy" difficulty happens to pick it here.
	got := pickSmartDiscard([]string{"9C", "2H"}, visible, "ai1", "easy", false)
	if got != "9C" {
		t.Fatalf("expected easy AI to discard the highest-penalty card 9C, got %q", got)
	}
}

func TestPickSmartDiscard_MediumAvoidsFeedingLiveMeld(t *testing.T) {
	cfg := rules.ProfileZolikClassic
	visible := VisibleState{
		Rules: cfg,
		Melds: map[string][][]string{"opponent": {{"5C", "6C", "7C", "8C"}}},
	}
	// 9C (9 pts) extends the opponent's run; 2H (2 pts) is a dead card. A
	// blind highest-points discard would hand over 9C — medium must not.
	got := pickSmartDiscard([]string{"9C", "2H"}, visible, "ai1", "medium", false)
	if got != "2H" {
		t.Fatalf("expected medium AI to avoid feeding the live run, got %q", got)
	}
}

func TestPickSmartDiscard_HardBreaksPointsTiesTowardAlreadyDiscardedRank(t *testing.T) {
	cfg := rules.ProfileZolikClassic
	visible := VisibleState{
		Rules: cfg,
		PlayerDiscards: map[string][]string{
			"human": {"QD"},
		},
	}
	// KH and QC are both safe (no live melds on the table) and tie on
	// points (face cards are all worth 10). The human already discarded a
	// queen (QD), so hard should let go of the other queen rather than a
	// fresh, never-seen king.
	got := pickSmartDiscard([]string{"KH", "QC"}, visible, "ai1", "hard", false)
	if got != "QC" {
		t.Fatalf("expected hard AI to break the points tie toward the already-discarded rank QC, got %q", got)
	}

	// Medium ignores discard history entirely — a tie stays a tie, resolved
	// by iteration order, but it must not blow up or panic.
	if got := pickSmartDiscard([]string{"KH", "QC"}, visible, "ai1", "medium", false); got != "KH" && got != "QC" {
		t.Fatalf("expected one of the tied cards from medium, got %q", got)
	}
}
