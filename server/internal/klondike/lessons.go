package klondike

import "zolik/server/internal/module"

var _ module.LessonProvider = (*Module)(nil)

// Lessons is Klondike's guided walk-through, written against the table's own
// resolved settings: how many cards the stock turns, how often the waste may be
// turned over, how the game is scored, and the two house rules about taking a
// card back and finishing automatically.
func (m *Module) Lessons(cfg module.MatchConfig) ([]module.Lesson, error) {
	t := tableOf(cfg)

	deal := module.Say("deal", "guide.klondike.deal", nil, module.AnchorTable).
		About("klondike.rules.solo", "klondike.rules.deal")

	var stock module.LessonStep
	if t.Draw >= 3 {
		stock = module.Say("stock", "guide.klondike.stock3", map[string]any{"n": t.Draw}, module.AnchorDrawPile).
			About("klondike.rules.draw3", "klondike.rules.stock")
	} else {
		stock = module.Say("stock", "guide.klondike.stock1", map[string]any{"n": t.Draw}, module.AnchorDrawPile).
			About("klondike.rules.draw1", "klondike.rules.stock")
	}

	var redeal module.LessonStep
	switch redealRule(t) {
	case "klondike.rules.redealsLimited":
		redeal = module.Say("redeals", "guide.klondike.redealsLimited", map[string]any{"n": t.Redeals}, module.AnchorDrawPile).
			About("klondike.rules.redealsLimited")
	case "klondike.rules.redealsNone":
		redeal = module.Say("redeals", "guide.klondike.redealsNone", nil, module.AnchorDrawPile).
			About("klondike.rules.redealsNone")
	default:
		redeal = module.Say("redeals", "guide.klondike.redealsUnlimited", nil, module.AnchorDrawPile).
			About("klondike.rules.redealsUnlimited")
	}

	var scoring module.LessonStep
	switch t.Scoring {
	case scoreVegas:
		scoring = module.Say("scoring", "guide.klondike.scoreVegas", nil, module.AnchorScore).
			About("klondike.rules.scoreVegas")
	case scoreNone:
		scoring = module.Say("scoring", "guide.klondike.scoreNone", nil, module.AnchorScore).
			About("klondike.rules.scoreNone")
	default:
		scoring = module.Say("scoring", "guide.klondike.scoreStandard", nil, module.AnchorScore).
			About("klondike.rules.scoreStandard")
	}

	var takeBack module.LessonStep
	if t.TakeBack {
		takeBack = module.Say("takeback", "guide.klondike.takeBack", nil, module.AnchorTable).
			About("klondike.rules.takeBack")
	} else {
		takeBack = module.Say("takeback", "guide.klondike.noTakeBack", nil, module.AnchorTable).
			About("klondike.rules.noTakeBack")
	}

	var finish module.LessonStep
	if t.AutoFinish {
		finish = module.Say("finish", "guide.klondike.autoFinish", nil, module.AnchorActionBar).
			About("klondike.rules.autoFinish")
	} else {
		finish = module.Say("finish", "guide.klondike.noAutoFinish", nil, module.AnchorActionBar).
			About("klondike.rules.noAutoFinish")
	}

	return []module.Lesson{{
		ID:         "basics",
		TitleKey:   "guide.klondike.basics.title",
		SummaryKey: "guide.klondike.basics.summary",
		Seed:       1,
		Bots:       0,
		Steps: []module.LessonStep{
			module.Say("goal", "guide.klondike.goal", nil, module.AnchorTable).
				About("klondike.rules.foundation", "klondike.rules.won"),
			deal,
			module.Do("firstmove", "guide.klondike.firstmove", nil, []string{VerbMove, VerbDraw},
				module.AnchorTable, module.AnchorActionBar).
				About("klondike.rules.build"),
			module.Say("build", "guide.klondike.build", nil, module.AnchorTable).
				About("klondike.rules.build", "klondike.rules.runs"),
			module.Say("flip", "guide.klondike.flip", nil, module.AnchorTable).
				About("klondike.rules.autoFlip"),
			module.Say("kings", "guide.klondike.kings", nil, module.AnchorTable).
				About("klondike.rules.kingOnly"),
			stock,
			redeal,
			module.Say("foundations", "guide.klondike.foundations", nil, module.AnchorTable).
				About("klondike.rules.foundation"),
			takeBack,
			finish,
			scoring,
			module.Say("undo", "guide.klondike.undo", nil, module.AnchorActionBar).
				About("klondike.rules.undo"),
			module.Do("freeplay", "guide.klondike.freeplay", nil, []string{VerbMove, VerbDraw, VerbRecycle},
				module.AnchorTable, module.AnchorActionBar).Times(4),
			module.Say("hint", "guide.klondike.hint", nil, module.AnchorHint),
			module.Say("done", "guide.klondike.done", nil).
				About("klondike.rules.lost"),
		},
	}}, nil
}
