package okobere

import "zolik/server/internal/module"

var _ module.LessonProvider = (*Module)(nil)

// Lessons is Oko bere's guided walk-through, written against the table's own
// stake, bank, rotation and oko payout.
func (m *Module) Lessons(cfg module.MatchConfig) ([]module.Lesson, error) {
	c := resolve(cfg)
	oko := module.Say("oko", "guide.okobere.oko3", map[string]any{"n": c.okoPays}, module.AnchorHand).
		About("okobere.rules.oko", "okobere.rules.okoPays3")
	if c.okoPays == 1 {
		oko = module.Say("oko", "guide.okobere.oko1", nil, module.AnchorHand).
			About("okobere.rules.oko", "okobere.rules.okoPays1")
	}
	return []module.Lesson{{
		ID:         "basics",
		TitleKey:   "guide.okobere.basics.title",
		SummaryKey: "guide.okobere.basics.summary",
		Seed:       1,
		Bots:       2,
		Steps: []module.LessonStep{
			module.Say("goal", "guide.okobere.goal", nil, module.AnchorHand).
				About("okobere.rules.goal"),
			module.Say("values", "guide.okobere.values", nil, module.AnchorHand).
				About("okobere.rules.values"),
			module.Say("bank", "guide.okobere.bank", map[string]any{"bank": c.bank, "chips": c.startingStack}, module.AnchorScore).
				About("okobere.rules.bank"),
			module.Say("deal", "guide.okobere.deal", nil, module.AnchorHand, module.AnchorSeats).
				About("okobere.rules.deal"),
			module.Say("stake", "guide.okobere.stake", map[string]any{"n": c.minBet}, module.AnchorActionBar).
				About("okobere.rules.stake"),
			module.Do("freeplay", "guide.okobere.freeplay", nil, []string{VerbBet, VerbCard, VerbStand},
				module.AnchorHand, module.AnchorActionBar).Times(4).
				About("okobere.rules.turn"),
			module.Say("bust", "guide.okobere.bust", nil, module.AnchorHand).About("okobere.rules.bust"),
			oko,
			module.Say("banker", "guide.okobere.banker", nil, module.AnchorSeats).
				About("okobere.rules.banker", "okobere.rules.bankerOko"),
			module.Say("settle", "guide.okobere.settle", nil, module.AnchorScore).
				About("okobere.rules.settle"),
			module.Say("rotation", "guide.okobere.rotation", map[string]any{"n": c.roundsPerBank}, module.AnchorSeats).
				About("okobere.rules.rotation"),
			module.Say("end", "guide.okobere.end", map[string]any{"n": c.bankTurns, "chips": c.startingStack}, module.AnchorScore).
				About("okobere.rules.end"),
			module.Say("hint", "guide.okobere.hint", nil, module.AnchorHint),
			module.Say("done", "guide.okobere.done", nil),
		},
	}}, nil
}
