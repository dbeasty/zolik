package blackpeter

import "zolik/server/internal/module"

var _ module.LessonProvider = (*Module)(nil)

// Lessons is Black Peter's guided walk-through, written against the table's
// own pairing rule and match length.
func (m *Module) Lessons(cfg module.MatchConfig) ([]module.Lesson, error) {
	t := tableOf(cfg)
	pairs := module.Say("pairs", "guide.blackpeter.pairsRank", nil, module.AnchorHand).
		About("blackpeter.rules.pairsRank", "blackpeter.rules.layPairs")
	if t.ByColour {
		pairs = module.Say("pairs", "guide.blackpeter.pairsColour", nil, module.AnchorHand).
			About("blackpeter.rules.pairsColour", "blackpeter.rules.layPairs")
	}
	return []module.Lesson{{
		ID:         "basics",
		TitleKey:   "guide.blackpeter.basics.title",
		SummaryKey: "guide.blackpeter.basics.summary",
		Seed:       1,
		Bots:       2,
		Steps: []module.LessonStep{
			module.Say("goal", "guide.blackpeter.goal", nil, module.AnchorHand).
				About("blackpeter.rules.goal"),
			module.Say("peter", "guide.blackpeter.peter", nil, module.AnchorHand).
				About("blackpeter.rules.deck"),
			pairs,
			module.Say("draw", "guide.blackpeter.draw", nil, module.AnchorSeats).
				About("blackpeter.rules.draw", "blackpeter.rules.pass"),
			module.Do("freeplay", "guide.blackpeter.freeplay", nil, []string{VerbDraw},
				module.AnchorHand, module.AnchorSeats).Times(3).
				About("blackpeter.rules.draw"),
			module.Say("out", "guide.blackpeter.out", nil, module.AnchorSeats).
				About("blackpeter.rules.out"),
			module.Say("lose", "guide.blackpeter.lose", nil, module.AnchorScore).
				About("blackpeter.rules.lose", "blackpeter.rules.match"),
			module.Say("done", "guide.blackpeter.done", nil),
		},
	}}, nil
}
