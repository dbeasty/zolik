package ai

import (
	"testing"

	"zolik/server/internal/rules"
)

// Reproduces game 6a8aa17ff767a3c62209d475: zolik_classic with a host-set
// 35-point initial-meld floor. The agent lays AC-2C-3C (6 natural points),
// which cannot put it down.
func TestAgentRespectsInitialMeldMinimumUnderZolikClassic(t *testing.T) {
	cfg := rules.ProfileZolikClassic
	cfg.InitialMeldMinimum = 35
	cfg.DiscardDrawMinRound = 3

	actor := "ai:medium:1:0"
	visible := VisibleState{
		GameNumber:  1,
		Round:       9,
		Phase:       string(rules.PhaseMeld),
		CurrentTurn: actor,
		Melds:       map[string][][]string{},
		MeldMeta:    map[string][]rules.MeldInfo{},
		RoundReqMet: map[string]bool{actor: false},
		TotalScores: map[string]int{},
		Rules:       cfg,
	}
	hand := []string{"AC", "2C", "3C", "8D", "JH", "JH", "2H", "5S", "8D", "6H", "3S", "5D", "6C"}

	act := NewHeuristicAgent("medium").ChooseAction(visible, hand)
	if act.Type == rules.ActionLayMeld {
		val := 0
		for _, c := range act.Cards {
			val += rules.NaturalCardValue(c, true)
		}
		t.Fatalf("agent laid %v worth %d natural points under a %d-point floor; it cannot get down with it",
			act.Cards, val, cfg.InitialMeldMinimum)
	}
	t.Logf("agent chose %s (ok)", act.Type)
}
