package ai

import (
	"testing"

	"zolik/server/internal/rules"
)

// The "AI never melds" report, reduced to its mechanical causes. They share
// a root: the agent ignored the meld structure of its own hand. It threw
// meld material into the discard pile, declined to lay melds the ruleset
// would happily accept, and — once down — kept picking a card off the
// discard pile only to throw it straight back, so no deal ever ended.
// Together these kept an agent off the table for a whole deal even while it
// held a finished meld.
//
// Each of these fails on the code as it was before its fix; see
// selfplay_test.go for the whole-match harness that surfaced them, none of
// them being visible in any single decision.

// TestPickSmartDiscard_KeepsACompletedMeldTogether is the core regression:
// the discard heuristic ranked purely on penalty points, so a hand holding a
// finished set of kings shed those kings first — highest points, nothing on
// the table to "feed" — dismantling a ready-to-lay meld one card per turn.
func TestPickSmartDiscard_KeepsACompletedMeldTogether(t *testing.T) {
	cfg := rules.ProfileZolikClassic
	visible := VisibleState{Rules: cfg}

	// KD/KH/KS is a complete set. 2H is junk. The blind "shed the costliest
	// card" rule picks a king (10 pts) over the 2 (2 pts).
	got := pickSmartDiscard([]string{"KD", "KH", "KS", "2H"}, visible, "ai1", "medium", false)
	if got != "2H" {
		t.Fatalf("expected the AI to keep its completed set of kings and discard 2H, got %q", got)
	}
}

// A run the agent is holding must survive the discard step too, not just a set.
func TestPickSmartDiscard_KeepsACompletedRunTogether(t *testing.T) {
	cfg := rules.ProfileZolikClassic // MinRunSize 3
	visible := VisibleState{Rules: cfg}

	got := pickSmartDiscard([]string{"9C", "TC", "JC", "3D"}, visible, "ai1", "medium", false)
	if got != "3D" {
		t.Fatalf("expected the AI to keep its completed run 9C-TC-JC and discard 3D, got %q", got)
	}
}

// Every difficulty keeps its own completed melds: an agent that dismantles
// a finished set every turn never gets one onto the table at all, which
// reads as "the AI doesn't meld" rather than as a beatable opponent. What
// still separates the difficulties is reading the *table* — easy happily
// feeds a live meld that medium and hard would protect.
func TestPickSmartDiscard_EasyKeepsItsMeldButStillFeedsOpponents(t *testing.T) {
	cfg := rules.ProfileZolikClassic
	plain := VisibleState{Rules: cfg}

	if got := pickSmartDiscard([]string{"KD", "KH", "KS", "2H"}, plain, "ai1", "easy", false); got != "2H" {
		t.Fatalf("even easy should keep its completed set and discard 2H, got %q", got)
	}

	// The difficulty ladder still bites: 9C extends the opponent's run and
	// is the higher-points card, and easy — unlike medium — hands it over.
	withOpponent := VisibleState{
		Rules: cfg,
		Melds: map[string][][]string{"opponent": {{"5C", "6C", "7C", "8C"}}},
	}
	if got := pickSmartDiscard([]string{"9C", "2H"}, withOpponent, "ai1", "easy", false); got != "9C" {
		t.Fatalf("easy should still feed the opponent's run, got %q", got)
	}
	if got := pickSmartDiscard([]string{"9C", "2H"}, withOpponent, "ai1", "medium", false); got != "2H" {
		t.Fatalf("medium should still protect against feeding the run, got %q", got)
	}
}

