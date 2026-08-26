package module

// Bots: how a seat nobody is sitting at gets played.
//
// This is Phase 6 of docs/one-architecture-plan.md, and it is the change that
// makes the module seam pay for itself. Before it, only Žolíky had opponents,
// because its AI was written into the Žolíky runtime; a new module got a lobby,
// a socket and a seat that never moved. Bots are a *runtime* concern — the
// runtime knows when a seat is a bot's and when it is that seat's turn — so it
// drives them, and a module only has to say how it wants to be played.
//
// A module says that in one of two ways, and most say it in one line.

// Bot chooses a move for a seat.
//
// It is handed the offer list rather than being expected to fetch one, so the
// simplest possible bot is a pure function of what a client would see anyway.
type Bot interface {
	Act(s State, playerID string, offers []ActionOffer) (Action, bool)
}

// Botted is implemented by a module that supplies its own bot.
//
// Optional. A module that does not implement it gets OfferBot with no
// preferences, which plays legally but without taste — it takes whichever
// offer happens to come first.
type Botted interface {
	Bot() Bot
}

// OfferBot is a bot that reads the offer list and nothing else.
//
// It is the same policy the conformance driver uses to falsify the
// abstraction, which is the point: if a game can be played from its offers,
// then it already has an opponent and nobody has to write one. A module tunes
// it by naming the verbs it would rather play first — a taste, not a rule.
//
// It is a baseline, not a good player. It never bluffs, never holds a card
// back and never counts anything. A module that wants better implements Botted
// and returns something that does; zolikmod does exactly that, so Žolíky's
// opponents stay as strong as they were before this existed.
func OfferBot(prefer ...string) Bot { return offerBot{prefer: prefer} }

type offerBot struct{ prefer []string }

func (b offerBot) Act(_ State, _ string, offers []ActionOffer) (Action, bool) {
	return ChooseAction(offers, b.prefer)
}

// BotFor returns the bot a module wants, or the default.
func BotFor(m GameModule) Bot {
	if b, ok := m.(Botted); ok {
		if bot := b.Bot(); bot != nil {
			return bot
		}
	}
	return OfferBot()
}

// ActiveSeat reports which player the module is waiting on, from the view
// alone.
//
// The runtime needs this to know whether the seat on turn is a bot's, and it
// cannot read game state to find out. It used to work the answer out by asking
// every player for their offers and seeing who had an enabled one — which
// worked, and was a strange way to learn something the module knew. Seat.Active
// is that answer, pushed.
//
// Falls back to the offer scan for a module that emits no seats, so this is an
// improvement rather than a new requirement.
func ActiveSeat(m GameModule, s State, viewer string, players []PlayerRef) string {
	if awaited := AwaitedSeats(m, s, viewer, players); len(awaited) > 0 {
		return awaited[0]
	}
	return ""
}

// AwaitedSeats is every seat the module is waiting on, in the order the board
// lists them.
//
// For almost all of a match there is exactly one, and ActiveSeat is that one.
// Then a match stops between rounds and every seat that has not yet said to go
// on is waited on at once — which is the first time "whose turn is it" has more
// than one answer, and the reason this exists rather than the runtime asking a
// question that quietly returns the first of several.
//
// The fallback for a module that emits no seats is the offer scan, exactly as
// before: a module that has never heard of any of this is unaffected.
func AwaitedSeats(m GameModule, s State, viewer string, players []PlayerRef) []string {
	if vm, err := m.View(s, viewer); err == nil && len(vm.Seats) > 0 {
		var out []string
		for _, seat := range vm.Seats {
			if seat.Active {
				out = append(out, seat.PlayerID)
			}
		}
		// Seats were emitted and none is active: nobody is on turn. That is an
		// answer, so it must not fall through to the scan below and invent one.
		return out
	}
	var out []string
	for _, p := range players {
		offers, err := m.LegalActions(s, p.ID)
		if err != nil {
			continue
		}
		for _, o := range offers {
			if o.Enabled {
				out = append(out, p.ID)
				break
			}
		}
	}
	return out
}
