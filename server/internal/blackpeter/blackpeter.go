// Package blackpeter implements Black Peter (Černý Petr, Schwarzer Peter), the
// children's pairing game, as a module.
//
// The rules are the common ground of the sources docs/blackpeter-rules.md
// lists, played with an ordinary 52-card pack: the jack of clubs comes out,
// which leaves the jack of spades with no partner, and that card is Black
// Peter. Every card is dealt. Pairs go down face up as soon as a player holds
// them. In turn, each player takes one card, unseen, from the player on their
// left who still holds cards, and puts down the pair it makes, if it makes
// one. A player with no cards left is out of the round. The last player
// holding a card holds Black Peter alone and takes a mark; after the agreed
// number of rounds the fewest marks win.
//
// There is no strategy in which card to take — the cards are face down — so
// the game is the same for every bot strength, and the module says so rather
// than pretending otherwise (bot.go).
package blackpeter

import (
	"zolik/server/internal/tricks"
)

// Module is Black Peter.
type Module struct{}

// New returns the module.
func New() *Module { return &Module{} }

// peter is Black Peter: the jack of spades, once the jack of clubs is out.
const (
	peter   = "JS"
	removed = "JC"
)

// buildDeck is the 51-card pack: a full French pack less the jack of clubs.
func buildDeck() []string {
	out := make([]string, 0, 51)
	for _, s := range "SHDC" {
		for _, r := range "A23456789TJQK" {
			c := string(r) + string(s)
			if c != removed {
				out = append(out, c)
			}
		}
	}
	return out
}

func red(card string) bool { s := tricks.Suit(card); return s == 'H' || s == 'D' }

// pairs reports whether two cards make a pair: the same rank, and the same
// colour too when the table plays that way. Black Peter pairs with nothing.
func pairs(a, b string, byColour bool) bool {
	if a == peter || b == peter || a == b {
		return false
	}
	if tricks.Rank(a) != tricks.Rank(b) {
		return false
	}
	return !byColour || red(a) == red(b)
}

// takePairs removes every pair from hand, keeping the rest in order, and
// returns the pairs found, two cards each. Each card pairs at most once.
func takePairs(hand []string, byColour bool) (rest []string, found [][]string) {
	used := make([]bool, len(hand))
	for i := range hand {
		if used[i] {
			continue
		}
		for j := i + 1; j < len(hand); j++ {
			if !used[j] && pairs(hand[i], hand[j], byColour) {
				used[i], used[j] = true, true
				found = append(found, []string{hand[i], hand[j]})
				break
			}
		}
	}
	for i, c := range hand {
		if !used[i] {
			rest = append(rest, c)
		}
	}
	return rest, found
}

// Option names, as the lobby sends them.
const (
	OptPairs  = "pairs"
	OptRounds = "rounds"
)

// Values of OptPairs.
const (
	pairsByRank   = 0
	pairsByColour = 1
)
