package lora

import (
	"sort"

	"zolik/server/internal/module"
	"zolik/server/internal/tricks"
)

// The ids this module draws its zones under, shared with the offers that
// point at them.
const (
	trickZoneID = "trick"
	quartZoneID = "quart"
	rowsZoneID  = "rows"
)

func handZoneID(playerID string) string { return "hand:" + playerID }

var (
	_ module.Ranked     = (*Module)(nil)
	_ module.OpenViewer = (*Module)(nil)
	_ module.Rounded    = (*Module)(nil)
	_ module.Botted     = (*Module)(nil)
)

// Descriptor is Lóra's self-description.
func (m *Module) Descriptor() module.ModuleDescriptor {
	return module.ModuleDescriptor{
		ID:         "lora",
		Label:      "Lóra",
		MinPlayers: seats,
		MaxPlayers: seats,
		Deck:       module.DeckGerman,
		Variations: []module.VariationSpec{{
			ID:    "classic",
			Label: "Classic",
			Summary: []module.Fact{
				{LabelKey: "lora.rules.goal"},
				{LabelKey: "lora.rules.talie"},
				{LabelKey: "lora.rules.maturita"},
			},
			Defaults: defaults,
		}},
		Options: []module.OptionSpec{
			{
				Name:  OptTalie,
				Type:  module.OptionEnumInt,
				Label: "Tálie",
				Help:  "How many tálie the match lasts. Each player leads one in a full match of four.",
				Choices: []module.OptionChoice{
					{Value: 1, Label: "1"},
					{Value: 2, Label: "2"},
					{Value: 4, Label: "4"},
				},
			},
			{
				Name:  OptMaturita,
				Type:  module.OptionEnumInt,
				Label: "Maturita",
				Help:  "End each tálie with the leader's maturita: a game of their choice, to be played without a single penalty point.",
				Choices: []module.OptionChoice{
					{Value: module.OptOff, Label: "Not played"},
					{Value: module.OptOn, Label: "Played"},
				},
			},
			module.BotSkillOption(),
			module.PauseOption(),
			module.StandInOption(),
		},
	}
}

var defaults = map[string]int{
	OptTalie:                     defaultTalie,
	OptMaturita:                  module.OptOn,
	module.OptBotSkill:           module.SkillOpt(module.SkillMedium),
	module.OptPauseBetweenRounds: module.OptOn,
	module.OptStandInAfter:       module.StandInAfterDefault,
}

type config struct {
	talies   int
	maturita bool
}

func resolve(cfg module.MatchConfig) config {
	opt := func(name string) int { return cfg.Opt(name, defaults[name]) }
	return config{
		talies:   max(1, opt(OptTalie)),
		maturita: opt(OptMaturita) == module.OptOn,
	}
}

// config is the match's options as a lobby would send them.
func (s *GameState) config() module.MatchConfig {
	return module.MatchConfig{Options: module.Options{
		OptTalie:    s.Talies,
		OptMaturita: module.BoolOpt(s.Maturita),
	}}
}

