package holdem

import (
	"strconv"

	"zolik/server/internal/module"
)

// Option names and the values each variation starts from.
const (
	OptStartingStack = "startingStack"
	OptBigBlind      = "bigBlind"
	OptHandLimit     = "handLimit"
)

type variationDefaults struct {
	startingStack int
	bigBlind      int
	handLimit     int
}

var variations = map[string]variationDefaults{
	// Freezeout: play until one player holds every chip.
	"freezeout": {startingStack: 1000, bigBlind: 20, handLimit: 0},
	// Timed: a fixed number of hands, most chips wins — and, unlike a
	// freezeout, it can genuinely end level. That is the case that made
	// Finished return a list of winners rather than one.
	"timed": {startingStack: 1000, bigBlind: 20, handLimit: 10},
}

func resolveVariation(cfg module.MatchConfig) variationDefaults {
	if v, ok := variations[cfg.Variation]; ok {
		return v
	}
	return variations["freezeout"]
}

// Descriptor is Hold'em's self-description.
//
// Note what it needs that no card game did: a chip count and a blind size. The
// option vocabulary expressed both without changing, which is the quiet half of
// this experiment's result — the descriptor was the part that did not bend.
func (m *Module) Descriptor() module.ModuleDescriptor {
	return module.ModuleDescriptor{
		ID:         "holdem",
		Label:      "Texas Hold'em",
		MinPlayers: 2,
		MaxPlayers: 9,
		Variations: []module.VariationSpec{
			{
				ID:    "freezeout",
				Label: "Freezeout",
				Summary: []module.Fact{
					{LabelKey: "holdem.rules.noLimit"},
					{LabelKey: "holdem.rules.lastPlayerStanding"},
				},
				Defaults: map[string]int{
					OptStartingStack: variations["freezeout"].startingStack,
					OptBigBlind:      variations["freezeout"].bigBlind,
					OptHandLimit:     variations["freezeout"].handLimit,
				},
			},
			{
				ID:    "timed",
				Label: "Fixed hands",
				Summary: []module.Fact{
					{LabelKey: "holdem.rules.noLimit"},
					{LabelKey: "holdem.rules.mostChipsWins"},
				},
				Defaults: map[string]int{
					OptStartingStack: variations["timed"].startingStack,
					OptBigBlind:      variations["timed"].bigBlind,
					OptHandLimit:     variations["timed"].handLimit,
				},
			},
		},
		Options: []module.OptionSpec{
			{
				Name:  OptStartingStack,
				Type:  module.OptionEnumInt,
				Label: "Starting chips",
				Help:  "How many chips each seat begins with.",
				Choices: []module.OptionChoice{
					{Value: 200, Label: "200 (short)"},
					{Value: 1000, Label: "1000"},
					{Value: 5000, Label: "5000"},
				},
			},
			{
				Name:  OptBigBlind,
				Type:  module.OptionEnumInt,
				Label: "Big blind",
				Help:  "The big blind; the small blind is half of it.",
				Choices: []module.OptionChoice{
					{Value: 10, Label: "10"},
					{Value: 20, Label: "20"},
					{Value: 50, Label: "50"},
				},
			},
			{
				Name:  OptHandLimit,
				Type:  module.OptionEnumInt,
				Label: "Hands",
				Help:  "How many hands to play; zero plays until one seat has every chip.",
				Choices: []module.OptionChoice{
					{Value: 0, Label: "Until one seat is left"},
					{Value: 5, Label: "5"},
					{Value: 10, Label: "10"},
					{Value: 30, Label: "30"},
				},
			},
		},
	}
}

