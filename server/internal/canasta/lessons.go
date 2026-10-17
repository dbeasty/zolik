package canasta

import "zolik/server/internal/module"

var _ module.LessonProvider = (*Module)(nil)

// lessonBots seats you, a partner and two opponents: four is the one seat
// count every variation takes, and the one where "partnership" means what the
// rules text says it means.
const lessonBots = 3

// Lessons is Canasta's guided walk-through, written against the table's own
// resolved rules (the same resolveRules Rules() and NewMatch read), so every
// number is this table's and every rule that exists in only one variation or
// behind one option is a step that is there or not.
func (m *Module) Lessons(cfg module.MatchConfig) ([]module.Lesson, error) {
	v := resolveRules(cfg, lessonBots+1)

	goOutText, goOutRule := "guide.canasta.goOutOne", "canasta.rules.oneCanastaToGoOut"
	if v.CanastasToGoOut > 1 {
		goOutText, goOutRule = "guide.canasta.goOutTwo", "canasta.rules.twoCanastasToGoOut"
	}

	redText := "guide.canasta.redThrees"
	if v.RedThreesNeed > 1 {
		redText = "guide.canasta.redThreesMany"
	}

	floorRule := "canasta.rules.meldFloorBands"
	if len(v.MeldFloors) > 3 {
		floorRule = "canasta.rules.meldFloorBandsFive"
	}

	steps := []module.LessonStep{
		module.Say("goal", "guide.canasta.goal", map[string]any{"n": v.TargetScore}, module.AnchorScore).
			About("canasta.rules.goal", "canasta.rules.end"),
		module.Say("setup", "guide.canasta.setup",
			map[string]any{"n": v.HandSize, "decks": v.Decks, "cards": v.Decks*52 + v.Decks*v.JokersPerDeck},
			module.AnchorHand).About("canasta.rules.deal", "canasta.rules.deck"),
	}

	// The turn, with the draw as the first thing the player does. Samba draws
	// two, so the text says how many.
	if v.DrawCount > 1 {
		steps = append(steps, module.Do("draw", "guide.canasta.drawMany", map[string]any{"n": v.DrawCount},
			[]string{VerbDraw}, module.AnchorDrawPile, module.AnchorActionBar).
			Held("guide.canasta.wrong.draw").About("canasta.rules.turn", "canasta.rules.drawCount"))
	} else {
		steps = append(steps, module.Do("draw", "guide.canasta.draw", nil,
			[]string{VerbDraw}, module.AnchorDrawPile, module.AnchorActionBar).
			Held("guide.canasta.wrong.draw").About("canasta.rules.turn"))
	}
	steps = append(steps,
		module.Say("turn", "guide.canasta.turn", nil, module.AnchorHand).
			About("canasta.rules.turn", "canasta.rules.turnDiscard"),
		module.Say("meld", "guide.canasta.meld", map[string]any{"n": minMeldSize}, module.AnchorTable).
			About("canasta.rules.meldShape", "canasta.rules.meldsAreShared"),
	)

	// Wild cards: a ratio at Samba, a flat limit elsewhere.
	if v.NaturalsPerWild > 0 {
		steps = append(steps, module.Say("wilds", "guide.canasta.wildsRatio",
			map[string]any{"n": v.NaturalsPerWild, "wilds": v.MaxWilds}, module.AnchorHand).
			About("canasta.rules.wildRatio"))
	} else {
		steps = append(steps, module.Say("wilds", "guide.canasta.wilds",
			map[string]any{"wilds": v.MaxWilds, "naturals": v.MinNaturals}, module.AnchorHand).
			About("canasta.rules.wildLimit"))
	}

	// One meld per rank, or as many as you like.
	if v.GroupsPerRank == 1 {
		steps = append(steps, module.Say("perRank", "guide.canasta.oneMeldPerRank", nil, module.AnchorTable).
			About("canasta.rules.oneMeldPerRank"))
	} else {
		steps = append(steps, module.Say("perRank", "guide.canasta.manyMelds", nil, module.AnchorTable).
			About("canasta.rules.meldsPerRankUnlimited"))
	}

	// Sequences exist only at tables that play them, clean or dirty.
	if v.Sequences {
		if v.DirtySequences {
			steps = append(steps, module.Say("sequences", "guide.canasta.sequencesDirty",
				map[string]any{"n": v.SambaBonus, "dirty": v.DirtySambaBonus}, module.AnchorTable).
				About("canasta.rules.dirtySequences", "canasta.rules.samba"))
		} else {
			steps = append(steps, module.Say("sequences", "guide.canasta.sequences",
				map[string]any{"n": v.SambaBonus}, module.AnchorTable).
				About("canasta.rules.sequences", "canasta.rules.samba"))
		}
	}

	// The first meld is the hands-on moment; loose, because the deal decides
	// what is on offer.
	steps = append(steps,
		module.Say("opening", "guide.canasta.opening",
			map[string]any{"n": v.MeldFloors[0].Min, "negative": negativeMeldFloor}, module.AnchorStatus).
			About(floorRule, "canasta.rules.layOffAfterOpening"),
		module.Do("lay", "guide.canasta.lay", map[string]any{"n": v.MeldFloors[0].Min},
			[]string{VerbLayMeld}, module.AnchorHand, module.AnchorActionBar).
			About(floorRule, "canasta.rules.meldShape"),
		module.Do("discard", "guide.canasta.discard", nil, []string{VerbDiscard},
			module.AnchorHand, module.AnchorActionBar).
			About("canasta.rules.turnDiscard"),
	)

	// Canastas and what they pay.
	canastaParams := map[string]any{"n": canastaSize, "natural": v.NaturalCanastaBonus, "mixed": v.MixedCanastaBonus}
	if v.GroupCanastaCloses {
		steps = append(steps, module.Say("canasta", "guide.canasta.canastaCloses", canastaParams, module.AnchorTable).
			About("canasta.rules.canasta", "canasta.rules.canastaCloses"))
	} else {
		steps = append(steps, module.Say("canasta", "guide.canasta.canasta", canastaParams, module.AnchorTable).
			About("canasta.rules.canasta"))
	}

	steps = append(steps,
		module.Say("redThrees", redText, map[string]any{"n": v.RedThreesNeed}, module.AnchorStatus).
			About("canasta.rules.redThrees"))

	// Black threes: Modern American never melds them, the others do on the way out.
	if v.BlackThreeMeld {
		steps = append(steps, module.Say("blackThrees", "guide.canasta.blackThreesGoOut",
			map[string]any{"n": blackThreeValue}, module.AnchorDiscard).
			About("canasta.rules.blackThreesGoOut", "canasta.rules.pileBlocked"))
	} else {
		steps = append(steps, module.Say("blackThrees", "guide.canasta.blackThreesNever",
			map[string]any{"n": blackThreeValue}, module.AnchorDiscard).
			About("canasta.rules.blackThreesNeverMeld", "canasta.rules.pileBlocked"))
	}

	// The pile: three different answers to what it costs to take it.
	switch {
	case v.PileAlwaysFrozen:
		steps = append(steps, module.Say("pile", "guide.canasta.pileFrozen", nil, module.AnchorDiscard).
			About("canasta.rules.pileAlwaysFrozen", "canasta.rules.pileTopCard"))
	case v.PileMeldCapture:
		steps = append(steps, module.Say("pile", "guide.canasta.pileOntoMeld", nil, module.AnchorDiscard).
			About("canasta.rules.pileOntoMeld", "canasta.rules.pileFrozenByWild", "canasta.rules.pileTopCard"))
	default:
		steps = append(steps, module.Say("pile", "guide.canasta.pilePair", nil, module.AnchorDiscard).
			About("canasta.rules.pileNoMeldCapture", "canasta.rules.pileFrozenByWild", "canasta.rules.pileTopCard"))
	}

	// House rules, each a step only where the table has it on.
	if v.TopOnlyCapture {
		steps = append(steps, module.Say("topOnly", "guide.canasta.topOnly", nil, module.AnchorDiscard).
			About("canasta.rules.topOnlyCapture"))
	}
	if v.WildMeld {
		steps = append(steps, module.Say("wildMeld", "guide.canasta.wildMeld",
			map[string]any{"n": v.WildCanastaBonus, "mixed": v.WildMixedCanastaBonus}, module.AnchorTable).
			About("canasta.rules.wildMeld"))
	}
	if v.Rearrange {
		steps = append(steps, module.Say("rearrange", "guide.canasta.rearrange", nil, module.AnchorTable).
			About("canasta.rules.rearrange"))
	}
	if v.Poach {
		steps = append(steps, module.Say("poach", "guide.canasta.poach", nil, module.AnchorTable).
			About("canasta.rules.poach"))
	}

	// Going out, and what it pays.
	steps = append(steps, module.Say("goOut", goOutText, nil, module.AnchorScore).
		About(goOutRule, "canasta.rules.goOutKeepsACard"))
	if v.ConcealedBonus > 0 {
		steps = append(steps, module.Say("goOutBonus", "guide.canasta.goOutBonusConcealed",
			map[string]any{"n": v.GoingOutBonus, "concealed": v.ConcealedBonus}, module.AnchorScore).
			About("canasta.rules.goingOutBonus"))
	} else {
		steps = append(steps, module.Say("goOutBonus", "guide.canasta.goOutBonus",
			map[string]any{"n": v.GoingOutBonus}, module.AnchorScore).
			About("canasta.rules.goingOutBonusFlat"))
	}

	freeVerbs := []string{VerbDraw, VerbTakePile, VerbLayMeld, VerbLayOff, VerbDiscard}
	steps = append(steps,
		module.Do("freeplay", "guide.canasta.freeplay", nil, freeVerbs,
			module.AnchorHand, module.AnchorActionBar).Times(4),
		module.Say("hint", "guide.canasta.hint", nil, module.AnchorHint),
		module.Say("done", "guide.canasta.done", nil),
	)

	return []module.Lesson{{
		ID:         "basics",
		TitleKey:   "guide.canasta.basics.title",
		SummaryKey: "guide.canasta.basics.summary",
		Seed:       1,
		Bots:       lessonBots,
		Steps:      steps,
	}}, nil
}
