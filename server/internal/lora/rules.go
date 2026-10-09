package lora

import "zolik/server/internal/module"

// Rules writes out Lóra for this table's options.
func (m *Module) Rules(mc module.MatchConfig) ([]module.RuleSection, error) {
	c := resolve(mc)
	out := []module.RuleSection{
		module.Section("lora.rules.section.goal",
			module.Fact{LabelKey: "lora.rules.goal"},
		),
		module.Section("lora.rules.section.cards",
			module.Fact{LabelKey: "lora.rules.deck"},
			module.Fact{LabelKey: "lora.rules.deal"},
		),
		module.Section("lora.rules.section.talie",
			module.Fact{LabelKey: "lora.rules.talie"},
			module.Fact{LabelKey: "lora.rules.length", Params: map[string]any{"n": c.talies}},
		),
		module.Section("lora.rules.section.tricks",
			module.Fact{LabelKey: "lora.rules.tricks.follow"},
			module.Fact{LabelKey: "lora.rules.tricks.redLead"},
			module.Fact{LabelKey: "lora.rules.tricks.stop"},
		),
		module.Section("lora.rules.section.games",
			module.Fact{LabelKey: "lora.rules.game.cervene"},
			module.Fact{LabelKey: "lora.rules.game.filky"},
			module.Fact{LabelKey: "lora.rules.game.prpo"},
			module.Fact{LabelKey: "lora.rules.game.vsechny"},
			module.Fact{LabelKey: "lora.rules.game.kral"},
		),
		module.Section("lora.rules.section.kvarty",
			module.Fact{LabelKey: "lora.rules.kvarty.lay"},
			module.Fact{LabelKey: "lora.rules.kvarty.add"},
			module.Fact{LabelKey: "lora.rules.kvarty.end"},
		),
		module.Section("lora.rules.section.desitky",
			module.Fact{LabelKey: "lora.rules.desitky.lay"},
			module.Fact{LabelKey: "lora.rules.desitky.tuk"},
			module.Fact{LabelKey: "lora.rules.desitky.end"},
		),
	}
	if c.maturita {
		out = append(out, module.Section("lora.rules.section.maturita",
			module.Fact{LabelKey: "lora.rules.maturita"},
			module.Fact{LabelKey: "lora.rules.maturita.choose"},
			module.Fact{LabelKey: "lora.rules.maturita.fail"},
		))
	}
	out = append(out, module.Section("lora.rules.section.end",
		module.Fact{LabelKey: "lora.rules.end"},
	))
	return out, nil
}
