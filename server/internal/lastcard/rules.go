package lastcard

import "zolik/server/internal/module"

var _ module.RulesProvider = (*Module)(nil)

// Rules writes out Last Card's rules for one table's actual options, resolved
// the same way NewMatch resolves them: the call and the challenge are stated
// only where they are played, and the scoring only where there is any.
func (m *Module) Rules(cfg module.MatchConfig) ([]module.RuleSection, error) {
	handSize := cfg.Opt(OptHandSize, defaultHandSize)
	target := cfg.Opt(OptTargetScore, defaultTargetScore)
	call := cfg.Opt(OptLastCardCall, module.OptOn) == module.OptOn
	challenge := cfg.Opt(OptDrawFourChallenge, module.OptOn) == module.OptOn
	stacking := cfg.Opt(OptStacking, stackOff)
	drawUntil := cfg.Opt(OptDrawUntilPlayable, module.OptOff) == module.OptOn
	sevenZero := cfg.Opt(OptSevenZero, module.OptOff) == module.OptOn

	goal := module.Fact{LabelKey: "lastcard.rules.goal"}
	end := module.Fact{LabelKey: "lastcard.rules.end"}
	if target > 0 {
		goal = module.Fact{LabelKey: "lastcard.rules.goal.points", Params: map[string]any{"target": target}}
		end = module.Fact{LabelKey: "lastcard.rules.end.points", Params: map[string]any{"target": target}}
	}
	drawFour := module.Fact{LabelKey: "lastcard.rules.wildDrawFour"}
	if challenge {
		drawFour = module.Fact{LabelKey: "lastcard.rules.wildDrawFourBluff"}
	}

	turnDraw := module.Fact{LabelKey: "lastcard.rules.turn.draw"}
	if drawUntil {
		turnDraw = module.Fact{LabelKey: "lastcard.rules.turn.drawUntil"}
	}

	out := []module.RuleSection{
		module.Section("lastcard.rules.section.goal", goal),
		module.Section("lastcard.rules.section.setup",
			module.Fact{LabelKey: "lastcard.rules.deck", Value: "108"},
			module.Fact{LabelKey: "lastcard.rules.deal", Params: map[string]any{"n": handSize}},
		),
		module.Section("lastcard.rules.section.turn",
			module.Fact{LabelKey: "lastcard.rules.turn.match"},
			turnDraw,
		),
		module.Section("lastcard.rules.section.special",
			module.Fact{LabelKey: "lastcard.rules.skip"},
			module.Fact{LabelKey: "lastcard.rules.reverse"},
			module.Fact{LabelKey: "lastcard.rules.drawTwo"},
			module.Fact{LabelKey: "lastcard.rules.wild"},
			drawFour,
		),
	}
	// House rules, each stated only at a table that plays it.
	var house []module.Fact
	switch stacking {
	case stackTwos:
		house = append(house, module.Fact{LabelKey: "lastcard.rules.stacking.twos"})
	case stackAny:
		house = append(house, module.Fact{LabelKey: "lastcard.rules.stacking.any"})
	}
	if sevenZero {
		house = append(house,
			module.Fact{LabelKey: "lastcard.rules.sevens"},
			module.Fact{LabelKey: "lastcard.rules.zeros"},
		)
	}
	if len(house) > 0 {
		out = append(out, module.Section("lastcard.rules.section.house", house...))
	}
	if call {
		out = append(out, module.Section("lastcard.rules.section.call",
			module.Fact{LabelKey: "lastcard.rules.call"},
			module.Fact{LabelKey: "lastcard.rules.catch"},
		))
	}
	if challenge {
		out = append(out, module.Section("lastcard.rules.section.challenge",
			module.Fact{LabelKey: "lastcard.rules.challenge"},
		))
	}
	if target > 0 {
		out = append(out, module.Section("lastcard.rules.section.scoring",
			module.Fact{LabelKey: "lastcard.rules.scoring", Params: map[string]any{"target": target}},
			module.Fact{LabelKey: "lastcard.rules.values"},
		))
	}
	return append(out, module.Section("lastcard.rules.section.end", end)), nil
}
