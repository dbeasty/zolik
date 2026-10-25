package snaps

import "zolik/server/internal/module"

var _ module.LessonProvider = (*Module)(nil)

// Lessons is the guided walk-through of Šnaps and Šedesát šest. The hand
// size, the exchange card, the match target and the rules the two games
// disagree on all come from the table's own resolved config.
func (m *Module) Lessons(cfg module.MatchConfig) ([]module.Lesson, error) {
	c := resolve(cfg)
	r := rulesFor(c.variation)
	is66 := c.variation == variation66

	setup := module.Say("setup", "guide.snaps.setup", map[string]any{"n": r.handSize}, module.AnchorHand).
		About("snaps.rules.deal5", "snaps.rules.deck20")
	points := module.Say("points", "guide.snaps.points", nil, module.AnchorHand).About("snaps.rules.points")
	if is66 {
		setup = setup.About("snaps.rules.deal6", "snaps.rules.deck24")
		points = points.About("snaps.rules.points66")
	}

	steps := []module.LessonStep{
		module.Say("goal", "guide.snaps.goal", map[string]any{"n": goal}, module.AnchorScore).
			About("snaps.rules.goal"),
		setup,
		points,
	}
	if c.showCardPoints {
		steps = append(steps, module.Say("showPoints", "guide.snaps.showPoints", nil, module.AnchorHand))
	}
	steps = append(steps,
		module.Say("trump", "guide.snaps.trump", nil, module.AnchorTable).
			About("snaps.rules.play.open"),
		module.Do("play", "guide.snaps.play", nil, []string{VerbPlay},
			module.AnchorHand, module.AnchorActionBar).
			About("snaps.rules.play.open"),
		module.Say("draw", "guide.snaps.draw", nil, module.AnchorDrawPile).
			About("snaps.rules.play.draw"),
		module.Say("marriage", "guide.snaps.marriage",
			map[string]any{"plain": marriagePlain, "trump": marriageTrump}, module.AnchorHand).
			About("snaps.rules.marriage"),
	)
	if is66 {
		steps = append(steps, module.Say("exchange66", "guide.snaps.exchange66", nil, module.AnchorDrawPile).
			About("snaps.rules.exchange66"))
	} else {
		steps = append(steps, module.Say("exchange", "guide.snaps.exchange", nil, module.AnchorDrawPile).
			About("snaps.rules.exchange"))
	}
	steps = append(steps,
		module.Say("close", "guide.snaps.close", nil, module.AnchorActionBar).
			About("snaps.rules.close", "snaps.rules.play.strict"),
	)
	if r.strictMarriages {
		steps = append(steps, module.Say("marriageOpenOnly", "guide.snaps.marriageOpenOnly", nil, module.AnchorHand).
			About("snaps.rules.marriageOpenOnly"))
	}
	steps = append(steps,
		module.Say("scoring", "guide.snaps.scoring", map[string]any{"n": schneider}, module.AnchorScore).
			About("snaps.rules.out", "snaps.rules.score.out"),
	)
	if r.lastTrickWins {
		steps = append(steps, module.Say("lastTrick", "guide.snaps.lastTrickSnaps", nil, module.AnchorDrawPile).
			About("snaps.rules.lastTrickSnaps"))
	} else {
		steps = append(steps, module.Say("lastTrick66", "guide.snaps.lastTrick66",
			map[string]any{"n": lastTrickPoints}, module.AnchorDrawPile).
			About("snaps.rules.lastTrick66"))
	}
	steps = append(steps,
		module.Say("match", "guide.snaps.match", map[string]any{"n": c.target}, module.AnchorScore).
			About("snaps.rules.end"),
		module.Do("freeplay", "guide.snaps.freeplay", nil, []string{VerbPlay, VerbExchange, VerbClose},
			module.AnchorHand, module.AnchorActionBar).Times(4),
		module.Say("hint", "guide.snaps.hint", nil, module.AnchorHint),
		module.Say("done", "guide.snaps.done", nil),
	)

	return []module.Lesson{{
		ID:         "basics",
		TitleKey:   "guide.snaps.basics.title",
		SummaryKey: "guide.snaps.basics.summary",
		Seed:       1,
		Bots:       1,
		Steps:      steps,
	}}, nil
}
