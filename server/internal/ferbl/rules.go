package ferbl

import "zolik/server/internal/module"

// Rules writes out Ferbl for one lobby's table.
func (m *Module) Rules(mc module.MatchConfig) ([]module.RuleSection, error) {
	c := resolve(mc)
	return []module.RuleSection{
		module.Section("ferbl.rules.section.goal",
			module.Fact{LabelKey: "ferbl.rules.goal"},
		),
		module.Section("ferbl.rules.section.cards",
			module.Fact{LabelKey: "ferbl.rules.deck"},
			module.Fact{LabelKey: "ferbl.rules.values"},
		),
		module.Section("ferbl.rules.section.hands",
			module.Fact{LabelKey: "ferbl.rules.fourKind"},
			module.Fact{LabelKey: "ferbl.rules.banda"},
			module.Fact{LabelKey: "ferbl.rules.threeKind"},
			module.Fact{LabelKey: "ferbl.rules.threeSuited"},
			module.Fact{LabelKey: "ferbl.rules.twoAces"},
			module.Fact{LabelKey: "ferbl.rules.twoSuited"},
			module.Fact{LabelKey: "ferbl.rules.suits"},
			module.Fact{LabelKey: "ferbl.rules.ignored"},
		),
		module.SectionOf("ferbl.rules.section.betting", []module.RuleItem{
			module.Rule("ferbl.rules.ante", map[string]any{"n": c.vizi}),
			module.Rule("ferbl.rules.round1", map[string]any{"n": c.vizi}),
			module.Rule("ferbl.rules.round2", map[string]any{"n": 2 * c.vizi}),
			module.Rule("ferbl.rules.cap", map[string]any{"n": maxRaises}),
			module.Rule("ferbl.rules.allIn", nil),
		}),
		module.Section("ferbl.rules.section.showdown",
			module.Fact{LabelKey: "ferbl.rules.showdown"},
		),
		module.Section("ferbl.rules.section.end",
			module.Fact{LabelKey: "ferbl.rules.end", Params: map[string]any{"n": c.hands, "chips": c.startingStack}},
		),
	}, nil
}
