package ai

import (
	"testing"

	"zolik/server/internal/rules"
)

// The agent half of the take-it-and-give-it-straight-back defect. The engine
// now refuses that discard (rules.ErrDiscardTakenCard), so an agent that
// still names it doesn't just play badly — it stalls its turn on a rejection
// it has no way to interpret, and the deal stops dead behind it. The
// self-play harness reproduced exactly that: agents wedged on seeds 7, 11, 15
// and 20 the moment the engine started enforcing the rule.

func visibleForDiscardTest(cfg rules.RulesConfig, taken string) VisibleState {
	return VisibleState{
		GameNumber:       1,
		Round:            3,
		Phase:            string(rules.PhaseMeld),
		CurrentTurn:      "p1",
		DiscardPile:      []string{},
		PlayerDiscards:   map[string][]string{},
		Melds:            map[string][][]string{},
		MeldMeta:         map[string][]rules.MeldInfo{},
		RoundReqMet:      map[string]bool{"p1": true},
		TotalScores:      map[string]int{},
		Rules:            cfg,
		DiscardTakenCard: taken,
	}
}

func TestPickSmartDiscard_NeverNamesTheCardJustTaken(t *testing.T) {
	cfg := openContinental()
	// KH is both the taken card and the highest-penalty card in hand, so a
	// points-led heuristic reaches for it first — which is exactly the case
	// that used to hand it straight back.
	hand := []string{"KH", "4S", "3S", "8D"}
	visible := visibleForDiscardTest(cfg, "KH")

	for _, difficulty := range []string{"easy", "medium", "hard"} {
		got := pickSmartDiscard(hand, visible, "p1", difficulty, false)
		if got == "KH" {
			t.Fatalf("%s: named the just-taken KH, which the engine will refuse", difficulty)
		}
	}
}

func TestPickSmartDiscard_FallsBackToTheTakenCardWhenNothingElseIsLegal(t *testing.T) {
	// Mirror of the engine's escape hatch: with only the taken card and an
	// undiscardable joker in hand, the taken card is the one legal discard,
	// and refusing to name it would stall the agent just as badly.
	cfg := rules.ProfileContinental
	hand := []string{"QH", "JOKER1"}
	visible := visibleForDiscardTest(cfg, "QH")

	got := pickSmartDiscard(hand, visible, "p1", "medium", false)
	if got != "QH" {
		t.Fatalf("expected the only legal discard QH, got %q", got)
	}
}

func TestPickSmartDiscard_TakenCardIsFreeAgainOnceItsTwinIsTheOneInHand(t *testing.T) {
	// Two decks: no marker set (the taken QH already went to the table), so
	// the QH still in hand is an ordinary card again.
	cfg := openContinental()
	hand := []string{"QH", "4S", "3S"}
	visible := visibleForDiscardTest(cfg, "")

	if got := pickSmartDiscard(hand, visible, "p1", "medium", false); got != "QH" {
		t.Fatalf("expected the highest-penalty QH once unmarked, got %q", got)
	}
}
