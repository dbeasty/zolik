package canasta

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
			"CARD_NOT_IN_HAND":   "not a rule — the client and server disagree about the hand",
			"NO_SUCH_MELD":       "not a rule — a meld id that is not on the table",
			"NOTHING_TO_UNDO":    "not a rule — there is simply nothing to take back",
			"UNDO_MELDS_FIRST":   "not a rule — the order an undo unwinds a turn in, not a rule of play",
			"GAME_NOT_ACTIVE":    "not a rule — the deal is over or has not started",
			"UNKNOWN_ACTION":     "not a rule — a verb this module does not have",
			"WRONG_PLAYER_COUNT": "not a rule of play — the table could not be seated",
		},
	}.Run()

	for _, n := range notes {
		t.Log(n)
	}
	for _, p := range problems {
		t.Error(p)
	}
}
