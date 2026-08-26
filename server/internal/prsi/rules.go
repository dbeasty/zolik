package prsi

import "zolik/server/internal/module"

var _ module.RulesProvider = (*Module)(nil)

// Rules writes out Prší's rules for one lobby's actual hand size, resolved
// the same way NewMatch resolves it.
func (m *Module) Rules(cfg module.MatchConfig) ([]module.RuleSection, error) {
	handSize := cfg.Opt(OptHandSize, defaultHandSize)

	return []module.RuleSection{
		{TitleKey: "prsi.rules.section.goal", Items: []module.Fact{
			{LabelKey: "prsi.rules.goal"},
		}},
		{TitleKey: "prsi.rules.section.setup", Items: []module.Fact{
			{LabelKey: "prsi.rules.deck", Value: "32"},
			{LabelKey: "prsi.rules.deal", Params: map[string]any{"n": handSize}},
		}},
		{TitleKey: "prsi.rules.section.turn", Items: []module.Fact{
			{LabelKey: "prsi.rules.turn.match"},
			{LabelKey: "prsi.rules.turn.draw"},
		}},
		{TitleKey: "prsi.rules.section.special", Items: []module.Fact{
			{LabelKey: "prsi.rules.sevens"},
			{LabelKey: "prsi.rules.aces"},
			{LabelKey: "prsi.rules.queens"},
		}},
		{TitleKey: "prsi.rules.section.end", Items: []module.Fact{
			{LabelKey: "prsi.rules.end"},
		}},
	}, nil
}
