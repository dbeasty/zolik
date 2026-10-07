package holdem

// The poker games this module deals, and what tells them apart.
//
// One engine plays them all: blinds, betting rounds, side pots and the
// showdown are the same arithmetic whether the cards are two in the hand and
// five on the felt or five in the hand and none. What differs is written down
// here once, and every rule that differs reads it from here — never from the
// variation ID, so a variation added later is a row in this table rather than
// a hunt through the engine for every `== "draw"`.

// VarDraw is Five-Card Draw, and VarOmaha Pot-Limit Omaha.
const (
	VarDraw  = "draw"
	VarOmaha = "omaha"
)

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
	// useHole is how many of a seat's own cards its hand must be made with,
	// exactly; zero is any number. Omaha's whole character is this rule —
	// exactly two of the four, exactly three of the board — and it is why a
	// board showing four hearts is not a flush for a hand holding one.
	useHole int
	// potLimit caps a raise at the size of the pot after calling, where the
	// other games let it go to the whole stack.
	potLimit bool
	// maxSeats is the most players the game can deal to. Draw stops at six:
	// thirty cards dealt and at most eighteen drawn is the most a 52-card deck
	// covers without reshuffling the discards, and a reshuffle is a rule this
	// module would rather not need.
	maxSeats int
}

var (
	holdemRules = ruleset{holeCards: 2, board: true, maxSeats: 9}
	drawRules   = ruleset{holeCards: 5, draw: true, maxDiscard: 3, maxSeats: 6}
	// Nine seats hold 36 cards and the board five more: 41 of 52.
	omahaRules = ruleset{holeCards: 4, board: true, useHole: 2, potLimit: true, maxSeats: 9}
)

// holdem reports the game the trained network and the Hold'em heuristics —
// Chen scores, push-fold charts, two-card board reads — were built for.
func (r ruleset) holdem() bool { return r == holdemRules }

// rulesFor is the game a variation deals. Anything that is neither Draw nor
// Omaha is Hold'em, including the two retired variations and a state stored
// before variations meant anything but a hand limit.
func rulesFor(variation string) ruleset {
	switch variation {
	case VarDraw:
		return drawRules
	case VarOmaha:
		return omahaRules
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

// bestOf is the best hand a seat holds, by the game's own rule for which of
// its cards may make it.
func (s *GameState) bestOf(st *Seat) HandRank {
	if n := s.rules().useHole; n > 0 {
		return BestUsing(st.Hole, s.Board, n)
	}
	return Best(s.handOf(st))
}

// canName reports whether a seat's cards make a five-card hand yet: enough of
// them, and in Omaha a flop to take three from.
func (s *GameState) canName(st *Seat) bool {
	if n := s.rules().useHole; n > 0 {
		return len(st.Hole) >= n && len(s.Board) >= 5-n
	}
	return len(st.Hole)+len(s.Board) >= 5
}

// BestUsing is the best five-card hand made from exactly n of hole and 5-n of
// board — Omaha's rule, n = 2. Every combination is tried: six pairs from four
// cards against ten triples from five is sixty hands, once per showdown.
func BestUsing(hole, board []string, n int) HandRank {
	var best HandRank
	first := true
	for _, h := range combos(hole, n) {
		for _, b := range combos(board, 5-n) {
			r := evaluateFive(append(append(make([]string, 0, 5), h...), b...))
			if first || r.Compare(best) > 0 {
				best, first = r, false
			}
		}
	}
	if first {
		return HandRank{Category: highCard}
	}
	return best
}

// combos is every k-card subset of cards, in order.
func combos(cards []string, k int) [][]string {
	if k == 0 {
		return [][]string{nil}
	}
	var out [][]string
	var walk func(start int, picked []string)
	walk = func(start int, picked []string) {
		if len(picked) == k {
			out = append(out, append([]string(nil), picked...))
			return
		}
		for i := start; i <= len(cards)-(k-len(picked)); i++ {
			walk(i+1, append(picked, cards[i]))
		}
	}
	walk(0, nil)
	return out
}

// raiseCap is the most a seat may raise to: its whole stack, or under pot
// limit the current bet plus the pot as it would stand once this seat had
// called — every chip on the table, and the call itself.
//
// Preflop at 10/20, first to act: 20 to call, 30 in the blinds, so a pot-sized
// raise is to 20 + 30 + 20 = 70, which is the figure every Omaha table uses.
func (s *GameState) raiseCap(seat *Seat) int {
	stack := seat.Bet + seat.Stack
	if !s.rules().potLimit {
		return stack
	}
	if pot := s.potLimitTo(seat); pot < stack {
		return pot
	}
	return stack
}

// potLimitTo is a pot-sized raise for this seat, whatever its stack.
func (s *GameState) potLimitTo(seat *Seat) int {
	call := s.CurrentBet - seat.Bet
	if call < 0 {
		call = 0
	}
	return s.CurrentBet + potNow(s) + call
}
