package sietemedio

import (
	"strconv"

	"zolik/server/internal/module"
)

const packZoneID = "pack"

func handZoneID(playerID string) string { return "hand:" + playerID }

// defaultsFor is one variation's whole starting position, for the descriptor.
// Built rather than typed out three times so a new option cannot be added to
// the schema and forgotten in a variation.
func defaultsFor(id string) map[string]int {
	v := variations[id]
	return map[string]int{
		OptStartingStack:             v.startingStack,
		OptMinStake:                  v.minStake,
		OptRounds:                    v.rounds,
		OptTies:                      v.ties,
		OptSevenPays:                 v.sevenPays,
		OptBank:                      v.bank,
		OptFirstCardDown:             module.BoolOpt(v.firstCardDown),
		module.OptPauseBetweenRounds: module.OptOn,
		module.OptBotSkill:           module.SkillOpt(module.SkillMedium),
		module.OptHints:              module.OptOn,
		module.OptLeaveAfterAway:     module.LeaveAfterAwayDefault,
	}
}

// Descriptor is Siete y Medio's self-description.
func (m *Module) Descriptor() module.ModuleDescriptor {
	return module.ModuleDescriptor{
		ID: "sietemedio", Label: "Siete y Medio", MinPlayers: 2, MaxPlayers: 8,
		// The forty-card Spanish pack. Presentational, like every other deck
		// on this server: the codes are the ordinary French ones and a client
		// that ignores this draws French faces and plays the game exactly as
		// well — it only loses the oros and the caballo.
		Deck: module.DeckSpanish,
		Variations: []module.VariationSpec{
			{
				ID: VarMexicana, Label: "Mexicana",
				Summary: []module.Fact{
					{LabelKey: "sietemedio.rules.firstCardDown"},
					{LabelKey: "sietemedio.rules.tiesToBank"},
					{LabelKey: "sietemedio.rules.bankOnSeven"},
				},
				Defaults: defaultsFor(VarMexicana),
			},
			{
				ID: VarCantina, Label: "Cantina",
				Summary: []module.Fact{
					{LabelKey: "sietemedio.rules.firstCardDown"},
					{LabelKey: "sietemedio.rules.tiesPush"},
					{LabelKey: "sietemedio.rules.bankRound"},
				},
				Defaults: defaultsFor(VarCantina),
			},
			{
				ID: VarAbierta, Label: "Abierta",
				Summary: []module.Fact{
					{LabelKey: "sietemedio.rules.allFaceUp"},
					{LabelKey: "sietemedio.rules.tiesToBank"},
					{LabelKey: "sietemedio.rules.bankRound"},
				},
				Defaults: defaultsFor(VarAbierta),
			},
		},
		Options: []module.OptionSpec{
			module.PauseOption(),
			module.BotSkillOption(),
			module.HintsOption(),
			module.LeaveAfterAwayOption(),
			{
				Name: OptStartingStack, Type: module.OptionEnumInt,
				Label: "Starting chips", Help: "How many chips each seat sits down with.",
				Choices: []module.OptionChoice{
					{Value: 200, Label: "200"}, {Value: 500, Label: "500"}, {Value: 2000, Label: "2000"},
				},
			},
			{
				Name: OptMinStake, Type: module.OptionEnumInt,
				Label: "Table minimum", Help: "The smallest stake a seat may play a round for.",
				Choices: []module.OptionChoice{
					{Value: 5, Label: "5"}, {Value: 10, Label: "10"}, {Value: 25, Label: "25"},
				},
			},
			{
				Name: OptRounds, Type: module.OptionEnumInt,
				Label: "Rounds", Help: "How many rounds the table plays; the most chips at the end wins.",
				Choices: []module.OptionChoice{
					{Value: 6, Label: "6"}, {Value: 12, Label: "12"}, {Value: 24, Label: "24"},
				},
			},
			{
				Name: OptTies, Type: module.OptionEnumInt,
				Label: "A level hand", Help: "Who takes it when a player and the bank hold the same total.",
				Choices: []module.OptionChoice{
					{Value: tiesToBank, Label: "The bank wins"}, {Value: tiesPush, Label: "Stakes back"},
				},
			},
			{
				Name: OptSevenPays, Type: module.OptionEnumInt,
				Label: "Seven and a half pays", Help: "What the hand the game is named after collects.",
				Choices: []module.OptionChoice{
					{Value: sevenPaysDouble, Label: "Double"}, {Value: sevenPaysEven, Label: "Even money"},
				},
			},
			{
				Name: OptBank, Type: module.OptionEnumInt,
				Label: "The bank", Help: "How the deal moves round the table.",
				Choices: []module.OptionChoice{
					{Value: bankOnSeven, Label: "To whoever makes 7½"},
					{Value: bankRound, Label: "Round the table"},
					{Value: bankFixed, Label: "Stays put"},
				},
			},
			{
				Name: OptFirstCardDown, Type: module.OptionEnumInt,
				Label: "The first card", Help: "Whether the card everyone is dealt first is theirs alone to see.",
				Choices: []module.OptionChoice{
					{Value: module.OptOn, Label: "Face down"}, {Value: module.OptOff, Label: "Face up"},
				},
			},
		},
	}
}

