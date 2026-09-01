package ginrummy

import (
	"strconv"

	"zolik/server/internal/module"
)

// Option names, and the values each variation starts from.
const (
	OptTargetScore = "targetScore"
	OptKnockLimit  = "knockLimit"
	OptBigGin      = "bigGin"
	OptLineBonuses = "lineBonuses"
)

// oklahomaSentinel is the knockLimit option value meaning "the upcard sets
// it" — the one real ruleset fork between the two variations, per the plan.
const oklahomaSentinel = 0

type variationDefaults struct {
	targetScore int
	knockLimit  int
	bigGin      bool
	lineBonuses bool
}

var variations = map[string]variationDefaults{
	"standard": {targetScore: 100, knockLimit: 10, bigGin: false, lineBonuses: true},
	"oklahoma": {targetScore: 100, knockLimit: oklahomaSentinel, bigGin: false, lineBonuses: true},
}

func resolveVariation(cfg module.MatchConfig) variationDefaults {
	if v, ok := variations[cfg.Variation]; ok {
		return v
	}
	return variations["standard"]
}

// Descriptor is Gin Rummy's self-description.
func (m *Module) Descriptor() module.ModuleDescriptor {
	return module.ModuleDescriptor{
		ID: "ginrummy", Label: "Gin Rummy", MinPlayers: 2, MaxPlayers: 2,
		Variations: []module.VariationSpec{
			{
				ID: "standard", Label: "Standard",
				Summary: []module.Fact{
					{LabelKey: "ginrummy.rules.deck", Value: "52"},
					{LabelKey: "ginrummy.rules.knockLimit", Params: map[string]any{"n": variations["standard"].knockLimit}},
				},
				Defaults: map[string]int{
					OptTargetScore:               variations["standard"].targetScore,
					OptKnockLimit:                variations["standard"].knockLimit,
					OptBigGin:                    module.BoolOpt(variations["standard"].bigGin),
					OptLineBonuses:               module.BoolOpt(variations["standard"].lineBonuses),
					module.OptPauseBetweenRounds: module.OptOn,
					module.OptBotSkill:           module.SkillOpt(module.SkillMedium),
				},
			},
			{
				ID: "oklahoma", Label: "Oklahoma",
				Summary: []module.Fact{
					{LabelKey: "ginrummy.rules.deck", Value: "52"},
					{LabelKey: "ginrummy.rules.oklahoma"},
				},
				Defaults: map[string]int{
					OptTargetScore:               variations["oklahoma"].targetScore,
					OptKnockLimit:                variations["oklahoma"].knockLimit,
					OptBigGin:                    module.BoolOpt(variations["oklahoma"].bigGin),
					OptLineBonuses:               module.BoolOpt(variations["oklahoma"].lineBonuses),
					module.OptPauseBetweenRounds: module.OptOn,
					module.OptBotSkill:           module.SkillOpt(module.SkillMedium),
				},
			},
		},
		Options: []module.OptionSpec{
			module.PauseOption(),
			module.BotSkillOption(),
			{
				Name: OptTargetScore, Type: module.OptionEnumInt,
				Label: "Target score", Help: "The score a player must pass to win the match.",
				Choices: []module.OptionChoice{
					{Value: 100, Label: "100"}, {Value: 150, Label: "150"},
					{Value: 250, Label: "250"}, {Value: 500, Label: "500"},
				},
			},
			{
				Name: OptKnockLimit, Type: module.OptionEnumInt,
				Label: "Knock limit", Help: "The deadwood a player may knock with — or let the upcard set it.",
				Choices: []module.OptionChoice{
					{Value: 10, Label: "10"},
					{Value: oklahomaSentinel, Label: "Oklahoma (the upcard sets it)"},
				},
			},
			{
				Name: OptBigGin, Type: module.OptionEnumInt,
				Label: "Big gin", Help: "Eleven cards melding with no discard, for a bonus.",
				Choices: []module.OptionChoice{
					{Value: module.OptOff, Label: "Off"}, {Value: module.OptOn, Label: "On (+25)"},
				},
			},
			{
				Name: OptLineBonuses, Type: module.OptionEnumInt,
				Label: "Line bonuses", Help: "25 per hand won, and 100 for the game (200 on a shutout).",
				Choices: []module.OptionChoice{
					{Value: module.OptOn, Label: "On"}, {Value: module.OptOff, Label: "Off"},
				},
			},
		},
	}
}

