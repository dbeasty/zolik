package canasta

import (
	"strconv"

	"zolik/server/internal/module"
)

// Option names and the values each variation starts from.
const (
	OptHandSize        = "handSize"
	OptTargetScore     = "targetScore"
	OptCanastasToGoOut = "canastasToGoOut"
)

type variationDefaults struct {
	handSize        int
	targetScore     int
	canastasToGoOut int
}

var variations = map[string]variationDefaults{
	// Classic: eleven cards, one canasta buys the right to go out, 5000 wins.
	"classic": {handSize: 11, targetScore: 5000, canastasToGoOut: 1},
	// Modern American: thirteen cards and two canastas to go out, which makes
	// deals longer and the discard pile far more valuable.
	"modern_american": {handSize: 13, targetScore: 5000, canastasToGoOut: 2},
}

func resolveVariation(cfg module.MatchConfig) variationDefaults {
	if v, ok := variations[cfg.Variation]; ok {
		return v
	}
	return variations["classic"]
}

// Descriptor is Canasta's self-description.
//
// The 500-point target is not a house rule anyone plays; it exists so a test —
// and a demo — can watch a whole match finish without sitting through fifteen
// deals. Declaring it rather than hard-coding a shortcut keeps the descriptor
// the only place a match's shape is decided.
func (m *Module) Descriptor() module.ModuleDescriptor {
	return module.ModuleDescriptor{
		ID:         "canasta",
		Label:      "Canasta",
		MinPlayers: 2,
		MaxPlayers: 4,
		Variations: []module.VariationSpec{
			{
				ID:    "classic",
				Label: "Classic",
				Summary: []module.Fact{
					{LabelKey: "canasta.rules.deck", Value: "108"},
					{LabelKey: "canasta.rules.canasta", Params: map[string]any{"n": canastaSize}},
					{LabelKey: "canasta.rules.redThrees"},
					{LabelKey: "canasta.rules.oneCanastaToGoOut"},
				},
				Defaults: map[string]int{
					OptHandSize:                  variations["classic"].handSize,
					OptTargetScore:               variations["classic"].targetScore,
					OptCanastasToGoOut:           variations["classic"].canastasToGoOut,
					module.OptPauseBetweenRounds: module.OptOn,
					module.OptBotSkill:           module.SkillOpt(module.SkillMedium),
				},
			},
			{
				ID:    "modern_american",
				Label: "Modern American",
				Summary: []module.Fact{
					{LabelKey: "canasta.rules.deck", Value: "108"},
					{LabelKey: "canasta.rules.canasta", Params: map[string]any{"n": canastaSize}},
					{LabelKey: "canasta.rules.redThrees"},
					{LabelKey: "canasta.rules.twoCanastasToGoOut"},
				},
				Defaults: map[string]int{
					OptHandSize:                  variations["modern_american"].handSize,
					OptTargetScore:               variations["modern_american"].targetScore,
					OptCanastasToGoOut:           variations["modern_american"].canastasToGoOut,
					module.OptPauseBetweenRounds: module.OptOn,
					module.OptBotSkill:           module.SkillOpt(module.SkillMedium),
				},
			},
		},
		Options: []module.OptionSpec{
			module.PauseOption(),
			module.BotSkillOption(),
			{
				Name:  OptHandSize,
				Type:  module.OptionEnumInt,
				Label: "Cards dealt",
				Help:  "How many cards each player starts a deal with.",
				Choices: []module.OptionChoice{
					{Value: 11, Label: "11"},
					{Value: 13, Label: "13"},
					{Value: 15, Label: "15"},
				},
			},
			{
				Name:  OptTargetScore,
				Type:  module.OptionEnumInt,
				Label: "Target score",
				Help:  "The score a partnership must pass to win the match.",
				Choices: []module.OptionChoice{
					{Value: 500, Label: "500 (short)"},
					{Value: 1000, Label: "1000"},
					{Value: 3000, Label: "3000"},
					{Value: 5000, Label: "5000"},
				},
			},
			{
				Name:  OptCanastasToGoOut,
				Type:  module.OptionEnumInt,
				Label: "Canastas to go out",
				Help:  "How many canastas a partnership needs before it may go out.",
				Choices: []module.OptionChoice{
					{Value: 1, Label: "1"},
					{Value: 2, Label: "2"},
				},
			},
		},
	}
}

