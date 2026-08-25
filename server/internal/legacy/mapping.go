package legacy

// The thirty-field hand-mapping between the legacy document and the engine.
//
// This is the code the module split exists to delete: two representations of
// one game, kept in step by hand, where forgetting a field is a silent bug.
// It survives here only because the migration has to read the old shape once.
// Nothing else calls it, and when the last `games` collection is gone this
// file goes with the package.

import (
	"zolik/server/internal/rules"
)

func toRulesLayOffSnapshot(s *LayOffSnapshot) *rules.LayOffSnapshot {
	if s == nil {
		return nil
	}
	return &rules.LayOffSnapshot{
		PlayerID:  s.PlayerID,
		MeldID:    s.MeldID,
		PrevCards: s.PrevCards,
		PrevMeta: rules.MeldInfo{
			MeldID:    s.PrevMeta.MeldID,
			Type:      rules.MeldType(s.PrevMeta.Type),
			OwnerID:   s.PrevMeta.OwnerID,
			WildCount: s.PrevMeta.WildCount,
		},
		Cards:                s.Cards,
		PrevDiscardTakenCard: s.PrevDiscardTakenCard,
		PrevOwnerReqMet:      s.PrevOwnerReqMet,
	}
}

func fromRulesLayOffSnapshot(s *rules.LayOffSnapshot) *LayOffSnapshot {
	if s == nil {
		return nil
	}
	return &LayOffSnapshot{
		PlayerID:  s.PlayerID,
		MeldID:    s.MeldID,
		PrevCards: s.PrevCards,
		PrevMeta: MeldInfo{
			MeldID:    s.PrevMeta.MeldID,
			Type:      string(s.PrevMeta.Type),
			OwnerID:   s.PrevMeta.OwnerID,
			WildCount: s.PrevMeta.WildCount,
		},
		Cards:                s.Cards,
		PrevDiscardTakenCard: s.PrevDiscardTakenCard,
		PrevOwnerReqMet:      s.PrevOwnerReqMet,
	}
}

func toRulesMeldLaidSnapshot(s *MeldLaidSnapshot) *rules.MeldLaidSnapshot {
	if s == nil {
		return nil
	}
	return &rules.MeldLaidSnapshot{
		PlayerID:                        s.PlayerID,
		MeldID:                          s.MeldID,
		Cards:                           s.Cards,
		PrevRoundReqMet:                 s.PrevRoundReqMet,
		PrevMeldsLaidThisTurn:           s.PrevMeldsLaidThisTurn,
		PrevDiscardDrawnCardPendingMeld: s.PrevDiscardDrawnCardPendingMeld,
		PrevDiscardTakenCard:            s.PrevDiscardTakenCard,
	}
}

func fromRulesMeldLaidSnapshot(s *rules.MeldLaidSnapshot) *MeldLaidSnapshot {
	if s == nil {
		return nil
	}
	return &MeldLaidSnapshot{
		PlayerID:                        s.PlayerID,
		MeldID:                          s.MeldID,
		Cards:                           s.Cards,
		PrevRoundReqMet:                 s.PrevRoundReqMet,
		PrevMeldsLaidThisTurn:           s.PrevMeldsLaidThisTurn,
		PrevDiscardDrawnCardPendingMeld: s.PrevDiscardDrawnCardPendingMeld,
		PrevDiscardTakenCard:            s.PrevDiscardTakenCard,
	}
}

