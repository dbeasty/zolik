package ferbl

import "zolik/server/internal/module"

var (
	_ module.RulesProvider     = (*Module)(nil)
	_ module.RuleIndexProvider = (*Module)(nil)
)

// ExplainRefusal names the written rules behind each code this engine can
// return, filtered to the ones this table actually states.
func (m *Module) ExplainRefusal(cfg module.MatchConfig, code string) []string {
	return explainWith(module.StatedRuleIDs(m, cfg), code)
}

func explainWith(stated map[string]bool, code string) []string {
	var out []string
	for _, id := range refusalRules(code) {
		if stated[id] {
			out = append(out, id)
		}
	}
	return out
}

func refusalRules(code string) []string {
	switch code {
	case ErrCannotCheck, ErrCannotBet, ErrNothingToCall, ErrCannotRaise, ErrNoNeedToFold, ErrNotYourTurn:
		return []string{"ferbl.rules.round1"}
	case ErrRaiseCap:
		return []string{"ferbl.rules.cap"}

	// --- not about the rules at all ---------------------------------------
	case ErrGameNotActive, ErrUnknownAction, "TOO_FEW_PLAYERS":
		return nil
	}
	return nil
}