// Protecting meld material must not outrank not feeding an opponent: if the
// only "safe" card is one of the AI's own meld cards, it still keeps the
// meld and sheds the card the opponent can't use.
func TestPickSmartDiscard_MeldProtectionDoesNotOverrideFeedingSafety(t *testing.T) {
	cfg := rules.ProfileZolikClassic
	visible := VisibleState{
		Rules: cfg,
		Melds: map[string][][]string{"opponent": {{"5C", "6C", "7C"}}},
	}
	// 8C would extend the opponent's run (dangerous) *and* is not part of
	// any meld in hand; 4D is the only other non-meld card. The AI must
	// shed 4D — never 8C (a gift), never one of its own kings.
	got := pickSmartDiscard([]string{"KD", "KH", "KS", "8C", "4D"}, visible, "ai1", "medium", false)
	if got != "4D" {
		t.Fatalf("expected 4D (safe, not meld material), got %q", got)
	}
}

// When every card belongs to a meld the agent still has to name a discard —
// it must return a real card from the hand rather than "" or a panic.
func TestPickSmartDiscard_AllCardsAreMeldMaterial(t *testing.T) {
	cfg := rules.ProfileZolikClassic
	visible := VisibleState{Rules: cfg}

	hand := []string{"KD", "KH", "KS"}
	got := pickSmartDiscard(hand, visible, "ai1", "medium", false)
	if !containsCard(hand, got) {
		t.Fatalf("expected a card from the hand, got %q", got)
	}
}

// TestHeuristicAgent_LaysAFreeSetUnderAnUnquotaedProfile: under Žolík
// Classic the contract is "one clean run" and nothing else, so
// findInitialMeldPlan only ever searched for runs. A finished set of kings
// in hand was therefore never laid — even though ValidateMeldAction accepts
// it (MeldContributesTowardRequirement short-circuits to true when
// FixedDealCount == 0) and ValidateDiscard deliberately lets the player
// discard afterwards. The agent simply left points in its hand.
func TestHeuristicAgent_LaysAFreeSetUnderAnUnquotaedProfile(t *testing.T) {
	agent := NewHeuristicAgent("medium")
	aiID := "ai1"

	state := rules.GameState{
		Status:      rules.StatusActive,
		Phase:       rules.PhaseMeld,
		GameNumber:  1,
		Round:       1,
		CurrentTurn: aiID,
		TurnOrder:   []string{aiID, "p2"},
		// A set of kings plus junk, and no clean run anywhere in the hand,
		// so the contract itself stays unmet this turn. The hand is left
		// comfortably bigger than the outstanding contract needs, so the
		// "don't meld away your contract material" guard doesn't apply.
		Hands:       map[string][]string{aiID: {"KD", "KH", "KS", "2H", "7C", "9D", "4S", "8H"}, "p2": {}},
		DrawPile:    []string{"4S"},
		DiscardPile: []string{},
		RoundReqMet: map[string]bool{aiID: false, "p2": false},
		Rules:       rules.ProfileZolikClassic,
		GameScores:  map[string][]int{aiID: {}},
		TotalScores: map[string]int{aiID: 0},
	}

	visible := VisibleState{
		GameNumber:  state.GameNumber,
		Round:       state.Round,
		Phase:       string(state.Phase),
		CurrentTurn: state.CurrentTurn,
		RoundReqMet: state.RoundReqMet,
		Rules:       state.Rules,
	}

	action := agent.ChooseAction(visible, state.Hands[aiID])
	if action.Type != rules.ActionLayMeld {
		t.Fatalf("expected the agent to lay its free set of kings, got %+v", action)
	}
	outcome, err := rules.ApplyAction(state, aiID, action)
	if err != nil {
		t.Fatalf("the engine rejected the meld the agent chose: %v", err)
	}
	if len(outcome.State.Melds[aiID]) != 1 {
		t.Fatalf("expected one meld on the table, got %v", outcome.State.Melds[aiID])
	}
	// And the turn must still be finishable: laying a non-contract meld
	// under this profile may not strand the player in the meld phase.
	if _, err := rules.ApplyAction(outcome.State, aiID, rules.Action{Type: rules.ActionDiscard, Card: "2H"}); err != nil {
		t.Fatalf("agent got stranded — could not discard after laying the free set: %v", err)
	}
}

