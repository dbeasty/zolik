package blackpeter

import "zolik/server/internal/module"

var (
	_ module.RulesProvider     = (*Module)(nil)
	_ module.RuleIndexProvider = (*Module)(nil)
)

// Rules writes out Black Peter for one lobby's table.
func (m *Module) Rules(mc module.MatchConfig) ([]module.RuleSection, error) {
	t := tableOf(mc)
	pairs := module.Rule("blackpeter.rules.pairsRank", nil)
	if t.ByColour {
		pairs = module.Rule("blackpeter.rules.pairsColour", nil)
	}
	return []module.RuleSection{
		module.Section("blackpeter.rules.section.goal",
			module.Fact{LabelKey: "blackpeter.rules.goal"},
		),
		module.SectionOf("blackpeter.rules.section.cards", []module.RuleItem{
			module.Rule("blackpeter.rules.deck", nil),
			pairs,
		}),
		module.SectionOf("blackpeter.rules.section.play", []module.RuleItem{
			module.Rule("blackpeter.rules.deal", nil),
			module.Rule("blackpeter.rules.layPairs", nil),
			module.Rule("blackpeter.rules.draw", nil),
			module.Rule("blackpeter.rules.pass", nil),
			module.Rule("blackpeter.rules.out", nil),
		}),
		module.SectionOf("blackpeter.rules.section.end", []module.RuleItem{
			module.Rule("blackpeter.rules.lose", nil),
			module.Rule("blackpeter.rules.nextDeal", nil),
			module.Rule("blackpeter.rules.match", map[string]any{"n": t.RoundLimit}),
		}),
	}, nil
}

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
	case ErrNotYourTurn:
		return []string{"blackpeter.rules.draw", "blackpeter.rules.pass"}
	case ErrOut:
		return []string{"blackpeter.rules.out"}

	// --- not about the rules at all ---------------------------------------
	case ErrGameNotActive, ErrUnknownAction, ErrNoCard, ErrPlayerCount:
		return nil
	}
	return nil
}
