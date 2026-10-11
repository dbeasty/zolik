package ginrummy

import "zolik/server/internal/module"

var _ module.LessonProvider = (*Module)(nil)

// Lessons is Gin Rummy's guided walk-through. It resolves the config exactly
// as Rules(cfg) does, so the knock limit, the target score, big gin and the
// line bonuses a table plays with are the ones the lesson talks about.
func (m *Module) Lessons(cfg module.MatchConfig) ([]module.Lesson, error) {
	v := resolveVariation(cfg)
	target := cfg.Opt(OptTargetScore, v.targetScore)
	knockLimit := cfg.Opt(OptKnockLimit, v.knockLimit)
	bigGin := cfg.Opt(OptBigGin, module.BoolOpt(v.bigGin)) == module.OptOn
	lineBonuses := cfg.Opt(OptLineBonuses, module.BoolOpt(v.lineBonuses)) == module.OptOn

	steps := []module.LessonStep{
		module.Say("goal", "guide.ginrummy.goal", map[string]any{"n": 10}, module.AnchorHand).
			About("ginrummy.rules.deal"),
		module.Say("melds", "guide.ginrummy.melds", nil, module.AnchorHand).
			About("ginrummy.rules.setsAndRuns", "ginrummy.rules.aceLow"),
		module.Say("upcard", "guide.ginrummy.upcard", nil, module.AnchorDiscard).
			About("ginrummy.rules.upcard", "ginrummy.rules.upcardDance"),
		module.Do("firstDraw", "guide.ginrummy.firstDraw", nil, []string{VerbDraw, VerbPass},
			module.AnchorActionBar, module.AnchorDrawPile).
			About("ginrummy.rules.upcardDance", "ginrummy.rules.drawDiscard"),
		module.Do("discard", "guide.ginrummy.discard", nil, []string{VerbDraw, VerbDiscard},
			module.AnchorHand, module.AnchorActionBar).
			About("ginrummy.rules.drawDiscard"),
	}

	if knockLimit == oklahomaSentinel {
		steps = append(steps, module.Say("knock", "guide.ginrummy.knockOklahoma", nil, module.AnchorActionBar).
			About("ginrummy.rules.oklahoma", "ginrummy.rules.knockOnDiscard"))
	} else {
		steps = append(steps, module.Say("knock", "guide.ginrummy.knockLimit", map[string]any{"n": knockLimit}, module.AnchorActionBar).
			About("ginrummy.rules.knockLimit", "ginrummy.rules.knockOnDiscard"))
	}
	steps = append(steps,
		module.Say("gin", "guide.ginrummy.gin", nil, module.AnchorActionBar).About("ginrummy.rules.gin"),
	)
	if bigGin {
		steps = append(steps, module.Say("bigGin", "guide.ginrummy.bigGin", map[string]any{"n": 25}, module.AnchorHand).
			About("ginrummy.rules.bigGinBonus"))
	}
	steps = append(steps,
		module.Say("layoff", "guide.ginrummy.layoff", nil, module.AnchorTable).About("ginrummy.rules.layoffDescription"),
		module.Say("scoring", "guide.ginrummy.scoring", map[string]any{"n": 25}, module.AnchorScore).
			About("ginrummy.rules.undercut", "ginrummy.rules.ginBonus"),
		module.Say("deadHand", "guide.ginrummy.deadHand", nil, module.AnchorDrawPile).
			About("ginrummy.rules.deadHandDescription"),
		module.Say("target", "guide.ginrummy.target", map[string]any{"n": target}, module.AnchorScore).
			About("ginrummy.rules.target"),
	)
	if lineBonuses {
		steps = append(steps, module.Say("lineBonuses", "guide.ginrummy.lineBonuses",
			map[string]any{"box": 25, "game": 100, "shutout": 200}, module.AnchorScore).
			About("ginrummy.rules.box", "ginrummy.rules.gameBonus", "ginrummy.rules.shutout"))
	}
	steps = append(steps,
		module.Do("freeplay", "guide.ginrummy.freeplay", nil,
			[]string{VerbDraw, VerbPass, VerbDiscard, VerbKnock, VerbLayOff, VerbFinishLayoff},
			module.AnchorHand, module.AnchorActionBar).Times(4),
		module.Say("hint", "guide.ginrummy.hint", nil, module.AnchorHint),
		module.Say("done", "guide.ginrummy.done", nil),
	)

	return []module.Lesson{{
		ID:         "basics",
		TitleKey:   "guide.ginrummy.basics.title",
		SummaryKey: "guide.ginrummy.basics.summary",
		Seed:       1,
		Bots:       1,
		Steps:      steps,
	}}, nil
}
