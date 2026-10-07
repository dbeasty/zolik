package lastcard

import (
	"strconv"

	"zolik/server/internal/module"
)

// Option names and defaults this module declares.
const (
	OptHandSize     = "handSize"
	defaultHandSize = 7
)

// Descriptor is Last Card's self-description.
func (m *Module) Descriptor() module.ModuleDescriptor {
	return module.ModuleDescriptor{
		ID:         "lastcard",
		Label:      "Last Card",
		MinPlayers: 2,
		MaxPlayers: 10,
		// Its own pack: the codes are "C-7", "T-S", "W4", and only a client
		// that knows this deck can draw them.
		Deck: module.DeckLastCard,
		Variations: []module.VariationSpec{
			{
				ID:    "classic",
				Label: "Classic",
				Summary: []module.Fact{
					{LabelKey: "lastcard.rules.deck", Value: "108"},
					{LabelKey: "lastcard.rules.turn.match"},
					{LabelKey: "lastcard.rules.wildDrawFour"},
				},
				Defaults: map[string]int{
					OptHandSize:               defaultHandSize,
					module.OptOpenDiscardPile: module.OptOff,
					module.OptBotSkill:        module.SkillOpt(module.SkillMedium),
				},
			},
		},
		Options: []module.OptionSpec{
			module.OpenDiscardPileOption(),
			module.BotSkillOption(),
			module.HintsOption(),
			{
				Name:  OptHandSize,
				Type:  module.OptionEnumInt,
				Label: "Cards dealt",
				Help:  "How many cards each player starts with.",
				Choices: []module.OptionChoice{
					{Value: 5, Label: "5"},
					{Value: 7, Label: "7"},
					{Value: 10, Label: "10"},
				},
			},
		},
	}
}

// View renders the board for one viewer. Every other hand is a count only.
func (m *Module) View(raw module.State, viewerID string) (module.ViewModel, error) {
	return m.view(raw, viewerID, false)
}

// OpenView renders the board with every hand face up, for replaying a game
// that is over.
func (m *Module) OpenView(raw module.State) (module.ViewModel, error) {
	return m.view(raw, "", true)
}

func (m *Module) view(raw module.State, viewerID string, reveal bool) (module.ViewModel, error) {
	s, err := decode(raw)
	if err != nil {
		return module.ViewModel{}, err
	}
	vm := module.ViewModel{}

	if viewerID != "" {
		own := s.Hands[viewerID]
		vm.Zones = append(vm.Zones, module.Zone{
			ID: handZoneID(viewerID), Kind: module.ZoneHand, OwnerID: viewerID,
			LabelKey: "zone.yourHand", Cards: cardViews(own), Count: len(own),
		})
	}
	for _, p := range s.TurnOrder {
		if p == viewerID {
			continue
		}
		z := module.Zone{
			ID: handZoneID(p), Kind: module.ZoneHand, OwnerID: p,
			LabelKey: "zone.opponentHand", Count: len(s.Hands[p]),
		}
		if reveal {
			z.Cards = cardViews(s.Hands[p])
		}
		vm.Zones = append(vm.Zones, z)
	}
	vm.Zones = append(vm.Zones,
		module.Zone{
			ID: drawZoneID, Kind: module.ZoneStack, LabelKey: "zone.drawPile",
			Count: len(s.DrawPile),
		},
		module.Zone{
			ID: discardZoneID, Kind: module.ZonePile, LabelKey: "zone.discardPile",
			Cards: cardViews(shownPile(s)), Count: len(s.DiscardPile),
		},
	)

	for _, p := range s.TurnOrder {
		vm.Seats = append(vm.Seats, module.Seat{
			PlayerID: p,
			Active:   s.Current == p && s.Status == "active",
			Facts: []module.Fact{{
				LabelKey: "seat.cards", Value: strconv.Itoa(len(s.Hands[p])),
				Params: map[string]any{"n": len(s.Hands[p])},
			}},
		})
	}

	// The colour in play is always worth saying — a wild on top carries none
	// of its own — and so is which way round the table play is going, which
	// nothing on the board shows otherwise.
	vm.Header = []module.Fact{
		{LabelKey: "header.deck", Value: strconv.Itoa(len(s.DrawPile))},
	}
	if c := s.colourInPlay(); c != "" {
		vm.Header = append(vm.Header, module.Fact{LabelKey: "lastcard.header.colour", Value: colourKey(c)})
	}
	if s.Direction < 0 {
		vm.Header = append(vm.Header, module.Fact{LabelKey: "lastcard.header.anticlockwise"})
	} else {
		vm.Header = append(vm.Header, module.Fact{LabelKey: "lastcard.header.clockwise"})
	}

	// The drawn card is the drawer's own business: everyone else saw a card
	// leave the pile face down, and nothing more.
	if s.DrawnCard != "" && viewerID == s.Current {
		vm.Prompts = append(vm.Prompts, module.Fact{
			LabelKey: "lastcard.prompt.playDrawnOrKeep",
			Params:   map[string]any{"card": s.DrawnCard},
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

// shownPile is how much of the discard pile this table publishes — all of it
// where the option is on, the top card alone otherwise.
func shownPile(s *GameState) []string {
	if s.OpenDiscard {
		return s.DiscardPile
	}
	if t := s.top(); t != "" {
		return []string{t}
	}
	return nil
}

// Bot is how Last Card wants a vacant seat played — see bot.go.
func (m *Module) Bot() module.Bot { return bot{} }

// Standings ranks by cards left, fewest first.
func (m *Module) Standings(raw module.State) ([]module.Standing, error) {
	s, err := decode(raw)
	if err != nil {
		return nil, err
	}
	// Negated, because RankByScore ranks highest-first and here fewer is
	// better; Shown carries the count as a player would say it.
	out := module.RankByScore(s.TurnOrder,
		func(id string) int { return -len(s.Hands[id]) }, "lastcard.unit.cardsLeft")
	for i := range out {
		shown := len(s.Hands[out[i].PlayerID])
		out[i].Shown = &shown
	}
	return out, nil
}