// View renders the table for one viewer.
//
// The hidden information here is the ordinary kind, unlike blackjack's: each
// seat's first card is that seat's own, hidden from everybody else and from
// nobody else's point of view. Cards drawn after it are dealt face up, so the
// filtering is per-card as well as per-seat — a seat's hand is partly visible
// for most of a round, which is the shape a client has to be handed rather
// than left to work out.
//
// The banker's card is hidden from the whole table, the banker excepted, until
// their turn comes — and then it is turned over, which is the one piece of
// ceremony this game has.
func (m *Module) View(raw module.State, viewerID string) (module.ViewModel, error) {
	return m.view(raw, viewerID, false)
}

// OpenView renders the board with nothing hidden, for replaying a game that is
// over.
func (m *Module) OpenView(raw module.State) (module.ViewModel, error) {
	return m.view(raw, "", true)
}

func (m *Module) view(raw module.State, viewerID string, reveal bool) (module.ViewModel, error) {
	s, err := decode(raw)
	if err != nil {
		return module.ViewModel{}, err
	}

	vm := module.ViewModel{}
	for i := range s.Seats {
		st := &s.Seats[i]
		z := module.Zone{
			ID: handZoneID(st.PlayerID), Kind: module.ZoneSpread, OwnerID: st.PlayerID,
			LabelKey: "sietemedio.zone.hand",
			Cards:    cardViews(visible(s, st, viewerID, reveal)),
			// The count is the whole hand even where the cards are not sent,
			// which is how this protocol hides a card: a client draws the
			// difference face down and nobody has to be told a secret to
			// render one.
			Count: len(st.Cards),
		}
		if st.PlayerID == viewerID {
			z.LabelKey = "sietemedio.zone.yourHand"
		} else if s.isBanker(i) {
			z.LabelKey = "sietemedio.zone.bankHand"
		}
		vm.Zones = append(vm.Zones, z)
	}
	vm.Zones = append(vm.Zones, module.Zone{
		ID: packZoneID, Kind: module.ZoneStack, LabelKey: "sietemedio.zone.pack", Count: len(s.Deck),
	})

	for i := range s.Seats {
		vm.Seats = append(vm.Seats, seatView(s, i, viewerID, reveal))
	}

	vm.Header = []module.Fact{
		{LabelKey: "sietemedio.header.round", Value: strconv.Itoa(s.RoundNumber),
			Params: map[string]any{"n": s.RoundNumber, "of": s.RoundLimit}},
		{LabelKey: "sietemedio.header.minStake", Value: strconv.Itoa(s.MinStake),
			Params: map[string]any{"n": s.MinStake}},
		{LabelKey: "sietemedio.header.target", Value: halvesText(Target)},
	}
	if b := s.banker(); b != nil {
		vm.Header = append(vm.Header, module.Fact{
			LabelKey: "sietemedio.header.bank",
			Params:   map[string]any{"playerId": b.PlayerID},
			Standing: true,
		})
	}

	vm.Status = append(vm.Status, roundStatus(s)...)
	if s.Status == "completed" {
		vm.Status = append(vm.Status, module.Fact{
			LabelKey: "status.winner",
			Params:   map[string]any{"winners": append([]string(nil), s.Winners...)},
		})
	}
	vm.Prompts = prompts(s, viewerID)
	return vm, nil
}

