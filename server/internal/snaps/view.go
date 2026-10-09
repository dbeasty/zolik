package snaps

import "zolik/server/internal/module"

// The ids this module draws its zones under, shared with the offers that
// point at them.
const (
	talonZoneID = "talon"
	trumpZoneID = "trumpCard"
	trickZoneID = "trick"
)

func handZoneID(playerID string) string { return "hand:" + playerID }

var (
	_ module.Ranked     = (*Module)(nil)
	_ module.OpenViewer = (*Module)(nil)
	_ module.Rounded    = (*Module)(nil)
	_ module.Botted     = (*Module)(nil)
)

// Descriptor is Šnaps's self-description.
func (m *Module) Descriptor() module.ModuleDescriptor {
	return module.ModuleDescriptor{
		ID:         "snaps",
		Label:      "Šnaps",
		MinPlayers: 2,
		MaxPlayers: 2,
		Deck:       module.DeckGerman,
		Variations: []module.VariationSpec{
			{
				ID:    variationSnaps,
				Label: "Šnaps",
				Summary: []module.Fact{
					{LabelKey: "snaps.rules.deck20"},
					{LabelKey: "snaps.rules.goal"},
					{LabelKey: "snaps.rules.close"},
				},
				Defaults: defaults,
			},
			{
				ID:    variation66,
				Label: "Šedesát šest",
				Summary: []module.Fact{
					{LabelKey: "snaps.rules.deck24"},
					{LabelKey: "snaps.rules.goal"},
					{LabelKey: "snaps.rules.marriageOpenOnly"},
				},
				Defaults: defaults,
			},
		},
		Options: []module.OptionSpec{
			{
				Name:  OptTarget,
				Type:  module.OptionEnumInt,
				Label: "Match",
				Help:  "How many game points win the match. A deal is worth 1, 2 or 3.",
				Choices: []module.OptionChoice{
					{Value: 3, Label: "3"},
					{Value: 7, Label: "7"},
					{Value: 11, Label: "11"},
				},
			},
			{
				Name:  OptShowCardPoints,
				Type:  module.OptionEnumInt,
				Label: "Show card points",
				Help:  "Show both players' card points as the deal is played. Your own count is always shown to you.",
				Choices: []module.OptionChoice{
					{Value: module.OptOff, Label: "Hidden"},
					{Value: module.OptOn, Label: "Shown"},
				},
			},
			module.BotSkillOption(),
			module.PauseOption(),
			module.StandInOption(),
		},
	}
}

var defaults = map[string]int{
	OptTarget:                    defaultTarget,
	OptShowCardPoints:            module.OptOff,
	module.OptBotSkill:           module.SkillOpt(module.SkillMedium),
	module.OptPauseBetweenRounds: module.OptOn,
	module.OptStandInAfter:       module.StandInAfterDefault,
}

// resolve reads a lobby's config against the defaults.
func resolve(cfg module.MatchConfig) config {
	variation := variationSnaps
	if cfg.Variation == variation66 {
		variation = variation66
	}
	opt := func(name string) int { return cfg.Opt(name, defaults[name]) }
	return config{
		variation:      variation,
		target:         max(1, opt(OptTarget)),
		showCardPoints: opt(OptShowCardPoints) == module.OptOn,
	}
}

// config is the match's options as a lobby would send them, for the rules
// a refusal points at.
func (s *GameState) config() module.MatchConfig {
	return module.MatchConfig{Variation: s.Variation, Options: module.Options{
		OptTarget:         s.Target,
		OptShowCardPoints: module.BoolOpt(s.ShowCardPoints),
	}}
}

// View is the board as one seat sees it.
//
// What is secret in Šnaps, and so filtered here: the other hand, except
// for the cards the table watched go into it, and the talon under its
// turned-up trump (all of it, once it is closed).
func (m *Module) View(raw module.State, viewerID string) (module.ViewModel, error) {
	return m.view(raw, viewerID, false)
}

// OpenView is the board with everything face up, for replaying a finished
// match.
func (m *Module) OpenView(raw module.State) (module.ViewModel, error) {
	return m.view(raw, "", true)
}

