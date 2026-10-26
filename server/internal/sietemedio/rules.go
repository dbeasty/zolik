package sietemedio

import "zolik/server/internal/module"

// Rules writes this table's rules out, resolved against the very same option
// values the engine will play by — see tableOf.
//
// Every house rule gets a sentence with its own number or its own branch in
// it, because the three things the sources disagree about — who takes a tie,
// what 7½ collects, how the bank moves — are not details a player can find out
// by trying. At this table the answer is a setting, and a rules screen that
// described a default would be describing somebody else's game.
func (m *Module) Rules(cfg module.MatchConfig) ([]module.RuleSection, error) {
	t := tableOf(cfg)

	table := []module.RuleItem{
		module.Rule("sietemedio.rules.pack", nil),
		module.Rule("sietemedio.rules.values", nil),
		module.Rule("sietemedio.rules.stack", map[string]any{"n": t.StartingStack}),
		module.Rule("sietemedio.rules.minStake", map[string]any{"n": t.MinStake}),
		module.Rule("sietemedio.rules.bankCovers", nil),
		module.Rule("sietemedio.rules.bankDoesNotStake", nil),
	}

	play := []module.RuleItem{
		module.Rule("sietemedio.rules.goal", nil),
		// The shape of a round, written down because it is what every
		// WRONG_PHASE in this game is about: a stake put up after the cards
		// are out, a card asked for while the table is still staking, a hand
		// played while the bank is drawing.
		module.Rule("sietemedio.rules.roundOrder", nil),
		module.Rule("sietemedio.rules.oneStakePerRound", nil),
		module.Rule("sietemedio.rules.stakeFromStack", nil),
	}
	if t.FirstCardDown {
		play = append(play, module.Rule("sietemedio.rules.firstCardDown", nil))
	} else {
		play = append(play, module.Rule("sietemedio.rules.allFaceUp", nil))
	}
	play = append(play,
		module.Rule("sietemedio.rules.hitStand", nil),
		module.Rule("sietemedio.rules.bust", nil),
		module.Rule("sietemedio.rules.sevenStands", nil),
	)

	bank := []module.RuleItem{
		module.Rule("sietemedio.rules.bankLast", nil),
		module.Rule("sietemedio.rules.bankBust", nil),
	}
	if t.TiesToBank {
		bank = append(bank, module.Rule("sietemedio.rules.tiesToBank", nil))
	} else {
		bank = append(bank, module.Rule("sietemedio.rules.tiesPush", nil))
	}
	if t.SevenDoubles {
		bank = append(bank, module.Rule("sietemedio.rules.sevenPaysDouble", nil))
	} else {
		bank = append(bank, module.Rule("sietemedio.rules.sevenPaysEven", nil))
	}
	bank = append(bank, bankMovesRule(t))

	return []module.RuleSection{
		module.SectionOf("sietemedio.rules.section.table", table),
		module.SectionOf("sietemedio.rules.section.play", play),
		module.SectionOf("sietemedio.rules.section.bank", bank),
		module.SectionOf("sietemedio.rules.section.end", []module.RuleItem{
			module.Rule("sietemedio.rules.rounds", map[string]any{"n": t.RoundLimit}),
			module.Rule("sietemedio.rules.mostChipsWins", nil),
			module.Rule("sietemedio.rules.brokeOut", map[string]any{"n": t.MinStake}),
		}),
		module.SectionOf("sietemedio.rules.section.leaving", leaveRules(cfg)),
	}, nil
}

// bankMovesRule is the one sentence with three spellings. A key per rule
// rather than one key with a word substituted into it, because "the bank goes
// to whoever makes seven and a half" and "the bank goes round the table" are
// different sentences in a language that inflects, and a locale bundle is
// where that difference belongs.
func bankMovesRule(t *GameState) module.RuleItem {
	switch t.BankPass {
	case bankRound:
		return module.Rule("sietemedio.rules.bankRound", nil)
	case bankFixed:
		return module.Rule("sietemedio.rules.bankFixed", nil)
	default:
		return module.Rule("sietemedio.rules.bankOnSeven", nil)
	}
}

// leaveRules say when a player may get up, and what happens to one who has
// gone away. A literal key per branch: see bankMovesRule.
func leaveRules(cfg module.MatchConfig) []module.RuleItem {
	out := []module.RuleItem{module.Rule("sietemedio.rules.leave", nil)}
	if mins := int(cfg.LeaveAfterAway().Minutes()); mins > 0 {
		return append(out, module.Rule("sietemedio.rules.leaveAway", map[string]any{"minutes": mins}))
	}
	return append(out, module.Rule("sietemedio.rules.leaveAwayNever", nil))
}