// View is the board as one seat sees it. Secret: the other hands.
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

	switch {
	case trickGame(s.Contract) && s.Contract != "":
		// The trick in progress; between tricks the last one stays until
		// the next lead.
		shown := s.Trick
		if len(shown) == 0 {
			shown = s.LastTrick
		}
		if playing || len(shown) > 0 {
			z := module.Zone{
				ID: trickZoneID, Kind: module.ZoneSpread, Shared: true,
				LabelKey: "lora.zone.trick", Count: len(shown),
			}
			if len(s.Trick) == 0 && len(s.LastTrick) > 0 {
				z.LabelKey = "lora.zone.lastTrick"
			}
			for _, pl := range shown {
				z.Cards = append(z.Cards, module.CardView{Card: pl.Card, By: s.Players[pl.Seat]})
			}
			vm.Zones = append(vm.Zones, z)
		}
	case s.Contract == gameKvarty:
		z := module.Zone{
			ID: quartZoneID, Kind: module.ZoneSpread, Shared: true,
			LabelKey: "lora.zone.quart", Count: len(s.Quart),
		}
		if len(s.Quart) > 0 {
			z.LabelKey = "lora.zone.lastQuart"
		}
		for _, pl := range s.Quart {
			z.Cards = append(z.Cards, module.CardView{Card: pl.Card, By: s.Players[pl.Seat]})
		}
		vm.Zones = append(vm.Zones, z)
	case s.Contract == gameDesitky:
		z := module.Zone{ID: rowsZoneID, Kind: module.ZoneSpread, Shared: true, LabelKey: "lora.zone.rows", Count: len(s.Laid)}
		for _, suit := range suits {
			var row []string
			for _, c := range s.Laid {
				if tricks.Suit(c) == byte(suit) {
					row = append(row, c)
				}
			}
			if len(row) == 0 {
				continue
			}
			sort.Slice(row, func(i, j int) bool { return rankIndex(row[i]) < rankIndex(row[j]) })
			z.Groups = append(z.Groups, module.Group{ID: "row:" + string(suit), Kind: "run", Cards: row})
		}
		vm.Zones = append(vm.Zones, z)
	}

	maturant := s.maturant()
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
				seat.LabelKeys = append(seat.LabelKeys, "lora.seat.dealer")
			}
			if p == maturant {
				seat.LabelKeys = append(seat.LabelKeys, "lora.seat.maturant")
			}
			if s.Contract != "" {
				seat.Facts = append(seat.Facts, module.Fact{
					LabelKey: "lora.seat.penalty", Params: map[string]any{"n": s.Points[p]},
				})
			}
			if trickGame(s.Contract) && s.Contract != "" {
				seat.Facts = append(seat.Facts, module.Fact{
					LabelKey: "lora.seat.tricks", Params: map[string]any{"n": s.Won[p]},
				})
			}
			if s.Contract == gameDesitky && s.Tuks[p] > 0 {
				seat.Facts = append(seat.Facts, module.Fact{
					LabelKey: "lora.seat.tuks", Params: map[string]any{"n": s.Tuks[p]},
				})
			}
		}
		vm.Seats = append(vm.Seats, seat)
	}

	vm.Header = []module.Fact{{
		LabelKey: "lora.header.talie", Params: map[string]any{"n": min(s.Talie+1, s.Talies), "of": s.Talies},
	}}
	if playing {
		switch {
		case s.maturita() && s.Contract == "":
			vm.Header = append(vm.Header, module.Fact{LabelKey: "lora.header.maturita", Params: map[string]any{"n": s.Attempt + 1}})
		case s.maturita():
			vm.Header = append(vm.Header,
				module.Fact{LabelKey: "lora.header.maturita", Params: map[string]any{"n": s.Attempt + 1}},
				module.Fact{LabelKey: "lora.header.game", Params: map[string]any{"game": gameKey(s.Contract)}})
		default:
			vm.Header = append(vm.Header, module.Fact{
				LabelKey: "lora.header.game", Params: map[string]any{"game": gameKey(s.Contract)},
			})
		}
	}

	if playing && viewerID == s.Current {
		vm.Prompts = append(vm.Prompts, s.prompt(viewerID))
	}
	return vm, nil
}

// prompt says what the seat on turn is to do.
func (s *GameState) prompt(p string) module.Fact {
	switch s.Phase {
	case phaseChoose:
		return module.Fact{LabelKey: "lora.prompt.choose"}
	case phaseQuart:
		return module.Fact{LabelKey: "lora.prompt.quart"}
	case phaseTens:
		switch {
		case s.Turned:
			return module.Fact{LabelKey: "lora.prompt.more"}
		case len(s.legal(p)) > 0:
			return module.Fact{LabelKey: "lora.prompt.lay"}
		}
		return module.Fact{LabelKey: "lora.prompt.tuk"}
	}
	if len(s.Trick) > 0 {
		return module.Fact{LabelKey: "lora.prompt.follow"}
	}
	if redLeadRule(s.Contract) {
		return module.Fact{LabelKey: "lora.prompt.leadNoRed"}
	}
	return module.Fact{LabelKey: "lora.prompt.lead"}
}

