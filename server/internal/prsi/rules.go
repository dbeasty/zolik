package prsi

import "zolik/server/internal/module"

var _ module.RulesProvider = (*Module)(nil)

// Rules writes out Prší's rules for one lobby's actual hand size, resolved
// the same way NewMatch resolves it.
func (m *Module) Rules(cfg module.MatchConfig) ([]module.RuleSection, error) {
	handSize := cfg.Opt(OptHandSize, defaultHandSize)

	return []module.RuleSection{
		module.Section("prsi.rules.section.goal",
			module.Fact{LabelKey: "prsi.rules.goal"},
		),
		module.Section("prsi.rules.section.setup",
			module.Fact{LabelKey: "prsi.rules.deck", Value: "32"},
			module.Fact{LabelKey: "prsi.rules.deal", Params: map[string]any{"n": handSize}},
		),
		module.Section("prsi.rules.section.turn",
			module.Fact{LabelKey: "prsi.rules.turn.match"},
			module.Fact{LabelKey: "prsi.rules.turn.draw"},
		),
		module.Section("prsi.rules.section.special",
			module.Fact{LabelKey: "prsi.rules.sevens"},
			module.Fact{LabelKey: "prsi.rules.aces"},
			module.Fact{LabelKey: "prsi.rules.queens"},
		),
		module.Section("prsi.rules.section.end",
			module.Fact{LabelKey: "prsi.rules.end"},
		),
	}, nil
}
