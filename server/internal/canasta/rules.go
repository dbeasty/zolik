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
		{LabelKey: "canasta.rules.deck", Value: deck, Params: map[string]any{"decks": v.Decks}},
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

	// How a turn is shaped, and what each part of it costs.
	//
	// This section exists because of the refusals rather than in spite of them.
	// Three quarters of what this engine can say no to is about *when* — you
	// have not drawn, you have already drawn, the pile is shut for a reason
	// that is nowhere on screen — and a code on its own ("WRONG_PHASE") is a
	// refusal a player cannot act on. ruleindex.go points those at the
	// sentences below, so the explanation a player reads is this table's own
	// rule rather than a second wording of it kept somewhere else.
	turn := []module.Fact{
		{LabelKey: "canasta.rules.turn"},
		{LabelKey: "canasta.rules.turnDiscard"},
		{LabelKey: "canasta.rules.pileTopCard"},
		{LabelKey: "canasta.rules.pileBlocked", Params: map[string]any{"n": blackThreeValue}},
	}
	// A pile frozen by a buried wild is a rule of every variation except the
	// one whose pile is frozen all deal anyway — there, the sentence above in
	// the melding section already says the whole of it, and stating both would
	// have a player reading about a thaw that never comes.
	if !v.PileAlwaysFrozen {
		turn = append(turn, module.Fact{LabelKey: "canasta.rules.pileFrozenByWild"})
	}

	melding := []module.Fact{
		{LabelKey: "canasta.rules.meldShape", Params: map[string]any{"n": minMeldSize}},
		{LabelKey: "canasta.rules.canasta", Params: map[string]any{"n": canastaSize}},
	}
	// The wild limits, in whichever of the two shapes this variation states
	// them. Samba's is a ratio — twice as many naturals as wilds — and reading
	// it as an absolute floor is how a player ends up believing a two-wild
	// meld needs two naturals when it needs four.
	if v.NaturalsPerWild > 0 {
		melding = append(melding, module.Fact{
			LabelKey: "canasta.rules.wildRatio",
			Params:   map[string]any{"n": v.NaturalsPerWild, "wilds": v.MaxWilds},
		})
	} else {
		melding = append(melding, module.Fact{
			LabelKey: "canasta.rules.wildLimit",
			Params:   map[string]any{"wilds": v.MaxWilds, "naturals": v.MinNaturals},
		})
	}
	// One meld per rank is the rule a player meets as "you already have that
	// rank down" — which reads as an error rather than as a rule unless the
	// rule is written somewhere. Samba caps nothing, and says so.
	if v.GroupsPerRank == 1 {
		melding = append(melding, module.Fact{LabelKey: "canasta.rules.oneMeldPerRank"})
	} else {
		melding = append(melding, module.Fact{LabelKey: "canasta.rules.meldsPerRankUnlimited"})
	}
	if v.GroupCanastaCloses {
		melding = append(melding, module.Fact{
			LabelKey: "canasta.rules.canastaCloses", Params: map[string]any{"n": canastaSize},
		})
	}
	melding = append(melding,
		module.Fact{LabelKey: "canasta.rules.meldsAreShared"},
		module.Fact{LabelKey: "canasta.rules.layOffAfterOpening"},
	)
	if v.Sequences {
		melding = append(melding,
			module.Fact{LabelKey: "canasta.rules.sequences"},
			module.Fact{LabelKey: "canasta.rules.samba", Params: map[string]any{"n": v.SambaBonus}},
		)
	}
	// Black threes are two rules in one sentence — what they do to the pile, and
	// whether they can ever leave a hand for the table — and which of the two
	// sentences a table states is the variation's, not a number to be filled in.
	// Modern American forbids the meld outright, so its players read that rather
	// than reading about a move they will never be offered.
	blackThrees := "canasta.rules.blackThreesNeverMeld"
	if v.BlackThreeMeld {
		blackThrees = "canasta.rules.blackThreesGoOut"
	}
	melding = append(melding, module.Fact{
		LabelKey: blackThrees, Params: map[string]any{"n": blackThreeValue},
	})
	// How the pile is taken — the clause players most often arrive at a table
	// disagreeing about, so every variation states its own answer.
	//
	// The always-frozen sentence answers it on its own ("two natural cards from
	// your hand" leaves no room for a meld to do the work), which is why Samba
	// stops there. The other two say it either way round rather than only when
	// the move is missing: that an unfinished meld can claim the pile is news to
	// anyone who learned the American game, and that it cannot is news to
	// everyone else.
	switch {
	case v.PileAlwaysFrozen:
		melding = append(melding, module.Fact{LabelKey: "canasta.rules.pileAlwaysFrozen"})
	case v.PileMeldCapture:
		melding = append(melding, module.Fact{LabelKey: "canasta.rules.pileOntoMeld"})
	default:
		melding = append(melding, module.Fact{LabelKey: "canasta.rules.pileNoMeldCapture"})
	}
	// The bands are read off the ruleset rather than written out again, so a
	// variation that adds one cannot end up describing the other's.
	//
	// Their thresholds too, and not only their minimums. The sentence used to
	// carry "up to 1500, up to 3000, up to 7000" as prose in twenty-four
	// translations, which held only for as long as no variation moved a band —
	// the one thing the comment above promises this cannot get wrong.
	//
	// Each band is named for where it ends rather than where it begins, because
	// that is how the sentence reads it: "{low} up to {lowUpTo}". A band's
	// ceiling is the next one's Above, so the last band has none and needs no
	// param — it is the "beyond that" one.
	bands := map[string]any{"negative": negativeMeldFloor}
	for i, name := range []string{"low", "mid", "high", "top"} {
		if i >= len(v.MeldFloors) {
			break
		}
		bands[name] = v.MeldFloors[i].Min
		if i+1 < len(v.MeldFloors) {
			bands[name+"UpTo"] = v.MeldFloors[i+1].Above
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
		module.Section("canasta.rules.section.turn", turn...),
		module.Section("canasta.rules.section.melding", melding...),
		module.Section("canasta.rules.section.end",
			module.Fact{LabelKey: goOutKey},
			module.Fact{LabelKey: "canasta.rules.goOutKeepsACard"},
			module.Fact{LabelKey: "canasta.rules.end", Params: map[string]any{"n": targetScore}},
		),
	}, nil
}