// View renders the table for one viewer.
//
// The hidden-information rule here is the simplest of the four modules and the
// strictest: a live hand's hole cards belong to their owner and nobody else,
// full stop. What makes it interesting is the exception — a showdown is public,
// so the *previous* hand's cards are shown to everyone, which is a reveal no
// other module in this codebase has.
func (m *Module) View(raw module.State, viewerID string) (module.ViewModel, error) {
	s, err := decode(raw)
	if err != nil {
		return module.ViewModel{}, err
	}

	vm := module.ViewModel{}

	for i := range s.Seats {
		st := &s.Seats[i]
		z := module.Zone{
			ID: "hole:" + st.PlayerID, Kind: module.ZoneHand, OwnerID: st.PlayerID,
			Count: len(st.Hole),
		}
		if st.PlayerID == viewerID {
			z.LabelKey = "zone.yourHand"
			z.Cards = cardViews(st.Hole)
		} else {
			z.LabelKey = "zone.opponentHand"
		}
		vm.Zones = append(vm.Zones, z)
	}

	vm.Zones = append(vm.Zones,
		module.Zone{
			ID: "board", Kind: module.ZoneSpread, LabelKey: "zone.board",
			Cards: cardViews(s.Board), Count: len(s.Board),
		},
		module.Zone{
			ID: "deck", Kind: module.ZoneStack, LabelKey: "zone.drawPile",
			Count: len(s.Deck),
		},
	)

	// The seats. This is the field poker added, and it is where every number
	// that is not a card lives.
	for i := range s.Seats {
		st := &s.Seats[i]
		seat := module.Seat{
			PlayerID: st.PlayerID,
			Active:   s.Current == i && s.Status == "active",
			Facts: []module.Fact{
				{LabelKey: "holdem.seat.stack", Value: strconv.Itoa(st.Stack),
					Params: map[string]any{"chips": st.Stack}},
			},
		}
		if st.Bet > 0 {
			seat.Facts = append(seat.Facts, module.Fact{
				LabelKey: "holdem.seat.bet", Value: strconv.Itoa(st.Bet),
				Params: map[string]any{"chips": st.Bet},
			})
		}
		if i == s.Button {
			seat.LabelKeys = append(seat.LabelKeys, "holdem.seat.dealer")
		}
		if st.Folded {
			seat.LabelKeys = append(seat.LabelKeys, "holdem.seat.folded")
		}
		if st.AllIn {
			seat.LabelKeys = append(seat.LabelKeys, "holdem.seat.allIn")
		}
		if st.Out {
			seat.LabelKeys = append(seat.LabelKeys, "holdem.seat.out")
		}
		vm.Seats = append(vm.Seats, seat)
	}

	vm.Header = []module.Fact{
		{LabelKey: "holdem.header.pot", Value: strconv.Itoa(s.Pot)},
		{LabelKey: "holdem.header.street", Value: s.Street},
		{LabelKey: "holdem.header.hand", Value: strconv.Itoa(s.HandNumber)},
		{LabelKey: "holdem.header.blinds",
			Value:  strconv.Itoa(s.SmallBlind) + "/" + strconv.Itoa(s.BigBlind),
			Params: map[string]any{"small": s.SmallBlind, "big": s.BigBlind}},
	}
	if s.HandLimit > 0 {
		vm.Header = append(vm.Header, module.Fact{
			LabelKey: "holdem.header.handLimit", Value: strconv.Itoa(s.HandLimit),
		})
	}

	// The previous hand, including any showdown. Public by the rules of the
	// game: cards turned face up at a showdown are turned face up for everyone.
	if s.LastHand != nil {
		for _, p := range s.LastHand.Pots {
			vm.Status = append(vm.Status, module.Fact{
				LabelKey: "holdem.status.pot", Value: strconv.Itoa(p.Amount),
				Params: map[string]any{
					"winners": p.Winners, "hand": p.LabelKey, "amount": p.Amount,
				},
			})
		}
		for _, sh := range s.LastHand.Shown {
			vm.Status = append(vm.Status, module.Fact{
				LabelKey: "holdem.status.shown", Value: sh.LabelKey,
				Params: map[string]any{
					"playerId": sh.PlayerID, "hole": sh.Hole, "best": sh.Best,
				},
			})
		}
	}

	if s.Status == "active" && s.Current >= 0 {
		if s.Seats[s.Current].PlayerID == viewerID {
			vm.Prompts = append(vm.Prompts, module.Fact{LabelKey: "holdem.prompt.yourAction"})
		} else {
			vm.Prompts = append(vm.Prompts, module.Fact{
				LabelKey: "holdem.prompt.waitingFor",
				Params:   map[string]any{"playerId": s.Seats[s.Current].PlayerID},
			})
		}
	}
	if s.Status == "completed" {
		vm.Status = append(vm.Status, module.Fact{
			LabelKey: "status.winner",
			Params:   map[string]any{"winners": append([]string(nil), s.Winners...)},
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
