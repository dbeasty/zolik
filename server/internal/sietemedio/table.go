package sietemedio

import "zolik/server/internal/module"

// tableOf resolves a lobby's variation and options into the table a match will
// actually be played on.
//
// One function, called by three: NewMatch freezes it onto the match, Rules
// writes it out in sentences, and ExplainRefusal points a refusal at those
// sentences. Resolving the options separately in each of them is how a rules
// screen ends up describing a table nobody is sitting at — the split brain
// extensibility-plan.md Phase 0 closed for the first game, kept shut here by
// construction rather than by care.
func tableOf(cfg module.MatchConfig) *GameState {
	v := resolveVariation(cfg)
	return &GameState{
		Variation:     cfg.Variation,
		MinStake:      cfg.Opt(OptMinStake, v.minStake),
		StartingStack: cfg.Opt(OptStartingStack, v.startingStack),
		RoundLimit:    cfg.Opt(OptRounds, v.rounds),
		TiesToBank:    cfg.Opt(OptTies, v.ties) == tiesToBank,
		SevenDoubles:  cfg.Opt(OptSevenPays, v.sevenPays) == sevenPaysDouble,
		BankPass:      cfg.Opt(OptBank, v.bank),
		FirstCardDown: cfg.Opt(OptFirstCardDown, module.BoolOpt(v.firstCardDown)) == module.OptOn,
		Pause:         cfg.PauseBetweenRounds(true),
	}
}

func resolveVariation(cfg module.MatchConfig) variationDefaults {
	if v, ok := variations[cfg.Variation]; ok {
		return v
	}
	return variations[VarMexicana]
}
