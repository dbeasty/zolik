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
	// OptShowdownReveal is whose hand goes face up on its own at a showdown.
	//
	// A house rule rather than a rule: a live table only makes the player
	// taking the chips prove it, while every online room has a switch for
	// showing everything. This module has always shown everything, so that
	// stays the default and the stricter reading is the choice.
	//
	// It changes what a viewer is *sent*, never what anybody may do — the same
	// discipline OptOpenDiscardPile keeps. A player whose hand it hides may
	// still turn it over themselves.
	OptShowdownReveal = "showdownReveal"
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
					OptStartingStack:   variations["freezeout"].startingStack,
					OptBigBlind:        variations["freezeout"].bigBlind,
					OptHandLimit:       variations["freezeout"].handLimit,
					OptShowdownReveal:  RevealEveryone,
					module.OptBotSkill: module.SkillOpt(module.SkillMedium),
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
					OptStartingStack:   variations["timed"].startingStack,
					OptBigBlind:        variations["timed"].bigBlind,
					OptHandLimit:       variations["timed"].handLimit,
					OptShowdownReveal:  RevealEveryone,
					module.OptBotSkill: module.SkillOpt(module.SkillMedium),
				},
			},
		},
		Options: []module.OptionSpec{
			// No pause option. Hold'em stops after every hand whatever the
			// table says, because the stop is where the cards are — see
			// GameState.Break. An option that decides nothing is worse than
			// no option: a lobby renders it as a working control.
			module.BotSkillOption(),
			{
				Name:  OptShowdownReveal,
				Type:  module.OptionEnumInt,
				Label: "Show at showdown",
				Help:  "Whose cards are turned face up when a hand is called. Either way, a player may always show their own.",
				Choices: []module.OptionChoice{
					{Value: RevealEveryone, Label: "Everyone who was called"},
					{Value: RevealWinners, Label: "The winner only"},
				},
			},
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
	return m.view(raw, viewerID, false)
}

// OpenView renders the board with every hole card face up, for replaying a
// hand that is over. The runtime only ever asks for it once a match is
// finished — showing hole cards mid-hand would be the whole game.
func (m *Module) OpenView(raw module.State) (module.ViewModel, error) {
	return m.view(raw, "", true)
}

