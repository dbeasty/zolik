package zolikmod

import (
	"zolik/server/internal/module"
	"zolik/server/internal/rules"
)

var _ module.RulesProvider = (*Module)(nil)

// Rules writes out Žolíky's rules for one lobby's actual choice of variation
// and options, resolved through the same resolveConfig NewMatch deals from —
// so a sentence here can never describe a table other than the one a player
// is looking at.
func (m *Module) Rules(mc module.MatchConfig) ([]module.RuleSection, error) {
	return ruleSections(resolveConfig(mc)), nil
}

// Section ids. Named constants because two things address them now: the rules
// screen anchors on them, and ExplainRefusal points refusals at the items
// inside them.
const (
	secGoal    = "zolik.rules.section.goal"
	secSetup   = "zolik.rules.section.setup"
	secTurn    = "zolik.rules.section.turn"
	secMelding = "zolik.rules.section.melding"
	secLayOff  = "zolik.rules.section.layoff"
	secEnd     = "zolik.rules.section.end"
)

// ruleSections is the index itself: every rule this engine enforces, written
// out once, resolved against cfg.
//
// It takes a resolved RulesConfig rather than a MatchConfig because two
// callers need it — the rules endpoint, which starts from a lobby's choices,
// and LegalActions, which starts from a game already in progress — and the
// resolved ruleset is the one thing both of them have.
//
// Completeness is not a matter of judgement here. Every code the validators
// can return must point at a sentence in this list, and ruleindex_test.go
// fails the build if one doesn't; roughly a third of what follows was added
// because that test found rules this engine had enforced, silently, since the
// beginning.
func ruleSections(cfg rules.RulesConfig) []module.RuleSection {
	contract := cfg.ContractFor(1)

	setup := []module.RuleItem{
		module.Rule("zolik.rules.deal", map[string]any{"n": cfg.DealSize}),
		module.Rule("zolik.rules.meldShapes", map[string]any{
			"set": cfg.MinSetSize, "run": cfg.MinRunSize,
		}),
		module.Rule("zolik.rules.wilds.setLimit", nil),
		module.Rule("zolik.rules.set.maxSize", map[string]any{"n": rules.MaxSetSize}),
		module.Rule("zolik.rules.run.aceBridge", nil),
	}

	turn := []module.RuleItem{module.Rule("zolik.rules.turn.draw", nil)}
	switch cfg.DiscardPickupMode {
	case rules.DiscardPickupTopOnly:
		turn = append(turn, module.Rule("zolik.rules.pickup.topOnly", nil))
	default:
		turn = append(turn, module.Rule("zolik.rules.pickup.anyFromPile", nil))
	}
	if cfg.DiscardDrawMinRound > 1 {
		turn = append(turn, module.Rule("zolik.rules.pickup.locked",
			map[string]any{"n": cfg.DiscardDrawMinRound}))
	} else {
		turn = append(turn, module.Rule("zolik.rules.pickup.open", nil))
	}
	turn = append(turn,
		// What taking from the pile costs you. Enforced since the first
		// version of the engine and, until the index was checked, written
		// down nowhere — so a player refused by it was told what had
		// happened and never why.
		module.Rule("zolik.rules.pickup.obligation", nil),
		module.Rule("zolik.rules.pickup.noReturn", nil),
		module.Rule("zolik.rules.turn.discard", nil),
	)
	if cfg.JokerDiscardRestricted {
		turn = append(turn, module.Rule("zolik.rules.jokers.restricted", nil))
	}
	turn = append(turn, module.Rule("zolik.rules.deck.reshuffle", nil))
	if cfg.DealStarter == rules.DealStarterWinner {
		turn = append(turn, module.Rule("zolik.rules.lead.winner", nil))
	} else {
		turn = append(turn, module.Rule("zolik.rules.lead.rotate", nil))
	}

	melding := []module.RuleItem{}
	if cfg.InitialMeldMinimum > 0 {
		melding = append(melding, module.Rule("zolik.rules.meldFloor.on",
			map[string]any{"n": cfg.InitialMeldMinimum}))
	} else {
		melding = append(melding, module.Rule("zolik.rules.meldFloor.off", nil))
	}
	if contract.RequireCleanRun {
		melding = append(melding, module.Rule("zolik.rules.cleanRun.on", nil))
	} else {
		melding = append(melding, module.Rule("zolik.rules.cleanRun.off", nil))
	}
	if cfg.FixedDealCount > 0 {
		melding = append(melding, module.Rule("zolik.rules.contracts.rotating",
			map[string]any{"n": cfg.FixedDealCount}))
	} else if contract.Sets > 0 || contract.Runs > 0 {
		melding = append(melding, module.Rule("zolik.rules.contracts.static",
			map[string]any{"sets": contract.Sets, "runs": contract.Runs}))
	}
	melding = append(melding, module.Rule("zolik.rules.contracts.contribution", nil))

	// Everything you may do to melds already on the table. Its own section
	// because it is a distinct phase of a turn, and because five of the
	// engine's refusal codes land in it.
	layOff := []module.RuleItem{
		module.Rule("zolik.rules.layoff.afterDown", nil),
		module.Rule("zolik.rules.layoff.runEnds", nil),
		module.Rule("zolik.rules.jokers.swap", nil),
	}

	end := []module.RuleItem{}
	if cfg.MatchEndMode == rules.MatchEndAfterDeals {
		end = append(end, module.Rule("zolik.rules.end.afterDeals",
			map[string]any{"n": cfg.FixedDealCount}))
	} else {
		end = append(end, module.Rule("zolik.rules.end.atScore",
			map[string]any{"n": cfg.TargetScore}))
	}

	return []module.RuleSection{
		module.SectionOf(secGoal, []module.RuleItem{module.Rule("zolik.rules.goal", nil)}),
		module.SectionOf(secSetup, setup),
		module.SectionOf(secTurn, turn),
		module.SectionOf(secMelding, melding),
		module.SectionOf(secLayOff, layOff),
		module.SectionOf(secEnd, end),
	}
}
