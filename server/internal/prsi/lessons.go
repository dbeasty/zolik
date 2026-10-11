package prsi

import "zolik/server/internal/module"

var _ module.LessonProvider = (*Module)(nil)

// Lessons is Prší's guided walk-through, written against the table's own
// hand size so "you start with 4 cards" is whatever this table deals.
func (m *Module) Lessons(cfg module.MatchConfig) ([]module.Lesson, error) {
	handSize := cfg.Opt(OptHandSize, defaultHandSize)
	return []module.Lesson{{
		ID:         "basics",
		TitleKey:   "guide.prsi.basics.title",
		SummaryKey: "guide.prsi.basics.summary",
		Seed:       11,
		Bots:       1,
		Steps: []module.LessonStep{
			module.Say("goal", "guide.prsi.goal", map[string]any{"n": handSize}, module.AnchorHand).
				About("prsi.rules.goal", "prsi.rules.deal"),
			module.Say("pile", "guide.prsi.pile", nil, module.AnchorDiscard).
				About("prsi.rules.turn.match"),
			module.Do("play", "guide.prsi.play", nil, []string{VerbPlay, VerbDraw},
				module.AnchorHand, module.AnchorActionBar).
				About("prsi.rules.turn.match", "prsi.rules.turn.draw"),
			module.Say("draw", "guide.prsi.draw", nil, module.AnchorDrawPile).
				About("prsi.rules.turn.draw"),
			module.Say("sevens", "guide.prsi.sevens", nil, module.AnchorHand).About("prsi.rules.sevens"),
			module.Say("aces", "guide.prsi.aces", nil, module.AnchorHand).About("prsi.rules.aces"),
			module.Say("queens", "guide.prsi.queens", nil, module.AnchorHand).About("prsi.rules.queens"),
			module.Do("freeplay", "guide.prsi.freeplay", nil, []string{VerbPlay, VerbDraw, VerbPass},
				module.AnchorHand, module.AnchorActionBar).Times(3),
			module.Say("hint", "guide.prsi.hint", nil, module.AnchorHint),
			module.Say("done", "guide.prsi.done", nil),
		},
	}}, nil
}
