package zolikmod

import (
	"testing"

	"zolik/server/internal/module"
)

// The two ends of the rule index, checked against each other.
//
// Between them these say: a player cannot be refused by this engine for a
// reason nothing in the written rules explains. That is the whole guarantee,
// and it is the reason the index is worth having rather than a per-code table
// of explanations somebody remembers to update.
//
// The walk itself lives in module.RuleIndexCheck, because every game runs it
// now and six copies of it would drift the first time one of them grew an idea
// the others did not get. Two source directories here, unlike the other
// modules: this one's refusals come from the shared rummy engine as well as
// from the module itself.
func TestRuleIndex(t *testing.T) {
	problems, notes := module.RuleIndexCheck{
		Module: New(),
		Dirs:   []string{"../rules"},
		Exempt: map[string]string{
			"CARD_NOT_IN_HAND": "not a rule — the client and server disagree about the hand",
			"NOTHING_TO_UNDO":  "not a rule — there is simply nothing to take back",
			"GAME_SUSPENDED":   "not a rule — the table is waiting for a player",
			"GAME_NOT_ACTIVE":  "not a rule — the match is over or has not started",
		},
	}.Run()

	for _, n := range notes {
		t.Log(n)
	}
	for _, p := range problems {
		t.Error(p)
	}
}
