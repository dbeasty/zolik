// Package lora implements Lóra, the Kladno penalty-point game for four, as a
// game module.
//
// It is Lóra as the Czech tournament rules write it down (the unified rules
// of the national championship, lora.game and lora.cz): a tálie of seven
// games dealt one after another with the same player leading — five trick
// games (červené, filky, první–poslední, všechny, červený král) and two
// laying-out games (kvarty, desítky) — then the leader's maturita, where they
// choose a game and must come out of it with no penalty at all. A match is
// four tálie, so everyone leads one and takes the maturita once; the fewest
// penalty points wins.
//
// docs/lora-rules.md lists where the app pins down what the written rules
// leave open, and what it leaves out (renonc, which the app cannot commit;
// čistá stovka; harakiri).
//
// Cards use the server's usual codes, the German suits mapped as in Mariáš:
// H červené, D kule, C žaludy, S zelené; J spodek, Q svršek (filek).
package lora

import (
	"strings"

	"zolik/server/internal/tricks"
)

// Module is Lóra.
type Module struct{}

// New returns the module.
func New() *Module { return &Module{} }

const (
	seats    = 4
	handSize = 8
	numTrick = 8 // tricks in a trick game: every card is dealt

	redKing = "KH" // bedrník
	red     = 'H'

	maturitaPenalty = 8
	maxAttempts     = 3
)

// order ranks cards within a suit for taking tricks: no trumps, the highest
// card of the suit led takes the trick.
const order = tricks.Order("AKQJT987")

// ascending is the ranks low to high, the order rows are laid in.
const ascending = "789TJQKA"

func rankIndex(card string) int { return strings.IndexByte(ascending, tricks.Rank(card)) }

func cardAt(i int, suit byte) string { return string(ascending[i]) + string(suit) }

// suits in the order the board lays the rows out.
const suits = "HDCS"

func buildDeck() []string {
	var out []string
	for _, s := range suits {
		for i := range ascending {
			out = append(out, cardAt(i, byte(s)))
		}
	}
	return out
}

// The seven games of a tálie, in the order they are dealt.
const (
	gameCervene = "cervene" // red cards, one each
	gameFilky   = "filky"   // svršci, two each
	gamePrPo    = "prpo"    // the first and the last trick, four each
	gameVsechny = "vsechny" // every trick, one each
	gameKral    = "kral"    // the red king, eight
	gameKvarty  = "kvarty"  // quarts: one a card left in hand
	gameDesitky = "desitky" // tens: one a card left, one a ťuk
)

var sequence = []string{gameCervene, gameFilky, gamePrPo, gameVsechny, gameKral, gameKvarty, gameDesitky}

func trickGame(g string) bool {
	switch g {
	case gameKvarty, gameDesitky:
		return false
	}
	return true
}

// redLeadRule is whether the leader may lead a red card only when they hold
// nothing else.
func redLeadRule(g string) bool { return g == gameCervene || g == gameKral }

// trickPenalty is what taking this trick costs in game g: the trick's
// cards, and whether it is the first or the last.
func trickPenalty(g string, trick []tricks.Play, number int) int {
	n := 0
	switch g {
	case gameCervene:
		for _, pl := range trick {
			if tricks.Suit(pl.Card) == red {
				n++
			}
		}
	case gameFilky:
		for _, pl := range trick {
			if tricks.Rank(pl.Card) == 'Q' {
				n += 2
			}
		}
	case gamePrPo:
		if number == 0 || number == numTrick-1 {
			n = 4
		}
	case gameVsechny:
		n = 1
	case gameKral:
		for _, pl := range trick {
			if pl.Card == redKing {
				n = 8
			}
		}
	}
	return n
}

// Option names, as the lobby sends them.
const (
	OptTalie    = "talie"
	OptMaturita = "maturita"
)

const defaultTalie = 4