// visible is the part of one seat's hand this viewer may see.
//
// Its own hand, always; a hand that has been turned over — gone past 7½, the
// bank at its turn, anything at settlement — to everybody; and otherwise every
// card except the one it was dealt first, because that is the only card in this
// game that is ever face down.
func visible(s *GameState, st *Seat, viewerID string, reveal bool) []string {
	if reveal || st.Shown || st.PlayerID == viewerID {
		return append([]string(nil), st.Cards...)
	}
	if s.FirstCardDown && len(st.Cards) > 0 {
		return append([]string(nil), st.Cards[1:]...)
	}
	return append([]string(nil), st.Cards...)
}

// seatView is one seat as the board shows it: what it is sitting on, what it
// has staked, and what its hand is worth to whoever is looking.
func seatView(s *GameState, i int, viewerID string, reveal bool) module.Seat {
	st := &s.Seats[i]
	seat := module.Seat{
		PlayerID: st.PlayerID,
		Active:   awaiting(s, i),
		Facts: []module.Fact{{
			LabelKey: "sietemedio.seat.stack", Value: strconv.Itoa(st.Stack),
			Params: map[string]any{"chips": st.Stack},
		}},
	}
	if st.Stake > 0 {
		seat.Facts = append(seat.Facts, module.Fact{
			LabelKey: "sietemedio.seat.stake", Value: strconv.Itoa(st.Stake),
			Params: map[string]any{"chips": st.Stake},
		})
	}
	// A total is a rule — the figures are worth half a point each — so it is
	// added up here rather than left to a client. Which total, though, depends
	// on who is looking: a hand with a card still face down gets the total of
	// what is on show, under a key that says so. Sending the real one would
	// hand every viewer the card the protocol just took out of the zone.
	if shown := visible(s, st, viewerID, reveal); len(st.Cards) > 0 {
		switch {
		case len(shown) == len(st.Cards):
			seat.Facts = append(seat.Facts, module.Fact{
				LabelKey: "sietemedio.seat.total", Value: halvesText(Total(shown)),
			})
		case len(shown) > 0:
			seat.Facts = append(seat.Facts, module.Fact{
				LabelKey: "sietemedio.seat.showing", Value: halvesText(Total(shown)),
			})
		}
		// A hand with nothing on show at all — the bank before its turn, or a
		// seat still on the card it was dealt — gets no total. "Showing 0" is
		// a number about a card nobody can see, and reads as a hand worth
		// nothing rather than as a hand nobody has looked at.
	}
	if s.isBanker(i) {
		seat.LabelKeys = append(seat.LabelKeys, "sietemedio.seat.bank")
	}
	switch {
	case st.Left:
		// Gone with their chips, which the stack fact above still states.
		seat.LabelKeys = append(seat.LabelKeys, "seat.left")
	case st.Leaving:
		seat.LabelKeys = append(seat.LabelKeys, "seat.leaving")
	case st.Out:
		seat.LabelKeys = append(seat.LabelKeys, "sietemedio.seat.out")
	case st.NoRoom:
		seat.LabelKeys = append(seat.LabelKeys, "sietemedio.seat.noRoom")
	}
	if st.Outcome == outcomeUnfinished && IsBust(st.Cards) {
		seat.LabelKeys = append(seat.LabelKeys, "sietemedio.seat.bust")
	}
	switch st.Outcome {
	case outcomeSeven:
		seat.LabelKeys = append(seat.LabelKeys, "sietemedio.seat.seven")
	case outcomeWin:
		seat.LabelKeys = append(seat.LabelKeys, "sietemedio.seat.won")
	case outcomePush:
		seat.LabelKeys = append(seat.LabelKeys, "sietemedio.seat.push")
	case outcomeLose:
		seat.LabelKeys = append(seat.LabelKeys, "sietemedio.seat.lost")
	case outcomeBust:
		seat.LabelKeys = append(seat.LabelKeys, "sietemedio.seat.bust")
	}
	return seat
}

