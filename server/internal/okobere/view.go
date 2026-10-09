package okobere

import (
	"strconv"

	"zolik/server/internal/module"
)

// The ids this module draws its zones under.
const deckZoneID = "deck"

func handZoneID(playerID string) string { return "hand:" + playerID }

var (
	_ module.Ranked     = (*Module)(nil)
	_ module.OpenViewer = (*Module)(nil)
	_ module.Rounded    = (*Module)(nil)
	_ module.Botted     = (*Module)(nil)
)

// Descriptor is Oko bere's self-description.
func (m *Module) Descriptor() module.ModuleDescriptor {
	return module.ModuleDescriptor{
		ID:         "okobere",
		Label:      "Oko bere",
		MinPlayers: 2,
		MaxPlayers: 6,
		Deck:       module.DeckGerman,
		Variations: []module.VariationSpec{{
			ID:    "classic",
			Label: "Classic",
			Summary: []module.Fact{
				{LabelKey: "okobere.rules.goal"},
				{LabelKey: "okobere.rules.values"},
				{LabelKey: "okobere.rules.oko"},
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
				Name: OptMinBet, Type: module.OptionEnumInt, Label: "Minimum stake",
				Help:    "The smallest stake a punter may put against the bank.",
				Choices: []module.OptionChoice{{Value: 1, Label: "1"}, {Value: 2, Label: "2"}, {Value: 5, Label: "5"}},
			},
			{
				Name: OptBank, Type: module.OptionEnumInt, Label: "Bank",
				Help:    "What a banker stakes when the bank comes to them; all the stakes on a round together can be no more.",
				Choices: []module.OptionChoice{{Value: 20, Label: "20"}, {Value: 50, Label: "50"}, {Value: 100, Label: "100"}},
			},
			{
				Name: OptRoundsPerBank, Type: module.OptionEnumInt, Label: "Rounds per bank",
				Help:    "How many rounds a banker holds the bank, unless it is broken first.",
				Choices: []module.OptionChoice{{Value: 3, Label: "3"}, {Value: 5, Label: "5"}, {Value: 8, Label: "8"}},
			},
			{
				Name: OptBankTurns, Type: module.OptionEnumInt, Label: "Times round the table",
				Help:    "How many times the bank goes round the table before the match ends.",
				Choices: []module.OptionChoice{{Value: 1, Label: "1"}, {Value: 2, Label: "2"}, {Value: 3, Label: "3"}},
			},
			{
				Name: OptOkoPays, Type: module.OptionEnumInt, Label: "Two aces pay",
				Help:    "What a punter's oko (two aces) collects: the stake, or three times it.",
				Choices: []module.OptionChoice{{Value: 1, Label: "The stake"}, {Value: 3, Label: "Three times"}},
			},
			module.BotSkillOption(),
			module.PauseOption(),
			module.StandInOption(),
		},
	}
}

var defaults = map[string]int{
	OptStartingStack:             100,
	OptMinBet:                    2,
	OptBank:                      50,
	OptRoundsPerBank:             5,
	OptBankTurns:                 1,
	OptOkoPays:                   3,
	module.OptBotSkill:           module.SkillOpt(module.SkillMedium),
	module.OptPauseBetweenRounds: module.OptOn,
	module.OptStandInAfter:       module.StandInAfterDefault,
}

type config struct {
	startingStack, minBet, bank, roundsPerBank, bankTurns, okoPays int
}

func resolve(cfg module.MatchConfig) config {
	opt := func(name string) int { return cfg.Opt(name, defaults[name]) }
	c := config{
		startingStack: max(1, opt(OptStartingStack)),
		minBet:        max(1, opt(OptMinBet)),
		bank:          max(1, opt(OptBank)),
		roundsPerBank: max(1, opt(OptRoundsPerBank)),
		bankTurns:     max(1, opt(OptBankTurns)),
		okoPays:       max(1, opt(OptOkoPays)),
	}
	c.bank = max(c.bank, c.minBet)
	return c
}

func (s *GameState) config() module.MatchConfig {
	return module.MatchConfig{Options: module.Options{
		OptStartingStack: s.StartingStack, OptMinBet: s.MinBet, OptBank: s.BankSize,
		OptRoundsPerBank: s.RoundsPerBank, OptBankTurns: s.BankTurns, OptOkoPays: s.OkoPays,
	}}
}

// View is the board as one seat sees it. Secret: every other hand, until the
// round is settled — a punter's cards lie face down, and so do the banker's
// while the punters play. A bust or an oko is shown at once, as it is
// declared at the table.
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
	b := s.bankerIndex()
	settled := s.Intermission.Open || s.Status != "active"

	for i, st := range s.Seats {
		mine := st.PlayerID == viewerID
		z := module.Zone{
			ID: handZoneID(st.PlayerID), Kind: module.ZoneSpread, OwnerID: st.PlayerID,
			LabelKey: "okobere.zone.hand", Count: len(st.Hand),
		}
		if mine {
			z.Kind, z.LabelKey = module.ZoneHand, "zone.yourHand"
		}
		if i == b {
			z.LabelKey = "okobere.zone.bankerHand"
		}
		open := mine || reveal || settled || st.Result == resultBust || st.Result == resultOko ||
			(i == b && s.Phase == phaseBank)
		if open {
			for _, c := range st.Hand {
				z.Cards = append(z.Cards, module.CardView{Card: c})
			}
		}
		vm.Zones = append(vm.Zones, z)
	}
	if len(s.Deck) > 0 && playing {
		vm.Zones = append(vm.Zones, module.Zone{ID: deckZoneID, Kind: module.ZoneStack, LabelKey: "okobere.zone.deck", Count: len(s.Deck)})
	}

	for i, st := range s.Seats {
		seat := module.Seat{
			PlayerID: st.PlayerID,
			Active:   (s.Intermission.Open && !s.Intermission.Ready[st.PlayerID]) || (playing && s.Current == st.PlayerID),
		}
		if s.Intermission.Open && s.Intermission.Ready[st.PlayerID] {
			seat.LabelKeys = append(seat.LabelKeys, "seat.ready")
		}
		seat.Facts = append(seat.Facts, module.Fact{
			LabelKey: "okobere.seat.chips", Value: strconv.Itoa(st.Stack), Params: map[string]any{"n": st.Stack},
		})
		if s.Status == "active" && i == b {
			seat.LabelKeys = append(seat.LabelKeys, "okobere.seat.banker")
			seat.Facts = append(seat.Facts, module.Fact{
				LabelKey: "okobere.seat.bank", Value: strconv.Itoa(s.Bank), Params: map[string]any{"n": s.Bank},
			})
		}
		if st.Bet > 0 {
			seat.Facts = append(seat.Facts, module.Fact{LabelKey: "okobere.seat.stake", Params: map[string]any{"n": st.Bet}})
		}
		switch st.Result {
		case resultBust:
			seat.LabelKeys = append(seat.LabelKeys, "okobere.seat.bust")
		case resultOko:
			seat.LabelKeys = append(seat.LabelKeys, "okobere.seat.oko")
		case resultSatOut:
			seat.LabelKeys = append(seat.LabelKeys, "okobere.seat.satOut")
		case resultWon:
			seat.LabelKeys = append(seat.LabelKeys, "okobere.seat.won")
		case resultLost:
			seat.LabelKeys = append(seat.LabelKeys, "okobere.seat.lost")
		}
		if (st.PlayerID == viewerID || reveal || settled) && len(st.Hand) > 0 {
			seat.Facts = append(seat.Facts, module.Fact{LabelKey: "okobere.seat.total", Params: map[string]any{"n": total(st.Hand)}})
		}
		vm.Seats = append(vm.Seats, seat)
	}

	vm.Header = []module.Fact{
		{LabelKey: "okobere.header.round", Params: map[string]any{"n": s.Round + 1}},
	}
	if s.Status == "active" {
		vm.Header = append(vm.Header,
			module.Fact{LabelKey: "okobere.header.banker", Params: map[string]any{"player": s.Seats[b].PlayerID}},
			module.Fact{LabelKey: "okobere.header.bankRound", Params: map[string]any{"n": min(s.RoundsInBank+1, s.RoundsPerBank), "of": s.RoundsPerBank}},
			module.Fact{LabelKey: "okobere.header.bankFree", Params: map[string]any{"n": s.bankFree()}},
		)
	}

	if playing && viewerID == s.Current {
		st := s.seat(viewerID)
		switch {
		case s.Phase == phaseBank:
			vm.Prompts = append(vm.Prompts, module.Fact{LabelKey: "okobere.prompt.bank", Params: map[string]any{"n": total(st.Hand)}})
		case st.Bet == 0:
			vm.Prompts = append(vm.Prompts, module.Fact{LabelKey: "okobere.prompt.bet", Params: map[string]any{"n": s.maxBet(st)}})
		default:
			vm.Prompts = append(vm.Prompts, module.Fact{LabelKey: "okobere.prompt.draw", Params: map[string]any{"n": total(st.Hand)}})
		}
	}
	return vm, nil
}

