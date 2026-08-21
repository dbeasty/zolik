package rules

import "testing"

// customFixedDealProfile is a rotating-contract profile whose match is five
// deals long instead of Continental's seven. It exists to catch code that
// asks "is GameNumber 7?" where it means "is this the ruleset's final deal?".
func customFixedDealProfile(dealCount int) RulesConfig {
	cfg := ProfileContinental
	cfg.Profile = "custom"
	cfg.FixedDealCount = dealCount
	cfg.InitialMeldMinimum = 0
	return cfg
}

// finalDealState puts a player one meld away from an empty hand on the last
// deal of a fixed-length match, with their contract already met — so laying
// that meld both empties the hand and legitimately ends the deal.
func finalDealState(cfg RulesConfig, playerID string) GameState {
	return GameState{
		Status:      StatusActive,
		Rules:       cfg,
		GameNumber:  cfg.FixedDealCount,
		Phase:       PhaseMeld,
		CurrentTurn: playerID,
		TurnOrder:   []string{playerID, "p2"},
		Round:       1,
		Hands:       map[string][]string{playerID: {"7H", "7D", "7C"}, "p2": {"KH", "QS"}},
		Melds:       map[string][][]string{},
		MeldMeta:    map[string][]MeldInfo{},
		RoundReqMet: map[string]bool{playerID: true, "p2": false},
		DrawPile:    []string{"2C"},
		DiscardPile: []string{"3D"},
		DeckSeed:    42,
		GameScores:  map[string][]int{playerID: {}, "p2": {}},
		TotalScores: map[string]int{playerID: 0, "p2": 0},
	}
}

// TestGoOutByMeld_EndsDealOnAnyProfilesFinalDeal is the regression guard for a
// hardcoded `GameNumber == 7` in ApplyAction: under a five-deal ruleset,
// melding away the last card on deal 5 must end the deal. Before the fix the
// meld was accepted (ValidateMeldAction asks IsFinalDeal, which is true here)
// but ApplyAction never fired the ending, leaving the match wedged with a
// player holding zero cards and no legal move.
func TestGoOutByMeld_EndsDealOnAnyProfilesFinalDeal(t *testing.T) {
	cfg := customFixedDealProfile(5)
	st := finalDealState(cfg, "p1")

	outcome, err := ApplyAction(st, "p1", Action{Type: ActionLayMeld, Cards: []string{"7H", "7D", "7C"}})
	if err != nil {
		t.Fatalf("expected the emptying meld to be legal on the final deal, got %v", err)
	}
	if len(outcome.State.Hands["p1"]) != 0 {
		t.Fatalf("expected p1's hand to be empty, got %v", outcome.State.Hands["p1"])
	}
	if !hasEvent(outcome.Events, "deal_ended") {
		t.Fatalf("expected deal_ended after going out by meld, got %v", eventTypes(outcome.Events))
	}
	if outcome.State.Status != StatusCompleted {
		t.Fatalf("expected the match to complete on its final deal, got status %q", outcome.State.Status)
	}
}

// TestGoOutByLayOff_EndsDealOnAnyProfilesFinalDeal is the same guard for the
// lay_off branch of ApplyAction, which carried its own copy of the check.
func TestGoOutByLayOff_EndsDealOnAnyProfilesFinalDeal(t *testing.T) {
	cfg := customFixedDealProfile(5)
	st := finalDealState(cfg, "p1")
	st.Hands["p1"] = []string{"9C"}
	st.Melds["p2"] = [][]string{{"5C", "6C", "7C", "8C"}}
	st.MeldMeta["p2"] = []MeldInfo{{MeldID: "meld_1", Type: MeldRun, OwnerID: "p2"}}

	outcome, err := ApplyAction(st, "p1", Action{
		Type:   ActionLayOff,
		MeldID: "meld_1",
		Card:   "9C",
	})
	if err != nil {
		t.Fatalf("expected the emptying lay-off to be legal on the final deal, got %v", err)
	}
	if len(outcome.State.Hands["p1"]) != 0 {
		t.Fatalf("expected p1's hand to be empty, got %v", outcome.State.Hands["p1"])
	}
	if !hasEvent(outcome.Events, "deal_ended") {
		t.Fatalf("expected deal_ended after going out by lay-off, got %v", eventTypes(outcome.Events))
	}
}

// TestGoOutByMeld_StillBlockedBeforeTheFinalDeal confirms the fix didn't turn
// the check into "always end the deal": on a non-final deal the emptying meld
// must still be rejected, because those deals require a closing discard.
func TestGoOutByMeld_StillBlockedBeforeTheFinalDeal(t *testing.T) {
	cfg := customFixedDealProfile(5)
	st := finalDealState(cfg, "p1")
	st.GameNumber = 4 // not the last deal of this five-deal match

	if _, err := ApplyAction(st, "p1", Action{Type: ActionLayMeld, Cards: []string{"7H", "7D", "7C"}}); err == nil {
		t.Fatalf("expected melding away the whole hand to be rejected before the final deal")
	}
}

// TestGoOutByMeld_NeverEndsDealUnderScoreLimitedMatch covers the other match
// shape: zolik_classic keeps re-dealing until someone crosses the target
// score, so it has no final deal at all and every deal needs a closing
// discard — including a deal that happens to be numbered 7.
func TestGoOutByMeld_NeverEndsDealUnderScoreLimitedMatch(t *testing.T) {
	st := finalDealState(ProfileZolikClassic, "p1")
	st.GameNumber = 7

	if _, err := ApplyAction(st, "p1", Action{Type: ActionLayMeld, Cards: []string{"7H", "7D", "7C"}}); err == nil {
		t.Fatalf("expected melding away the whole hand to be rejected under a score-limited match, even on deal 7")
	}
}

func eventTypes(events []StateEvent) []string {
	out := make([]string, 0, len(events))
	for _, e := range events {
		out = append(out, e.Type)
	}
	return out
}

func hasEvent(events []StateEvent, t string) bool {
	for _, e := range events {
		if e.Type == t {
			return true
		}
	}
	return false
}
