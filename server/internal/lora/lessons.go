package lora

import "zolik/server/internal/module"

var _ module.LessonProvider = (*Module)(nil)

// Lessons is Lóra's guided walk-through, written against the table's own
// length (talie) and maturita option.
func (m *Module) Lessons(cfg module.MatchConfig) ([]module.Lesson, error) {
	c := resolve(cfg)
	steps := []module.LessonStep{
		module.Say("goal", "guide.lora.goal", nil, module.AnchorScore).
			About("lora.rules.goal"),
		module.Say("setup", "guide.lora.setup", map[string]any{"n": handSize, "talie": c.talies}, module.AnchorHand).
			About("lora.rules.deal", "lora.rules.length"),
		module.Say("talie", "guide.lora.talie", nil, module.AnchorStatus).
			About("lora.rules.talie"),
		module.Say("follow", "guide.lora.follow", nil, module.AnchorTable).
			About("lora.rules.tricks.follow"),
		module.Do("play", "guide.lora.play", nil, []string{VerbPlay},
			module.AnchorHand, module.AnchorActionBar).
			About("lora.rules.tricks.follow"),
		module.Say("penalties", "guide.lora.penalties", nil, module.AnchorScore).
			About("lora.rules.game.cervene", "lora.rules.game.filky", "lora.rules.game.prpo",
				"lora.rules.game.vsechny", "lora.rules.game.kral"),
		module.Say("redlead", "guide.lora.redlead", nil, module.AnchorHand).
			About("lora.rules.tricks.redLead", "lora.rules.tricks.stop"),
		module.Say("layouts", "guide.lora.layouts", nil, module.AnchorTable).
			About("lora.rules.kvarty.lay", "lora.rules.kvarty.add", "lora.rules.desitky.lay", "lora.rules.desitky.tuk"),
		module.Say("layoutEnd", "guide.lora.layoutEnd", nil, module.AnchorScore).
			About("lora.rules.kvarty.end", "lora.rules.desitky.end"),
	}
	if c.maturita {
		steps = append(steps,
			module.Say("maturita", "guide.lora.maturita", map[string]any{"n": maturitaPenalty, "tries": maxAttempts}, module.AnchorStatus).
				About("lora.rules.maturita", "lora.rules.maturita.choose", "lora.rules.maturita.fail"))
	}
	steps = append(steps,
		module.Do("freeplay", "guide.lora.freeplay", nil, []string{VerbPlay},
			module.AnchorHand, module.AnchorActionBar).Times(4),
		module.Say("hint", "guide.lora.hint", nil, module.AnchorHint),
		module.Say("done", "guide.lora.done", nil),
	)
	return []module.Lesson{{
		ID:         "basics",
		TitleKey:   "guide.lora.basics.title",
		SummaryKey: "guide.lora.basics.summary",
		Seed:       1,
		Bots:       3,
		Steps:      steps,
	}}, nil
}
