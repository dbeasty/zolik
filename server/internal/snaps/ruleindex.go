package snaps

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
	case ErrMustFollow, ErrMustBeat, ErrMustTrump:
		return []string{"snaps.rules.play.strict"}
	case ErrPickOneCard:
		return []string{"snaps.rules.play.open"}
	case ErrNoExchangeCard, ErrExchangeNoTrick:
		return []string{"snaps.rules.exchange", "snaps.rules.exchange66"}
	case ErrNotOnLead:
		return []string{"snaps.rules.exchange", "snaps.rules.exchange66", "snaps.rules.close"}
	case ErrTalonClosed:
		return []string{"snaps.rules.close"}
	case ErrNotYourTurn:
		return []string{"snaps.rules.play.open"}

	// --- not about the rules at all ---------------------------------------
	case ErrGameNotActive, ErrUnknownAction, ErrCardNotInHand, "TOO_FEW_PLAYERS":
		return nil
	}
	return nil
}
