package klondike

import (
	"testing"

	"zolik/server/internal/module"
)

func TestRuleIndex(t *testing.T) {
	problems, notes := module.RuleIndexCheck{
		Module: New(),
		Dirs:   []string{"."},
		Exempt: map[string]string{
			"GAME_NOT_ACTIVE":    "not a rule — the game is over",
			"UNKNOWN_ACTION":     "not a rule — a verb this module does not have",
			"WRONG_PLAYER_COUNT": "not a rule of play — the table could not be seated",
			"NO_SUCH_CARD":       "not a rule — a card that is not where the client said",
		},
	}.Run()
	for _, n := range notes {
		t.Log(n)
	}
	for _, p := range problems {
		t.Error(p)
	}
}