// View renders the board for one viewer.
//
// Hands are private except for one case: a knocker lays their whole hand face
// up, so once a knock has happened the knocker's hand is Cards, not Count, for
// every viewer — the melds zone groups which of those same cards form a run
// or a set, which is grouping information a spread-of-cards zone alone cannot
// carry. The defender's hand stays hidden until they lay a card off, at which
// point it becomes visible the only way it truly is: as part of the meld it
// joined.
func (m *Module) View(raw module.State, viewerID string) (module.ViewModel, error) {
	s, err := decode(raw)
	if err != nil {
		return module.ViewModel{}, err
	}

	vm := module.ViewModel{}

	for _, p := range s.Players {
		hand := s.Hands[p]
		switch {
		case p == viewerID:
			vm.Zones = append(vm.Zones, module.Zone{
				ID: handZoneID(p), Kind: module.ZoneHand, OwnerID: p,
				LabelKey: "zone.yourHand", Cards: cardViews(hand), Count: len(hand),
			})
		case s.Knocker != "" && p == s.Knocker:
			vm.Zones = append(vm.Zones, module.Zone{
				ID: handZoneID(p), Kind: module.ZoneHand, OwnerID: p,
				LabelKey: "ginrummy.zone.knockerHand", Cards: cardViews(hand), Count: len(hand),
			})
		default:
			vm.Zones = append(vm.Zones, module.Zone{
				ID: handZoneID(p), Kind: module.ZoneHand, OwnerID: p,
				LabelKey: "zone.opponentHand", Count: len(hand),
			})
		}
	}

	vm.Zones = append(vm.Zones,
		module.Zone{ID: stockZoneID, Kind: module.ZoneStack, LabelKey: "zone.drawPile", Count: len(s.Stock)},
		module.Zone{ID: discardZoneID, Kind: module.ZonePile, LabelKey: "zone.discardPile",
			Cards: cardViews(topOnly(s.DiscardPile)), Count: len(s.DiscardPile)},
	)

	if len(s.KnockerMelds) > 0 {
		z := module.Zone{ID: meldsZoneID, Kind: module.ZoneSpread, LabelKey: "ginrummy.zone.melds"}
		for _, mm := range s.KnockerMelds {
			z.Groups = append(z.Groups, module.Group{ID: mm.ID, Kind: mm.Kind, Cards: append([]string(nil), mm.Cards...)})
			z.Count += len(mm.Cards)
		}
		vm.Zones = append(vm.Zones, z)
	}

	for _, p := range s.Players {
		seat := module.Seat{
			PlayerID: p,
			Active:   (s.Intermission.Open && !s.Intermission.Ready[p]) || (s.Current == p && s.Status == "active"),
			Facts: []module.Fact{
				{LabelKey: "seat.cards", Value: strconv.Itoa(len(s.Hands[p])), Params: map[string]any{"n": len(s.Hands[p])}},
				{LabelKey: "ginrummy.seat.score", Value: strconv.Itoa(s.Scores[p]), Params: map[string]any{"score": s.Scores[p]}},
			},
		}
		if p == s.Dealer {
			seat.LabelKeys = []string{"ginrummy.seat.dealer"}
		}
		vm.Seats = append(vm.Seats, seat)
	}

	vm.Header = []module.Fact{
		{LabelKey: "header.deck", Value: strconv.Itoa(len(s.Stock))},
		{LabelKey: "header.target", Value: strconv.Itoa(s.TargetScore)},
		{LabelKey: "ginrummy.header.hand", Value: strconv.Itoa(s.HandNumber + 1), Params: map[string]any{"n": s.HandNumber + 1}},
	}

	if s.Knocker != "" && s.KnockGin {
		vm.Status = append(vm.Status, module.Fact{
			LabelKey: "ginrummy.status.gin", Value: s.Knocker,
			Params: map[string]any{"playerId": s.Knocker, "deadwood": s.KnockerDeadwood},
		})
	} else if s.Knocker != "" {
		vm.Status = append(vm.Status, module.Fact{
			LabelKey: "ginrummy.status.knocked", Value: s.Knocker,
			Params: map[string]any{"playerId": s.Knocker, "deadwood": s.KnockerDeadwood},
		})
	}

	if s.Current == viewerID && s.Status == "active" {
		switch s.Phase {
		case phaseUpcardNonDealer, phaseUpcardDealer:
			vm.Prompts = append(vm.Prompts, module.Fact{LabelKey: "ginrummy.prompt.upcardDecision"})
		case phaseDiscard:
			vm.Prompts = append(vm.Prompts, module.Fact{LabelKey: "ginrummy.prompt.yourTurnDiscard"})
		case phaseLayoff:
			vm.Prompts = append(vm.Prompts, module.Fact{LabelKey: "ginrummy.prompt.layoff"})
		default:
			vm.Prompts = append(vm.Prompts, module.Fact{LabelKey: "ginrummy.prompt.yourTurnDraw"})
		}
	}

	if len(s.Rounds) > 0 {
		last := s.Rounds[len(s.Rounds)-1]
		vm.Status = append(vm.Status, module.Fact{
			LabelKey: "ginrummy.status.lastHand", Value: last.Winner,
			Params: map[string]any{"winner": last.Winner, "kind": last.Kind, "delta": last.HandDelta},
		})
	}

	if s.Status == "completed" {
		vm.Status = append(vm.Status, module.Fact{
			LabelKey: "status.winner", Value: s.WinnerID,
			Params: map[string]any{"winners": []string{s.WinnerID}},
		})
	}

	return vm, nil
}

func cardViews(cards []string) []module.CardView {
	out := make([]module.CardView, 0, len(cards))
	for _, c := range cards {
		out = append(out, module.CardView{Card: c})
	}
	return out
}

// topOnly is what the discard pile shows: its top card, or nothing while it
// is empty (during the upcard dance, once someone has taken it).
func topOnly(pile []string) []string {
	if len(pile) == 0 {
		return nil
	}
	return []string{pile[len(pile)-1]}
}

// Bot is Gin Rummy's own player — see bot.go. module.OfferBot cannot play
// this game at all: every legal discard is offered, so which one to throw is
// the entire game, and an offer-preference bot would throw them in hand order.
func (m *Module) Bot() module.Bot { return bot{} }

// Standings ranks by running score, higher first — no negation, unlike a
// penalty-scored rummy, because a gin rummy line score already counts up.
func (m *Module) Standings(raw module.State) ([]module.Standing, error) {
	s, err := decode(raw)
	if err != nil {
		return nil, err
	}
	return module.RankByScore(s.Players, func(id string) int { return s.Scores[id] }, "ginrummy.unit.points"), nil
}
