package lastcard

import "zolik/server/internal/module"

var _ module.RulesProvider = (*Module)(nil)

// Rules writes out Last Card's rules for one lobby's actual hand size,
// resolved the same way NewMatch resolves it.
func (m *Module) Rules(cfg module.MatchConfig) ([]module.RuleSection, error) {
	handSize := cfg.Opt(OptHandSize, defaultHandSize)

	return []module.RuleSection{
		module.Section("lastcard.rules.section.goal",
			module.Fact{LabelKey: "lastcard.rules.goal"},
		),
		module.Section("lastcard.rules.section.setup",
			module.Fact{LabelKey: "lastcard.rules.deck", Value: "108"},
			module.Fact{LabelKey: "lastcard.rules.deal", Params: map[string]any{"n": handSize}},
		),
		module.Section("lastcard.rules.section.turn",
			module.Fact{LabelKey: "lastcard.rules.turn.match"},
			module.Fact{LabelKey: "lastcard.rules.turn.draw"},
		),
		module.Section("lastcard.rules.section.special",
			module.Fact{LabelKey: "lastcard.rules.skip"},
			module.Fact{LabelKey: "lastcard.rules.reverse"},
			module.Fact{LabelKey: "lastcard.rules.drawTwo"},
			module.Fact{LabelKey: "lastcard.rules.wild"},
			module.Fact{LabelKey: "lastcard.rules.wildDrawFour"},
		),
		module.Section("lastcard.rules.section.end",
			module.Fact{LabelKey: "lastcard.rules.end"},
		),
	}, nil
}
