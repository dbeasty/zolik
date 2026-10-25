package blackjack

import "zolik/server/internal/module"

var _ module.LessonProvider = (*Module)(nil)

// Lessons is Blackjack's guided walk-through, written against the table's own
// resolved rules (tableOf, the same resolution Rules and NewMatch use): the
// chips, the minimum, the blackjack payout, the soft-17 rule and the optional
// moves all come from the config, so the guide never describes another table.
func (m *Module) Lessons(cfg module.MatchConfig) ([]module.Lesson, error) {
	t := tableOf(cfg)

	steps := []module.LessonStep{
		module.Say("goal", "guide.blackjack.goal", nil, module.AnchorHand).
			About("blackjack.rules.goal", "blackjack.rules.aces"),
		module.Say("setup", "guide.blackjack.setup",
			map[string]any{"stack": t.StartingStack, "min": t.MinBet, "n": t.Decks, "rounds": t.RoundLimit},
			module.AnchorScore).
			About("blackjack.rules.decks", "blackjack.rules.stack", "blackjack.rules.minBet", "blackjack.rules.rounds"),
		module.Do("bet", "guide.blackjack.bet", map[string]any{"min": t.MinBet}, []string{VerbBet},
			module.AnchorActionBar).
			About("blackjack.rules.oneStakePerRound", "blackjack.rules.stakeFromStack"),
		module.Say("order", "guide.blackjack.order", nil, module.AnchorTable).
			About("blackjack.rules.roundOrder", "blackjack.rules.faceUp"),
	}
	if t.AllowInsurance {
		steps = append(steps,
			module.Say("insurance", "guide.blackjack.insurance", nil, module.AnchorActionBar).
				About("blackjack.rules.insurance"))
	} else {
		steps = append(steps,
			module.Say("noInsurance", "guide.blackjack.noInsurance", nil, module.AnchorActionBar).
				About("blackjack.rules.noInsurance"))
	}
	steps = append(steps,
		module.Do("hit", "guide.blackjack.hit", nil, []string{VerbHit, VerbStand},
			module.AnchorHand, module.AnchorActionBar).
			About("blackjack.rules.hitStand", "blackjack.rules.goal"),
		module.Say("double", "guide.blackjack.double", nil, module.AnchorActionBar).
			About("blackjack.rules.double"),
	)
	if t.MaxSplits > 0 {
		steps = append(steps,
			module.Say("split", "guide.blackjack.split",
				map[string]any{"n": t.MaxSplits, "hands": t.MaxSplits + 1}, module.AnchorActionBar).
				About("blackjack.rules.split", "blackjack.rules.splitAces"))
		if t.DoubleAfterSplit {
			steps = append(steps, module.Say("doubleAfterSplit", "guide.blackjack.doubleAfterSplit", nil).
				About("blackjack.rules.doubleAfterSplit"))
		} else {
			steps = append(steps, module.Say("noDoubleAfterSplit", "guide.blackjack.noDoubleAfterSplit", nil).
				About("blackjack.rules.noDoubleAfterSplit"))
		}
	} else {
		steps = append(steps,
			module.Say("noSplit", "guide.blackjack.noSplit", nil, module.AnchorActionBar).
				About("blackjack.rules.noSplit"))
	}
	if t.AllowSurrender {
		steps = append(steps,
			module.Say("surrender", "guide.blackjack.surrender", nil, module.AnchorActionBar).
				About("blackjack.rules.surrender"))
	} else {
		steps = append(steps,
			module.Say("noSurrender", "guide.blackjack.noSurrender", nil, module.AnchorActionBar).
				About("blackjack.rules.noSurrender"))
	}

	if t.DealerHitsSoft17 {
		steps = append(steps, module.Say("dealerHits", "guide.blackjack.dealerHits", nil, module.AnchorTable).
			About("blackjack.rules.dealerDraws", "blackjack.rules.hitsSoft17"))
	} else {
		steps = append(steps, module.Say("dealerStands", "guide.blackjack.dealerStands", nil, module.AnchorTable).
			About("blackjack.rules.dealerDraws", "blackjack.rules.standsSoft17"))
	}

	switch t.BlackjackPayout {
	case pays6to5:
		steps = append(steps, module.Say("pays6to5", "guide.blackjack.pays6to5", nil, module.AnchorHand).
			About("blackjack.rules.blackjack", "blackjack.rules.pays6to5"))
	case paysEven:
		steps = append(steps, module.Say("paysEven", "guide.blackjack.paysEven", nil, module.AnchorHand).
			About("blackjack.rules.blackjack", "blackjack.rules.paysEven"))
	default:
		steps = append(steps, module.Say("pays3to2", "guide.blackjack.pays3to2", nil, module.AnchorHand).
			About("blackjack.rules.blackjack", "blackjack.rules.pays3to2"))
	}

	verbs := []string{VerbBet, VerbHit, VerbStand, VerbDouble, VerbSplit}
	if t.AllowInsurance {
		verbs = append(verbs, VerbInsure, VerbDecline)
	}
	if t.AllowSurrender {
		verbs = append(verbs, VerbSurrender)
	}
	steps = append(steps,
		module.Do("freeplay", "guide.blackjack.freeplay", nil, verbs,
			module.AnchorActionBar, module.AnchorHand).Times(4),
		module.Say("end", "guide.blackjack.end", map[string]any{"rounds": t.RoundLimit}, module.AnchorScore).
			About("blackjack.rules.mostChipsWins", "blackjack.rules.bustedOut"),
		module.Say("hint", "guide.blackjack.hint", nil, module.AnchorHint),
		module.Say("done", "guide.blackjack.done", nil),
	)

	return []module.Lesson{{
		ID:         "basics",
		TitleKey:   "guide.blackjack.basics.title",
		SummaryKey: "guide.blackjack.basics.summary",
		Seed:       1,
		Bots:       1,
		Steps:      steps,
	}}, nil
}