// downDrawState is a down (round-requirement-met) agent facing a draw
// choice, with one meld on the table and a given top discard.
func downDrawState(top string, hand []string) VisibleState {
	return VisibleState{
		GameNumber:  1,
		Round:       5, // past any discard-pickup lock
		Phase:       string(rules.PhaseDraw),
		CurrentTurn: "ai1",
		DiscardPile: []string{"2H", top},
		Melds:       map[string][][]string{"ai1": {{"5C", "6C", "7C"}}},
		MeldMeta: map[string][]rules.MeldInfo{
			"ai1": {{MeldID: "meld_1", Type: rules.MeldRun, OwnerID: "ai1"}},
		},
		RoundReqMet: map[string]bool{"ai1": true},
		Rules:       rules.ProfileZolikClassic,
	}
}

// TestHeuristicAgent_DownAgentSkipsAUselessDiscardPickup is the regression
// for the livelock that stopped deals ever finishing: a down agent took the
// top of the discard pile unconditionally ("already down: a discard pickup
// is unrestricted"), then pickSmartDiscard immediately identified that same
// card as the worst in hand and threw it straight back. With every agent
// down, one useless card circulated forever — no melds, no lay-offs, nobody
// going out — while the deck still held dozens of untouched cards.
func TestHeuristicAgent_DownAgentSkipsAUselessDiscardPickup(t *testing.T) {
	agent := NewHeuristicAgent("medium")

	// AD extends nothing on the table and combines with nothing in hand.
	visible := downDrawState("AD", []string{"3D", "JD"})
	action := agent.ChooseAction(visible, []string{"3D", "JD"})
	if action.Type != rules.ActionDrawCard {
		t.Fatalf("expected a draw, got %+v", action)
	}
	if action.DrawFrom != rules.DrawFromDeck {
		t.Fatalf("expected the deck rather than a card it would discard right back, got %+v", action)
	}
}

func TestHeuristicAgent_DownAgentTakesADiscardThatExtendsATableMeld(t *testing.T) {
	agent := NewHeuristicAgent("medium")

	// 8C extends the table run 5C-6C-7C.
	hand := []string{"3D", "JD"}
	action := agent.ChooseAction(downDrawState("8C", hand), hand)
	if action.Type != rules.ActionDrawCard || action.DrawFrom != rules.DrawFromDiscard {
		t.Fatalf("expected the agent to take the discard it can lay off, got %+v", action)
	}
}

func TestHeuristicAgent_DownAgentTakesADiscardThatCompletesAMeldInHand(t *testing.T) {
	agent := NewHeuristicAgent("medium")

	// KS completes KD-KH-KS with the cards already held.
	hand := []string{"KD", "KH", "3D"}
	action := agent.ChooseAction(downDrawState("KS", hand), hand)
	if action.Type != rules.ActionDrawCard || action.DrawFrom != rules.DrawFromDiscard {
		t.Fatalf("expected the agent to take the discard that completes a set, got %+v", action)
	}
}

// TestHeuristicAgent_WontMeldAwayItsRemainingContractMaterial: laying free
// melds is only worth it while the contract is still reachable. A player who
// is not down cannot go out at all (ValidateDiscard gates going out on
// RoundReqMet), so shrinking the hand past the point where the outstanding
// contract could still be built throws the deal away — and walks toward the
// engine's one dead end, a hand of nothing but undiscardable jokers.
func TestHeuristicAgent_WontMeldAwayItsRemainingContractMaterial(t *testing.T) {
	agent := NewHeuristicAgent("medium")
	aiID := "ai1"

	// Žolík still owes a clean run (3 cards) plus a card to discard, so a
	// 5-card hand may not spend 3 of them on a set of fives.
	hand := []string{"5H", "5S", "5D", "JC", "6D"}
	visible := VisibleState{
		GameNumber:  1,
		Round:       4,
		Phase:       string(rules.PhaseMeld),
		CurrentTurn: aiID,
		RoundReqMet: map[string]bool{aiID: false},
		Rules:       rules.ProfileZolikClassic,
	}

	action := agent.ChooseAction(visible, hand)
	if action.Type == rules.ActionLayMeld {
		t.Fatalf("expected no meld — it would leave too few cards to ever build the clean run, got %+v", action)
	}
	if action.Type != rules.ActionDiscard {
		t.Fatalf("expected a discard, got %+v", action)
	}
}

