package rummytiles

import (
	"strconv"

	"zolik/server/internal/module"
)

// Option names, and the values each variation starts from.
const (
	OptTargetScore    = "targetScore"
	OptRoundLimit     = "roundLimit"
	OptPoolExhaustion = "poolExhaustion"
)

type variationDefaults struct {
	targetScore              int
	roundLimit               int
	poolExhaustionLowestWins bool
}

var variations = map[string]variationDefaults{
	"standard": {targetScore: 200, roundLimit: 0, poolExhaustionLowestWins: true},
}

func resolveVariation(cfg module.MatchConfig) variationDefaults {
	if v, ok := variations[cfg.Variation]; ok {
		return v
	}
	return variations["standard"]
}

// Descriptor is Rummy Tiles' self-description. Two to four seats — see §6.2
// of the plan this module implements: two-handed is a played form, not a
// degenerate case, and the cheaper table to run the conformance and dead-end
// corpus against.
func (m *Module) Descriptor() module.ModuleDescriptor {
	return module.ModuleDescriptor{
		ID: "rummytiles", Label: "Rummy Tiles", MinPlayers: 2, MaxPlayers: 4,
		Variations: []module.VariationSpec{
			{
				ID: "standard", Label: "Standard",
				Summary: []module.Fact{
					{LabelKey: "rummytiles.rules.tiles", Value: "106"},
					{LabelKey: "rummytiles.rules.dealCount", Value: "14"},
				},
				Defaults: map[string]int{
					OptTargetScore:               variations["standard"].targetScore,
					OptRoundLimit:                variations["standard"].roundLimit,
					OptPoolExhaustion:            module.BoolOpt(variations["standard"].poolExhaustionLowestWins),
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
				Label: "Target score", Help: "First player to pass this score wins the match. Off disables it in favor of a round limit.",
				Choices: []module.OptionChoice{
					{Value: 0, Label: "Off"}, {Value: 200, Label: "200"},
					{Value: 300, Label: "300"}, {Value: 500, Label: "500"},
				},
			},
			{
				Name: OptRoundLimit, Type: module.OptionEnumInt,
				Label: "Round limit", Help: "The match ends after this many rounds, highest score wins. Off disables it in favor of a target score.",
				Choices: []module.OptionChoice{
					{Value: 0, Label: "Off"}, {Value: 5, Label: "5"},
					{Value: 10, Label: "10"}, {Value: 15, Label: "15"},
				},
			},
			{
				Name: OptPoolExhaustion, Type: module.OptionEnumInt,
				Label: "Pool exhaustion", Help: "If the pool runs dry and nobody can play, who — if anyone — is recorded as winning the round.",
				Choices: []module.OptionChoice{
					{Value: module.OptOn, Label: "Lowest hand wins the round"},
					{Value: module.OptOff, Label: "Nobody wins the round"},
				},
			},
		},
	}
}

// View renders the board for one viewer.
//
// A player's own hand is theirs alone; the workspace belongs to whoever is on
// turn and is visible to everyone the moment it exists, because a real table
// shows a rearrangement in progress to every player at it — there is nothing
// to hide about a manipulation that has not been committed yet, only about
// what remains in a hand.
func (m *Module) View(raw module.State, viewerID string) (module.ViewModel, error) {
	s, err := decode(raw)
	if err != nil {
		return module.ViewModel{}, err
	}
	vm := module.ViewModel{}

	for _, p := range s.Players {
		hand := s.Hands[p]
		if p == viewerID {
			vm.Zones = append(vm.Zones, module.Zone{
				ID: handZoneID(p), Kind: module.ZoneHand, OwnerID: p,
				LabelKey: "zone.yourHand", Cards: cardViews(hand), Count: len(hand),
			})
			continue
		}
		vm.Zones = append(vm.Zones, module.Zone{
			ID: handZoneID(p), Kind: module.ZoneHand, OwnerID: p,
			LabelKey: "zone.opponentHand", Count: len(hand),
		})
	}

	vm.Zones = append(vm.Zones,
		module.Zone{ID: poolZoneID, Kind: module.ZoneStack, LabelKey: "rummytiles.zone.pool", Count: len(s.Pool)},
	)

	sets := s.Sets
	if s.Workspace != nil {
		sets = s.Workspace.Sets
	}
	tableZone := module.Zone{ID: tableZoneID, Kind: module.ZoneSpread, LabelKey: "rummytiles.zone.table"}
	for _, set := range sets {
		g := module.Group{ID: set.ID, Kind: set.Kind, Cards: append([]string(nil), set.Cards...)}
		if set.Kind == "" {
			g.BadgeKeys = []string{"rummytiles.badge.invalid"}
		}
		tableZone.Groups = append(tableZone.Groups, g)
		tableZone.Count += len(set.Cards)
	}
	vm.Zones = append(vm.Zones, tableZone)

	if s.Workspace != nil && len(s.Workspace.Tray) > 0 {
		vm.Zones = append(vm.Zones, module.Zone{
			ID: trayZoneID, Kind: module.ZoneSpread, LabelKey: "rummytiles.zone.tray",
			Cards: cardViews(s.Workspace.Tray), Count: len(s.Workspace.Tray),
		})
	}

	for _, p := range s.Players {
		seat := module.Seat{
			PlayerID: p,
			Active:   (s.Intermission.Open && !s.Intermission.Ready[p]) || (s.Current == p && s.Status == "active"),
			Facts: []module.Fact{
				{LabelKey: "seat.cards", Value: strconv.Itoa(len(s.Hands[p])), Params: map[string]any{"n": len(s.Hands[p])}},
				{LabelKey: "rummytiles.seat.score", Value: strconv.Itoa(s.Scores[p]), Params: map[string]any{"score": s.Scores[p]}},
			},
		}
		if !s.InitialMeld[p] {
			seat.LabelKeys = []string{"rummytiles.seat.notOpened"}
		}
		vm.Seats = append(vm.Seats, seat)
	}

	vm.Header = []module.Fact{
		{LabelKey: "rummytiles.header.pool", Value: strconv.Itoa(len(s.Pool)), Params: map[string]any{"n": len(s.Pool)}},
		{LabelKey: "rummytiles.header.round", Value: strconv.Itoa(s.RoundNumber + 1), Params: map[string]any{"n": s.RoundNumber + 1}},
	}
	if s.TargetScore > 0 {
		vm.Header = append(vm.Header, module.Fact{LabelKey: "header.target", Value: strconv.Itoa(s.TargetScore)})
	}

	if s.Current == viewerID && s.Status == "active" {
		key := "rummytiles.prompt.yourTurn"
		if !s.InitialMeld[viewerID] {
			key = "rummytiles.prompt.initialMeld"
		}
		vm.Prompts = append(vm.Prompts, module.Fact{LabelKey: key, Params: map[string]any{"n": 30}})
	}

	if len(s.Rounds) > 0 {
		last := s.Rounds[len(s.Rounds)-1]
		vm.Status = append(vm.Status, module.Fact{
			LabelKey: "rummytiles.status.lastRound", Value: last.Winner,
			Params: map[string]any{"winner": last.Winner, "kind": last.Kind},
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

func cardViews(tiles []string) []module.CardView {
	out := make([]module.CardView, 0, len(tiles))
	for _, t := range tiles {
		out = append(out, module.CardView{Card: t})
	}
	return out
}

// Bot is Rummy Tiles' own player — see bot.go. module.OfferBot cannot play
// this game at all: the offers describe single moves and the game is about
// combinations, so a bot taking whichever offer comes first would shuffle
// the table forever.
func (m *Module) Bot() module.Bot { return bot{} }

// Standings ranks by running score, higher first.
func (m *Module) Standings(raw module.State) ([]module.Standing, error) {
	s, err := decode(raw)
	if err != nil {
		return nil, err
	}
	return module.RankByScore(s.Players, func(id string) int { return s.Scores[id] }, "rummytiles.unit.points"), nil
}
