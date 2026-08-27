package holdem

import "zolik/server/internal/module"

var _ module.RulesProvider = (*Module)(nil)

// Rules writes out Hold'em's rules for one lobby's actual stack, blind and
// hand-limit choices, resolved the same way the engine resolves them.
func (m *Module) Rules(cfg module.MatchConfig) ([]module.RuleSection, error) {
	v := resolveVariation(cfg)
	stack := cfg.Opt(OptStartingStack, v.startingStack)
	bigBlind := cfg.Opt(OptBigBlind, v.bigBlind)
	handLimit := cfg.Opt(OptHandLimit, v.handLimit)

	// holdem.rules.mostChipsWins and .lastPlayerStanding are also used
	// param-free in Descriptor's Summary, so they stay param-free here too —
	// the hand count gets its own key rather than being folded into either.
	end := []module.Fact{{LabelKey: "holdem.rules.noLimit"}}
	if handLimit > 0 {
		end = append(end,
			module.Fact{LabelKey: "holdem.rules.handLimit", Params: map[string]any{"n": handLimit}},
			module.Fact{LabelKey: "holdem.rules.mostChipsWins"},
		)
	} else {
		end = append(end, module.Fact{LabelKey: "holdem.rules.lastPlayerStanding"})
	}

	return []module.RuleSection{
		module.Section("holdem.rules.section.goal",
			module.Fact{LabelKey: "holdem.rules.goal"},
		),
		module.Section("holdem.rules.section.setup",
			module.Fact{LabelKey: "holdem.rules.stack", Params: map[string]any{"n": stack}},
			module.Fact{LabelKey: "holdem.rules.blinds", Params: map[string]any{"sb": bigBlind / 2, "bb": bigBlind}},
		),
		module.Section("holdem.rules.section.betting",
			module.Fact{LabelKey: "holdem.rules.streets"},
			module.Fact{LabelKey: "holdem.rules.showdown"},
		),
		module.Section("holdem.rules.section.end", end...),
	}, nil
}
