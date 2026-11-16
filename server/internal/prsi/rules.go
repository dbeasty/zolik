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
			// Why six players cannot be dealt six: 36 from a 32-card pack.
			module.Fact{LabelKey: "prsi.rules.dealLimit", Params: map[string]any{"seats": 6, "n": maxHandSize(6)}},
			// NewMatch: the turned-up card acts on the first player, and a
			// svršek is never the one turned up while another card can be.
			module.Fact{LabelKey: "prsi.rules.firstCard"},
		),
		module.Section("prsi.rules.section.turn",
			module.Fact{LabelKey: "prsi.rules.turn.match"},
			module.Fact{LabelKey: "prsi.rules.turn.draw"},
			module.Fact{LabelKey: "prsi.rules.turn.nothingToDraw"},
		),
		module.Section("prsi.rules.section.special",
			module.Fact{LabelKey: "prsi.rules.sevens"},
			// applyPlay: a 7 answering a 7 adds two to the debt.
			module.Fact{LabelKey: "prsi.rules.sevens.stack"},
			module.Fact{LabelKey: "prsi.rules.aces"},
			module.Fact{LabelKey: "prsi.rules.queens"},
			// playable: a wild goes on anything — except while a 7 or an
			// ace waits to be answered.
			module.Fact{LabelKey: "prsi.rules.queens.anything"},
		),
		module.Section("prsi.rules.section.end",
			module.Fact{LabelKey: "prsi.rules.end"},
			module.Fact{LabelKey: "prsi.rules.end.blocked"},
		),
	}, nil
}
