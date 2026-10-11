package zolikmod

import (
	"zolik/server/internal/module"
	"zolik/server/internal/rules"
)

var _ module.LessonProvider = (*Module)(nil)

// Lessons is Žolíky's guided walk-through. It reads the same resolved config
// Rules(cfg) and NewMatch do, so a step about the first-meld floor, the
// clean run, the discard pile or the contract is present only when this table
// really has that rule, and every number comes from the config.
func (m *Module) Lessons(mc module.MatchConfig) ([]module.Lesson, error) {
	cfg := resolveConfig(mc)
	contract := cfg.ContractFor(1)

	steps := []module.LessonStep{
		module.Say("goal", "guide.zolik.goal", map[string]any{"n": cfg.DealSize}, module.AnchorHand).
			About("zolik.rules.goal", "zolik.rules.deal"),
		module.Say("shapes", "guide.zolik.shapes",
			map[string]any{"set": cfg.MinSetSize, "run": cfg.MinRunSize}, module.AnchorHand).
			About("zolik.rules.meldShapes"),
		module.Do("draw", "guide.zolik.draw", nil, []string{string(rules.VerbDraw)},
			module.AnchorDrawPile, module.AnchorDiscard, module.AnchorActionBar).
			About("zolik.rules.turn.draw"),
	}

	if cfg.DiscardPickupMode == rules.DiscardPickupTopOnly {
		steps = append(steps, module.Say("pile", "guide.zolik.pile.topOnly", nil, module.AnchorDiscard).
			About("zolik.rules.pickup.topOnly", "zolik.rules.pickup.obligation"))
	} else {
		steps = append(steps, module.Say("pile", "guide.zolik.pile.any", nil, module.AnchorDiscard).
			About("zolik.rules.pickup.anyFromPile", "zolik.rules.pickup.obligation"))
	}
	if cfg.DiscardDrawMinRound > 1 {
		steps = append(steps, module.Say("locked", "guide.zolik.pile.locked",
			map[string]any{"n": cfg.DiscardDrawMinRound}, module.AnchorDiscard).
			About("zolik.rules.pickup.locked"))
	}

	steps = append(steps,
		module.Do("discard", "guide.zolik.discard", nil, []string{string(rules.VerbDiscard)},
			module.AnchorHand, module.AnchorActionBar).
			About("zolik.rules.turn.discard", "zolik.rules.pickup.noReturn"),
	)
	if cfg.JokerDiscardRestricted {
		steps = append(steps, module.Say("jokerKeep", "guide.zolik.jokerKeep", nil, module.AnchorHand).
			About("zolik.rules.jokers.restricted"))
	}

	if cfg.FixedDealCount > 0 {
		steps = append(steps, module.Say("contract", "guide.zolik.contract.rotating",
			map[string]any{"n": cfg.FixedDealCount}, module.AnchorStatus).
			About("zolik.rules.contracts.rotating", "zolik.rules.contracts.contribution"))
	} else {
		steps = append(steps, module.Say("contract", "guide.zolik.contract.none", nil, module.AnchorStatus).
			About("zolik.rules.contracts.contribution"))
	}
	if cfg.InitialMeldMinimum > 0 {
		steps = append(steps, module.Say("floor", "guide.zolik.floor.on",
			map[string]any{"n": cfg.InitialMeldMinimum}, module.AnchorHand).
			About("zolik.rules.meldFloor.on"))
	}
	if contract.RequireCleanRun {
		steps = append(steps, module.Say("cleanRun", "guide.zolik.cleanRun.on", nil, module.AnchorHand).
			About("zolik.rules.cleanRun.on"))
	}

	steps = append(steps,
		module.Say("layoff", "guide.zolik.layoff", nil, module.AnchorTable).
			About("zolik.rules.layoff.afterDown", "zolik.rules.layoff.runEnds"),
	)
	if cfg.JokerReclaimMustPlay {
		steps = append(steps, module.Say("jokers", "guide.zolik.jokers.must", nil, module.AnchorTable).
			About("zolik.rules.jokers.swap", "zolik.rules.jokers.reclaim.on"))
	} else {
		steps = append(steps, module.Say("jokers", "guide.zolik.jokers.keep", nil, module.AnchorTable).
			About("zolik.rules.jokers.swap", "zolik.rules.jokers.reclaim.off"))
	}

	steps = append(steps,
		module.Do("freeplay", "guide.zolik.freeplay", nil,
			[]string{string(rules.VerbDraw), string(rules.VerbLayMeld), string(rules.VerbLayOff),
				string(rules.VerbSwapJoker), string(rules.VerbDiscard)},
			module.AnchorHand, module.AnchorActionBar).Times(4),
		module.Say("hint", "guide.zolik.hint", nil, module.AnchorHint),
		module.Say("done", "guide.zolik.done", nil),
	)

	return []module.Lesson{{
		ID:         "basics",
		TitleKey:   "guide.zolik.basics.title",
		SummaryKey: "guide.zolik.basics.summary",
		Seed:       1,
		Bots:       1,
		Steps:      steps,
	}}, nil
}
