package module

// Joining a game under way, and dealing a round around somebody who is away.

// LateSeater is a module that can seat somebody at a match already under way
// — a poker cash table taking a new player between hands, a Blackjack table
// taking one at the next round. It returns the match with the seat added, or
// a refusal (a tournament, a full table) the runtime passes on.
//
// The new seat must not be dealt into a hand already in play: it joins at
// the next deal, and says so on the board until then.
type LateSeater interface {
	SeatLate(s State, p PlayerRef) (State, []Event, error)
}

// DealsAround is a round-based module that can deal a hand around a seat:
// leave it out of the next deals — no cards, never on turn, scoring nothing —
// until it is dealt back in. The runtime uses it for a player who is away
// when the table asks for that (OptDealAroundAway): a bot finishes the hand
// in play for them, and the deals after it go on without them.
//
// A module may refuse to deal around so many seats that too few would be
// left to play; it then keeps dealing everybody in.
type DealsAround interface {
	DealAround(s State, playerID string, out bool) (State, error)
}

// OptDealAroundAway is whether a player who is away is dealt out of the next
// deals (on) or has a bot play every deal for them until they are back (off).
const OptDealAroundAway = "dealAroundAway"

// DealAroundOption is the ready-made spec a DealsAround module declares.
func DealAroundOption() OptionSpec {
	return OptionSpec{
		Name:  OptDealAroundAway,
		Type:  OptionEnumInt,
		Label: "While a player is away",
		Help:  "A bot finishes the hand in play for a player whose connection dropped. After that, they can sit out the next deals until they return, or the bot can go on playing for them.",
		Choices: []OptionChoice{
			{Value: OptOn, Label: "Deal them out"},
			{Value: OptOff, Label: "A bot keeps playing"},
		},
	}
}

// DealAroundAway reads the option, against the module's own default.
func (c MatchConfig) DealAroundAway(dflt bool) bool {
	return c.Opt(OptDealAroundAway, BoolOpt(dflt)) == OptOn
}
