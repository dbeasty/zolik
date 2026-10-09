package lastcard

import (
	"strconv"

	"zolik/server/internal/module"
)

// Option names and defaults this module declares.
const (
	OptHandSize     = "handSize"
	defaultHandSize = 7

	// OptTargetScore ends the match when a player's total reaches it; zero
	// is a single deal.
	OptTargetScore     = "targetScore"
	defaultTargetScore = 500

	// OptLastCardCall is whether a player must say "Last card!".
	OptLastCardCall = "lastCardCall"
	// OptDrawFourChallenge is whether a Wild Draw Four may be challenged.
	OptDrawFourChallenge = "drawFourChallenge"

	// OptScoring is how points are kept: winner-takes (the default) or
	// lowest total wins.
	OptScoring = "scoring"

	// House rules, every one off by default.
	OptStacking          = "stacking"
	OptDrawUntilPlayable = "drawUntilPlayable"
	OptSevenZero         = "sevenZero"
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
					{LabelKey: "lastcard.rules.call"},
					{LabelKey: "lastcard.rules.scoring", Params: map[string]any{"target": defaultTargetScore}},
				},
				Defaults: map[string]int{
					OptHandSize:                  defaultHandSize,
					OptTargetScore:               defaultTargetScore,
					OptScoring:                   scoreWinnerTakes,
					OptLastCardCall:              module.OptOn,
					OptDrawFourChallenge:         module.OptOn,
					OptStacking:                  stackOff,
					OptDrawUntilPlayable:         module.OptOff,
					OptSevenZero:                 module.OptOff,
					module.OptPauseBetweenRounds: module.OptOn,
					module.OptOpenDiscardPile:    module.OptOff,
					module.OptBotSkill:           module.SkillOpt(module.SkillMedium),
					module.OptStandInAfter:       module.StandInAfterDefault,
				},
			},
		},
		Options: []module.OptionSpec{
			{
				Name:  OptTargetScore,
				Type:  module.OptionEnumInt,
				Label: "Target score",
				Help:  "Play deal after deal until someone's total reaches this — or play a single deal.",
				Choices: []module.OptionChoice{
					{Value: 0, Label: "One deal"},
					{Value: 200, Label: "200"},
					{Value: 500, Label: "500"},
				},
			},
			{
				Name:  OptScoring,
				Type:  module.OptionEnumInt,
				Label: "Scoring",
				Help:  "The winner of each deal scores everyone else's cards — or everyone pays for their own, and the lowest total wins.",
				Choices: []module.OptionChoice{
					{Value: scoreWinnerTakes, Label: "Winner takes the points"},
					{Value: scoreLowest, Label: "Lowest total wins"},
				},
			},
			{
				Name:  OptLastCardCall,
				Type:  module.OptionEnumInt,
				Label: "Calling “Last card!”",
				Help:  "Say it before the play that leaves you one card, or the next player can catch you for two cards.",
				Choices: []module.OptionChoice{
					{Value: module.OptOn, Label: "On"},
					{Value: module.OptOff, Label: "Off"},
				},
			},
			{
				Name:  OptDrawFourChallenge,
				Type:  module.OptionEnumInt,
				Label: "Wild Draw Four challenge",
				Help:  "A Wild Draw Four may be bluffed, and the next player may challenge it.",
				Choices: []module.OptionChoice{
					{Value: module.OptOn, Label: "On"},
					{Value: module.OptOff, Label: "Off"},
				},
			},
			{
				Name:  OptStacking,
				Type:  module.OptionEnumInt,
				Label: "Stacking draw cards",
				Help:  "Answer a draw card with one of your own and pass the whole stack on.",
				Choices: []module.OptionChoice{
					{Value: stackOff, Label: "Off"},
					{Value: stackTwos, Label: "Draw Two on Draw Two"},
					{Value: stackAny, Label: "Any draw card"},
				},
			},
			{
				Name:  OptDrawUntilPlayable,
				Type:  module.OptionEnumInt,
				Label: "Draw until you can play",
				Help:  "Keep drawing until a card you can play turns up.",
				Choices: []module.OptionChoice{
					{Value: module.OptOff, Label: "Off"},
					{Value: module.OptOn, Label: "On"},
				},
			},
			{
				Name:  OptSevenZero,
				Type:  module.OptionEnumInt,
				Label: "Sevens and zeros",
				Help:  "A 7 swaps your hand with the shortest one; a 0 passes every hand along.",
				Choices: []module.OptionChoice{
					{Value: module.OptOff, Label: "Off"},
					{Value: module.OptOn, Label: "On"},
				},
			},
			module.PauseOption(),
			module.OpenDiscardPileOption(),
			module.BotSkillOption(),
			module.StandInOption(),
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

// configOf is the table's configuration as its state records it, which is
// what the written rules — and the refusals pointing into them — have to be
// resolved against: a table with the call switched off states no rule about
// calling, and a refusal must not point at one.
func configOf(s *GameState) module.MatchConfig {
	return module.MatchConfig{Options: module.Options{
		OptHandSize:          s.HandSize,
		OptTargetScore:       s.TargetScore,
		OptScoring:           s.Scoring,
		OptLastCardCall:      module.BoolOpt(s.CallOn),
		OptDrawFourChallenge: module.BoolOpt(s.ChallengeOn),
		OptStacking:          s.Stacking,
		OptDrawUntilPlayable: module.BoolOpt(s.DrawUntil),
		OptSevenZero:         module.BoolOpt(s.SevenZero),
	}}
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
			Cards: pileViews(s), Count: len(s.DiscardPile),
		},
	)

	var seats []module.Seat
	if s.Intermission.Open {
		seats = s.Intermission.Seats(s.TurnOrder)
	} else {
		for _, p := range s.TurnOrder {
			seats = append(seats, module.Seat{PlayerID: p, Active: s.Current == p && s.Status == "active"})
		}
	}
	for i := range seats {
		p := seats[i].PlayerID
		seats[i].Facts = append(seats[i].Facts, module.Fact{
			LabelKey: "seat.cards", Value: strconv.Itoa(len(s.Hands[p])),
			Params: map[string]any{"n": len(s.Hands[p])},
		})
		if s.TargetScore > 0 && s.Scoring == scoreLowest {
			seats[i].Facts = append(seats[i].Facts, module.Fact{
				LabelKey: "lastcard.seat.penalty", Value: strconv.Itoa(s.Scores[p]),
				Params: map[string]any{"n": s.Scores[p]},
			})
		} else if s.TargetScore > 0 {
			seats[i].Facts = append(seats[i].Facts, module.Fact{
				LabelKey: "lastcard.seat.points", Value: strconv.Itoa(s.Scores[p]),
				Params: map[string]any{"n": s.Scores[p]},
			})
		}
		// One card left, said out loud. A player who went to one card in
		// silence wears no badge — which is exactly the clue a table reads.
		if s.CallOn && len(s.Hands[p]) == 1 && s.Unannounced != p && s.Status == "active" {
			seats[i].LabelKeys = append(seats[i].LabelKeys, "lastcard.seat.lastCard")
		}
	}
	vm.Seats = seats

	// The colour in play is always worth saying — a wild on top carries none
	// of its own — and so is which way round the table play is going, which
	// nothing on the board shows otherwise.
	vm.Header = []module.Fact{
		{LabelKey: "header.deck", Value: strconv.Itoa(len(s.DrawPile))},
	}
	if c := s.colourInPlay(); c != "" {
		vm.Header = append(vm.Header, module.Fact{LabelKey: "lastcard.header.colour", Value: colourKey(c)})
	}
	if s.direction() < 0 {
		vm.Header = append(vm.Header, module.Fact{LabelKey: "lastcard.header.anticlockwise"})
	} else {
		vm.Header = append(vm.Header, module.Fact{LabelKey: "lastcard.header.clockwise"})
	}
	if s.TargetScore > 0 {
		// The deal being played — or, while the table reads the score, the
		// one just finished rather than the one not yet dealt.
		deal := s.DealNumber + 1
		if s.Intermission.Open {
			deal = s.DealNumber
		}
		vm.Header = append(vm.Header,
			module.Fact{LabelKey: "lastcard.header.deal", Params: map[string]any{"n": deal}},
			module.Fact{LabelKey: "header.target", Value: strconv.Itoa(s.TargetScore)},
		)
	}

	// The prompts are the viewer's own business: a drawn card, a Wild Draw
	// Four to answer, a player to catch, a hand shown to a challenger.
	if viewerID != "" && viewerID == s.Current && s.Status == "active" {
		if s.DrawnCard != "" {
			vm.Prompts = append(vm.Prompts, module.Fact{
				LabelKey: "lastcard.prompt.playDrawnOrKeep",
				Params:   map[string]any{"card": s.DrawnCard},
			})
		}
		if s.PendingDraw > 0 && s.DrawFour == nil {
			vm.Prompts = append(vm.Prompts, module.Fact{
				LabelKey: "lastcard.prompt.stack",
				Params:   map[string]any{"n": s.PendingDraw},
			})
		}
		if s.DrawFour != nil && s.DrawFour.Victim == viewerID {
			vm.Prompts = append(vm.Prompts, module.Fact{
				LabelKey: "lastcard.prompt.drawFour",
				Params:   map[string]any{"player": s.DrawFour.Player},
			})
		}
		if s.Unannounced != "" && s.Unannounced != viewerID {
			vm.Prompts = append(vm.Prompts, module.Fact{
				LabelKey: "lastcard.prompt.catch",
				Params:   map[string]any{"player": s.Unannounced},
			})
		}
	}
	if r := s.Reveal; r != nil && r.Viewer != "" && r.Viewer == viewerID {
		if r.Bluff {
			vm.Prompts = append(vm.Prompts, module.Fact{
				LabelKey: "lastcard.prompt.bluffCaught",
				Params:   map[string]any{"player": r.Owner, "cards": r.Cards},
			})
		} else {
			vm.Prompts = append(vm.Prompts, module.Fact{
				LabelKey: "lastcard.prompt.noBluff",
				Params:   map[string]any{"player": r.Owner, "cards": r.Cards},
			})
		}
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

// pileViews is the discard pile as drawn, with the colour a wild on top was
// named carried on the card itself (CardView.As), so the card a player looks
// at says the colour in play rather than leaving it to the header.
func pileViews(s *GameState) []module.CardView {
	out := cardViews(shownPile(s))
	if n := len(out); n > 0 && isWild(out[n-1].Card) && s.DeclaredColour != "" {
		out[n-1].As = s.DeclaredColour
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

// Standings ranks by points where the table plays for them, and by cards
// left in a single deal.
func (m *Module) Standings(raw module.State) ([]module.Standing, error) {
	s, err := decode(raw)
	if err != nil {
		return nil, err
	}
	if s.TargetScore > 0 && s.Scoring == scoreLowest {
		// Lowest wins: ranked on the negated total, shown as the total. The
		// match's winner is first whatever a tie at the bottom would say.
		out := module.RankByScore(s.TurnOrder,
			func(id string) int { return -s.Scores[id] }, "lastcard.unit.penalty")
		for i := range out {
			shown := s.Scores[out[i].PlayerID]
			out[i].Shown = &shown
		}
		return out, nil
	}
	if s.TargetScore > 0 {
		return module.RankByScore(s.TurnOrder,
			func(id string) int { return s.Scores[id] }, "lastcard.unit.points"), nil
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

var _ module.Rounded = (*Module)(nil)

// Rounds is the deal-by-deal score sheet. Only a deal's winner scores, and
// their lines say what the points were made of.
func (m *Module) Rounds(raw module.State) (module.RoundLog, error) {
	s, err := decode(raw)
	if err != nil {
		return module.RoundLog{}, err
	}
	log := module.RoundLog{
		LabelKey:   "lastcard.round.deal",
		Rounds:     []module.RoundResult{},
		Paused:     s.Intermission.Open,
		WaitingFor: s.Intermission.Waiting(s.TurnOrder),
	}
	for _, d := range s.Deals {
		r := module.RoundResult{Number: d.Number, Winners: []string{d.Winner}}
		for _, p := range s.TurnOrder {
			rs := module.RoundScore{PlayerID: p, Total: d.Totals[p]}
			if d.Charged != nil {
				// Lowest wins: every other player's own hand, charged to them.
				b := d.Charged[p]
				rs.Delta = b.Total()
				if b.Numbers > 0 {
					rs.Lines = append(rs.Lines, module.ScoreLine{LabelKey: "lastcard.score.numbers", Points: b.Numbers})
				}
				if b.Actions > 0 {
					rs.Lines = append(rs.Lines, module.ScoreLine{LabelKey: "lastcard.score.actions", Points: b.Actions})
				}
				if b.Wilds > 0 {
					rs.Lines = append(rs.Lines, module.ScoreLine{LabelKey: "lastcard.score.wilds", Points: b.Wilds})
				}
				r.Scores = append(r.Scores, rs)
				continue
			}
			if p == d.Winner {
				rs.Delta = d.Points
				if d.Numbers > 0 {
					rs.Lines = append(rs.Lines, module.ScoreLine{LabelKey: "lastcard.score.numbers", Points: d.Numbers})
				}
				if d.Actions > 0 {
					rs.Lines = append(rs.Lines, module.ScoreLine{LabelKey: "lastcard.score.actions", Points: d.Actions})
				}
				if d.Wilds > 0 {
					rs.Lines = append(rs.Lines, module.ScoreLine{LabelKey: "lastcard.score.wilds", Points: d.Wilds})
				}
			}
			r.Scores = append(r.Scores, rs)
		}
		log.Rounds = append(log.Rounds, r)
	}
	return log, nil
}
