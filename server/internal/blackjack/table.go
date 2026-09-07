package blackjack

import "zolik/server/internal/module"

// tableOf resolves a lobby's variation and options into the table a match
// will actually be played on.
//
// One function, called by three: NewMatch freezes it onto the match, Rules
// writes it out in sentences, and ExplainRefusal points a refusal at those
// sentences. Resolving the options separately in each of them is how a rules
// screen ends up describing a table nobody is sitting at — the exact split
// brain `extensibility-plan.md` Phase 0 closed for the first game, kept shut
// here by construction rather than by care.
func tableOf(cfg module.MatchConfig) *GameState {
	v := resolveVariation(cfg)
	return &GameState{
		Variation:        cfg.Variation,
		Decks:            cfg.Opt(OptDecks, v.decks),
		MinBet:           cfg.Opt(OptMinBet, v.minBet),
		StartingStack:    cfg.Opt(OptStartingStack, v.startingStack),
		RoundLimit:       cfg.Opt(OptRounds, v.rounds),
		DealerHitsSoft17: cfg.Opt(OptDealerHitsSoft17, module.BoolOpt(v.hitsSoft17)) == module.OptOn,
		BlackjackPayout:  cfg.Opt(OptBlackjackPays, v.blackjackPays),
		AllowSurrender:   cfg.Opt(OptSurrender, module.BoolOpt(v.surrender)) == module.OptOn,
		DoubleAfterSplit: cfg.Opt(OptDoubleAfterSplit, module.BoolOpt(v.doubleAfterSplit)) == module.OptOn,
		MaxSplits:        cfg.Opt(OptMaxSplits, v.maxSplits),
		AllowInsurance:   cfg.Opt(OptInsurance, module.BoolOpt(v.insurance)) == module.OptOn,
		Pause:            cfg.PauseBetweenRounds(true),
	}
}
