package canasta

import "zolik/server/internal/module"

var _ module.RulesProvider = (*Module)(nil)

// Rules writes out Canasta's rules for one lobby's actual hand size, target
// score and go-out requirement, resolved the same way the engine resolves
// them (engine.go).
func (m *Module) Rules(cfg module.MatchConfig) ([]module.RuleSection, error) {
	v := resolveVariation(cfg)
	handSize := cfg.Opt(OptHandSize, v.handSize)
	targetScore := cfg.Opt(OptTargetScore, v.targetScore)
	canastasToGoOut := cfg.Opt(OptCanastasToGoOut, v.canastasToGoOut)

	// Reuses the exact keys Descriptor's Summary already ships (param-free —
	// the sentence spells "one"/"two" out in words) rather than a second,
	// numbered phrasing of the same fact.
	goOutKey := "canasta.rules.oneCanastaToGoOut"
	if canastasToGoOut > 1 {
		goOutKey = "canasta.rules.twoCanastasToGoOut"
	}

	return []module.RuleSection{
		module.Section("canasta.rules.section.goal",
			module.Fact{LabelKey: "canasta.rules.goal", Params: map[string]any{"n": targetScore}},
		),
		module.Section("canasta.rules.section.setup",
			module.Fact{LabelKey: "canasta.rules.deck", Value: "108"},
			module.Fact{LabelKey: "canasta.rules.deal", Params: map[string]any{"n": handSize}},
			module.Fact{LabelKey: "canasta.rules.redThrees"},
		),
		module.Section("canasta.rules.section.melding",
			module.Fact{LabelKey: "canasta.rules.canasta", Params: map[string]any{"n": canastaSize}},
			module.Fact{LabelKey: "canasta.rules.meldFloorBands", Params: map[string]any{
				"negative": 15, "low": 50, "mid": 90, "high": 120,
			}},
		),
		module.Section("canasta.rules.section.end",
			module.Fact{LabelKey: goOutKey},
			module.Fact{LabelKey: "canasta.rules.end", Params: map[string]any{"n": targetScore}},
		),
	}, nil
}
