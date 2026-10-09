package sedma

import "zolik/server/internal/module"

// The ids this module draws its zones under, shared with the offers that
// point at them.
const (
	stockZoneID = "stock"
	trickZoneID = "trick"
)

func handZoneID(playerID string) string { return "hand:" + playerID }

var (
	_ module.Ranked     = (*Module)(nil)
	_ module.OpenViewer = (*Module)(nil)
	_ module.Rounded    = (*Module)(nil)
	_ module.Botted     = (*Module)(nil)
	_ module.Seated     = (*Module)(nil)
)

// Descriptor is Sedma's self-description.
func (m *Module) Descriptor() module.ModuleDescriptor {
	return module.ModuleDescriptor{
		ID:         "sedma",
		Label:      "Sedma",
		MinPlayers: 2,
		MaxPlayers: 4,
		Deck:       module.DeckGerman,
		Variations: []module.VariationSpec{{
			ID:    "classic",
			Label: "Classic",
			Summary: []module.Fact{
				{LabelKey: "sedma.rules.capture"},
				{LabelKey: "sedma.rules.goal"},
				{LabelKey: "sedma.rules.continue"},
			},
			Defaults: defaults,
		}},
		Options: []module.OptionSpec{
			{
				Name:  OptTarget,
				Type:  module.OptionEnumInt,
				Label: "Play to",
				Help:  "How many stakes win the match. A hand is worth 1, 2 or 3.",
				Choices: []module.OptionChoice{
					{Value: 3, Label: "3"},
					{Value: 5, Label: "5"},
					{Value: 10, Label: "10"},
				},
			},
			{
				Name:  OptShowCardPoints,
				Type:  module.OptionEnumInt,
				Label: "Show card points",
				Help:  "Show every side's points as the hand is played. Your own side's are always shown to you.",
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

type config struct {
	target         int
	showCardPoints bool
}

func resolve(cfg module.MatchConfig) config {
	opt := func(name string) int { return cfg.Opt(name, defaults[name]) }
	return config{
		target:         max(1, opt(OptTarget)),
		showCardPoints: opt(OptShowCardPoints) == module.OptOn,
	}
}

// config is the match's options as a lobby would send them.
func (s *GameState) config() module.MatchConfig {
	return module.MatchConfig{Options: module.Options{
		OptTarget:         s.Target,
		OptShowCardPoints: module.BoolOpt(s.ShowCardPoints),
	}}
}

// Sides are the partnerships at a table of four, partners opposite; nil
// otherwise, where everyone plays alone.
func (m *Module) Sides(_ module.MatchConfig, players []module.PlayerRef) [][]string {
	if len(players) != 4 {
		return nil
	}
	return [][]string{{players[0].ID, players[2].ID}, {players[1].ID, players[3].ID}}
}

// View is the board as one seat sees it. Secret: the other hands and the
// stock.
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
		if seated && s.teams() && s.side(p) == s.side(viewerID) {
			z.LabelKey = "sedma.zone.partnerHand"
		}
		if reveal {
			for _, c := range s.Hands[p] {
				z.Cards = append(z.Cards, module.CardView{Card: c})
			}
		}
		vm.Zones = append(vm.Zones, z)
	}

	// The trick, every round of it, each card marked with who played it;
	// between tricks the last one stays until the next lead.
	shown := s.Trick
	if len(shown) == 0 {
		shown = s.LastTrick
	}
	if playing || len(shown) > 0 {
		z := module.Zone{
			ID: trickZoneID, Kind: module.ZoneSpread, Shared: true,
			LabelKey: "sedma.zone.trick", Count: len(shown),
		}
		if len(s.Trick) == 0 && len(s.LastTrick) > 0 {
			z.LabelKey = "sedma.zone.lastTrick"
		}
		for _, pl := range shown {
			z.Cards = append(z.Cards, module.CardView{Card: pl.Card, By: s.Players[pl.Seat]})
		}
		vm.Zones = append(vm.Zones, z)
	}
	if len(s.Stock) > 0 {
		z := module.Zone{ID: stockZoneID, Kind: module.ZoneStack, LabelKey: "sedma.zone.stock", Count: len(s.Stock)}
		if reveal {
			for _, c := range s.Stock {
				z.Cards = append(z.Cards, module.CardView{Card: c})
			}
		}
		vm.Zones = append(vm.Zones, z)
	}

	winner := ""
	if len(s.Trick) > 0 {
		winner = s.Players[winning(s.Trick)]
	}
	for _, p := range s.Players {
		seat := module.Seat{
			PlayerID: p,
			Active:   (s.Intermission.Open && !s.Intermission.Ready[p]) || (playing && s.Current == p),
		}
		if s.teams() {
			seat.Side = s.side(p)
		}
		if s.Intermission.Open && s.Intermission.Ready[p] {
			seat.LabelKeys = append(seat.LabelKeys, "seat.ready")
		}
		if playing {
			if p == s.dealer() {
				seat.LabelKeys = append(seat.LabelKeys, "sedma.seat.dealer")
			}
			if p == winner {
				seat.LabelKeys = append(seat.LabelKeys, "sedma.seat.winning")
			}
			if s.ShowCardPoints || reveal {
				seat.Facts = append(seat.Facts, module.Fact{
					LabelKey: "sedma.seat.points", Params: map[string]any{"n": s.Points[s.side(p)]},
				})
			}
		}
		vm.Seats = append(vm.Seats, seat)
	}

	vm.Header = []module.Fact{{LabelKey: "sedma.header.hand", Params: map[string]any{"n": s.Deal + 1}}}
	if playing && seated && !s.ShowCardPoints {
		n := s.Points[s.side(viewerID)]
		if s.teams() {
			vm.Header = append(vm.Header, module.Fact{LabelKey: "sedma.header.sidePoints", Params: map[string]any{"n": n}})
		} else {
			vm.Header = append(vm.Header, module.Fact{LabelKey: "sedma.header.yourPoints", Params: map[string]any{"n": n}})
		}
	}
	if playing && len(s.Trick) > 0 {
		vm.Header = append(vm.Header, module.Fact{
			LabelKey: "sedma.header.led", Params: map[string]any{"rank": rankKey(s.led())},
		})
	}

	if playing && viewerID == s.Current {
		switch {
		case s.Phase == phaseDecide:
			vm.Prompts = append(vm.Prompts, module.Fact{
				LabelKey: "sedma.prompt.decide", Params: map[string]any{"rank": rankKey(s.led())},
			})
		case len(s.Trick) == 0:
			vm.Prompts = append(vm.Prompts, module.Fact{LabelKey: "sedma.prompt.lead"})
		default:
			vm.Prompts = append(vm.Prompts, module.Fact{LabelKey: "sedma.prompt.play"})
		}
	}
	return vm, nil
}

// rankKey names a rank in a sentence, each key written out so the manifest
// finds it.
func rankKey(r byte) string {
	switch r {
	case 'A':
		return "sedma.rank.A"
	case 'K':
		return "sedma.rank.K"
	case 'Q':
		return "sedma.rank.Q"
	case 'J':
		return "sedma.rank.J"
	case 'T':
		return "sedma.rank.T"
	case '9':
		return "sedma.rank.9"
	case '8':
		return "sedma.rank.8"
	}
	return "sedma.rank.7"
}

func kindFact(kind string) module.Fact {
	switch kind {
	case kindAllPoints:
		return module.Fact{LabelKey: "sedma.round.allPoints"}
	case kindAllCards:
		return module.Fact{LabelKey: "sedma.round.allCards"}
	case kindTie:
		return module.Fact{LabelKey: "sedma.round.tie"}
	case kindNone:
		return module.Fact{LabelKey: "sedma.round.none"}
	}
	return module.Fact{LabelKey: "sedma.round.won"}
}

// Standings ranks by stakes, higher first.
func (m *Module) Standings(raw module.State) ([]module.Standing, error) {
	s, err := decode(raw)
	if err != nil {
		return nil, err
	}
	return module.RankByScore(s.Players, func(id string) int { return s.Scores[id] }, "sedma.unit.stakes"), nil
}

// Rounds is the hand-by-hand history.
func (m *Module) Rounds(raw module.State) (module.RoundLog, error) {
	s, err := decode(raw)
	if err != nil {
		return module.RoundLog{}, err
	}
	log := module.RoundLog{
		LabelKey:   "sedma.round.hand",
		Rounds:     []module.RoundResult{},
		Paused:     s.Intermission.Open,
		WaitingFor: s.Intermission.Waiting(s.Players),
	}
	for _, h := range s.Rounds {
		r := module.RoundResult{Number: h.Number, Winners: h.Winners}
		r.Facts = append(r.Facts, kindFact(h.Kind))
		seen := map[string]bool{}
		for _, p := range s.Players {
			side := sideOf(s.seat(p), len(s.Players))
			if seen[side] {
				continue
			}
			seen[side] = true
			r.Facts = append(r.Facts, module.Fact{
				LabelKey: "sedma.round.points", Params: map[string]any{"player": p, "n": h.Points[p]},
			})
		}
		won := map[string]bool{}
		for _, w := range h.Winners {
			won[w] = true
		}
		for _, p := range s.Players {
			delta := 0
			if won[p] {
				delta = h.Stakes
			}
			r.Scores = append(r.Scores, module.RoundScore{PlayerID: p, Delta: delta, Total: h.Totals[p]})
		}
		log.Rounds = append(log.Rounds, r)
	}
	return log, nil
}

// Bot is Sedma's own bot — see bot.go.
func (m *Module) Bot() module.Bot { return bot{} }
