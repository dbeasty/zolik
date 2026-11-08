// Package pinochle implements Pinochle, the American trick-and-meld game
// descended from Bezique, as a game module.
//
// One module, four tables, the way Poker carries its forms:
//
//   - Partnership: four players in two sides, forty-eight cards, an auction,
//     the bidder names trumps and the bidding side must make its bid.
//   - Cutthroat: three players each for themselves, fifteen cards each and a
//     three-card widow the bidder takes up and buries three from.
//   - Two-handed: the older game, with a stock, a turned-up trump and a meld
//     after each trick won while the stock lasts.
//   - Double-deck: four players in two sides on eighty cards (no nines),
//     twenty each, and the larger double melds.
//
// The rules are written out in our own words in docs/pinochle-rules.md,
// which also says where sources disagree and which way the app goes.
//
// Cards use the server's usual codes, rank then suit ("TH" is a ten of
// hearts). The pack holds two (or four) of every card, so codes repeat, as
// they do in Canasta; everything here counts cards rather than naming them.
package pinochle

import "zolik/server/internal/tricks"

// Module is Pinochle.
type Module struct{}

// New returns the module.
func New() *Module { return &Module{} }

// Variation ids.
const (
	VarPartnership = "partnership"
	VarCutthroat   = "cutthroat"
	VarTwoHanded   = "twohanded"
	VarDoubleDeck  = "doubledeck"
)

// Option names, as the lobby sends them.
const (
	OptScoring   = "scoring"
	OptMinBid    = "minBid"
	OptTarget    = "target"
	OptPass      = "passCards"
	OptMeldNeeds = "meldNeeds"
	OptAllPass   = "allPass"
	// OptShowCardPoints shows every side's meld and points taken as the hand
	// is played, the running score; off, each player sees only their own.
	OptShowCardPoints = "showCardPoints"
)

// Scoring choices.
const (
	ScoringCounters = 0 // A, 10, K count 10 each
	ScoringClassic  = 1 // A 11, 10 10, K 4, Q 3, J 2
)

// Meld-needs choices.
const (
	MeldNeedsTrick   = 0
	MeldNeedsCounter = 1
)

// All-pass choices.
const (
	AllPassDealerStuck = 0
	AllPassRedeal      = 1
)

const (
	suits     = "HCDS" // hearts, clubs, diamonds, spades: alternating colours
	bidStep   = 10
	maxBidTop = 4000
)

// table is the shape of one variation: how many play, on what pack, and
// what a hand is worth.
type table struct {
	seats     int
	copies    int    // of every card in the pack
	ranks     string // high to low
	handSize  int
	widow     int // cards set aside for the bidder (Cutthroat)
	partners  bool
	auction   bool
	lastTrick int // the last trick's bonus
	minBid    int // the variation's default
	target    int // the variation's default
}

func tableOf(variation string) table {
	switch variation {
	case VarCutthroat:
		return table{seats: 3, copies: 2, ranks: "ATKQJ9", handSize: 15, widow: 3, auction: true, lastTrick: 10, minBid: 250, target: 1500}
	case VarTwoHanded:
		return table{seats: 2, copies: 2, ranks: "ATKQJ9", handSize: 12, lastTrick: 10, target: 1000}
	case VarDoubleDeck:
		return table{seats: 4, copies: 4, ranks: "ATKQJ", handSize: 20, partners: true, auction: true, lastTrick: 20, minBid: 500, target: 1500}
	}
	return table{seats: 4, copies: 2, ranks: "ATKQJ9", handSize: 12, partners: true, auction: true, lastTrick: 10, minBid: 250, target: 1500}
}

func knownVariation(v string) bool {
	switch v {
	case VarPartnership, VarCutthroat, VarTwoHanded, VarDoubleDeck:
		return true
	}
	return false
}

// order is the rank order inside a suit: the ten between the ace and the
// king, as in every Bezique game.
func (t table) order() tricks.Order { return tricks.Order(t.ranks) }

// buildDeck is every card of the pack, copies included.
func (t table) buildDeck() []string {
	out := make([]string, 0, t.copies*len(t.ranks)*4)
	for c := 0; c < t.copies; c++ {
		for _, s := range suits {
			for _, r := range t.ranks {
				out = append(out, string(r)+string(s))
			}
		}
	}
	return out
}

// cardPoints is what one card is worth in the tricks it is taken in.
func cardPoints(card string, scoring int) int {
	r := tricks.Rank(card)
	if scoring == ScoringClassic {
		switch r {
		case 'A':
			return 11
		case 'T':
			return 10
		case 'K':
			return 4
		case 'Q':
			return 3
		case 'J':
			return 2
		}
		return 0
	}
	switch r {
	case 'A', 'T', 'K':
		return 10
	}
	return 0
}

// isCounter reports whether a card scores in a trick under either system:
// the "meld needs a counter" rule reads this.
func isCounter(card string, scoring int) bool { return cardPoints(card, scoring) > 0 }

// handTotal is every point in the tricks of one hand: 250 on a single pack
// under both systems, 500 on the double.
func (t table) handTotal(scoring int) int {
	n := t.lastTrick
	for _, c := range t.buildDeck() {
		n += cardPoints(c, scoring)
	}
	return n
}
