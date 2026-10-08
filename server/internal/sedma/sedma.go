// Package sedma implements Sedma, the Czech and Slovak pub game where a
// seven takes everything, as a game module.
//
// It is Sedma as John McLeod writes it down on pagat.com
// (pagat.com/sedma/sedma.html): four players in two partnerships (two or
// three play alone), four cards each from a stock, no suit to follow, and a
// trick won by the last card of the led rank or the last seven. Only aces and
// tens count, ten each, and the last trick ten more: ninety in a hand.
//
// docs/sedma-rules.md lists where the app differs from the page and from a
// pub table: the deal passes to the left every hand, and the player to the
// dealer's left leads.
//
// Cards use the server's usual codes, the German suits mapped as in Mariáš.
package sedma

import "zolik/server/internal/tricks"

// Module is Sedma.
type Module struct{}

// New returns the module.
func New() *Module { return &Module{} }

const (
	handSize        = 4
	pointsSharp     = 10 // eso, desítka
	pointsLastTrick = 10
	pointsTotal     = 90
)

// cardPoints is a card's worth: ten for an ace or a ten, else nothing.
func cardPoints(card string) int {
	switch tricks.Rank(card) {
	case 'A', 'T':
		return pointsSharp
	}
	return 0
}

// buildDeck is the pack for a table of n: thirty-two cards, or thirty at a
// table of three, where two eights come out (Pagat).
func buildDeck(n int) []string {
	var out []string
	for _, s := range "HDCS" {
		for _, r := range "AKQJT987" {
			c := string(r) + string(s)
			if n == 3 && (c == "8D" || c == "8S") {
				continue
			}
			out = append(out, c)
		}
	}
	return out
}

// captures reports whether card takes a trick led with rank led: the same
// rank, or a seven.
func captures(card string, led byte) bool {
	r := tricks.Rank(card)
	return r == led || r == '7'
}

// Option names, as the lobby sends them.
const (
	OptTarget         = "target"
	OptShowCardPoints = "showCardPoints"
)

const defaultTarget = 5