func (m *Module) view(raw module.State, viewerID string, reveal bool) (module.ViewModel, error) {
	s, err := decode(raw)
	if err != nil {
		return module.ViewModel{}, err
	}
	vm := module.ViewModel{}
	playing := !s.Intermission.Open && s.Status == "active"
	seated := viewerID != "" && s.seat(viewerID) >= 0

	if seated {
		z := module.Zone{
			ID: handZoneID(viewerID), Kind: module.ZoneHand, OwnerID: viewerID,
			LabelKey: "zone.yourHand", Count: len(s.Hands[viewerID]),
		}
		for _, c := range s.Hands[viewerID] {
			z.Cards = append(z.Cards, module.CardView{Card: c})
		}
		vm.Zones = append(vm.Zones, z)
	}
	for _, p := range s.Players {
		if p == viewerID {
			continue
		}
		z := module.Zone{
			ID: handZoneID(p), Kind: module.ZoneHand, OwnerID: p,
			LabelKey: "zone.opponentHand", Count: len(s.Hands[p]),
		}
		if reveal {
			for _, c := range s.Hands[p] {
				z.Cards = append(z.Cards, module.CardView{Card: c})
			}
		}
		vm.Zones = append(vm.Zones, z)
	}

	// The trick, each card toward whoever played it; between tricks the one
	// just taken stays until the next lead.
	shown := s.Trick
	if len(shown) == 0 {
		shown = s.LastTrick
	}
	if playing || len(shown) > 0 {
		z := module.Zone{
			ID: trickZoneID, Kind: module.ZoneSpread, Shared: true, Arrange: module.ArrangeBySeat,
			LabelKey: "snaps.zone.trick", Count: len(shown),
		}
		if len(s.Trick) == 0 && len(s.LastTrick) > 0 {
			z.LabelKey = "snaps.zone.lastTrick"
		}
		for _, pl := range shown {
			z.Cards = append(z.Cards, module.CardView{Card: pl.Card, By: s.Players[pl.Seat]})
		}
		vm.Zones = append(vm.Zones, z)
	}

	// The talon, face down, and the trump turned up under it while it is
	// open. Closed, the trump lies face down on top.
	if len(s.Talon) > 0 {
		hidden := len(s.Talon) - 1
		z := module.Zone{ID: talonZoneID, Kind: module.ZoneStack, LabelKey: "snaps.zone.talon"}
		if s.ClosedBy != "" {
			hidden = len(s.Talon)
			z.LabelKey = "snaps.zone.talonClosed"
		}
		z.Count = hidden
		if reveal {
			for _, c := range s.Talon[:hidden] {
				z.Cards = append(z.Cards, module.CardView{Card: c})
			}
		}
		vm.Zones = append(vm.Zones, z)
		if up := s.turnedUp(); up != "" {
			vm.Zones = append(vm.Zones, module.Zone{
				ID: trumpZoneID, Kind: module.ZoneSpread, Shared: true, LabelKey: "snaps.zone.trumpCard",
				Count: 1, Cards: []module.CardView{{Card: up}},
			})
		}
	}

	for _, p := range s.Players {
		seat := module.Seat{
			PlayerID: p,
			Active:   (s.Intermission.Open && !s.Intermission.Ready[p]) || (playing && s.Current == p),
		}
		if s.Intermission.Open && s.Intermission.Ready[p] {
			seat.LabelKeys = append(seat.LabelKeys, "seat.ready")
		}
		if playing {
			if p == s.dealer() {
				seat.LabelKeys = append(seat.LabelKeys, "snaps.seat.dealer")
			}
			if p == s.ClosedBy {
				seat.LabelKeys = append(seat.LabelKeys, "snaps.seat.closed")
			}
			seat.Facts = append(seat.Facts, module.Fact{
				LabelKey: "snaps.seat.tricks", Params: map[string]any{"n": s.TricksWon[p]},
			})
			if s.ShowCardPoints || reveal {
				seat.Facts = append(seat.Facts, module.Fact{
					LabelKey: "snaps.seat.points", Params: map[string]any{"n": s.total(p)},
				})
			}
			for _, suit := range s.Marriages[p] {
				seat.Facts = append(seat.Facts, module.Fact{
					LabelKey: "snaps.seat.marriage",
					Params:   map[string]any{"suit": suitKey(suit), "n": s.marriageValue(suit)},
				})
			}
		}
		vm.Seats = append(vm.Seats, seat)
	}

	vm.Header = []module.Fact{
		{LabelKey: "snaps.header.deal", Params: map[string]any{"n": s.Deal + 1}},
		{LabelKey: "snaps.header.trumps", Params: map[string]any{"suit": suitKey(s.Trump)}},
	}
	if playing && s.ClosedBy != "" {
		vm.Header = append(vm.Header, module.Fact{
			LabelKey: "snaps.header.closed", Params: map[string]any{"player": s.ClosedBy},
		})
	} else if playing && len(s.Talon) == 0 {
		vm.Header = append(vm.Header, module.Fact{LabelKey: "snaps.header.talonOut"})
	}
	// A player can always count their own tricks, so their count is theirs
	// to see whatever the table shows the other.
	if playing && seated && !s.ShowCardPoints {
		vm.Header = append(vm.Header, module.Fact{
			LabelKey: "snaps.header.yourPoints", Params: map[string]any{"n": s.total(viewerID)},
		})
	}

	if playing && viewerID == s.Current {
		switch {
		case len(s.Trick) > 0 && s.strict():
			vm.Prompts = append(vm.Prompts, module.Fact{LabelKey: "snaps.prompt.followStrict"})
		case len(s.Trick) > 0:
			vm.Prompts = append(vm.Prompts, module.Fact{LabelKey: "snaps.prompt.follow"})
		default:
			vm.Prompts = append(vm.Prompts, module.Fact{LabelKey: "snaps.prompt.lead"})
		}
	}
	return vm, nil
}

