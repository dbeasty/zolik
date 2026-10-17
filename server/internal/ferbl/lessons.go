package ferbl

import "zolik/server/internal/module"

var _ module.LessonProvider = (*Module)(nil)

// Lessons is Ferbl's guided walk-through. Every number a table can change
// (the vizi, the starting chips, the match length) is passed in Params.
func (m *Module) Lessons(cfg module.MatchConfig) ([]module.Lesson, error) {
	c := resolve(cfg)
	return []module.Lesson{{
		ID:         "basics",
		TitleKey:   "guide.ferbl.basics.title",
		SummaryKey: "guide.ferbl.basics.summary",
		Seed:       1,
		Bots:       2,
		Steps: []module.LessonStep{
			module.Say("goal", "guide.ferbl.goal", map[string]any{"chips": c.startingStack, "hands": c.hands}, module.AnchorHand).
				About("ferbl.rules.goal", "ferbl.rules.end"),
			module.Say("deck", "guide.ferbl.deck", nil, module.AnchorHand).
				About("ferbl.rules.deck", "ferbl.rules.values"),
			module.Say("ante", "guide.ferbl.ante", map[string]any{"n": c.vizi}, module.AnchorTable).
				About("ferbl.rules.ante"),
			module.Say("hands", "guide.ferbl.hands", nil, module.AnchorHand).
				About("ferbl.rules.fourKind", "ferbl.rules.banda", "ferbl.rules.threeKind"),
			module.Say("lower", "guide.ferbl.lower", nil, module.AnchorHand).
				About("ferbl.rules.threeSuited", "ferbl.rules.twoAces", "ferbl.rules.twoSuited", "ferbl.rules.suits", "ferbl.rules.ignored"),
			module.Do("bet", "guide.ferbl.bet", map[string]any{"n": c.vizi},
				[]string{VerbCheck, VerbBet, VerbCall, VerbRaise, VerbFold}, module.AnchorActionBar).
				About("ferbl.rules.round1"),
			module.Say("respond", "guide.ferbl.respond", nil, module.AnchorActionBar).
				About("ferbl.rules.round1"),
			module.Say("round2", "guide.ferbl.round2", map[string]any{"n": 2 * c.vizi}, module.AnchorHand).
				About("ferbl.rules.round2"),
			module.Say("cap", "guide.ferbl.cap", map[string]any{"n": maxRaises}, module.AnchorActionBar).
				About("ferbl.rules.cap", "ferbl.rules.allIn"),
			module.Do("freeplay", "guide.ferbl.freeplay", nil,
				[]string{VerbCheck, VerbBet, VerbCall, VerbRaise, VerbFold}, module.AnchorActionBar).Times(4),
			module.Say("showdown", "guide.ferbl.showdown", nil, module.AnchorTable).
				About("ferbl.rules.showdown"),
			module.Say("hint", "guide.ferbl.hint", nil, module.AnchorHint),
			module.Say("done", "guide.ferbl.done", nil),
		},
	}}, nil
}