// gameKey names a game, each key written out so the manifest finds it.
func gameKey(g string) string {
	switch g {
	case gameCervene:
		return "lora.game.cervene"
	case gameFilky:
		return "lora.game.filky"
	case gamePrPo:
		return "lora.game.prpo"
	case gameVsechny:
		return "lora.game.vsechny"
	case gameKral:
		return "lora.game.kral"
	case gameKvarty:
		return "lora.game.kvarty"
	}
	return "lora.game.desitky"
}

// Standings ranks by penalty points, fewest first. A maturant who failed
// their last attempt finishes last whatever their points.
func (m *Module) Standings(raw module.State) ([]module.Standing, error) {
	s, err := decode(raw)
	if err != nil {
		return nil, err
	}
	var ranked []string
	for _, p := range s.Players {
		if p != s.Failed {
			ranked = append(ranked, p)
		}
	}
	// Score is negated so the runtime ranks higher-is-better; Shown is the
	// penalty as the scoresheet writes it.
	score := func(id string) int { return -s.Scores[id] }
	out := module.RankByScores(ranked, "lora.unit.points", score)
	if s.Failed != "" {
		out = append(out, module.Standing{
			PlayerID: s.Failed, Rank: len(s.Players), Score: score(s.Failed), LabelKey: "lora.unit.points",
			Facts: []module.Fact{{LabelKey: "lora.standing.failed"}},
		})
	}
	for i := range out {
		shown := s.Scores[out[i].PlayerID]
		out[i].Shown = &shown
	}
	return out, nil
}

// Rounds is the deal-by-deal history.
func (m *Module) Rounds(raw module.State) (module.RoundLog, error) {
	s, err := decode(raw)
	if err != nil {
		return module.RoundLog{}, err
	}
	log := module.RoundLog{
		LabelKey:   "lora.round.deal",
		Rounds:     []module.RoundResult{},
		Paused:     s.Intermission.Open,
		WaitingFor: s.Intermission.Waiting(s.Players),
	}
	for _, d := range s.Rounds {
		r := module.RoundResult{Number: d.Number}
		r.Headline = &module.Fact{LabelKey: "lora.round.game", Params: map[string]any{"game": gameKey(d.Contract)}}
		switch {
		case d.Maturant != "" && d.Passed:
			r.Facts = append(r.Facts, module.Fact{LabelKey: "lora.round.passed", Params: map[string]any{"player": d.Maturant}})
			r.Winners = []string{d.Maturant}
		case d.Maturant != "":
			r.Facts = append(r.Facts, module.Fact{LabelKey: "lora.round.failed", Params: map[string]any{"player": d.Maturant}})
		default:
			best := -1
			for _, p := range s.Players {
				if best < 0 || d.Deltas[p] < best {
					best = d.Deltas[p]
				}
			}
			for _, p := range s.Players {
				if d.Deltas[p] == best {
					r.Winners = append(r.Winners, p)
				}
			}
		}
		for _, p := range s.Players {
			delta, total := d.Deltas[p], d.Totals[p]
			// Negated, like Standing.Score; Shown is what a player reads.
			r.Scores = append(r.Scores, module.RoundScore{
				PlayerID: p, Delta: -delta, Total: -total, Shown: &delta, ShownTotal: &total,
			})
		}
		log.Rounds = append(log.Rounds, r)
	}
	return log, nil
}

// Bot is Lóra's own bot — see bot.go.
func (m *Module) Bot() module.Bot { return bot{} }
