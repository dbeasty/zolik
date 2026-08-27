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
	cfg := resolveConfig(mc)
	contract := cfg.ContractFor(1)

	setup := []module.Fact{
		{LabelKey: "zolik.rules.deal", Params: map[string]any{"n": cfg.DealSize}},
		{LabelKey: "zolik.rules.meldShapes", Params: map[string]any{
			"set": cfg.MinSetSize, "run": cfg.MinRunSize,
		}},
	}

	turn := []module.Fact{{LabelKey: "zolik.rules.turn.draw"}}
	switch cfg.DiscardPickupMode {
	case rules.DiscardPickupTopOnly:
		turn = append(turn, module.Fact{LabelKey: "zolik.rules.pickup.topOnly"})
	default:
		turn = append(turn, module.Fact{LabelKey: "zolik.rules.pickup.anyFromPile"})
	}
	if cfg.DiscardDrawMinRound > 1 {
		turn = append(turn, module.Fact{
			LabelKey: "zolik.rules.pickup.locked", Params: map[string]any{"n": cfg.DiscardDrawMinRound},
		})
	} else {
		turn = append(turn, module.Fact{LabelKey: "zolik.rules.pickup.open"})
	}
	turn = append(turn, module.Fact{LabelKey: "zolik.rules.turn.discard"})
	if cfg.JokerDiscardRestricted {
		turn = append(turn, module.Fact{LabelKey: "zolik.rules.jokers.restricted"})
	}
	if cfg.DealStarter == rules.DealStarterWinner {
		turn = append(turn, module.Fact{LabelKey: "zolik.rules.lead.winner"})
	} else {
		turn = append(turn, module.Fact{LabelKey: "zolik.rules.lead.rotate"})
	}

	melding := []module.Fact{}
	if cfg.InitialMeldMinimum > 0 {
		melding = append(melding, module.Fact{
			LabelKey: "zolik.rules.meldFloor.on", Params: map[string]any{"n": cfg.InitialMeldMinimum},
		})
	} else {
		melding = append(melding, module.Fact{LabelKey: "zolik.rules.meldFloor.off"})
	}
	if contract.RequireCleanRun {
		melding = append(melding, module.Fact{LabelKey: "zolik.rules.cleanRun.on"})
	} else {
		melding = append(melding, module.Fact{LabelKey: "zolik.rules.cleanRun.off"})
	}
	if cfg.FixedDealCount > 0 {
		melding = append(melding, module.Fact{
			LabelKey: "zolik.rules.contracts.rotating", Params: map[string]any{"n": cfg.FixedDealCount},
		})
	} else if contract.Sets > 0 || contract.Runs > 0 {
		melding = append(melding, module.Fact{
			LabelKey: "zolik.rules.contracts.static",
			Params:   map[string]any{"sets": contract.Sets, "runs": contract.Runs},
		})
	}

	end := []module.Fact{}
	if cfg.MatchEndMode == rules.MatchEndAfterDeals {
		end = append(end, module.Fact{
			LabelKey: "zolik.rules.end.afterDeals", Params: map[string]any{"n": cfg.FixedDealCount},
		})
	} else {
		end = append(end, module.Fact{
			LabelKey: "zolik.rules.end.atScore", Params: map[string]any{"n": cfg.TargetScore},
		})
	}

	return []module.RuleSection{
		{TitleKey: "zolik.rules.section.goal", Items: []module.Fact{{LabelKey: "zolik.rules.goal"}}},
		{TitleKey: "zolik.rules.section.setup", Items: setup},
		{TitleKey: "zolik.rules.section.turn", Items: turn},
		{TitleKey: "zolik.rules.section.melding", Items: melding},
		{TitleKey: "zolik.rules.section.end", Items: end},
	}, nil
}
