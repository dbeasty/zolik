// Package tricks is the part of a trick-taking game that does not depend on
// which game it is: what was led, who is winning, and which cards a hand may
// play next.
//
// It knows nothing about modules, seats' names, contracts or scoring. A game
// hands it a rank order, a trump suit (or none) and a Policy, and gets back
// answers. Mariáš is its first user; the rules that differ between trick
// games — above all whether a follower must beat the card on the table — are
// the Policy, so a later Whist or Bridge is a different Policy, not a fork.
//
// Cards are the server's usual two-character codes, rank then suit ("TH" is
// the ten of hearts). Mariáš's German suits use the same codes (see
// docs/marias-plan.md §1); only their faces differ.
package tricks

import "strings"

// Order ranks cards within a suit, highest first, one character per rank:
// "ATKQJ987" is Mariáš with trumps, "AKQJT987" is Mariáš's Betl and Durch.
// A rank missing from the order sorts below every rank in it.
type Order string

// Beats reports whether rank a is higher than rank b in this order.
func (o Order) Beats(a, b byte) bool {
	return o.index(a) < o.index(b)
}

func (o Order) index(r byte) int {
	if i := strings.IndexByte(string(o), r); i >= 0 {
		return i
	}
	return len(o)
}

// Policy is what a follower owes the trick.
type Policy int

const (
	// FollowOnly: follow the led suit if you can, otherwise play anything.
	// Whist and Bridge.
	FollowOnly Policy = iota
	// FollowBeatTrump is Mariáš's "přebíjet": follow the led suit and beat
	// the best card of it on the table if you can; if you cannot follow,
	// play a trump, and beat the best trump on the table if you can; only
	// with neither the led suit nor a trump may you play anything.
	//
	// Beating is owed only while it could win the trick: once the led suit
	// has been trumped, a follower who follows suit may play any card of it.
	FollowBeatTrump
)

// Rank and Suit split a card code.
func Rank(card string) byte { return card[0] }
func Suit(card string) byte { return card[1] }

// NoTrump is the trump argument for a game played without one.
const NoTrump byte = 0

// Play is one card on the table and the seat that played it.
type Play struct {
	Seat int    `json:"seat"`
	Card string `json:"card"`
}

// Trick is the cards played to one trick so far, in the order they were
// played. The first play is the lead.
type Trick struct {
	Plays []Play `json:"plays"`
}

// Led is the suit of the lead, or 0 while nothing has been played.
func (t Trick) Led() byte {
	if len(t.Plays) == 0 {
		return 0
	}
	return Suit(t.Plays[0].Card)
}

// Winning is the play currently taking the trick: the best trump if any
// trump was played, otherwise the best card of the led suit. Discards of
// other suits never win. ok is false for an empty trick.
func (t Trick) Winning(trump byte, o Order) (best Play, ok bool) {
	if len(t.Plays) == 0 {
		return Play{}, false
	}
	best = t.Plays[0]
	for _, p := range t.Plays[1:] {
		if beats(p.Card, best.Card, trump, o) {
			best = p
		}
	}
	return best, true
}

// beats reports whether card c, played after card best, takes the trick from
// it. best is always the led suit or a trump, since it started as the lead
// and only ever gets replaced by a card that beats it.
func beats(c, best string, trump byte, o Order) bool {
	cs, bs := Suit(c), Suit(best)
	switch {
	case cs == bs:
		return o.Beats(Rank(c), Rank(best))
	case trump != NoTrump && cs == trump:
		return true // a trump over a card of the led suit
	default:
		return false
	}
}

// Legal is the cards from hand that may be played to t next, in the hand's
// own order. On the lead that is the whole hand.
func Legal(hand []string, t Trick, trump byte, o Order, p Policy) []string {
	if len(t.Plays) == 0 {
		return append([]string(nil), hand...)
	}
	led := t.Led()
	if follow := ofSuit(hand, led); len(follow) > 0 {
		if p == FollowOnly {
			return follow
		}
		best, _ := t.Winning(trump, o)
		if Suit(best.Card) != led {
			return follow // trumped already: no card of the led suit can win
		}
		return beatingOrAll(follow, best.Card, o)
	}
	if p == FollowOnly || trump == NoTrump {
		return append([]string(nil), hand...)
	}
	trumps := ofSuit(hand, trump)
	if len(trumps) == 0 {
		return append([]string(nil), hand...)
	}
	best, _ := t.Winning(trump, o)
	if Suit(best.Card) != trump {
		return trumps // any trump beats the led suit
	}
	return beatingOrAll(trumps, best.Card, o)
}

// beatingOrAll is the cards that outrank best, or all of cards when none do.
// Every card in cards shares best's suit.
func beatingOrAll(cards []string, best string, o Order) []string {
	var higher []string
	for _, c := range cards {
		if o.Beats(Rank(c), Rank(best)) {
			higher = append(higher, c)
		}
	}
	if len(higher) == 0 {
		return cards
	}
	return higher
}

func ofSuit(hand []string, suit byte) []string {
	var out []string
	for _, c := range hand {
		if Suit(c) == suit {
			out = append(out, c)
		}
	}
	return out
}
