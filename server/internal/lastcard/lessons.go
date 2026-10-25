package lastcard

import "zolik/server/internal/module"

var _ module.LessonProvider = (*Module)(nil)

// Lessons is Last Card's guided walk-through. Every house rule and the
// scoring are read from the table's own options, so a step appears only
// where the rule is played and every number is the table's.
func (m *Module) Lessons(cfg module.MatchConfig) ([]module.Lesson, error) {
	handSize := cfg.Opt(OptHandSize, defaultHandSize)
	target := cfg.Opt(OptTargetScore, defaultTargetScore)
	call := cfg.Opt(OptLastCardCall, module.OptOn) == module.OptOn
	challenge := cfg.Opt(OptDrawFourChallenge, module.OptOn) == module.OptOn
	stacking := cfg.Opt(OptStacking, stackOff)
	drawUntil := cfg.Opt(OptDrawUntilPlayable, module.OptOff) == module.OptOn
	sevenZero := cfg.Opt(OptSevenZero, module.OptOff) == module.OptOn
	swap := cfg.Opt(OptSwapCards, module.OptOff) == module.OptOn
	mayhem := cfg.Opt(OptMayhemCards, module.OptOff) == module.OptOn
	lowest := cfg.Opt(OptScoring, scoreWinnerTakes) == scoreLowest

	tp := map[string]any{"target": target}
	goalRule := "lastcard.rules.goal"
	switch {
	case target > 0 && lowest:
		goalRule = "lastcard.rules.goal.lowest"
	case target > 0:
		goalRule = "lastcard.rules.goal.points"
	}

	steps := []module.LessonStep{
		module.Say("goal", "guide.lastcard.goal", map[string]any{"n": handSize}, module.AnchorHand).
			About(goalRule, "lastcard.rules.deal"),
		module.Say("pile", "guide.lastcard.pile", nil, module.AnchorDiscard).
			About("lastcard.rules.turn.match"),
		module.Do("play", "guide.lastcard.play", nil, []string{VerbPlay, VerbDraw},
			module.AnchorHand, module.AnchorActionBar).
			About("lastcard.rules.turn.match"),
	}

	if drawUntil {
		steps = append(steps, module.Say("draw", "guide.lastcard.drawUntil", nil, module.AnchorDrawPile).
			About("lastcard.rules.turn.drawUntil"))
	} else {
		steps = append(steps, module.Say("draw", "guide.lastcard.draw", nil, module.AnchorDrawPile).
			About("lastcard.rules.turn.draw"))
	}

	steps = append(steps,
		module.Say("actions", "guide.lastcard.actions", nil, module.AnchorHand).
			About("lastcard.rules.skip", "lastcard.rules.reverse", "lastcard.rules.drawTwo"),
	)
	if challenge {
		steps = append(steps,
			module.Say("wilds", "guide.lastcard.wildsBluff", nil, module.AnchorHand).
				About("lastcard.rules.wild", "lastcard.rules.wildDrawFourBluff"),
			module.Say("challenge", "guide.lastcard.challenge", nil, module.AnchorActionBar).
				About("lastcard.rules.challenge"),
		)
	} else {
		steps = append(steps, module.Say("wilds", "guide.lastcard.wilds", nil, module.AnchorHand).
			About("lastcard.rules.wild", "lastcard.rules.wildDrawFour"))
	}

	switch stacking {
	case stackTwos:
		steps = append(steps, module.Say("stacking", "guide.lastcard.stackingTwos", nil, module.AnchorHand).
			About("lastcard.rules.stacking.twos"))
	case stackAny:
		steps = append(steps, module.Say("stacking", "guide.lastcard.stackingAny", nil, module.AnchorHand).
			About("lastcard.rules.stacking.any"))
	}
	if sevenZero {
		steps = append(steps, module.Say("sevenZero", "guide.lastcard.sevenZero", nil, module.AnchorHand).
			About("lastcard.rules.sevens", "lastcard.rules.zeros"))
	}
	if swap {
		steps = append(steps, module.Say("wildSwap", "guide.lastcard.wildSwap", nil, module.AnchorHand).
			About("lastcard.rules.wildSwap"))
	}
	if mayhem {
		steps = append(steps, module.Say("wildMayhem", "guide.lastcard.wildMayhem", nil, module.AnchorHand).
			About("lastcard.rules.wildMayhem"))
	}
	if call {
		steps = append(steps, module.Say("call", "guide.lastcard.call", nil, module.AnchorActionBar).
			About("lastcard.rules.call", "lastcard.rules.catch"))
	}

	switch {
	case target > 0 && lowest:
		steps = append(steps, module.Say("scoring", "guide.lastcard.scoringLowest", tp, module.AnchorScore).
			About("lastcard.rules.scoring.lowest", "lastcard.rules.values"))
	case target > 0:
		steps = append(steps, module.Say("scoring", "guide.lastcard.scoring", tp, module.AnchorScore).
			About("lastcard.rules.scoring", "lastcard.rules.values"))
	default:
		steps = append(steps, module.Say("scoring", "guide.lastcard.single", nil, module.AnchorScore).
			About("lastcard.rules.end"))
	}

	free := []string{VerbPlay, VerbDraw, VerbPass}
	if call {
		free = append(free, VerbCall, VerbCatch)
	}
	if challenge {
		free = append(free, VerbChallenge, VerbAccept)
	}
	steps = append(steps,
		module.Do("freeplay", "guide.lastcard.freeplay", nil, free,
			module.AnchorHand, module.AnchorActionBar).Times(3),
		module.Say("hint", "guide.lastcard.hint", nil, module.AnchorHint),
		module.Say("done", "guide.lastcard.done", nil),
	)

	return []module.Lesson{{
		ID:         "basics",
		TitleKey:   "guide.lastcard.basics.title",
		SummaryKey: "guide.lastcard.basics.summary",
		Seed:       1,
		Bots:       1,
		Steps:      steps,
	}}, nil
}
