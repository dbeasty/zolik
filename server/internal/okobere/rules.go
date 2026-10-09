package okobere

import "zolik/server/internal/module"

// Rules writes out Oko bere for one lobby's table.
func (m *Module) Rules(mc module.MatchConfig) ([]module.RuleSection, error) {
	c := resolve(mc)
	oko := module.Rule("okobere.rules.okoPays3", map[string]any{"n": c.okoPays})
	if c.okoPays == 1 {
		oko = module.Rule("okobere.rules.okoPays1", nil)
	}
	return []module.RuleSection{
		module.Section("okobere.rules.section.goal",
			module.Fact{LabelKey: "okobere.rules.goal"},
		),
		module.Section("okobere.rules.section.cards",
			module.Fact{LabelKey: "okobere.rules.deck"},
			module.Fact{LabelKey: "okobere.rules.values"},
			module.Fact{LabelKey: "okobere.rules.oko"},
		),
		module.Section("okobere.rules.section.bank",
			module.Fact{LabelKey: "okobere.rules.bank", Params: map[string]any{"bank": c.bank}},
			module.Fact{LabelKey: "okobere.rules.rotation", Params: map[string]any{"n": c.roundsPerBank}},
		),
		module.SectionOf("okobere.rules.section.play", []module.RuleItem{
			module.Rule("okobere.rules.deal", nil),
			module.Rule("okobere.rules.stake", map[string]any{"n": c.minBet}),
			module.Rule("okobere.rules.turn", nil),
			module.Rule("okobere.rules.bust", nil),
			oko,
			module.Rule("okobere.rules.banker", nil),
		}),
		module.Section("okobere.rules.section.settle",
			module.Fact{LabelKey: "okobere.rules.settle"},
			module.Fact{LabelKey: "okobere.rules.bankerOko"},
		),
		module.Section("okobere.rules.section.end",
			module.Fact{LabelKey: "okobere.rules.end", Params: map[string]any{"n": c.bankTurns, "chips": c.startingStack}},
		),
	}, nil
}
