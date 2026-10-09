// Package okobere implements Oko bere, the Czech pub twenty-one played with
// the Mariáš pack against a bank that goes round the table, as a module.
//
// The rules are the common ground of the Czech sources docs/okobere-rules.md
// lists: the 32-card German pack; 7, 8, 9 and 10 at face value, spodek and
// svršek 1, král 2, eso 11; as close to 21 as you can without going over;
// two aces as the first two cards (oko) win at once; ties go to the bank.
// One player holds the bank, stakes it, and plays last; the others bet
// against it, each no more than the bank still has free. The bank passes to
// the left after a set number of rounds, or as soon as it is broken.
//
// Cards use the server's usual codes, the German suits mapped as in Mariáš.
package okobere

import "zolik/server/internal/tricks"

// Module is Oko bere.
type Module struct{}

// New returns the module.
func New() *Module { return &Module{} }

const (
	goal      = 21
	deckRanks = "AKQJT987"
)

// value is what a card counts.
func value(card string) int {
	switch tricks.Rank(card) {
	case 'A':
		return 11
	case 'K':
		return 2
	case 'Q', 'J':
		return 1
	case 'T':
		return 10
	}
	return int(tricks.Rank(card) - '0')
}

// total is a hand's count.
func total(hand []string) int {
	n := 0
	for _, c := range hand {
		n += value(c)
	}
	return n
}

// oko is two aces as the first two cards.
func oko(hand []string) bool {
	return len(hand) == 2 && tricks.Rank(hand[0]) == 'A' && tricks.Rank(hand[1]) == 'A'
}

func buildDeck() []string {
	out := make([]string, 0, 32)
	for _, s := range "HDCS" {
		for _, r := range deckRanks {
			out = append(out, string(r)+string(s))
		}
	}
	return out
}

// Option names, as the lobby sends them.
const (
	OptStartingStack = "startingStack"
	OptMinBet        = "minBet"
	OptBank          = "bank"
	OptRoundsPerBank = "roundsPerBank"
	OptBankTurns     = "bankTurns"
	OptOkoPays       = "okoPays"
)
