package marias

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
	case ErrNoZLidu:
		return []string{"marias.rules.deal.trump"}
	case ErrPickOneCard:
		return []string{"marias.rules.deal.trump", "marias.rules.play.follow"}
	case ErrNoSeven:
		return []string{"marias.rules.game.sedma", "marias.rules.bid.proti"}
	case ErrDiscardTwo, ErrSharpInTalon:
		return []string{"marias.rules.deal.talon"}
	case ErrSevenInTalon:
		return []string{"marias.rules.game.sedma", "marias.rules.deal.talon"}
	case ErrNotHigher:
		return []string{"marias.rules.bid.takeover"}
	case ErrNotYoursToFlek, ErrNoSuchPart:
		return []string{"marias.rules.bid.flek"}
	case ErrFlekLimit:
		return []string{"marias.rules.bid.flekLimit"}
	case ErrProtiNotNow:
		return []string{"marias.rules.bid.proti"}
	case ErrMustFollow, ErrMustBeat:
		return []string{"marias.rules.play.follow"}
	case ErrMustTrump, ErrMustOvertrump:
		return []string{"marias.rules.play.trump"}
	case ErrKeepSeven:
		return []string{"marias.rules.play.seven"}

	case ErrNotYourTurn, ErrNotNow:
		return []string{"marias.rules.deal.turns"}

	// --- not about the rules at all ---------------------------------------
	case ErrGameNotActive, ErrUnknownAction, ErrCardNotInHand, "TOO_FEW_PLAYERS":
		return nil
	}
	return nil
}