// View renders the board for one viewer.
//
// Per-viewer filtering lives here because only this package knows what is
// secret in this game — and Canasta's answer is unusual enough to be worth
// stating: hands are private, the stock is face down, melds and red threes are
// public to *both* partnerships, and the discard pile shows only its top card
// even though everyone at a real table has watched it being built.
func (m *Module) View(raw module.State, viewerID string) (module.ViewModel, error) {
	s, err := decode(raw)
	if err != nil {
		return module.ViewModel{}, err
	}

	vm := module.ViewModel{}

	own := s.Hands[viewerID]
	vm.Zones = append(vm.Zones, module.Zone{
		ID: handZoneID(viewerID), Kind: module.ZoneHand, OwnerID: viewerID,
		LabelKey: "zone.yourHand", Cards: cardViews(own), Count: len(own),
	})
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
			Cards: cardViews(topOnly(s)), Count: len(s.DiscardPile),
		},
	)

	// One spread per partnership. Melds are groups inside it, badged with what
	// they have become — a client renders "canasta" without knowing that seven
	// is the number.
	for i := range s.Teams {
		t := &s.Teams[i]
		z := module.Zone{
			ID: meldsZoneID(t.ID), Kind: module.ZoneSpread,
			LabelKey: "zone.teamMelds", Count: 0,
		}
		for _, mm := range t.Melds {
			g := module.Group{ID: mm.ID, Kind: "set", Cards: append([]string(nil), mm.Cards...)}
			if mm.isCanasta() {
				if mm.isNatural() {
					g.BadgeKeys = append(g.BadgeKeys, "badge.naturalCanasta")
				} else {
					g.BadgeKeys = append(g.BadgeKeys, "badge.mixedCanasta")
				}
			}
			z.Groups = append(z.Groups, g)
			z.Count += len(mm.Cards)
		}
		vm.Zones = append(vm.Zones, z)

		if len(t.RedThrees) > 0 {
			vm.Zones = append(vm.Zones, module.Zone{
				ID: redThreesZoneID(t.ID), Kind: module.ZoneSpread,
				LabelKey: "zone.redThrees",
				Cards:    cardViews(t.RedThrees), Count: len(t.RedThrees),
			})
		}
	}

	// The seats. Canasta's numbers belong to a partnership rather than a
	// player, so each seat carries its side's score and canasta count — which
	// is exactly the sort of thing that had nowhere to go before Seats existed
	// and had to be smuggled through Status facts.
	for _, p := range s.TurnOrder {
		t := s.team(p)
		seat := module.Seat{
			PlayerID: p,
			Active:   s.Break.Open && !s.Break.Ready[p] || s.Current == p && s.Status == "active",
			Facts: []module.Fact{
				{LabelKey: "seat.cards", Value: strconv.Itoa(len(s.Hands[p])),
					Params: map[string]any{"n": len(s.Hands[p])}},
			},
		}
		if t != nil {
			seat.Facts = append(seat.Facts,
				module.Fact{LabelKey: "canasta.seat.teamScore", Value: strconv.Itoa(t.Score),
					Params: map[string]any{"team": t.ID, "score": t.Score}},
				module.Fact{LabelKey: "canasta.seat.canastas", Value: strconv.Itoa(t.canastas()),
					Params: map[string]any{"n": t.canastas()}},
			)
			if !t.HasMelded {
				seat.LabelKeys = append(seat.LabelKeys, "canasta.seat.notOpened")
			}
		}
		vm.Seats = append(vm.Seats, seat)
	}

	vm.Header = []module.Fact{
		{LabelKey: "header.deck", Value: strconv.Itoa(len(s.DrawPile))},
		// The count goes in the params, not only in Value: the bundle words this
		// one as "Deal {n}", so a fact carrying the number as a bare value left
		// the placeholder on screen — the header read "Deal (n)". Value stays
		// for a client that reads it.
		{LabelKey: "header.deal", Value: strconv.Itoa(s.DealNumber + 1),
			Params: map[string]any{"n": s.DealNumber + 1}},
		{LabelKey: "header.target", Value: strconv.Itoa(s.TargetScore)},
	}
	if s.Frozen {
		vm.Header = append(vm.Header, module.Fact{LabelKey: "header.pileFrozen"})
	}

	// The scoreboard is the whole reason this game needs teams on the wire:
	// `Finished` can only name one winner, so a UI reads standings from here.
	for i := range s.Teams {
		t := &s.Teams[i]
		vm.Status = append(vm.Status, module.Fact{
			LabelKey: "status.teamScore",
			Value:    strconv.Itoa(t.Score),
			Params: map[string]any{
				"team":      t.ID,
				"players":   t.Players,
				"canastas":  t.canastas(),
				"redThrees": len(t.RedThrees),
				"hasMelded": t.HasMelded,
			},
		})
	}

	if vt := s.team(viewerID); vt != nil && s.Status == "active" {
		if !vt.HasMelded {
			vm.Prompts = append(vm.Prompts, module.Fact{
				LabelKey: "prompt.initialMeld",
				Params:   map[string]any{"n": initialMeldMinimum(vt.Score)},
			})
		}
		if !canGoOut(s, vt) {
			vm.Prompts = append(vm.Prompts, module.Fact{
				LabelKey: "prompt.canastasNeeded",
				Params:   map[string]any{"n": s.CanastasToGoOut - vt.canastas()},
			})
		}
	}
	if s.Current == viewerID {
		key := "prompt.yourTurnDraw"
		if s.Phase == phaseMeld {
			key = "prompt.yourTurnMeld"
		}
		vm.Prompts = append(vm.Prompts, module.Fact{LabelKey: key})
	}

	if s.LastDeal != nil {
		for _, tr := range s.LastDeal.Teams {
			vm.Status = append(vm.Status, module.Fact{
				LabelKey: "status.lastDeal",
				Value:    strconv.Itoa(tr.Total),
				Params: map[string]any{
					"team": tr.TeamID, "meldCards": tr.MeldCards, "canastas": tr.Canastas,
					"redThrees": tr.RedThrees, "goingOut": tr.GoingOut, "inHand": tr.InHand,
				},
			})
		}
	}
	if s.Status == "completed" {
		vm.Status = append(vm.Status, module.Fact{
			// The winner travels as a list in Params as well as a bare id in
			// Value: one seat's win, a partnership's and a split pot's are the
			// same sentence to a client, and every module saying it the same
			// way is what lets there be one sentence rather than one per game.
			LabelKey: "status.winner", Value: s.WinnerID,
			Params: map[string]any{"team": s.WinnerTeam, "winners": winnerIDs(s)},
		})
	}
	return vm, nil
}

