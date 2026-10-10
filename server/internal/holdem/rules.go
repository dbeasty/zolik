package holdem

import "zolik/server/internal/module"

var _ module.RulesProvider = (*Module)(nil)

// Rules writes out the poker game's rules for one lobby's actual stack, blind and
// hand-limit choices, resolved the same way the engine resolves them.
func (m *Module) Rules(cfg module.MatchConfig) ([]module.RuleSection, error) {
	v := resolveVariation(cfg)
	stack := cfg.Opt(OptStartingStack, v.startingStack)
	bigBlind := cfg.Opt(OptBigBlind, v.bigBlind)
	handLimit := cfg.Opt(OptHandLimit, v.handLimit)

	// holdem.rules.mostChipsWins and .lastPlayerStanding are also used
	// param-free in Descriptor's Summary, so they stay param-free here too —
	// the hand count gets its own key rather than being folded into either.
	game := rulesFor(cfg.Variation)
	end := []module.Fact{{LabelKey: "holdem.rules.noLimit"}}
	if game.potLimit {
		end = []module.Fact{{LabelKey: "holdem.rules.potLimit"}}
	}
	if handLimit > 0 {
		end = append(end,
			module.Fact{LabelKey: "holdem.rules.handLimit", Params: map[string]any{"n": handLimit}},
			module.Fact{LabelKey: "holdem.rules.mostChipsWins"},
		)
	} else {
		end = append(end, module.Fact{LabelKey: "holdem.rules.lastPlayerStanding"})
	}

	setup := []module.Fact{
		{LabelKey: "holdem.rules.stack", Params: map[string]any{"n": stack}},
		{LabelKey: "holdem.rules.blinds", Params: map[string]any{"sb": bigBlind / 2, "bb": bigBlind}},
	}
	// The betting section is the same rules in either game, around a
	// different shape of hand: four rounds with a board, or two with a draw
	// between them.
	betting := []module.Fact{{LabelKey: "holdem.rules.streets"}}
	if game.useHole > 0 {
		setup = append(setup, module.Fact{LabelKey: "holdem.rules.omaha.deal"})
		betting = append(betting, module.Fact{LabelKey: "holdem.rules.omaha.useTwo"})
	}
	if game.draw {
		setup = append(setup, module.Fact{LabelKey: "holdem.rules.draw.deal"})
		betting = []module.Fact{
			{LabelKey: "holdem.rules.draw.streets"},
			{LabelKey: "holdem.rules.draw.draw", Params: map[string]any{"n": drawRules.maxDiscard}},
			{LabelKey: "holdem.rules.draw.public"},
		}
	}
	// Going all in is always allowed — except at a pot-limit table, where a
	// stack bigger than the pot can only go in as far as the pot. One
	// sentence per table rather than one with a hole in it.
	allIn := module.Fact{LabelKey: "holdem.rules.allIn"}
	if game.potLimit {
		allIn = module.Fact{LabelKey: "holdem.rules.omaha.allIn"}
	}
	betting = append(betting,
		module.Fact{LabelKey: "holdem.rules.checkOrCall"},
		module.Fact{LabelKey: "holdem.rules.minRaise"},
		allIn,
		module.Fact{LabelKey: "holdem.rules.foldedOut"},
		module.Fact{LabelKey: "holdem.rules.showdown"},
	)

	return []module.RuleSection{
		module.Section("holdem.rules.section.goal",
			module.Fact{LabelKey: "holdem.rules.goal"},
		),
		module.Section("holdem.rules.section.setup", setup...),
		// The mechanics of a betting round, and not only its shape.
		//
		// Written out because this is where every refusal in the game comes
		// from: "you can't check", "a raise has to be at least the last one",
		// "you don't have that many chips" are all one rule each, and none of
		// them was stated anywhere a player could read it before being told
		// no. See ruleindex.go, which points each of those codes here.
		module.Section("holdem.rules.section.betting", betting...),
		// What happens after the chips are pushed, which is a section this
		// game did not have because it used to be over by then.
		//
		// The reveal setting is stated here as a sentence per value rather
		// than one sentence with the value in it, because the two describe
		// genuinely different tables. The right to show your own hand is
		// stated unconditionally: it is true at both settings, and a rule that
		// only existed at one of them could not be pointed at by a refusal
		// that can happen at either.
		module.Section("holdem.rules.section.showdown",
			revealRule(cfg, v),
			module.Fact{LabelKey: "holdem.rules.showYourOwn"},
			module.Fact{LabelKey: "holdem.rules.showOnce"},
			module.Fact{LabelKey: "holdem.rules.stopEveryHand"},
		),
		module.Section("holdem.rules.section.end", end...),
		// Whether the chips in front of you are yours to take: the one rule
		// that decides what a player who has to go may do.
		module.Section("holdem.rules.section.table", tableRules(cfg)...),
	}, nil
}

// tableRules are the sentences for the table's format: a tournament played to
// the last chip, or a cash table anybody may get up from. Literal Facts per
// branch, for revealRule's reason.
func tableRules(cfg module.MatchConfig) []module.Fact {
	if cfg.Opt(OptFormat, FormatTournament) != FormatCash {
		return []module.Fact{{LabelKey: "holdem.rules.tournament"}}
	}
	out := []module.Fact{{LabelKey: "holdem.rules.cash.leave"}, {LabelKey: "holdem.rules.cash.join"}}
	if mins := int(cfg.LeaveAfterAway().Minutes()); mins > 0 {
		out = append(out, module.Fact{LabelKey: "holdem.rules.cash.away", Params: map[string]any{"minutes": mins}})
	} else {
		out = append(out, module.Fact{LabelKey: "holdem.rules.cash.awayNever"})
	}
	return out
}

// revealRule is the sentence for the reveal setting this table is playing.
//
// Two literal Facts rather than one built from a variable: `module.CollectKeys`
// reads this file rather than running it, and a key that only exists behind a
// local has no line in the manifest and therefore no line in any locale.
func revealRule(cfg module.MatchConfig, v variationDefaults) module.Fact {
	if cfg.Opt(OptShowdownReveal, RevealEveryone) == RevealWinners {
		return module.Fact{LabelKey: "holdem.rules.revealWinners"}
	}
	return module.Fact{LabelKey: "holdem.rules.revealEveryone"}
}
