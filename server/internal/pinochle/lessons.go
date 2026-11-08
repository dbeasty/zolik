package pinochle

import "zolik/server/internal/module"

var _ module.LessonProvider = (*Module)(nil)

// Lessons is a guided first hand for the table being set up: its variation
// decides which steps there are, and the numbers come from its options.
func (m *Module) Lessons(cfg module.MatchConfig) ([]module.Lesson, error) {
	c := resolve(cfg)
	t := tableOf(c.variation)
	var steps []module.LessonStep
	switch c.variation {
	case VarTwoHanded:
		steps = append(steps,
			module.Say("goal", "guide.pinochle.goal.twohanded", map[string]any{"n": c.target}, module.AnchorScore).
				About("pinochle.rules.goal.twohanded", "pinochle.rules.target"),
			module.Say("deal", "guide.pinochle.deal.twohanded", map[string]any{"n": t.handSize}, module.AnchorDrawPile).
				About("pinochle.rules.deal", "pinochle.rules.turnUp"),
			module.Do("lead", "guide.pinochle.lead.twohanded", nil, []string{VerbPlay}, module.AnchorHand, module.AnchorTable).
				About("pinochle.rules.stockFree"),
			module.Say("melds", "guide.pinochle.melds", nil, module.AnchorRules).
				About("pinochle.rules.meld.run", "pinochle.rules.meld.pinochle"),
			module.Do("meldturn", "guide.pinochle.meldturn", nil, []string{VerbMeld, VerbDraw}, module.AnchorActionBar).
				About("pinochle.rules.meld.onWin", "pinochle.rules.meld.onePerTurn"),
			module.Say("late", "guide.pinochle.late", nil, module.AnchorTable).
				About("pinochle.rules.late", "pinochle.rules.follow", "pinochle.rules.beat"),
		)
	default:
		goal := "guide.pinochle.goal.partnership"
		goalRule := "pinochle.rules.goal.partnership"
		switch c.variation {
		case VarCutthroat:
			goal, goalRule = "guide.pinochle.goal.cutthroat", "pinochle.rules.goal.cutthroat"
		case VarDoubleDeck:
			goal, goalRule = "guide.pinochle.goal.doubledeck", "pinochle.rules.goal.doubledeck"
		}
		steps = append(steps,
			module.Say("goal", goal, map[string]any{"n": c.target}, module.AnchorScore).
				About(goalRule, "pinochle.rules.target"),
			module.Say("melds", "guide.pinochle.melds", nil, module.AnchorRules).
				About("pinochle.rules.meld.run", "pinochle.rules.meld.pinochle", "pinochle.rules.meld.classes"),
			module.Do("bid", "guide.pinochle.bid", map[string]any{"n": c.minBid}, []string{VerbBid, VerbPass}, module.AnchorActionBar).
				About("pinochle.rules.auction", "pinochle.rules.passFinal"),
			module.Say("trump", "guide.pinochle.trump", nil, module.AnchorActionBar).
				About("pinochle.rules.trump"),
		)
		if t.widow > 0 {
			steps = append(steps, module.Say("widow", "guide.pinochle.widow", map[string]any{"n": t.widow}, module.AnchorTable).
				About("pinochle.rules.takeWidow", "pinochle.rules.bury"))
		}
		steps = append(steps,
			module.Say("meldShown", "guide.pinochle.meldShown", nil, module.AnchorTable).
				About("pinochle.rules.meld.shown"),
			module.Do("play", "guide.pinochle.play", nil, []string{VerbPlay}, module.AnchorHand, module.AnchorTable).
				About("pinochle.rules.follow", "pinochle.rules.beat", "pinochle.rules.trumpIn"),
			module.Say("scoring", "guide.pinochle.scoring", nil, module.AnchorScore).
				About("pinochle.rules.made", "pinochle.rules.set"),
		)
	}
	steps = append(steps,
		module.Do("freeplay", "guide.pinochle.freeplay", nil, []string{VerbPlay}, module.AnchorHand).Times(4),
		module.Say("hint", "guide.pinochle.hint", nil, module.AnchorHint),
		module.Say("done", "guide.pinochle.done", nil),
	)
	return []module.Lesson{{
		ID:         "basics",
		TitleKey:   "guide.pinochle.basics.title",
		SummaryKey: "guide.pinochle.basics.summary",
		Seed:       1,
		Bots:       t.seats - 1,
		Steps:      steps,
	}}, nil
}
