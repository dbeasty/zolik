package rummytiles

import "zolik/server/internal/module"

var _ module.RulesProvider = (*Module)(nil)

// Rules is Rummy Tiles' written rules, config-aware for the options a player
// can actually change.
func (m *Module) Rules(cfg module.MatchConfig) ([]module.RuleSection, error) {
	v := resolveVariation(cfg)
	target := cfg.Opt(OptTargetScore, v.targetScore)
	roundLimit := cfg.Opt(OptRoundLimit, v.roundLimit)
	poolLowestWins := cfg.Opt(OptPoolExhaustion, module.BoolOpt(v.poolExhaustionLowestWins)) == module.OptOn

	sections := []module.RuleSection{
		module.Section("rummytiles.rules.setup",
			module.Fact{LabelKey: "rummytiles.rules.tiles", Value: "106"},
			module.Fact{LabelKey: "rummytiles.rules.dealCount", Value: "14"},
		),
		module.Section("rummytiles.rules.sets",
			module.Fact{LabelKey: "rummytiles.rules.group"},
			module.Fact{LabelKey: "rummytiles.rules.run"},
			module.Fact{LabelKey: "rummytiles.rules.noWrap"},
			module.Fact{LabelKey: "rummytiles.rules.joker"},
			// The opening minimum and the end-of-round penalty are both counted
			// in tile values, so the values are stated once, beside the joker
			// whose two prices (in a set, in a hand) differ.
			module.Fact{LabelKey: "rummytiles.rules.tileValues", Params: map[string]any{
				"joker": tileValue("JOKER1"),
			}},
		),
		module.Section("rummytiles.rules.initialMeld",
			module.Fact{LabelKey: "rummytiles.rules.initialMeldDescription", Params: map[string]any{"n": initialMeldFloor}},
		),
		module.Section("rummytiles.rules.turn",
			module.Fact{LabelKey: "rummytiles.rules.turnDescription"},
			module.Fact{LabelKey: "rummytiles.rules.noDiscard"},
		),
		module.Section("rummytiles.rules.jokerTaking",
			module.Fact{LabelKey: "rummytiles.rules.jokerTakingDescription"},
		),
		module.Section("rummytiles.rules.ending",
			module.Fact{LabelKey: "rummytiles.rules.goingOut"},
		),
	}

	if poolLowestWins {
		sections = append(sections, module.Section("rummytiles.rules.poolExhaustion",
			module.Fact{LabelKey: "rummytiles.rules.poolExhaustionLowestWins"},
		))
	} else {
		sections = append(sections, module.Section("rummytiles.rules.poolExhaustion",
			module.Fact{LabelKey: "rummytiles.rules.poolExhaustionNoWinner"},
		))
	}

	if target > 0 {
		sections = append(sections, module.Section("rummytiles.rules.match",
			module.Fact{LabelKey: "rummytiles.rules.target", Params: map[string]any{"n": target}},
		))
	} else {
		sections = append(sections, module.Section("rummytiles.rules.match",
			module.Fact{LabelKey: "rummytiles.rules.roundLimit", Params: map[string]any{"n": roundLimit}},
		))
	}

	return sections, nil
}
