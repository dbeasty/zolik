package blackjack

import (
	"strconv"

	"zolik/server/internal/module"
)

// Option names. Every one of them is a house rule rather than a law of the
// game: which of them a table uses is exactly what distinguishes one casino's
// blackjack from another's, and a rule a table can change is a declared
// option here rather than a constant in the engine.
const (
	OptStartingStack    = "startingStack"
	OptMinBet           = "minBet"
	OptRounds           = "rounds"
	OptDecks            = "decks"
	OptDealerHitsSoft17 = "dealerHitsSoft17"
	OptBlackjackPays    = "blackjackPays"
	OptSurrender        = "surrender"
	OptDoubleAfterSplit = "doubleAfterSplit"
	OptMaxSplits        = "maxSplits"
	OptInsurance        = "insurance"
)

// Blackjack payouts, as a percentage of the stake. 3:2 is the game as it was
// written; 6:5 is the same game with about 1.4% more house edge, which is why
// it is worth a player being able to see which table they are at.
const (
	pays3to2   = 150
	pays6to5   = 120
	paysEven   = 100
	splitsNone = 0
)

type variationDefaults struct {
	decks            int
	minBet           int
	startingStack    int
	rounds           int
	hitsSoft17       bool
	blackjackPays    int
	surrender        bool
	doubleAfterSplit bool
	maxSplits        int
	insurance        bool
}

// The three shipped tables are real rulesets, not sliders at arbitrary
// positions: each is a game somebody actually deals.
var variations = map[string]variationDefaults{
	// The Strip game: six decks, dealer stands on soft 17, blackjack pays 3:2.
	"vegas": {
		decks: 6, minBet: 10, startingStack: 500, rounds: 10,
		hitsSoft17: false, blackjackPays: pays3to2, surrender: false,
		doubleAfterSplit: true, maxSplits: 3, insurance: true,
	},
	// Atlantic City: eight decks and the same dealer rule, plus late
	// surrender — the one table here where giving a hand up is a move.
	"atlantic": {
		decks: 8, minBet: 10, startingStack: 500, rounds: 10,
		hitsSoft17: false, blackjackPays: pays3to2, surrender: true,
		doubleAfterSplit: true, maxSplits: 3, insurance: true,
	},
	// The modern single-decker: one deck, which sounds generous, dealt with a
	// 6:5 blackjack and no double after splitting, which is not.
	"single": {
		decks: 1, minBet: 10, startingStack: 500, rounds: 10,
		hitsSoft17: true, blackjackPays: pays6to5, surrender: false,
		doubleAfterSplit: false, maxSplits: 1, insurance: true,
	},
}

func resolveVariation(cfg module.MatchConfig) variationDefaults {
	if v, ok := variations[cfg.Variation]; ok {
		return v
	}
	return variations["vegas"]
}

// defaultsFor is one variation's whole starting position, for the descriptor.
// Built rather than typed out three times so a new option cannot be added to
// the schema and forgotten in a variation.
func defaultsFor(id string) map[string]int {
	v := variations[id]
	return map[string]int{
		OptStartingStack:             v.startingStack,
		OptMinBet:                    v.minBet,
		OptRounds:                    v.rounds,
		OptDecks:                     v.decks,
		OptDealerHitsSoft17:          module.BoolOpt(v.hitsSoft17),
		OptBlackjackPays:             v.blackjackPays,
		OptSurrender:                 module.BoolOpt(v.surrender),
		OptDoubleAfterSplit:          module.BoolOpt(v.doubleAfterSplit),
		OptMaxSplits:                 v.maxSplits,
		OptInsurance:                 module.BoolOpt(v.insurance),
		module.OptPauseBetweenRounds: module.OptOn,
		module.OptBotSkill:           module.SkillOpt(module.SkillMedium),
	}
}

