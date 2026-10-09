// Package ferbl implements Ferbl, the Czech pub vying game of four cards
// and suits, as a game module.
//
// The hands are Pagat's Ferbli (pagat.com/vying/ferbli.html), the Hungarian
// game Czech Ferbl comes from: 32 German-suited cards, eso 11, král, svršek,
// spodek and ten 10, the rest their number; four cards each, dealt two and
// two; and the hands, best first: four of a kind, banda (four of a suit,
// 41 down to 34), three of a kind, three of a suit, two aces, two of a suit,
// and a card of each suit. Cards outside the hand's own combination count
// for nothing.
//
// The betting is a plain limit game rather than Ferbli's charging and
// counter-charging, which no Czech source describes the same way twice:
// everyone antes the vizi, there is a round of betting on two cards and
// another on four, a bet is one vizi then two, and three raises close a
// round. docs/ferbl-rules.md lists every choice.
package ferbl

import (
	"sort"

	"zolik/server/internal/tricks"
)

// Module is Ferbl.
type Module struct{}

// New returns the module.
func New() *Module { return &Module{} }

const (
	handSize  = 4
	maxRaises = 3
	deckRanks = "AKQJT987"
)

// value is what a card counts in a suit total.
func value(card string) int {
	switch tricks.Rank(card) {
	case 'A':
		return 11
	case 'K', 'Q', 'J', 'T':
		return 10
	}
	return int(tricks.Rank(card) - '0')
}

// rankOrder ranks cards of a kind: eso highest, then král, svršek, spodek,
// ten, nine, eight, seven.
func rankOrder(card string) int {
	switch tricks.Rank(card) {
	case 'A':
		return 7
	case 'K':
		return 6
	case 'Q':
		return 5
	case 'J':
		return 4
	case 'T':
		return 3
	}
	return int(tricks.Rank(card)-'0') - 7
}

// Hand categories, lowest first.
const (
	catSuits       = 1 // a card of each suit: the highest card
	catTwoSuited   = 2 // two of a suit: their total
	catTwoAces     = 3
	catThreeSuited = 4 // three of a suit: their total
	catThreeKind   = 5
	catBanda       = 6 // four of a suit: their total
	catFourKind    = 7
)

// score ranks a four-card hand: higher is better, equal is a tie.
func score(hand []string) int {
	byRank := map[byte][]string{}
	bySuit := map[byte][]string{}
	for _, c := range hand {
		byRank[tricks.Rank(c)] = append(byRank[tricks.Rank(c)], c)
		bySuit[tricks.Suit(c)] = append(bySuit[tricks.Suit(c)], c)
	}
	best := 0
	consider := func(cat, v int) { best = max(best, cat*100+v) }
	for _, cards := range byRank {
		switch len(cards) {
		case 4:
			consider(catFourKind, rankOrder(cards[0]))
		case 3:
			consider(catThreeKind, rankOrder(cards[0]))
		case 2:
			if tricks.Rank(cards[0]) == 'A' {
				consider(catTwoAces, 0)
			}
		}
	}
	for _, cards := range bySuit {
		sum := 0
		for _, c := range cards {
			sum += value(c)
		}
		switch len(cards) {
		case 4:
			consider(catBanda, sum)
		case 3:
			consider(catThreeSuited, sum)
		case 2:
			consider(catTwoSuited, sum)
		case 1:
			if len(bySuit) == 4 {
				consider(catSuits, value(cards[0]))
			}
		}
	}
	if best == 0 {
		// Fewer than four cards (a hand still being dealt): its best card.
		for _, c := range hand {
			consider(0, value(c))
		}
	}
	return best
}

// category is the hand's kind, for naming it.
func category(s int) int { return s / 100 }

func buildDeck() []string {
	out := make([]string, 0, 32)
	for _, s := range "HDCS" {
		for _, r := range deckRanks {
			out = append(out, string(r)+string(s))
		}
	}
	return out
}

func sortHand(hand []string) {
	sort.SliceStable(hand, func(i, j int) bool {
		a, b := hand[i], hand[j]
		if tricks.Suit(a) != tricks.Suit(b) {
			return tricks.Suit(a) < tricks.Suit(b)
		}
		return rankOrder(a) > rankOrder(b)
	})
}

// Option names, as the lobby sends them.
const (
	OptStartingStack = "startingStack"
	OptVizi          = "vizi"
	OptHands         = "hands"
)
