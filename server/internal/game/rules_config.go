package game

import (
	"zolik/server/internal/models"
	"zolik/server/internal/rules"
)

// GameRules returns the ruleset a game actually runs under.
//
// The config is persisted on the document (models.Game.Rules) and is the only
// source consulted for a game written by a current server. It is deliberately
// NOT re-derived from the profile name on every load: doing that silently
// discarded per-game house-rule overrides, and meant that editing a constant
// in a shipped profile retroactively changed the rules of every game already
// in progress.
//
// Documents written before models.Game.Rules existed have no stored config, so
// they are migrated here: the named profile supplies the base, and the two
// legacy scalar columns (which were the only overridable knobs at the time)
// are layered back on top. The migrated value is written back to the document
// by fromRulesState on the game's next action.
func GameRules(g models.Game) rules.RulesConfig {
	if g.Rules != nil {
		return rules.ResolveConfig(toRulesConfig(*g.Rules))
	}
	cfg := rules.ResolveProfile(g.RulesProfile)
	cfg.InitialMeldMinimum = g.InitialMeldMinimum
	cfg.DiscardDrawMinRound = g.DiscardDrawMinRound
	return cfg
}

// setGameRules freezes cfg onto the document as the game's ruleset, keeping
// the profile name and the two legacy scalar columns mirrored so older
// readers (and the lobby REST responses) stay consistent with it.
func setGameRules(g *models.Game, cfg rules.RulesConfig) {
	cfg = rules.ResolveConfig(cfg)
	g.Rules = fromRulesConfig(cfg)
	g.InitialMeldMinimum = cfg.InitialMeldMinimum
	g.DiscardDrawMinRound = cfg.DiscardDrawMinRound
	if g.RulesProfile == "" {
		g.RulesProfile = cfg.Profile
	}
}

func toRulesConfig(c models.RulesConfig) rules.RulesConfig {
	return rules.RulesConfig{
		Profile:                c.Profile,
		DealSize:               c.DealSize,
		MinSetSize:             c.MinSetSize,
		MinRunSize:             c.MinRunSize,
		InitialMeldMinimum:     c.InitialMeldMinimum,
		DiscardDrawMinRound:    c.DiscardDrawMinRound,
		DiscardPickupMode:      rules.DiscardPickupMode(c.DiscardPickupMode),
		JokerDiscardRestricted: c.JokerDiscardRestricted,
		FixedDealCount:         c.FixedDealCount,
		StaticContract: rules.ContractRequirement{
			Sets:            c.StaticContract.Sets,
			Runs:            c.StaticContract.Runs,
			RequireCleanRun: c.StaticContract.RequireCleanRun,
		},
		MatchEndMode: rules.MatchEndMode(c.MatchEndMode),
		TargetScore:  c.TargetScore,
	}
}

func fromRulesConfig(c rules.RulesConfig) *models.RulesConfig {
	return &models.RulesConfig{
		Profile:                c.Profile,
		DealSize:               c.DealSize,
		MinSetSize:             c.MinSetSize,
		MinRunSize:             c.MinRunSize,
		InitialMeldMinimum:     c.InitialMeldMinimum,
		DiscardDrawMinRound:    c.DiscardDrawMinRound,
		DiscardPickupMode:      string(c.DiscardPickupMode),
		JokerDiscardRestricted: c.JokerDiscardRestricted,
		FixedDealCount:         c.FixedDealCount,
		StaticContract: models.ContractRequirement{
			Sets:            c.StaticContract.Sets,
			Runs:            c.StaticContract.Runs,
			RequireCleanRun: c.StaticContract.RequireCleanRun,
		},
		MatchEndMode: string(c.MatchEndMode),
		TargetScore:  c.TargetScore,
	}
}

// applyRuleOverrides layers the host's lobby settings onto a base profile.
// Both are optional; a nil pointer leaves the profile's own default in place.
// Profile is left naming the base variation, since that is what the clients
// key their rule summaries off — the overrides themselves are sent alongside.
func applyRuleOverrides(cfg rules.RulesConfig, initialMeldMinimum, discardDrawMinRound *int) rules.RulesConfig {
	if initialMeldMinimum != nil {
		cfg.InitialMeldMinimum = *initialMeldMinimum
	}
	if discardDrawMinRound != nil {
		cfg.DiscardDrawMinRound = *discardDrawMinRound
	}
	return cfg
}