// Descriptor is Blackjack's self-description.
func (m *Module) Descriptor() module.ModuleDescriptor {
	return module.ModuleDescriptor{
		ID: "blackjack", Label: "Blackjack", MinPlayers: 2, MaxPlayers: 7,
		Variations: []module.VariationSpec{
			{
				ID: "vegas", Label: "Vegas Strip",
				Summary: []module.Fact{
					{LabelKey: "blackjack.rules.decks", Params: map[string]any{"n": variations["vegas"].decks}},
					{LabelKey: "blackjack.rules.standsSoft17"},
					{LabelKey: "blackjack.rules.pays3to2"},
				},
				Defaults: defaultsFor("vegas"),
			},
			{
				ID: "atlantic", Label: "Atlantic City",
				Summary: []module.Fact{
					{LabelKey: "blackjack.rules.decks", Params: map[string]any{"n": variations["atlantic"].decks}},
					{LabelKey: "blackjack.rules.standsSoft17"},
					{LabelKey: "blackjack.rules.surrender"},
				},
				Defaults: defaultsFor("atlantic"),
			},
			{
				ID: "single", Label: "Single deck",
				Summary: []module.Fact{
					{LabelKey: "blackjack.rules.decks", Params: map[string]any{"n": variations["single"].decks}},
					{LabelKey: "blackjack.rules.hitsSoft17"},
					{LabelKey: "blackjack.rules.pays6to5"},
				},
				Defaults: defaultsFor("single"),
			},
		},
		Options: []module.OptionSpec{
			module.PauseOption(),
			module.BotSkillOption(),
			{
				Name: OptStartingStack, Type: module.OptionEnumInt,
				Label: "Starting chips", Help: "How many chips each seat sits down with.",
				Choices: []module.OptionChoice{
					{Value: 200, Label: "200"}, {Value: 500, Label: "500"}, {Value: 2000, Label: "2000"},
				},
			},
			{
				Name: OptMinBet, Type: module.OptionEnumInt,
				Label: "Table minimum", Help: "The smallest stake a seat may play a round for.",
				Choices: []module.OptionChoice{
					{Value: 5, Label: "5"}, {Value: 10, Label: "10"}, {Value: 25, Label: "25"},
				},
			},
			{
				Name: OptRounds, Type: module.OptionEnumInt,
				Label: "Rounds", Help: "How many rounds the table plays; the most chips at the end wins.",
				Choices: []module.OptionChoice{
					{Value: 5, Label: "5"}, {Value: 10, Label: "10"}, {Value: 20, Label: "20"},
				},
			},
			{
				Name: OptDecks, Type: module.OptionEnumInt,
				Label: "Decks", Help: "How many decks are shuffled into the shoe.",
				Choices: []module.OptionChoice{
					{Value: 1, Label: "1"}, {Value: 2, Label: "2"},
					{Value: 6, Label: "6"}, {Value: 8, Label: "8"},
				},
			},
			{
				Name: OptDealerHitsSoft17, Type: module.OptionEnumInt,
				Label: "Dealer on soft 17", Help: "Whether the dealer draws to a seventeen made with an ace.",
				Choices: []module.OptionChoice{
					{Value: module.OptOff, Label: "Stands"}, {Value: module.OptOn, Label: "Hits"},
				},
			},
			{
				Name: OptBlackjackPays, Type: module.OptionEnumInt,
				Label: "Blackjack pays", Help: "What a natural twenty-one pays.",
				Choices: []module.OptionChoice{
					{Value: pays3to2, Label: "3:2"}, {Value: pays6to5, Label: "6:5"},
					{Value: paysEven, Label: "Even money"},
				},
			},
			{
				Name: OptMaxSplits, Type: module.OptionEnumInt,
				Label: "Splitting", Help: "How many times a pair may be split.",
				Choices: []module.OptionChoice{
					{Value: splitsNone, Label: "No splitting"},
					{Value: 1, Label: "Once (two hands)"},
					{Value: 3, Label: "Three times (four hands)"},
				},
			},
			{
				Name: OptDoubleAfterSplit, Type: module.OptionEnumInt,
				Label: "Double after split", Help: "Whether a hand that came out of a split may be doubled.",
				Choices: []module.OptionChoice{
					{Value: module.OptOn, Label: "Allowed"}, {Value: module.OptOff, Label: "Not allowed"},
				},
			},
			{
				Name: OptSurrender, Type: module.OptionEnumInt,
				Label: "Surrender", Help: "Whether a first hand may be given up for half the stake.",
				Choices: []module.OptionChoice{
					{Value: module.OptOff, Label: "Off"}, {Value: module.OptOn, Label: "Late surrender"},
				},
			},
			{
				Name: OptInsurance, Type: module.OptionEnumInt,
				Label: "Insurance", Help: "Whether a dealer ace offers insurance at 2:1.",
				Choices: []module.OptionChoice{
					{Value: module.OptOn, Label: "Offered"}, {Value: module.OptOff, Label: "Not offered"},
				},
			},
		},
	}
}

