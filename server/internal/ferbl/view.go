package ferbl

import (
	"strconv"

	"zolik/server/internal/module"
)

func handZoneID(playerID string) string { return "hand:" + playerID }

const potZoneID = "pot"

var (
	_ module.Ranked     = (*Module)(nil)
	_ module.OpenViewer = (*Module)(nil)
	_ module.Rounded    = (*Module)(nil)
	_ module.Botted     = (*Module)(nil)
)

// Descriptor is Ferbl's self-description.
func (m *Module) Descriptor() module.ModuleDescriptor {
	return module.ModuleDescriptor{
		ID:         "ferbl",
		Label:      "Ferbl",
		MinPlayers: 2,
		MaxPlayers: 6,
		Deck:       module.DeckGerman,
		Variations: []module.VariationSpec{{
			ID:    "classic",
			Label: "Classic",
			Summary: []module.Fact{
				{LabelKey: "ferbl.rules.goal"},
				{LabelKey: "ferbl.rules.values"},
				{LabelKey: "ferbl.rules.fourKind"},
			},
			Defaults: defaults,
		}},
		Options: []module.OptionSpec{
			{
				Name: OptStartingStack, Type: module.OptionEnumInt, Label: "Starting chips",
				Help:    "The chips each player starts with.",
				Choices: []module.OptionChoice{{Value: 100, Label: "100"}, {Value: 200, Label: "200"}, {Value: 500, Label: "500"}},
			},
			{
				Name: OptVizi, Type: module.OptionEnumInt, Label: "Vizi",
				Help:    "The ante every player pays in each hand, and the bet on two cards; a bet on four is twice it.",
				Choices: []module.OptionChoice{{Value: 1, Label: "1"}, {Value: 2, Label: "2"}, {Value: 5, Label: "5"}},
			},
			{
				Name: OptHands, Type: module.OptionEnumInt, Label: "Hands",
				Help:    "How many hands the match lasts, unless only one player has chips left before then.",
				Choices: []module.OptionChoice{{Value: 10, Label: "10"}, {Value: 20, Label: "20"}, {Value: 30, Label: "30"}},
			},
			module.BotSkillOption(),
			module.PauseOption(),
		},
	}
}

var defaults = map[string]int{
	OptStartingStack:             100,
	OptVizi:                      2,
	OptHands:                     20,
	module.OptBotSkill:           module.SkillOpt(module.SkillMedium),
	module.OptPauseBetweenRounds: module.OptOn,
}

type config struct{ startingStack, vizi, hands int }

func resolve(cfg module.MatchConfig) config {
	opt := func(name string) int { return cfg.Opt(name, defaults[name]) }
	return config{
		startingStack: max(1, opt(OptStartingStack)),
		vizi:          max(1, opt(OptVizi)),
		hands:         max(1, opt(OptHands)),
	}
}

func (s *GameState) config() module.MatchConfig {
	return module.MatchConfig{Options: module.Options{
		OptStartingStack: s.StartingStack, OptVizi: s.Vizi, OptHands: s.Hands,
	}}
}

// View is the board as one seat sees it. Secret: every other hand, until it
// is turned up at the showdown. A folded hand is never shown.
func (m *Module) View(raw module.State, viewerID string) (module.ViewModel, error) {
	return m.view(raw, viewerID, false)
}

