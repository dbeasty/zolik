package snaps

import "zolik/server/internal/module"

// Rules writes out Šnaps or Šedesát šest for one lobby's table.
//
// Every sentence is a rule id a refusal can point at, so the two games share
// the sentences they agree on and each states its own where they differ.
func (m *Module) Rules(mc module.MatchConfig) ([]module.RuleSection, error) {
	c := resolve(mc)
	r := rulesFor(c.variation)
	is66 := c.variation == variation66

	deck, pointsKey, dealKey := "snaps.rules.deck20", "snaps.rules.points", "snaps.rules.deal5"
	if is66 {
		deck, pointsKey, dealKey = "snaps.rules.deck24", "snaps.rules.points66", "snaps.rules.deal6"
	}

	exchange := module.Rule("snaps.rules.exchange", nil)
	if r.exchangeAfterTrick {
		exchange = module.Rule("snaps.rules.exchange66", nil)
	}
	play := []module.RuleItem{
		module.Rule("snaps.rules.play.open", nil),
		module.Rule("snaps.rules.play.draw", nil),
		module.Rule("snaps.rules.play.strict", nil),
		exchange,
		module.Rule("snaps.rules.marriage", map[string]any{"plain": marriagePlain, "trump": marriageTrump}),
	}
	if r.strictMarriages {
		play = append(play, module.Rule("snaps.rules.marriageOpenOnly", nil))
	}

	ending := []module.RuleItem{
		module.Rule("snaps.rules.out", map[string]any{"n": goal}),
		module.Rule("snaps.rules.score.out", map[string]any{"n": schneider}),
		module.Rule("snaps.rules.close", nil),
	}
	if r.scoreAtClosing {
		ending = append(ending, module.Rule("snaps.rules.score.closedAtClosing", nil))
	} else {
		ending = append(ending, module.Rule("snaps.rules.score.closedAtEnd", nil))
	}
	ending = append(ending, module.Rule("snaps.rules.score.closerFailed", nil))
	if r.lastTrickWins {
		ending = append(ending, module.Rule("snaps.rules.lastTrickSnaps", nil))
	} else {
		ending = append(ending, module.Rule("snaps.rules.lastTrick66", map[string]any{"n": lastTrickPoints}))
	}

	return []module.RuleSection{
		module.Section("snaps.rules.section.goal",
			module.Fact{LabelKey: "snaps.rules.goal"},
		),
		module.Section("snaps.rules.section.cards",
			module.Fact{LabelKey: deck},
			module.Fact{LabelKey: pointsKey},
		),
		module.Section("snaps.rules.section.deal",
			module.Fact{LabelKey: dealKey},
			module.Fact{LabelKey: "snaps.rules.deal.turns"},
		),
		module.SectionOf("snaps.rules.section.play", play),
		module.SectionOf("snaps.rules.section.ending", ending),
		module.Section("snaps.rules.section.end",
			module.Fact{LabelKey: "snaps.rules.end", Params: map[string]any{"n": c.target}},
		),
	}, nil
}
