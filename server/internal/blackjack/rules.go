package blackjack

import "zolik/server/internal/module"

var (
	_ module.RulesProvider     = (*Module)(nil)
	_ module.RuleIndexProvider = (*Module)(nil)
)

// Rules writes this table's rules out, resolved against the very same option
// values the engine will play by — see tableOf.
//
// Every house rule gets a sentence with its own number in it, because "may I
// double after splitting" is not a detail a player can find out by trying: at
// this table the answer is a setting, and a rules screen that described a
// default would be describing somebody else's game.
func (m *Module) Rules(cfg module.MatchConfig) ([]module.RuleSection, error) {
	t := tableOf(cfg)

	play := []module.RuleItem{
		module.Rule("blackjack.rules.goal", nil),
		module.Rule("blackjack.rules.hitStand", nil),
		module.Rule("blackjack.rules.aces", nil),
		module.Rule("blackjack.rules.blackjack", nil),
		payoutRule(t),
		module.Rule("blackjack.rules.double", nil),
	}
	if t.DoubleAfterSplit {
		play = append(play, module.Rule("blackjack.rules.doubleAfterSplit", nil))
	} else {
		play = append(play, module.Rule("blackjack.rules.noDoubleAfterSplit", nil))
	}
	if t.MaxSplits > 0 {
		play = append(play,
			module.Rule("blackjack.rules.split", map[string]any{"n": t.MaxSplits, "hands": t.MaxSplits + 1}),
			module.Rule("blackjack.rules.splitAces", nil),
		)
	} else {
		play = append(play, module.Rule("blackjack.rules.noSplit", nil))
	}
	if t.AllowSurrender {
		play = append(play, module.Rule("blackjack.rules.surrender", nil))
	} else {
		play = append(play, module.Rule("blackjack.rules.noSurrender", nil))
	}

	dealer := []module.RuleItem{module.Rule("blackjack.rules.dealerDraws", nil)}
	if t.DealerHitsSoft17 {
		dealer = append(dealer, module.Rule("blackjack.rules.hitsSoft17", nil))
	} else {
		dealer = append(dealer, module.Rule("blackjack.rules.standsSoft17", nil))
	}
	dealer = append(dealer, module.Rule("blackjack.rules.dealerPeeks", nil))
	if t.AllowInsurance {
		dealer = append(dealer, module.Rule("blackjack.rules.insurance", nil))
	} else {
		dealer = append(dealer, module.Rule("blackjack.rules.noInsurance", nil))
	}

	return []module.RuleSection{
		module.SectionOf("blackjack.rules.section.table", []module.RuleItem{
			module.Rule("blackjack.rules.decks", map[string]any{"n": t.Decks}),
			module.Rule("blackjack.rules.stack", map[string]any{"n": t.StartingStack}),
			module.Rule("blackjack.rules.minBet", map[string]any{"n": t.MinBet}),
			module.Rule("blackjack.rules.faceUp", nil),
		}),
		module.SectionOf("blackjack.rules.section.play", play),
		module.SectionOf("blackjack.rules.section.dealer", dealer),
		module.SectionOf("blackjack.rules.section.end", []module.RuleItem{
			module.Rule("blackjack.rules.rounds", map[string]any{"n": t.RoundLimit}),
			module.Rule("blackjack.rules.mostChipsWins", nil),
			module.Rule("blackjack.rules.bustedOut", map[string]any{"n": t.MinBet}),
		}),
	}, nil
}

// payoutRule is the one sentence with three spellings. A key per payout
// rather than one key with a ratio in it, because "pays three to two" and
// "pays six to five" are different sentences in a language that inflects, and
// a locale bundle is where that difference belongs.
func payoutRule(t *GameState) module.RuleItem {
	switch t.BlackjackPayout {
	case pays6to5:
		return module.Rule("blackjack.rules.pays6to5", nil)
	case paysEven:
		return module.Rule("blackjack.rules.paysEven", nil)
	default:
		return module.Rule("blackjack.rules.pays3to2", nil)
	}
}

// ExplainRefusal names the written rules behind a refusal at this table.
//
// It shares its whole mapping with the offer list (ruleIDsFor), and filters
// the answer against the rules this config actually states — so an id can
// never point at a sentence a player cannot go and read.
func (m *Module) ExplainRefusal(cfg module.MatchConfig, code string) []string {
	sections, err := m.Rules(cfg)
	if err != nil {
		return nil
	}
	stated := module.RuleIDsIn(sections)
	var out []string
	for _, id := range ruleIDsFor(tableOf(cfg), code) {
		if stated[id] {
			out = append(out, id)
		}
	}
	return out
}