// awaiting reports the module waiting on this seat — the same question
// LegalActions answers with an enabled offer, asked of the board.
func awaiting(s *GameState, i int) bool {
	st := &s.Seats[i]
	if s.Status != "active" {
		return false
	}
	if s.Break.Open {
		return !s.Break.Ready[st.PlayerID]
	}
	switch s.Phase {
	case phaseStakes:
		return !st.Out && !st.NoRoom && !s.isBanker(i) && st.Stake == 0
	case phasePlay, phaseBank:
		return s.Current == i
	}
	return false
}

// roundStatus is what the last settled round did, for the space above the
// table. It names a total, never a card: the round is over, but this box is
// also what a player reads three rounds later.
func roundStatus(s *GameState) []module.Fact {
	if len(s.Rounds) == 0 {
		return nil
	}
	last := s.Rounds[len(s.Rounds)-1]
	switch {
	case last.BankSeven:
		return []module.Fact{{
			LabelKey: "sietemedio.status.bankSeven",
			Params:   map[string]any{"playerId": last.Banker},
		}}
	case last.BankBust:
		return []module.Fact{{
			LabelKey: "sietemedio.status.bankBust", Value: halvesText(last.BankTotal),
			Params: map[string]any{"playerId": last.Banker, "total": halvesText(last.BankTotal)},
		}}
	default:
		return []module.Fact{{
			LabelKey: "sietemedio.status.bankStands", Value: halvesText(last.BankTotal),
			Params: map[string]any{"playerId": last.Banker, "total": halvesText(last.BankTotal)},
		}}
	}
}

// prompts says what this viewer is being asked for, in the words of the phase
// they are being asked in.
func prompts(s *GameState, viewerID string) []module.Fact {
	if s.Status != "active" || s.Break.Open {
		return nil
	}
	i := s.seatIndex(viewerID)
	if i < 0 {
		return nil
	}
	if !awaiting(s, i) {
		if s.Current >= 0 && (s.Phase == phasePlay || s.Phase == phaseBank) {
			return []module.Fact{{
				LabelKey: "sietemedio.prompt.waitingFor",
				Params:   map[string]any{"playerId": s.Seats[s.Current].PlayerID},
			}}
		}
		return nil
	}
	switch s.Phase {
	case phaseStakes:
		return []module.Fact{{LabelKey: "sietemedio.prompt.placeStake"}}
	case phaseBank:
		return []module.Fact{{LabelKey: "sietemedio.prompt.bankMove"}}
	case phasePlay:
		return []module.Fact{{LabelKey: "sietemedio.prompt.yourMove"}}
	}
	return nil
}

func cardViews(cards []string) []module.CardView {
	out := make([]module.CardView, 0, len(cards))
	for _, c := range cards {
		out = append(out, module.CardView{Card: c})
	}
	return out
}

// Bot is this module's own player — see bot.go. module.OfferBot cannot play
// the game: hit and stand are offered on every hand, so which of the two to
// take is the entire game, and an offer-preference bot either draws itself
// past 7½ every round or stands on a half.
func (m *Module) Bot() module.Bot { return bot{} }

// Standings ranks the table by chips, the only measure this game has.
func (m *Module) Standings(raw module.State) ([]module.Standing, error) {
	s, err := decode(raw)
	if err != nil {
		return nil, err
	}
	stacks := map[string]int{}
	for i := range s.Seats {
		stacks[s.Seats[i].PlayerID] = s.Seats[i].Stack
	}
	return module.RankByScore(order(s), func(id string) int { return stacks[id] }, "sietemedio.unit.chips"), nil
}

// SitOut is how an absent seat plays: the table's minimum to stay in the
// round, and never draw.
func (m *Module) SitOut() []string {
	return []string{VerbStand, VerbStake}
}