func (m *Module) view(raw module.State, viewerID string, reveal bool) (module.ViewModel, error) {
	s, err := decode(raw)
	if err != nil {
		return module.ViewModel{}, err
	}

	vm := module.ViewModel{}

	// Whose hand is face up, and whether anybody is looking at a showdown at
	// all. Read from LastHand.Shown rather than from who happens to still be
	// holding cards: during the stop *every* seat still holds the hand it
	// played, and "has cards" is not the same question as "showed them".
	open := showdownOpen(s)
	faceUp := map[string][]string{}
	if open && s.LastHand != nil {
		for _, sh := range s.LastHand.Shown {
			faceUp[sh.PlayerID] = sh.Hole
		}
	}

	for i := range s.Seats {
		st := &s.Seats[i]
		z := module.Zone{
			ID: "hole:" + st.PlayerID, Kind: module.ZoneHand, OwnerID: st.PlayerID,
			Count: len(st.Hole),
		}
		switch {
		case st.PlayerID == viewerID:
			z.LabelKey = "zone.yourHand"
			z.Cards = cardViews(st.Hole)
		case reveal:
			// A replay of a finished hand, where everything is open. Ahead of
			// the showdown case because it is the wider of the two: a replay
			// shows the hands nobody turned over as well as the ones they did.
			z.LabelKey = "zone.opponentHand"
			z.Cards = cardViews(st.Hole)
		case faceUp[st.PlayerID] != nil:
			// Turned over at the showdown, so it is everyone's to see. Drawn
			// in the seat's own hand zone rather than in a second one beside
			// it: a player has one hand, and two zones holding the same two
			// cards is the board disagreeing with itself.
			z.LabelKey = "zone.shownHand"
			z.Cards = cardViews(faceUp[st.PlayerID])
			z.Count = len(faceUp[st.PlayerID])
		default:
			z.LabelKey = "zone.opponentHand"
		}
		vm.Zones = append(vm.Zones, z)
	}

	vm.Zones = append(vm.Zones,
		module.Zone{
			// Shared: the five are the table's, not a player's and not a
			// side's, so the board is drawn up on the felt beside the deck
			// rather than down among the players' spreads.
			ID: "board", Kind: module.ZoneSpread, LabelKey: "zone.board", Shared: true,
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
			Active: s.Break.Open && !s.Break.Ready[s.Seats[i].PlayerID] ||
				s.Current == i && s.Status == "active",
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
		// Who took the hand that is on the table, marked on the seat rather
		// than left to be read off a sentence underneath it.
		if open && took(s.LastHand, st.PlayerID) {
			seat.LabelKeys = append(seat.LabelKeys, "holdem.seat.won")
		}
		if open && mucked(s.LastHand, st.PlayerID) {
			seat.LabelKeys = append(seat.LabelKeys, "holdem.seat.mucked")
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

	// The hand on the table, including any showdown. Public by the rules of the
	// game: cards turned face up at a showdown are turned face up for everyone.
	//
	// Scoped to the showdown, where it used to run whenever a LastHand existed
	// at all — which is from the moment a hand ends until the moment the next
	// one does. "p2 showed a flush" sat under a board p2's flush had nothing
	// to do with for the whole of the following hand.
	if open && s.LastHand != nil {
		for _, p := range s.LastHand.Pots {
			// A pot nobody contested has no hand to name — the winner never
			// showed one. Its own key rather than the same key with an empty
			// "hand" in it, because a client renders a key into a sentence and
			// a sentence with a hole in it reads as a bug.
			//
			// Both written out as literals in the LabelKey position, rather
			// than picked into a variable first. `module.CollectKeys` reads
			// source and does not run it, so the variable version put neither
			// of these into serverKeys.json — the two sentences a player reads
			// at the end of every hand were outside the locale lock the whole
			// time they have existed.
			if p.LabelKey == "" {
				vm.Status = append(vm.Status, module.Fact{
					LabelKey: "holdem.status.potUncontested",
					Value:    strconv.Itoa(p.Amount),
					Params: map[string]any{
						"winners": p.Winners, "hand": p.LabelKey, "amount": p.Amount,
					},
				})
			} else if len(p.Cards) == 0 {
				// A split pot: the winners tie on rank but not necessarily
				// on suits, so there is no one set of five to name. Each
				// winner's own cards are on their "showed" line.
				vm.Status = append(vm.Status, module.Fact{
					LabelKey: "holdem.status.potSplit",
					Value:    strconv.Itoa(p.Amount),
					Params: map[string]any{
						"winners": p.Winners, "hand": p.LabelKey, "amount": p.Amount,
					},
				})
			} else {
				vm.Status = append(vm.Status, module.Fact{
					LabelKey: "holdem.status.pot",
					Value:    strconv.Itoa(p.Amount),
					Params: map[string]any{
						"winners": p.Winners, "hand": p.LabelKey, "amount": p.Amount,
						"cards": p.Cards,
					},
				})
			}
		}
		for _, sh := range s.LastHand.Shown {
			// A hand with no name is one shown for the look of it — turned
			// over after everyone folded, against a board that may not even
			// be complete. It gets a sentence that names no hand, rather than
			// the showdown sentence with a hole where the hand goes.
			//
			// Both params maps written out in full rather than shared through
			// a local, for the same reason both keys are literals: the scan
			// that builds the manifest reads which placeholders a key carries
			// off the map literal sitting beside it, and a hoisted one leaves
			// the key looking like it sends nothing.
			if sh.LabelKey == "" {
				vm.Status = append(vm.Status, module.Fact{
					LabelKey: "holdem.status.shownVoluntary",
					Params: map[string]any{
						"playerId": sh.PlayerID, "hole": sh.Hole, "best": sh.Best,
					},
				})
			} else {
				vm.Status = append(vm.Status, module.Fact{
					LabelKey: "holdem.status.shown", Value: sh.LabelKey,
					Params: map[string]any{
						"playerId": sh.PlayerID, "hole": sh.Hole, "best": sh.Best,
					},
				})
			}
		}
		for _, id := range s.LastHand.Mucked {
			vm.Status = append(vm.Status, module.Fact{
				LabelKey: "holdem.status.mucked",
				Params:   map[string]any{"playerId": id},
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

// showdownOpen reports the table looking at a finished hand rather than
// playing one.
//
// Two states, not one. The usual case is the stop between hands. The other is
// a match that has ended: the hand that ended it never opens a stop — there is
// nothing to go on to — and its board, its hole cards and its result all stay
// exactly where they are, because `dealHand` is the only thing that clears
// them and it is never reached. Without the second case the deciding hand of
// every match would be the one hand nobody was shown.
func showdownOpen(s *GameState) bool {
	return s.Break.Open || s.Status == "completed"
}

// took reports this player having been pushed chips from the hand just played.
func took(res *HandResult, playerID string) bool {
	if res == nil {
		return false
	}
	for _, p := range res.Pots {
		for _, w := range p.Winners {
			if w == playerID {
				return true
			}
		}
	}
	return false
}

// mucked reports this player having reached the showdown without showing.
func mucked(res *HandResult, playerID string) bool {
	if res == nil {
		return false
	}
	for _, id := range res.Mucked {
		if id == playerID {
			return true
		}
	}
	return false
}

func cardViews(cards []string) []module.CardView {
	out := make([]module.CardView, 0, len(cards))
	for _, c := range cards {
		out = append(out, module.CardView{Card: c})
	}
	return out
}

// Bot lives in bot.go: poker is the one module here whose bot cannot be a
// preference over the offer list, because the offer list never mentions the
// two cards the decision turns on.

// Standings ranks the table by chips, which is the only measure poker has.
func (m *Module) Standings(raw module.State) ([]module.Standing, error) {
	s, err := decode(raw)
	if err != nil {
		return nil, err
	}
	order := make([]string, 0, len(s.Seats))
	stacks := map[string]int{}
	for i := range s.Seats {
		order = append(order, s.Seats[i].PlayerID)
		stacks[s.Seats[i].PlayerID] = s.Seats[i].Stack
	}
	return module.RankByScore(order, func(id string) int { return stacks[id] }, "holdem.unit.chips"), nil
}