func toRulesTurnMeldSnapshot(s *TurnMeldSnapshot) *rules.TurnMeldSnapshot {
	if s == nil {
		return nil
	}
	meldMeta := map[string][]rules.MeldInfo{}
	for owner, infos := range s.MeldMeta {
		for _, mi := range infos {
			meldMeta[owner] = append(meldMeta[owner], rules.MeldInfo{
				MeldID:    mi.MeldID,
				Type:      rules.MeldType(mi.Type),
				OwnerID:   mi.OwnerID,
				WildCount: mi.WildCount,
			})
		}
	}
	return &rules.TurnMeldSnapshot{
		PlayerID:                    s.PlayerID,
		Hands:                       s.Hands,
		Melds:                       s.Melds,
		MeldMeta:                    meldMeta,
		RoundReqMet:                 s.RoundReqMet,
		AllRoundReqMet:              s.AllRoundReqMet,
		MeldsLaidThisTurn:           s.MeldsLaidThisTurn,
		DiscardDrawnCardPendingMeld: s.DiscardDrawnCardPendingMeld,
		DiscardTakenCard:            s.DiscardTakenCard,
		DiscardDrawnCards:           s.DiscardDrawnCards,
		DiscardPile:                 s.DiscardPile,
		NextMeldSeq:                 s.NextMeldSeq,
	}
}

func fromRulesTurnMeldSnapshot(s *rules.TurnMeldSnapshot) *TurnMeldSnapshot {
	if s == nil {
		return nil
	}
	meldMeta := map[string][]MeldInfo{}
	for owner, infos := range s.MeldMeta {
		for _, mi := range infos {
			meldMeta[owner] = append(meldMeta[owner], MeldInfo{
				MeldID:    mi.MeldID,
				Type:      string(mi.Type),
				OwnerID:   mi.OwnerID,
				WildCount: mi.WildCount,
			})
		}
	}
	return &TurnMeldSnapshot{
		PlayerID:                    s.PlayerID,
		Hands:                       s.Hands,
		Melds:                       s.Melds,
		MeldMeta:                    meldMeta,
		RoundReqMet:                 s.RoundReqMet,
		AllRoundReqMet:              s.AllRoundReqMet,
		MeldsLaidThisTurn:           s.MeldsLaidThisTurn,
		DiscardDrawnCardPendingMeld: s.DiscardDrawnCardPendingMeld,
		DiscardTakenCard:            s.DiscardTakenCard,
		DiscardDrawnCards:           s.DiscardDrawnCards,
		DiscardPile:                 s.DiscardPile,
		NextMeldSeq:                 s.NextMeldSeq,
	}
}

// GameRules returns the ruleset a game actually runs under.
//
// The config is persisted on the document (Game.Rules) and is the only
// source consulted for a game written by a current server. It is deliberately
// NOT re-derived from the profile name on every load: doing that silently
// discarded per-game house-rule overrides, and meant that editing a constant
// in a shipped profile retroactively changed the rules of every game already
// in progress.
//
// Documents written before Game.Rules existed have no stored config, so
// they are migrated here: the named profile supplies the base, and the two
// legacy scalar columns (which were the only overridable knobs at the time)
// are layered back on top. The migrated value is written back to the document
// by fromRulesState on the game's next action.
func GameRules(g Game) rules.RulesConfig {
	if g.Rules != nil {
		return rules.ResolveConfig(toRulesConfig(*g.Rules))
	}
	cfg := rules.ResolveProfile(g.RulesProfile)
	cfg.InitialMeldMinimum = g.InitialMeldMinimum
	cfg.DiscardDrawMinRound = g.DiscardDrawMinRound
	return cfg
}

func toRulesConfig(c RulesConfig) rules.RulesConfig {
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

func fromRulesConfig(c rules.RulesConfig) *RulesConfig {
	return &RulesConfig{
		Profile:                c.Profile,
		DealSize:               c.DealSize,
		MinSetSize:             c.MinSetSize,
		MinRunSize:             c.MinRunSize,
		InitialMeldMinimum:     c.InitialMeldMinimum,
		DiscardDrawMinRound:    c.DiscardDrawMinRound,
		DiscardPickupMode:      string(c.DiscardPickupMode),
		JokerDiscardRestricted: c.JokerDiscardRestricted,
		FixedDealCount:         c.FixedDealCount,
		StaticContract: ContractRequirement{
			Sets:            c.StaticContract.Sets,
			Runs:            c.StaticContract.Runs,
			RequireCleanRun: c.StaticContract.RequireCleanRun,
		},
		MatchEndMode: string(c.MatchEndMode),
		TargetScore:  c.TargetScore,
	}
}
