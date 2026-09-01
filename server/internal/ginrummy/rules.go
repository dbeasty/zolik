package ginrummy

import "zolik/server/internal/module"

var _ module.RulesProvider = (*Module)(nil)

// Rules is Gin Rummy's written rules, config-aware the way every option a
// player can change is worth stating a number for rather than leaving to a
// tooltip.
func (m *Module) Rules(cfg module.MatchConfig) ([]module.RuleSection, error) {
	v := resolveVariation(cfg)
	target := cfg.Opt(OptTargetScore, v.targetScore)
	knockLimit := cfg.Opt(OptKnockLimit, v.knockLimit)
	bigGin := cfg.Opt(OptBigGin, module.BoolOpt(v.bigGin)) == module.OptOn
	lineBonuses := cfg.Opt(OptLineBonuses, module.BoolOpt(v.lineBonuses)) == module.OptOn

	sections := []module.RuleSection{
		module.Section("ginrummy.rules.setup",
			module.Fact{LabelKey: "ginrummy.rules.deck", Value: "52"},
			module.Fact{LabelKey: "ginrummy.rules.deal", Value: "10"},
			module.Fact{LabelKey: "ginrummy.rules.upcard"},
		),
		module.Section("ginrummy.rules.turn",
			module.Fact{LabelKey: "ginrummy.rules.drawDiscard"},
		),
		module.Section("ginrummy.rules.melds",
			module.Fact{LabelKey: "ginrummy.rules.setsAndRuns"},
			module.Fact{LabelKey: "ginrummy.rules.aceLow"},
		),
	}

	if knockLimit == oklahomaSentinel {
		sections = append(sections, module.Section("ginrummy.rules.knocking",
			module.Fact{LabelKey: "ginrummy.rules.oklahoma"},
			module.Fact{LabelKey: "ginrummy.rules.gin"},
		))
	} else {
		sections = append(sections, module.Section("ginrummy.rules.knocking",
			module.Fact{LabelKey: "ginrummy.rules.knockLimit", Params: map[string]any{"n": knockLimit}},
			module.Fact{LabelKey: "ginrummy.rules.gin"},
		))
	}
	if bigGin {
		sections = append(sections, module.Section("ginrummy.rules.bigGin",
			module.Fact{LabelKey: "ginrummy.rules.bigGinBonus", Params: map[string]any{"n": 25}},
		))
	}

	sections = append(sections,
		module.Section("ginrummy.rules.layoff",
			module.Fact{LabelKey: "ginrummy.rules.layoffDescription"},
		),
		module.Section("ginrummy.rules.deadHand",
			module.Fact{LabelKey: "ginrummy.rules.deadHandDescription"},
		),
		module.Section("ginrummy.rules.scoring",
			module.Fact{LabelKey: "ginrummy.rules.undercut", Params: map[string]any{"n": 25}},
			module.Fact{LabelKey: "ginrummy.rules.ginBonus", Params: map[string]any{"n": 25}},
		),
		module.Section("ginrummy.rules.match",
			module.Fact{LabelKey: "ginrummy.rules.target", Params: map[string]any{"n": target}},
		),
	)

	if lineBonuses {
		sections = append(sections, module.Section("ginrummy.rules.lineBonuses",
			module.Fact{LabelKey: "ginrummy.rules.box", Params: map[string]any{"n": 25}},
			module.Fact{LabelKey: "ginrummy.rules.gameBonus", Params: map[string]any{"n": 100}},
			module.Fact{LabelKey: "ginrummy.rules.shutout", Params: map[string]any{"n": 200}},
		))
	}

	return sections, nil
}
