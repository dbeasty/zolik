package canasta

import (
	"strconv"

	"zolik/server/internal/module"
)

var _ module.RulesProvider = (*Module)(nil)

// Rules writes out Canasta's rules for one lobby's actual hand size, target
// score and go-out requirement, resolved the same way the engine resolves
// them (engine.go).
func (m *Module) Rules(cfg module.MatchConfig) ([]module.RuleSection, error) {
	v := resolveVariation(cfg.Variation)
	handSize := cfg.Opt(OptHandSize, v.HandSize)
	targetScore := cfg.Opt(OptTargetScore, v.TargetScore)
	canastasToGoOut := cfg.Opt(OptCanastasToGoOut, v.CanastasToGoOut)

	// Reuses the exact keys Descriptor's Summary already ships (param-free —
	// the sentence spells "one"/"two" out in words) rather than a second,
	// numbered phrasing of the same fact.
	goOutKey := "canasta.rules.oneCanastaToGoOut"
	if canastasToGoOut > 1 {
		goOutKey = "canasta.rules.twoCanastasToGoOut"
	}

	deck := strconv.Itoa(v.Decks*52 + v.Decks*v.JokersPerDeck)

	setup := []module.Fact{
		{LabelKey: "canasta.rules.deck", Value: deck},
		{LabelKey: "canasta.rules.deal", Params: map[string]any{"n": handSize}},
		{LabelKey: "canasta.rules.redThrees"},
	}
	// Drawing two is Samba's, and it is the first thing a player notices, so it
	// belongs in the setup rather than buried in the melding rules.
	if v.DrawCount > 1 {
		setup = append(setup, module.Fact{
			LabelKey: "canasta.rules.drawCount", Params: map[string]any{"n": v.DrawCount},
		})
	}

	melding := []module.Fact{
		{LabelKey: "canasta.rules.canasta", Params: map[string]any{"n": canastaSize}},
	}
	if v.Sequences {
		melding = append(melding,
			module.Fact{LabelKey: "canasta.rules.sequences"},
			module.Fact{LabelKey: "canasta.rules.samba", Params: map[string]any{"n": v.SambaBonus}},
		)
	}
	if v.PileAlwaysFrozen {
		melding = append(melding, module.Fact{LabelKey: "canasta.rules.pileAlwaysFrozen"})
	}
	// The bands are read off the ruleset rather than written out again, so a
	// variation that adds one cannot end up describing the other's.
	bands := map[string]any{"negative": negativeMeldFloor}
	for i, name := range []string{"low", "mid", "high", "top"} {
		if i < len(v.MeldFloors) {
			bands[name] = v.MeldFloors[i].Min
		}
	}
	floorKey := "canasta.rules.meldFloorBands"
	if len(v.MeldFloors) > 3 {
		floorKey = "canasta.rules.meldFloorBandsFive"
	}
	melding = append(melding, module.Fact{LabelKey: floorKey, Params: bands})

	return []module.RuleSection{
		module.Section("canasta.rules.section.goal",
			module.Fact{LabelKey: "canasta.rules.goal", Params: map[string]any{"n": targetScore}},
		),
		module.Section("canasta.rules.section.setup", setup...),
		module.Section("canasta.rules.section.melding", melding...),
		module.Section("canasta.rules.section.end",
			module.Fact{LabelKey: goOutKey},
			module.Fact{LabelKey: "canasta.rules.end", Params: map[string]any{"n": targetScore}},
		),
	}, nil
}