// Standings ranks by chips, the bank counted for whoever holds it.
func (m *Module) Standings(raw module.State) ([]module.Standing, error) {
	s, err := decode(raw)
	if err != nil {
		return nil, err
	}
	return module.RankByScore(s.players(), s.worth, "okobere.unit.chips"), nil
}

func resultFact(result string) module.Fact {
	switch result {
	case resultWon:
		return module.Fact{LabelKey: "okobere.seat.won"}
	case resultLost:
		return module.Fact{LabelKey: "okobere.seat.lost"}
	case resultBust:
		return module.Fact{LabelKey: "okobere.seat.bust"}
	case resultOko:
		return module.Fact{LabelKey: "okobere.seat.oko"}
	}
	return module.Fact{LabelKey: "okobere.seat.satOut"}
}

// Rounds is the round-by-round history: who banked, what the bank turned
// up, and how each punter came out.
func (m *Module) Rounds(raw module.State) (module.RoundLog, error) {
	s, err := decode(raw)
	if err != nil {
		return module.RoundLog{}, err
	}
	log := module.RoundLog{
		LabelKey:   "okobere.round.round",
		Rounds:     []module.RoundResult{},
		Paused:     s.Intermission.Open,
		WaitingFor: s.Intermission.Waiting(s.players()),
	}
	for _, r := range s.Rounds {
		rr := module.RoundResult{Number: r.Number}
		if r.BankerOko {
			rr.Headline = &module.Fact{LabelKey: "okobere.round.bankerOko", Params: map[string]any{"player": r.Banker}}
		} else {
			rr.Headline = &module.Fact{LabelKey: "okobere.round.banker", Params: map[string]any{"player": r.Banker, "n": r.BankerTotal}}
		}
		if r.BankBroken {
			rr.Facts = append(rr.Facts, module.Fact{LabelKey: "okobere.round.broken"})
		}
		for _, id := range s.players() {
			if id == r.Banker || r.Results[id] == resultSatOut {
				continue
			}
			rr.Facts = append(rr.Facts, module.Fact{LabelKey: "okobere.round.punter", Params: map[string]any{
				"player": id, "result": resultFact(r.Results[id]).LabelKey,
			}})
		}
		for _, id := range s.players() {
			if r.Deltas[id] > 0 {
				rr.Winners = append(rr.Winners, id)
			}
			rr.Scores = append(rr.Scores, module.RoundScore{PlayerID: id, Delta: r.Deltas[id], Total: r.Stacks[id]})
		}
		log.Rounds = append(log.Rounds, rr)
	}
	return log, nil
}

// Bot is Oko bere's own bot — see bot.go.
func (m *Module) Bot() module.Bot { return bot{} }