// winnerIDs is everyone who won, which in a partnership game is a team rather
// than a player. Reads the same fields `Finished` reads, so the sentence on
// screen and the match's recorded result can never name different people.
func winnerIDs(s *GameState) []string {
	if s.WinnerTeam >= 0 && s.WinnerTeam < len(s.Teams) {
		return append([]string(nil), s.Teams[s.WinnerTeam].Players...)
	}
	if s.WinnerID == "" {
		return nil
	}
	return []string{s.WinnerID}
}

func cardViews(cards []string) []module.CardView {
	out := make([]module.CardView, 0, len(cards))
	for _, c := range cards {
		out = append(out, module.CardView{Card: c})
	}
	return out
}

// topOnly is what the discard pile shows. What is buried under it is not in
// play until somebody takes the whole thing, and sending it would invite a
// client to reason about cards its player cannot legally see.
func topOnly(s *GameState) []string {
	if t := s.top(); t != "" {
		return []string{t}
	}
	return nil
}

// Bot is how Canasta wants a vacant seat played: build the table first, take
// the pile when it is offered, and discard only because a turn has to end.
//
// module.OfferBot is enough here where it would not be for Žolíky, because a
// Canasta meld ships as exact cards rather than a shape to solve — the same
// property that lets the conformance driver play this game to a winner.
func (m *Module) Bot() module.Bot {
	return module.OfferBot(VerbLayMeld, VerbLayOff, VerbTakePile, VerbDraw, VerbDiscard)
}

// Standings ranks by partnership score, so both members of a side share a rank
// — which is the case that made module.Standing allow ties in the first place.
func (m *Module) Standings(raw module.State) ([]module.Standing, error) {
	s, err := decode(raw)
	if err != nil {
		return nil, err
	}
	return module.RankByScore(s.TurnOrder, func(id string) int {
		if t := s.team(id); t != nil {
			return t.Score
		}
		return 0
	}, "canasta.unit.points"), nil
}