// Keys for the words a fact names rather than says.
func suitKey(suit string) string { return module.GermanSuitKey(suit) }

// reasonFact says why a deal ended, each key written out so the manifest
// finds it.
func reasonFact(reason string) module.Fact {
	switch reason {
	case reasonCloserFailed:
		return module.Fact{LabelKey: "snaps.round.closerFailed"}
	case reasonLastTrick:
		return module.Fact{LabelKey: "snaps.round.lastTrick"}
	case reasonHigher:
		return module.Fact{LabelKey: "snaps.round.higher"}
	case reasonDraw:
		return module.Fact{LabelKey: "snaps.round.draw"}
	}
	return module.Fact{LabelKey: "snaps.round.out"}
}

// Standings ranks by game points, higher first.
func (m *Module) Standings(raw module.State) ([]module.Standing, error) {
	s, err := decode(raw)
	if err != nil {
		return nil, err
	}
	return module.RankByScore(s.Players, func(id string) int { return s.Scores[id] }, "snaps.unit.gamePoints"), nil
}

// Rounds is the deal-by-deal history: who won each and why. It names
// players and counts, never a card.
func (m *Module) Rounds(raw module.State) (module.RoundLog, error) {
	s, err := decode(raw)
	if err != nil {
		return module.RoundLog{}, err
	}
	log := module.RoundLog{
		LabelKey:   "snaps.round.deal",
		Rounds:     []module.RoundResult{},
		Paused:     s.Intermission.Open,
		WaitingFor: s.Intermission.Waiting(s.Players),
	}
	for _, d := range s.Rounds {
		r := module.RoundResult{Number: d.Number}
		if d.Winner == "" {
			r.Headline = &module.Fact{LabelKey: "snaps.round.drawn"}
		} else {
			r.Headline = &module.Fact{
				LabelKey: "snaps.round.headline",
				Params:   map[string]any{"player": d.Winner, "n": d.GamePoints},
			}
			r.Winners = []string{d.Winner}
		}
		r.Facts = append(r.Facts, reasonFact(d.Reason))
		if d.ClosedBy != "" {
			r.Facts = append(r.Facts, module.Fact{
				LabelKey: "snaps.header.closed", Params: map[string]any{"player": d.ClosedBy},
			})
		}
		for _, p := range s.Players {
			r.Facts = append(r.Facts, module.Fact{
				LabelKey: "snaps.round.points", Params: map[string]any{"player": p, "n": d.Points[p]},
			})
		}
		for _, p := range s.Players {
			delta := 0
			if p == d.Winner {
				delta = d.GamePoints
			}
			r.Scores = append(r.Scores, module.RoundScore{PlayerID: p, Delta: delta, Total: d.Totals[p]})
		}
		log.Rounds = append(log.Rounds, r)
	}
	return log, nil
}

// Bot is Šnaps's own bot — see bot.go.
func (m *Module) Bot() module.Bot { return bot{} }
