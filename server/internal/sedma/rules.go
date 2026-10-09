package sedma

import "zolik/server/internal/module"

// Rules writes out Sedma. Rules has no seat count to read, so the sentences
// that depend on it say when they apply.
func (m *Module) Rules(mc module.MatchConfig) ([]module.RuleSection, error) {
	c := resolve(mc)
	return []module.RuleSection{
		module.Section("sedma.rules.section.goal",
			module.Fact{LabelKey: "sedma.rules.goal"},
		),
		module.Section("sedma.rules.section.cards",
			module.Fact{LabelKey: "sedma.rules.deck"},
			module.Fact{LabelKey: "sedma.rules.points"},
		),
		module.Section("sedma.rules.section.deal",
			module.Fact{LabelKey: "sedma.rules.teams"},
			module.Fact{LabelKey: "sedma.rules.deal"},
		),
		module.Section("sedma.rules.section.play",
			module.Fact{LabelKey: "sedma.rules.play"},
			module.Fact{LabelKey: "sedma.rules.capture"},
			module.Fact{LabelKey: "sedma.rules.continue"},
			module.Fact{LabelKey: "sedma.rules.draw"},
		),
		module.Section("sedma.rules.section.scoring",
			module.Fact{LabelKey: "sedma.rules.score"},
			module.Fact{LabelKey: "sedma.rules.score3"},
		),
		module.Section("sedma.rules.section.end",
			module.Fact{LabelKey: "sedma.rules.end", Params: map[string]any{"n": c.target}},
		),
	}, nil
}
