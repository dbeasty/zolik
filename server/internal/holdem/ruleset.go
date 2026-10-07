package holdem

// The poker games this module deals, and what tells them apart.
//
// One engine plays them all: blinds, betting rounds, side pots and the
// showdown are the same arithmetic whether the cards are two in the hand and
// five on the felt or five in the hand and none. What differs is written down
// here once, and every rule that differs reads it from here — never from the
// variation ID, so a variation added later is a row in this table rather than
// a hunt through the engine for every `== "draw"`.

// VarDraw is Five-Card Draw.
const VarDraw = "draw"

// ruleset is what a poker game is made of, as far as the engine cares.
type ruleset struct {
	// holeCards is how many cards each seat is dealt.
	holeCards int
	// board is whether there are community cards — the flop, turn and river.
	board bool
	// draw is whether there is a draw between two betting rounds, in which a
	// seat swaps up to maxDiscard of its cards for new ones.
	draw       bool
	maxDiscard int
	// maxSeats is the most players the game can deal to. Draw stops at six:
	// thirty cards dealt and at most eighteen drawn is the most a 52-card deck
	// covers without reshuffling the discards, and a reshuffle is a rule this
	// module would rather not need.
	maxSeats int
}

var (
	holdemRules = ruleset{holeCards: 2, board: true, maxSeats: 9}
	drawRules   = ruleset{holeCards: 5, draw: true, maxDiscard: 3, maxSeats: 6}
)

// rulesFor is the game a variation deals. Everything that is not Draw is
// Hold'em, including the two retired variations and a state stored before
// variations meant anything but a hand limit.
func rulesFor(variation string) ruleset {
	if variation == VarDraw {
		return drawRules
	}
	return holdemRules
}

func (s *GameState) rules() ruleset { return rulesFor(s.Variation) }

// Draw's streets. Hold'em's are in state.go; a hand is played on one set or the
// other, never a mix.
const (
	// streetPredraw is the betting round on the five cards as dealt.
	streetPredraw = "predraw"
	// streetDraw is the draw itself: no betting, each seat in turn from the
	// left of the button discards and is dealt replacements, or stands pat.
	streetDraw = "draw"
	// streetPostdraw is the betting round on the hands as drawn.
	streetPostdraw = "postdraw"
)

// firstStreet is where a hand's betting opens, with the blinds in.
func (r ruleset) firstStreet() string {
	if r.draw {
		return streetPredraw
	}
	return streetPreflop
}

// lastStreet is the betting round after which the hand goes to a showdown.
func (r ruleset) lastStreet() string {
	if r.draw {
		return streetPostdraw
	}
	return streetRiver
}

// opening reports the street the blinds are posted on — the one the rules
// about limping, opening and the big blind's option are about.
func (s *GameState) opening() bool {
	return s.Street == streetPreflop || s.Street == streetPredraw
}

// handOf is the cards a seat makes its hand from: its own, and the board when
// the game has one. Draw has no board, so this is the five in the hand.
func (s *GameState) handOf(st *Seat) []string {
	return append(append(make([]string, 0, len(st.Hole)+len(s.Board)), st.Hole...), s.Board...)
}