// TestFindLayOff_WontStrandTheHandOnUndiscardableJokers: under a profile
// where a joker may only be discarded as the card that ends the hand, a
// player left holding two jokers has no legal move at all — they cannot
// meld two jokers, and every discard is refused. The agent used to walk
// straight into that by laying off its last natural card.
func TestFindLayOff_WontStrandTheHandOnUndiscardableJokers(t *testing.T) {
	cfg := rules.ProfileZolikClassic // JokerDiscardRestricted
	melds := map[string][][]string{"p1": {{"5C", "6C", "7C"}}}
	meta := map[string][]rules.MeldInfo{
		"p1": {{MeldID: "meld_1", Type: rules.MeldRun, OwnerID: "p1", WildCount: 0}},
	}

	// 8C extends the run, but shedding it leaves [JOKER1 JOKER2] — a hand
	// with no legal discard and no legal meld. The agent must keep 8C.
	if meldID, card, ok := findLayOff(meta, melds, []string{"JOKER1", "JOKER2", "8C"}, cfg, 1); ok {
		t.Fatalf("expected no lay-off (it would strand two jokers), got %s onto %s", card, meldID)
	}

	// One joker left over is fine: a lone joker is exactly the card a player
	// who is down is allowed to discard to go out.
	if _, card, ok := findLayOff(meta, melds, []string{"JOKER1", "8C"}, cfg, 1); !ok || card != "8C" {
		t.Fatalf("expected 8C to be laid off, leaving a single discardable joker; got %q ok=%v", card, ok)
	}
}

// The same freedom must NOT be taken under a quota profile: Continental
// counts sets and runs toward a fixed contract, and a player who lays a
// partial combination cannot discard until they complete it
// (ErrIncompleteInitialMeld). The agent must keep refusing to start there.
func TestHeuristicAgent_StillWontLayAPartialContractUnderAQuotaProfile(t *testing.T) {
	agent := NewHeuristicAgent("medium")
	aiID := "ai1"

	state := rules.GameState{
		Status:      rules.StatusActive,
		Phase:       rules.PhaseMeld,
		GameNumber:  1, // Continental deal 1 needs 2 sets
		Round:       1,
		CurrentTurn: aiID,
		TurnOrder:   []string{aiID, "p2"},
		Hands:       map[string][]string{aiID: {"KD", "KH", "KS", "2H", "7C", "9D"}, "p2": {}},
		DrawPile:    []string{"4S"},
		RoundReqMet: map[string]bool{aiID: false, "p2": false},
		Rules:       rules.ProfileContinental,
		GameScores:  map[string][]int{aiID: {}},
		TotalScores: map[string]int{aiID: 0},
	}

	visible := VisibleState{
		GameNumber:  state.GameNumber,
		Round:       state.Round,
		Phase:       string(state.Phase),
		CurrentTurn: state.CurrentTurn,
		RoundReqMet: state.RoundReqMet,
		Rules:       state.Rules,
	}

	action := agent.ChooseAction(visible, state.Hands[aiID])
	if action.Type == rules.ActionLayMeld {
		t.Fatalf("expected no meld under a quota contract it cannot finish, got %+v", action)
	}
	if _, err := rules.ApplyAction(state, aiID, action); err != nil {
		t.Fatalf("fallback action %+v was rejected: %v", action, err)
	}
}
