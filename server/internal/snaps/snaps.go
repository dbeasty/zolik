// Package snaps implements Šnaps and Šedesát šest, the two-player marriage
// games of Central Europe, as a game module.
//
// Šnaps is Schnapsen as John McLeod writes it down on pagat.com
// (pagat.com/marriage/schnaps.html): twenty German-suited cards, five each,
// the trump spodek exchanged for the turned-up trump. Šedesát šest is his
// two-player Sixty-six (pagat.com/marriage/66.html): the nines go back in,
// six cards each, and the trump nine is the one exchanged. The two share
// everything else, and differ only in the switches of a ruleset.
//
// The simplifications are the ones docs/snaps-rules.md lists and a lobby can
// see: going out is automatic the moment a player may claim 66, so nobody can
// miscount and nobody need count at all; and a marriage is announced by
// leading one of its cards while holding the other.
//
// Cards use the server's usual codes, the German suits mapped as in Mariáš:
// H červené, D kule, C žaludy, S zelené; J is the spodek, Q the svršek.
package snaps

import "zolik/server/internal/tricks"

// Module is Šnaps.
type Module struct{}

// New returns the module.
func New() *Module { return &Module{} }

// The rank order within a suit, highest first. A nine, where there is one,
// ranks under the spodek.
const order tricks.Order = "ATKQJ9"

// Card points, and what a deal is played for.
const (
	goal      = 66 // card points that win a deal
	schneider = 33 // under this, the loser pays double

	marriagePlain = 20
	marriageTrump = 40

	lastTrickPoints = 10 // Šedesát šest, when the talon ran out
)

func cardPoints(card string) int {
	switch tricks.Rank(card) {
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

// Variations.
const (
	variationSnaps = "snaps"
	variation66    = "sedesatSest"
)

// Option names, as the lobby sends them.
const (
	OptTarget         = "target"
	OptShowCardPoints = "showCardPoints"
)

const defaultTarget = 7

// ruleset is everything that tells the two games apart.
type ruleset struct {
	ranks    string // the ranks dealt, in every suit
	handSize int
	// exchange is the trump rank swapped for the turned-up card.
	exchange byte
	// exchangeAfterTrick: only a player who has taken a trick may exchange.
	exchangeAfterTrick bool
	// strictMarriages: no marriage once the talon is closed or used up.
	strictMarriages bool
	// lastTrickBonus: card points for the last trick when the talon ran out.
	lastTrickBonus int
	// lastTrickWins: when the cards run out with nobody at 66, the last
	// trick wins the deal (Šnaps) rather than the higher total (Sixty-six).
	lastTrickWins bool
	// scoreAtClosing: a closer who goes out is paid on the other's tricks
	// and points as they were when the talon was closed (Šnaps), not as
	// they are at the end (Sixty-six).
	scoreAtClosing bool
	// lateOutPays3: when the closer's opponent goes out, they score 3 if
	// they had no trick when the talon was closed (Šnaps); otherwise 2.
	lateOutPays3 bool
}

func rulesFor(variation string) ruleset {
	if variation == variation66 {
		return ruleset{
			ranks: "ATKQJ9", handSize: 6, exchange: '9',
			exchangeAfterTrick: true, strictMarriages: true,
			lastTrickBonus: lastTrickPoints,
		}
	}
	return ruleset{
		ranks: "ATKQJ", handSize: 5, exchange: 'J',
		lastTrickWins: true, scoreAtClosing: true, lateOutPays3: true,
	}
}

// config is a match config resolved against the variation's defaults.
type config struct {
	variation      string
	target         int
	showCardPoints bool
}
