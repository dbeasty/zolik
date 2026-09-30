package holdem

import (
	"testing"

	"zolik/server/internal/module"
)

// Every refusal this engine can hand a player points at a rule that table
// actually states — see module.RuleIndexCheck for what that buys.
func TestRuleIndex(t *testing.T) {
	problems, notes := module.RuleIndexCheck{
		Module: New(),
		Dirs:   []string{"."},
		Exempt: map[string]string{
			"AMOUNT_NOT_A_NUMBER": "not a rule — the client sent something that is not a figure",
			"GAME_NOT_ACTIVE":     "not a rule — the hand is over or has not started",
			"UNKNOWN_ACTION":      "not a rule — a verb this module does not have",
			"WRONG_PLAYER_COUNT":  "not a rule of play — the table could not be seated",
		},
	}.Run()

	for _, n := range notes {
		t.Log(n)
	}
	for _, p := range problems {
		t.Error(p)
	}
}
