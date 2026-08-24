package prsi

import (
	"strconv"

	"zolik/server/internal/module"
)

// Option names and defaults this module declares.
const (
	OptHandSize     = "handSize"
	defaultHandSize = 4
)

// Descriptor is Prší's self-description.
//
// Note what is *absent* compared with Žolíky's: no meld minimum, no
// discard-lock round, no contract, no deal count. The descriptor shape did not
// have to grow to express a game that has none of those — a knob a game does
// not have is simply a knob it does not declare, and every client renders the
// difference without being told.
func (m *Module) Descriptor() module.ModuleDescriptor {
	return module.ModuleDescriptor{
		ID:         "prsi",
		Label:      "Prší",
		MinPlayers: 2,
		MaxPlayers: 6,
		Variations: []module.VariationSpec{
			{
				ID:    "classic",
				Label: "Classic",
				Summary: []module.Fact{
					{LabelKey: "prsi.rules.deck", Value: "32"},
					{LabelKey: "prsi.rules.sevens"},
					{LabelKey: "prsi.rules.aces"},
					{LabelKey: "prsi.rules.queens"},
				},
				Defaults: map[string]int{OptHandSize: defaultHandSize},
			},
		},
		Options: []module.OptionSpec{
			{
				Name:  OptHandSize,
				Type:  module.OptionEnumInt,
				Label: "Cards dealt",
				Help:  "How many cards each player starts with.",
				Choices: []module.OptionChoice{
					{Value: 4, Label: "4"},
					{Value: 5, Label: "5"},
					{Value: 6, Label: "6"},
				},
			},
		},
	}
}

// View renders the board for one viewer.
//
// Hidden-information filtering lives here, which is the point of View being a
// module method: only this package knows that a Prší hand is private, that the
// draw pile is face down, and that the discard pile shows only its top card.
// The runtime never has to be told any of it.
func (m *Module) View(raw module.State, viewerID string) (module.ViewModel, error) {
	s, err := decode(raw)
	if err != nil {
		return module.ViewModel{}, err
	}

	vm := module.ViewModel{}

	// The viewer's own hand, face up.
	own := s.Hands[viewerID]
	vm.Zones = append(vm.Zones, module.Zone{
		ID: "hand:" + viewerID, Kind: module.ZoneHand, OwnerID: viewerID,
		LabelKey: "zone.yourHand", Cards: cardViews(own), Count: len(own),
	})

	// Everyone else: a count only. This is the whole anti-cheat surface for
	// this game, and it is four lines rather than a 40-line projection.
	for _, p := range s.TurnOrder {
		if p == viewerID {
			continue
		}
		vm.Zones = append(vm.Zones, module.Zone{
			ID: "hand:" + p, Kind: module.ZoneHand, OwnerID: p,
			LabelKey: "zone.opponentHand", Count: len(s.Hands[p]),
		})
	}

	vm.Zones = append(vm.Zones,
		module.Zone{
			ID: "draw", Kind: module.ZoneStack, LabelKey: "zone.drawPile",
			Count: len(s.DrawPile),
		},
		module.Zone{
			ID: "discard", Kind: module.ZonePile, LabelKey: "zone.discardPile",
			Cards: cardViews(topOnly(s)), Count: len(s.DiscardPile),
		},
	)

	// The seats. Prší needs almost nothing here — a card count and whose turn
	// it is — which is the point: a game with no chips, no score and no
	// partnerships declares none of them, exactly as it declares no options.
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

	vm.Header = []module.Fact{
		{LabelKey: "header.deck", Value: strconv.Itoa(len(s.DrawPile))},
	}
	if s.DeclaredSuit != "" {
		vm.Header = append(vm.Header, module.Fact{
			LabelKey: "header.suitInPlay", Value: s.DeclaredSuit,
		})
	}

	// Obligations are prompts, not something the client works out from state.
	if s.PendingDraw > 0 {
		vm.Prompts = append(vm.Prompts, module.Fact{
			LabelKey: "prompt.mustDrawOrAnswerSeven",
			Params:   map[string]any{"n": s.PendingDraw},
		})
	}
	if s.SkipPending {
		vm.Prompts = append(vm.Prompts, module.Fact{LabelKey: "prompt.skipPending"})
	}
	if s.Status == "completed" {
		vm.Status = append(vm.Status, module.Fact{
			LabelKey: "status.winner", Value: s.WinnerID,
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

// topOnly is what the discard pile shows: its top card. What is buried under
// it is not secret exactly, but it is not in play either, and sending it would
// invite a client to reason about it.
func topOnly(s *GameState) []string {
	if t := s.top(); t != "" {
		return []string{t}
	}
	return nil
}