// View renders the table for one viewer.
//
// There is exactly one piece of hidden information in this game, and it is
// the dealer's hole card: player cards are dealt face up, which is not a
// simplification but how a shoe game is actually dealt. So this is the one
// module whose per-viewer filtering does not depend on the viewer at all —
// every seat sees the same table, and the thing being hidden is hidden from
// everybody, the dealer included until the moment they turn it over.
func (m *Module) View(raw module.State, viewerID string) (module.ViewModel, error) {
	s, err := decode(raw)
	if err != nil {
		return module.ViewModel{}, err
	}

	vm := module.ViewModel{}

	dealer := module.Zone{
		ID: dealerZoneID, Kind: module.ZoneSpread, LabelKey: "blackjack.zone.dealer",
		Cards: cardViews(s.shownDealer()), Count: len(s.Dealer),
	}
	vm.Zones = append(vm.Zones, dealer)

	// A seat's box is a spread rather than a hand: the cards are face up on
	// the table, and after a split there is more than one of them, which is
	// what groups are for.
	for i := range s.Seats {
		st := &s.Seats[i]
		z := module.Zone{
			ID: boxZoneID(st.PlayerID), Kind: module.ZoneSpread, OwnerID: st.PlayerID,
			LabelKey: "blackjack.zone.box",
		}
		if st.PlayerID == viewerID {
			z.LabelKey = "blackjack.zone.yourBox"
		}
		for j := range st.Hands {
			h := &st.Hands[j]
			z.Count += len(h.Cards)
			z.Groups = append(z.Groups, module.Group{
				ID: h.ID, Kind: "hand", Cards: append([]string(nil), h.Cards...),
				BadgeKeys: handBadges(s, i, j),
			})
		}
		vm.Zones = append(vm.Zones, z)
	}

	vm.Zones = append(vm.Zones, module.Zone{
		ID: shoeZoneID, Kind: module.ZoneStack, LabelKey: "blackjack.zone.shoe", Count: len(s.Shoe),
	})

	for i := range s.Seats {
		vm.Seats = append(vm.Seats, seatView(s, i))
	}

	vm.Header = []module.Fact{
		{LabelKey: "blackjack.header.round", Value: strconv.Itoa(s.RoundNumber),
			Params: map[string]any{"n": s.RoundNumber, "of": s.RoundLimit}},
		{LabelKey: "blackjack.header.minBet", Value: strconv.Itoa(s.MinBet),
			Params: map[string]any{"n": s.MinBet}},
		{LabelKey: "blackjack.header.decks", Value: strconv.Itoa(s.Decks),
			Params: map[string]any{"n": s.Decks}},
	}
	if shown := s.shownDealer(); len(shown) > 0 {
		total, soft := Total(shown)
		// Written as a literal and then overridden, rather than picked into a
		// variable first: the manifest that proves every key the server can
		// send has wording reads these out of the source, and a key that
		// arrives through a variable is invisible to it.
		fact := module.Fact{
			LabelKey: "blackjack.header.dealerTotal",
			Value:    strconv.Itoa(total), Params: map[string]any{"n": total},
		}
		if soft {
			fact.LabelKey = "blackjack.header.dealerSoftTotal"
		}
		vm.Header = append(vm.Header, fact)
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

// seatView is one seat as the board shows it: what it is sitting on, what it
// has in play, and what its hands are worth.
func seatView(s *GameState, i int) module.Seat {
	st := &s.Seats[i]
	seat := module.Seat{
		PlayerID: st.PlayerID,
		Active:   awaiting(s, i),
		Facts: []module.Fact{{
			LabelKey: "blackjack.seat.stack", Value: strconv.Itoa(st.Stack),
			Params: map[string]any{"chips": st.Stack},
		}},
	}
	if st.Bet > 0 {
		seat.Facts = append(seat.Facts, module.Fact{
			LabelKey: "blackjack.seat.bet", Value: strconv.Itoa(st.Bet),
			Params: map[string]any{"chips": st.Bet},
		})
	}
	if st.Insurance > 0 {
		seat.Facts = append(seat.Facts, module.Fact{
			LabelKey: "blackjack.seat.insurance", Value: strconv.Itoa(st.Insurance),
			Params: map[string]any{"chips": st.Insurance},
		})
	}
	// A hand's total is a rule — an ace is eleven only while it fits — so it
	// is computed here rather than left to a client to add up.
	for j := range st.Hands {
		total, soft := Total(st.Hands[j].Cards)
		fact := module.Fact{
			LabelKey: "blackjack.seat.total",
			Value:    strconv.Itoa(total), Params: map[string]any{"n": total},
		}
		if soft {
			fact.LabelKey = "blackjack.seat.softTotal"
		}
		seat.Facts = append(seat.Facts, fact)
	}
	if st.Out {
		seat.LabelKeys = append(seat.LabelKeys, "blackjack.seat.out")
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
	case phaseBets:
		return !st.Out && st.Bet == 0
	case phaseInsurance:
		return st.inRound() && !st.InsuranceAnswered
	case phasePlay:
		return s.Current == i
	}
	return false
}

// handBadges marks what a client should show against one hand: which one is
// in play, and what became of the rest.
func handBadges(s *GameState, seatIdx, handIdx int) []string {
	h := &s.Seats[seatIdx].Hands[handIdx]
	var out []string
	if s.Phase == phasePlay && s.Current == seatIdx && s.CurrentHand == handIdx {
		out = append(out, "blackjack.badge.inPlay")
	}
	if h.Doubled {
		out = append(out, "blackjack.badge.doubled")
	}
	if h.Split {
		out = append(out, "blackjack.badge.split")
	}
	if isNatural(h.Cards) && !h.Split {
		out = append(out, "blackjack.badge.blackjack")
	} else if IsBust(h.Cards) {
		out = append(out, "blackjack.badge.bust")
	}
	switch h.Outcome {
	case outcomeWin, outcomeBlackjack:
		out = append(out, "blackjack.badge.won")
	case outcomePush:
		out = append(out, "blackjack.badge.push")
	case outcomeLose:
		out = append(out, "blackjack.badge.lost")
	case outcomeSurrender:
		out = append(out, "blackjack.badge.surrendered")
	}
	return out
}

// roundStatus is what the last settled round did, for the space above the
// table. Cards are fine here — the round is over and the dealer's hand has
// been turned over — unlike the round log, which outlives the match.
func roundStatus(s *GameState) []module.Fact {
	if len(s.Rounds) == 0 {
		return nil
	}
	last := s.Rounds[len(s.Rounds)-1]
	out := []module.Fact{}
	switch {
	case last.DealerBlackjack:
		out = append(out, module.Fact{LabelKey: "blackjack.status.dealerBlackjack"})
	case last.DealerBust:
		out = append(out, module.Fact{
			LabelKey: "blackjack.status.dealerBust", Value: strconv.Itoa(last.DealerTotal),
			Params: map[string]any{"n": last.DealerTotal},
		})
	default:
		out = append(out, module.Fact{
			LabelKey: "blackjack.status.dealerStands", Value: strconv.Itoa(last.DealerTotal),
			Params: map[string]any{"n": last.DealerTotal},
		})
	}
	return out
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
		if s.Phase == phasePlay && s.Current >= 0 {
			return []module.Fact{{
				LabelKey: "blackjack.prompt.waitingFor",
				Params:   map[string]any{"playerId": s.Seats[s.Current].PlayerID},
			}}
		}
		return nil
	}
	switch s.Phase {
	case phaseBets:
		return []module.Fact{{LabelKey: "blackjack.prompt.placeBet"}}
	case phaseInsurance:
		return []module.Fact{{LabelKey: "blackjack.prompt.insurance"}}
	case phasePlay:
		return []module.Fact{{LabelKey: "blackjack.prompt.yourMove"}}
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

// Bot is Blackjack's own player — see bot.go. module.OfferBot cannot play
// this game sensibly: every legal move is offered on every hand, so which one
// to take is the entire game, and an offer-preference bot would hit until it
// went bust or stand on four.
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
	return module.RankByScore(order(s), func(id string) int { return stacks[id] }, "blackjack.unit.chips"), nil
}
