package canasta

import (
	"strconv"

	"zolik/server/internal/module"
)

// Option names. The values each variation starts from live in ruleset.go.
const (
	OptHandSize        = "handSize"
	OptTargetScore     = "targetScore"
	OptCanastasToGoOut = "canastasToGoOut"
)

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
		MaxPlayers: 6,
		Variations: []module.VariationSpec{
			{
				ID:         "classic",
				MaxPlayers: variations["classic"].MaxSeats,
				Label:      "Classic",
				Summary: []module.Fact{
					{LabelKey: "canasta.rules.deck", Value: "108", Params: map[string]any{"decks": variations["classic"].Decks}},
					{LabelKey: "canasta.rules.canasta", Params: map[string]any{"n": canastaSize}},
					{LabelKey: "canasta.rules.redThrees"},
					{LabelKey: "canasta.rules.oneCanastaToGoOut"},
				},
				Defaults: map[string]int{
					OptHandSize:                  variations["classic"].HandSize,
					OptTargetScore:               variations["classic"].TargetScore,
					OptCanastasToGoOut:           variations["classic"].CanastasToGoOut,
					module.OptPauseBetweenRounds: module.OptOn,
					module.OptOpenDiscardPile:    module.OptOff,
					module.OptBotSkill:           module.SkillOpt(module.SkillMedium),
				},
			},
			{
				ID:         "modern_american",
				MaxPlayers: variations["modern_american"].MaxSeats,
				Label:      "Modern American",
				Summary: []module.Fact{
					{LabelKey: "canasta.rules.deck", Value: "108", Params: map[string]any{"decks": variations["modern_american"].Decks}},
					{LabelKey: "canasta.rules.canasta", Params: map[string]any{"n": canastaSize}},
					{LabelKey: "canasta.rules.redThrees"},
					{LabelKey: "canasta.rules.twoCanastasToGoOut"},
					// The one rule that tells this variation from Classic at a
					// glance. Two canastas to go out is a number Classic could
					// be set to; a black three that can never be melded is not,
					// and it is the reason the pile matters as much as it does
					// here — so it belongs in the pitch, not only in the rules.
					{LabelKey: "canasta.rules.blackThreesNeverMeld", Params: map[string]any{"n": blackThreeValue}},
				},
				Defaults: map[string]int{
					OptHandSize:                  variations["modern_american"].HandSize,
					OptTargetScore:               variations["modern_american"].TargetScore,
					OptCanastasToGoOut:           variations["modern_american"].CanastasToGoOut,
					module.OptPauseBetweenRounds: module.OptOn,
					module.OptOpenDiscardPile:    module.OptOff,
					module.OptBotSkill:           module.SkillOpt(module.SkillMedium),
				},
			},
			{
				ID:    "samba",
				Label: "Samba",
				// Three decks, sequences and a pile nobody takes cheaply. The
				// seat range is the variation's own: six on 162 cards, where
				// the other two seat four on 108.
				MaxPlayers: variations["samba"].MaxSeats,
				Summary: []module.Fact{
					{LabelKey: "canasta.rules.deck", Value: "162", Params: map[string]any{"decks": variations["samba"].Decks}},
					{LabelKey: "canasta.rules.sequences"},
					{LabelKey: "canasta.rules.samba", Params: map[string]any{"n": variations["samba"].SambaBonus}},
					{LabelKey: "canasta.rules.pileAlwaysFrozen"},
					{LabelKey: "canasta.rules.twoCanastasToGoOut"},
				},
				Defaults: map[string]int{
					OptHandSize:                  variations["samba"].HandSize,
					OptTargetScore:               variations["samba"].TargetScore,
					OptCanastasToGoOut:           variations["samba"].CanastasToGoOut,
					module.OptPauseBetweenRounds: module.OptOn,
					module.OptOpenDiscardPile:    module.OptOff,
					module.OptBotSkill:           module.SkillOpt(module.SkillMedium),
				},
			},
		},
		Options: []module.OptionSpec{
			module.PauseOption(),
			// Off by default, which is how this game has always dealt: the pile
			// is what a capture wins whole, so what lies under the top card is
			// the thing the deal is played over, and a table that wants it
			// readable says so.
			module.OpenDiscardPileOption(),
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
					{Value: 10000, Label: "10000"},
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
	r := s.rules()

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
			Cards: cardViews(shownPile(s)), Count: len(s.DiscardPile),
		},
	)

	// One spread per partnership. Melds are groups inside it, badged with what
	// they have become — a client renders "canasta" without knowing that seven
	// is the number.
	viewerTeam := s.team(viewerID)
	for i := range s.Teams {
		t := &s.Teams[i]
		var z module.Zone
		if viewerTeam != nil && t.ID == viewerTeam.ID {
			z = module.Zone{
				ID: meldsZoneID(t.ID), Kind: module.ZoneSpread,
				LabelKey: "zone.teamMelds", Count: 0,
			}
		} else {
			z = module.Zone{
				ID: meldsZoneID(t.ID), Kind: module.ZoneSpread,
				LabelKey: "zone.opponentMelds", Count: 0,
			}
		}
		for _, mm := range t.Melds {
			g := module.Group{ID: mm.ID, Kind: mm.kind(), Cards: append([]string(nil), mm.Cards...)}
			if mm.isCanasta() {
				switch {
				case mm.kind() == meldRun:
					// Seven in a suit is a samba, and worth saying so: it is the
					// biggest single number on a Samba scoresheet.
					g.BadgeKeys = append(g.BadgeKeys, "badge.samba")
				case mm.isNatural():
					g.BadgeKeys = append(g.BadgeKeys, "badge.naturalCanasta")
				default:
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
	//
	// Sides are set only where sides exist. `s.Teams` is already the answer
	// seatsToTeams gave when the match was dealt, so a side per seat means
	// every player is their own side — heads-up, three, five — and saying so
	// would put a partnership badge on a game that has no partners.
	sided := len(s.Teams) < len(s.TurnOrder)
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
			if sided {
				seat.Side = strconv.Itoa(t.ID)
			}
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
				Params:   map[string]any{"n": r.meldFloor(vt.Score)},
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
		// Both spelled out rather than assigned to one variable: a key that
		// reaches Fact through a variable is invisible to cmd/dump-keys, so it
		// never reaches the manifest and no locale is ever checked for it.
		// These two were exactly that until the sweep in keys_test.go found them.
		if s.Phase == phaseMeld {
			vm.Prompts = append(vm.Prompts, module.Fact{LabelKey: "prompt.yourTurnMeld"})
		} else {
			vm.Prompts = append(vm.Prompts, module.Fact{LabelKey: "prompt.yourTurnDraw"})
		}
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
// shownPile is how much of the discard pile this table publishes: the whole
// thing where the option is on, and otherwise the top card alone.
//
// Nothing secret is being kept either way — every card in it was discarded
// face up in front of everybody. What the folded pile protects is the memory
// it takes to play Canasta well, which is why it is the default and why it is
// a table's own choice rather than the module's.
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

// Bot is how Canasta wants a vacant seat played: build the table first, take
// the pile when it is offered, and discard only because a turn has to end.
//
// module.OfferBot is enough here where it would not be for Žolíky, because a
// Canasta meld ships as exact cards rather than a shape to solve — the same
// property that lets the conformance driver play this game to a winner.
func (m *Module) Bot() module.Bot {
	return module.OfferBot(VerbLayMeld, VerbLayOff, VerbTakePile, VerbTakeTop, VerbDraw, VerbDiscard)
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
