package lora

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
	case ErrMustFollow, ErrPickOneCard:
		return []string{"lora.rules.tricks.follow"}
	case ErrNoRedLead:
		return []string{"lora.rules.tricks.redLead"}
	case ErrCannotLay, ErrNothingLaid:
		return []string{"lora.rules.desitky.lay"}
	case ErrMustLay:
		return []string{"lora.rules.desitky.tuk"}
	case ErrNotChoosing, ErrNoSuchGame, ErrPrPoThirdOnly:
		return []string{"lora.rules.maturita.choose"}
	case ErrNotYourTurn:
		return []string{"lora.rules.talie"}

	// --- not about the rules at all ---------------------------------------
	case ErrGameNotActive, ErrUnknownAction, ErrCardNotInHand, "TOO_FEW_PLAYERS":
		return nil
	}
	return nil
}
