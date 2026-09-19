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
				Defaults: map[string]int{
					OptHandSize:               defaultHandSize,
					module.OptOpenDiscardPile: module.OptOff,
					module.OptBotSkill:        module.SkillOpt(module.SkillMedium),
				},
			},
		},
		Options: []module.OptionSpec{
			// Off by default, which is the pub table this game is played on:
			// the pile is a stack of cards nobody turns over, and what went
			// into it is remembered rather than looked up.
			module.OpenDiscardPileOption(),
			module.BotSkillOption(),
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
	//
	// (Zone ids come from the helpers below rather than being written out, so
	// the offers can point at the same strings — an offer naming a zone id no
	// zone has is a drop target nobody can hit.)
	own := s.Hands[viewerID]
	vm.Zones = append(vm.Zones, module.Zone{
		ID: handZoneID(viewerID), Kind: module.ZoneHand, OwnerID: viewerID,
		LabelKey: "zone.yourHand", Cards: cardViews(own), Count: len(own),
	})

	// Everyone else: a count only. This is the whole anti-cheat surface for
	// this game, and it is four lines rather than a 40-line projection.
	for _, p := range s.TurnOrder {
		if p == viewerID {
			continue
		}
		vm.Zones = append(vm.Zones, module.Zone{
			ID: handZoneID(p), Kind: module.ZoneHand, OwnerID: p,
			LabelKey: "zone.opponentHand", Count: len(s.Hands[p]),
		})
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
			// The winner travels as a list in Params as well as a bare id in
			// Value: one seat's win and a split are the same sentence to a
			// client, and every module saying it the same way is what lets
			// there be one sentence rather than one per game.
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

// topOnly is what the discard pile shows: its top card. What is buried under
// it is not secret exactly, but it is not in play either, and sending it would
// invite a client to reason about it.
// shownPile is how much of the discard pile this table publishes — all of it
// where the option is on, the top card alone otherwise. Both are public
// either way: every card in the pile was played face up, and the pile is
// shuffled when it is recycled, so an open pile says nothing about the order
// of the draw pile it becomes.
func shownPile(s *GameState) []string {
	if s.OpenDiscard {
		return s.DiscardPile
	}
	return topOnly(s)
}

func topOnly(s *GameState) []string {
	if t := s.top(); t != "" {
		return []string{t}
	}
	return nil
}

// Bot is how Prší wants a vacant seat played: shed a card, take a skip if one
// is owed, and draw only when there is nothing else — and choose *which* card,
// which is the part module.OfferBot could not do and the part this game is. See
// bot.go.
func (m *Module) Bot() module.Bot { return bot{} }

// Standings ranks by cards left, fewest first — which is both the state of the
// race mid-deal and the result at the end of it, since the winner is whoever
// reaches zero.
func (m *Module) Standings(raw module.State) ([]module.Standing, error) {
	s, err := decode(raw)
	if err != nil {
		return nil, err
	}
	// Negated, because RankByScore ranks highest-first and here fewer is
	// better. Doing it this way rather than adding a direction flag keeps one
	// ranking implementation with one tie rule.
	out := module.RankByScore(s.TurnOrder,
		func(id string) int { return -len(s.Hands[id]) }, "prsi.unit.cardsLeft")
	for i := range out {
		// Score stays negated so the runtime can rank and record it without a
		// sense of direction; Shown is the count as a player would say it.
		//
		// Without this the negation showed through onto every Prší board in
		// production: the seat strip read "-4 cards left" under both players,
		// on a live game, for as long as the module has existed. Žolíky has
		// carried the same two lines since its own penalties printed
		// backwards — see zolikmod.Module.Standings.
		shown := len(s.Hands[out[i].PlayerID])
		out[i].Shown = &shown
	}
	return out, nil
}
