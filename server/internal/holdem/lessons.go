package holdem

import "zolik/server/internal/module"

var _ module.LessonProvider = (*Module)(nil)

// Lessons is poker's guided walk-through. Stack, blinds, hand count, table
// format, reveal setting and the game itself (Hold'em, Omaha, Draw) all come
// from the same resolution Rules(cfg) uses, so each step says what this
// table actually plays.
func (m *Module) Lessons(cfg module.MatchConfig) ([]module.Lesson, error) {
	v := resolveVariation(cfg)
	game := rulesFor(cfg.Variation)
	stack := cfg.Opt(OptStartingStack, v.startingStack)
	bigBlind := cfg.Opt(OptBigBlind, v.bigBlind)
	handLimit := cfg.Opt(OptHandLimit, v.handLimit)
	cash := cfg.Opt(OptFormat, FormatTournament) == FormatCash
	winnersOnly := cfg.Opt(OptShowdownReveal, RevealEveryone) == RevealWinners
	blinds := map[string]any{"n": stack, "sb": bigBlind / 2, "bb": bigBlind}

	betVerbs := []string{VerbCheck, VerbCall, VerbRaise, VerbFold}
	allVerbs := betVerbs
	if game.draw {
		allVerbs = append(append([]string{}, betVerbs...), VerbStand, VerbDiscard)
	}

	steps := []module.LessonStep{
		module.Say("goal", "guide.holdem.goal", nil, module.AnchorTable).About("holdem.rules.goal"),
	}

	switch {
	case game.draw:
		steps = append(steps,
			module.Say("setup", "guide.holdem.setup.draw", blinds, module.AnchorHand, module.AnchorScore).
				About("holdem.rules.stack", "holdem.rules.blinds", "holdem.rules.draw.deal"),
			module.Say("streets", "guide.holdem.streets.draw", nil, module.AnchorTable).
				About("holdem.rules.draw.streets"),
		)
	case game.useHole > 0:
		steps = append(steps,
			module.Say("setup", "guide.holdem.setup.omaha", blinds, module.AnchorHand, module.AnchorScore).
				About("holdem.rules.stack", "holdem.rules.blinds", "holdem.rules.omaha.deal"),
			module.Say("streets", "guide.holdem.streets", nil, module.AnchorTable).
				About("holdem.rules.streets"),
		)
	default:
		steps = append(steps,
			module.Say("setup", "guide.holdem.setup", blinds, module.AnchorHand, module.AnchorScore).
				About("holdem.rules.stack", "holdem.rules.blinds"),
			module.Say("streets", "guide.holdem.streets", nil, module.AnchorTable).
				About("holdem.rules.streets"),
		)
	}

	steps = append(steps,
		module.Do("bet", "guide.holdem.bet", nil, betVerbs, module.AnchorActionBar).
			About("holdem.rules.checkOrCall", "holdem.rules.minRaise"),
		module.Say("checkcall", "guide.holdem.checkcall", nil, module.AnchorActionBar).
			About("holdem.rules.checkOrCall"),
		module.Say("raise", "guide.holdem.raise", nil, module.AnchorActionBar).
			About("holdem.rules.minRaise"),
	)

	if game.potLimit {
		steps = append(steps,
			module.Say("potlimit", "guide.holdem.potlimit", nil, module.AnchorActionBar).
				About("holdem.rules.potLimit", "holdem.rules.omaha.allIn"),
			module.Say("usetwo", "guide.holdem.usetwo", nil, module.AnchorHand).
				About("holdem.rules.omaha.useTwo"),
		)
	} else {
		steps = append(steps,
			module.Say("allin", "guide.holdem.allin", nil, module.AnchorActionBar).
				About("holdem.rules.noLimit", "holdem.rules.allIn"),
		)
	}

	if game.draw {
		steps = append(steps,
			module.Say("draw", "guide.holdem.draw", map[string]any{"n": drawRules.maxDiscard}, module.AnchorHand).
				About("holdem.rules.draw.draw", "holdem.rules.draw.public"),
			module.Do("swap", "guide.holdem.swap", nil, []string{VerbStand, VerbDiscard},
				module.AnchorHand, module.AnchorActionBar).
				About("holdem.rules.draw.draw"),
		)
	}

	steps = append(steps,
		module.Say("fold", "guide.holdem.fold", nil, module.AnchorActionBar).About("holdem.rules.foldedOut"),
		module.Say("showdown", "guide.holdem.showdown", nil, module.AnchorTable).About("holdem.rules.showdown"),
	)
	if winnersOnly {
		steps = append(steps, module.Say("reveal", "guide.holdem.reveal.winners", nil, module.AnchorTable).
			About("holdem.rules.revealWinners", "holdem.rules.showYourOwn"))
	} else {
		steps = append(steps, module.Say("reveal", "guide.holdem.reveal.everyone", nil, module.AnchorTable).
			About("holdem.rules.revealEveryone", "holdem.rules.showYourOwn"))
	}

	steps = append(steps,
		module.Do("freeplay", "guide.holdem.freeplay", nil, allVerbs, module.AnchorActionBar).Times(4),
	)

	if handLimit > 0 {
		steps = append(steps, module.Say("end", "guide.holdem.end.hands", map[string]any{"n": handLimit}, module.AnchorScore).
			About("holdem.rules.handLimit", "holdem.rules.mostChipsWins"))
	} else {
		steps = append(steps, module.Say("end", "guide.holdem.end.lastStanding", nil, module.AnchorScore).
			About("holdem.rules.lastPlayerStanding"))
	}
	if cash {
		steps = append(steps, module.Say("table", "guide.holdem.table.cash", nil, module.AnchorStatus).
			About("holdem.rules.cash.leave"))
	} else {
		steps = append(steps, module.Say("table", "guide.holdem.table.tournament", nil, module.AnchorStatus).
			About("holdem.rules.tournament"))
	}

	steps = append(steps,
		module.Say("hint", "guide.holdem.hint", nil, module.AnchorHint),
		module.Say("done", "guide.holdem.done", nil),
	)

	return []module.Lesson{{
		ID:         "basics",
		TitleKey:   "guide.holdem.basics.title",
		SummaryKey: "guide.holdem.basics.summary",
		Seed:       1,
		Bots:       2,
		Steps:      steps,
	}}, nil
}
