package rummytiles

import "zolik/server/internal/module"

var _ module.LessonProvider = (*Module)(nil)

// Lessons is Rummy Tiles' guided walk-through. The match goal and the
// pool-exhaustion rule follow the table's options, resolved the way Rules(cfg)
// resolves them.
func (m *Module) Lessons(cfg module.MatchConfig) ([]module.Lesson, error) {
	v := resolveVariation(cfg)
	target := cfg.Opt(OptTargetScore, v.targetScore)
	roundLimit := cfg.Opt(OptRoundLimit, v.roundLimit)
	poolLowestWins := cfg.Opt(OptPoolExhaustion, module.BoolOpt(v.poolExhaustionLowestWins)) == module.OptOn

	steps := []module.LessonStep{
		module.Say("goal", "guide.rummytiles.goal", nil, module.AnchorHand).
			About("rummytiles.rules.goingOut"),
		module.Say("setup", "guide.rummytiles.setup", map[string]any{"n": 14}, module.AnchorHand, module.AnchorDrawPile).
			About("rummytiles.rules.dealCount", "rummytiles.rules.tiles"),
		module.Say("sets", "guide.rummytiles.sets", nil, module.AnchorTable).
			About("rummytiles.rules.group", "rummytiles.rules.run", "rummytiles.rules.noWrap"),
		module.Say("joker", "guide.rummytiles.joker", nil, module.AnchorTable).
			About("rummytiles.rules.joker"),
		module.Say("meld", "guide.rummytiles.meld", map[string]any{"n": initialMeldFloor}, module.AnchorTable).
			About("rummytiles.rules.initialMeldDescription"),
		module.Do("turn", "guide.rummytiles.turn", nil, []string{VerbPlace, VerbCommit, VerbDraw},
			module.AnchorHand, module.AnchorActionBar).
			About("rummytiles.rules.turnDescription", "rummytiles.rules.noDiscard"),
		module.Say("nodiscard", "guide.rummytiles.nodiscard", nil, module.AnchorDrawPile).
			About("rummytiles.rules.noDiscard"),
		module.Say("rearrange", "guide.rummytiles.rearrange", nil, module.AnchorTable, module.AnchorActionBar).
			About("rummytiles.rules.turnDescription"),
		module.Say("jokerTaking", "guide.rummytiles.jokerTaking", nil, module.AnchorTable).
			About("rummytiles.rules.jokerTakingDescription"),
		module.Do("freeplay", "guide.rummytiles.freeplay", nil,
			[]string{VerbPlace, VerbAdd, VerbTake, VerbSplit, VerbSwapJoker, VerbResetTurn, VerbCommit, VerbDraw},
			module.AnchorHand, module.AnchorActionBar).Times(3),
	}

	if poolLowestWins {
		steps = append(steps, module.Say("pool", "guide.rummytiles.poolLowest", nil, module.AnchorDrawPile).
			About("rummytiles.rules.poolExhaustionLowestWins"))
	} else {
		steps = append(steps, module.Say("pool", "guide.rummytiles.poolNone", nil, module.AnchorDrawPile).
			About("rummytiles.rules.poolExhaustionNoWinner"))
	}

	if target > 0 {
		steps = append(steps, module.Say("match", "guide.rummytiles.target", map[string]any{"n": target}, module.AnchorScore).
			About("rummytiles.rules.target"))
	} else {
		steps = append(steps, module.Say("match", "guide.rummytiles.roundLimit", map[string]any{"n": roundLimit}, module.AnchorScore).
			About("rummytiles.rules.roundLimit"))
	}

	steps = append(steps,
		module.Say("hint", "guide.rummytiles.hint", nil, module.AnchorHint),
		module.Say("done", "guide.rummytiles.done", nil),
	)

	return []module.Lesson{{
		ID:         "basics",
		TitleKey:   "guide.rummytiles.basics.title",
		SummaryKey: "guide.rummytiles.basics.summary",
		Seed:       1,
		Bots:       1,
		Steps:      steps,
	}}, nil
}