// OpenView is the board with everything face up, for replaying a match.
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

	for _, st := range s.Seats {
		mine := st.PlayerID == viewerID
		z := module.Zone{
			ID: handZoneID(st.PlayerID), Kind: module.ZoneSpread, OwnerID: st.PlayerID,
			LabelKey: "ferbl.zone.hand", Count: len(st.Hand),
		}
		if mine {
			z.Kind, z.LabelKey = module.ZoneHand, "zone.yourHand"
		}
		if mine || reveal || st.Shown {
			for _, c := range st.Hand {
				z.Cards = append(z.Cards, module.CardView{Card: c})
			}
		}
		vm.Zones = append(vm.Zones, z)
	}

	d := s.dealerIndex()
	for i, st := range s.Seats {
		seat := module.Seat{
			PlayerID: st.PlayerID,
			Active:   (s.Intermission.Open && !s.Intermission.Ready[st.PlayerID]) || (playing && s.Current == st.PlayerID),
		}
		if s.Intermission.Open && s.Intermission.Ready[st.PlayerID] {
			seat.LabelKeys = append(seat.LabelKeys, "seat.ready")
		}
		seat.Facts = append(seat.Facts, module.Fact{
			LabelKey: "ferbl.seat.chips", Value: strconv.Itoa(st.Stack), Params: map[string]any{"n": st.Stack},
		})
		if s.Status == "active" && i == d {
			seat.LabelKeys = append(seat.LabelKeys, "ferbl.seat.dealer")
		}
		switch {
		case st.Folded:
			seat.LabelKeys = append(seat.LabelKeys, "ferbl.seat.folded")
		case st.In && st.Stack == 0 && st.live():
			seat.LabelKeys = append(seat.LabelKeys, "ferbl.seat.allIn")
		case !st.In && s.Status == "active":
			seat.LabelKeys = append(seat.LabelKeys, "ferbl.seat.out")
		}
		if st.Total > 0 {
			seat.Facts = append(seat.Facts, module.Fact{LabelKey: "ferbl.seat.inPot", Params: map[string]any{"n": st.Total}})
		}
		if (st.Shown || reveal || st.PlayerID == viewerID) && len(st.Hand) == handSize {
			seat.Facts = append(seat.Facts, handFact(score(st.Hand)))
		}
		vm.Seats = append(vm.Seats, seat)
	}

	vm.Header = []module.Fact{
		{LabelKey: "ferbl.header.hand", Params: map[string]any{"n": min(s.Hand+1, s.Hands), "of": s.Hands}},
	}
	if playing {
		vm.Header = append(vm.Header, module.Fact{LabelKey: "ferbl.header.pot", Params: map[string]any{"n": s.pot()}})
		if s.CurrentBet > 0 {
			vm.Header = append(vm.Header, module.Fact{LabelKey: "ferbl.header.bet", Params: map[string]any{"n": s.CurrentBet, "raises": s.Raises}})
		}
	}
	if playing && viewerID == s.Current {
		st := s.seat(viewerID)
		if owe := s.toCall(st); owe > 0 {
			vm.Prompts = append(vm.Prompts, module.Fact{LabelKey: "ferbl.prompt.facing", Params: map[string]any{"n": owe}})
		} else {
			vm.Prompts = append(vm.Prompts, module.Fact{LabelKey: "ferbl.prompt.open", Params: map[string]any{"n": s.unit()}})
		}
	}
	return vm, nil
}

// handFact names a hand: its kind and the figure it is ranked by.
func handFact(sc int) module.Fact {
	v := sc % 100
	switch category(sc) {
	case catFourKind:
		return module.Fact{LabelKey: "ferbl.hand.fourKind"}
	case catBanda:
		return module.Fact{LabelKey: "ferbl.hand.banda", Params: map[string]any{"n": v}}
	case catThreeKind:
		return module.Fact{LabelKey: "ferbl.hand.threeKind"}
	case catThreeSuited:
		return module.Fact{LabelKey: "ferbl.hand.threeSuited", Params: map[string]any{"n": v}}
	case catTwoAces:
		return module.Fact{LabelKey: "ferbl.hand.twoAces"}
	case catTwoSuited:
		return module.Fact{LabelKey: "ferbl.hand.twoSuited", Params: map[string]any{"n": v}}
	}
	return module.Fact{LabelKey: "ferbl.hand.suits", Params: map[string]any{"n": v}}
}

// handName is a hand's kind as a word for a sentence, each key written out
// so the manifest finds it.
func handName(sc int) string {
	switch category(sc) {
	case catFourKind:
		return "ferbl.handName.fourKind"
	case catBanda:
		return "ferbl.handName.banda"
	case catThreeKind:
		return "ferbl.handName.threeKind"
	case catThreeSuited:
		return "ferbl.handName.threeSuited"
	case catTwoAces:
		return "ferbl.handName.twoAces"
	case catTwoSuited:
		return "ferbl.handName.twoSuited"
	}
	return "ferbl.handName.suits"
}

// Standings ranks by chips.
func (m *Module) Standings(raw module.State) ([]module.Standing, error) {
	s, err := decode(raw)
	if err != nil {
		return nil, err
	}
	return module.RankByScore(s.players(), func(id string) int { return s.seat(id).Stack }, "ferbl.unit.chips"), nil
}

// Rounds is the hand-by-hand history: who took the pot, and the hands
// turned up for it.
func (m *Module) Rounds(raw module.State) (module.RoundLog, error) {
	s, err := decode(raw)
	if err != nil {
		return module.RoundLog{}, err
	}
	log := module.RoundLog{
		LabelKey: "ferbl.round.hand", Rounds: []module.RoundResult{},
		Paused: s.Intermission.Open, WaitingFor: s.Intermission.Waiting(s.players()),
	}
	for _, h := range s.Rounds {
		r := module.RoundResult{Number: h.Number, Winners: h.Winners}
		r.Headline = &module.Fact{LabelKey: "ferbl.round.pot", Params: map[string]any{"n": h.Pot}}
		for _, id := range s.players() {
			if sc, ok := h.Scores[id]; ok {
				r.Facts = append(r.Facts, module.Fact{LabelKey: "ferbl.round.shown", Params: map[string]any{
					"player": id, "hand": handName(sc),
				}})
			}
		}
		for _, id := range s.players() {
			r.Scores = append(r.Scores, module.RoundScore{PlayerID: id, Delta: h.Deltas[id], Total: h.Stacks[id]})
		}
		log.Rounds = append(log.Rounds, r)
	}
	return log, nil
}

// Bot is Ferbl's own bot — see bot.go.
func (m *Module) Bot() module.Bot { return bot{} }
