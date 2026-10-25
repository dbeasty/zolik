// Package sietemedio implements Siete y Medio — seven and a half, the Spanish
// game played round a Mexican table — as a game module.
//
// It is blackjack's ancestor and not a dialect of it. The target is 7½ on a
// forty-card Spanish pack, and the bank is a seat rather than the house: one
// of the players deals, covers everybody's stake out of their own stack, plays
// last, and can lose the bank to whoever makes exactly 7½.
//
// That last shape is not new here — internal/okobere, the Czech pub
// twenty-one, already banks round the table, and anyone reading this should
// read that one too. What this game does not share with it is the pack (forty
// Spanish cards against thirty-two German ones), the arithmetic (figures worth
// half a point, which is why nothing in this package holds a whole-number
// total), the information model (a face-down first card per seat, rather than
// a bank that turns over at the end) and how the bank moves (to whoever makes
// exactly 7½, rather than on a round count). A separate module rather than a
// variation of Oko bere, for the reason Prší and Sedma are separate: the two
// are cousins, not the same game under two names.
//
// docs/sietemedio-rules.md has the rules, the sources, and which of them
// disagree (ties, the 7½ bonus, how the bank moves — all three are table
// options here rather than constants, which is this codebase's standing rule
// for a house rule).
package sietemedio

import "math/rand"

// Totals are held in halves, and this is 7½.
//
// The three figures are worth half a point each, and half-points in integer
// arithmetic is the one rounding error that would silently mis-settle a round
// — a player on 7½ reading as 7, which is a loss. So nothing in this package
// ever holds a total as a whole number: the unit is the half, the target is
// fifteen of them, and halvesText is the only place the ½ is written out.
const Target = 15

// The forty-card Spanish pack, in the French codes this server uses
// everywhere.
//
// The suits line up with the pack the Spanish suits became — D→oros, H→copas,
// S→espadas, C→bastos — and the figures with the ones the French pack kept a
// slot for: J→sota, Q→caballo, K→rey. So "KD" is the rey de oros and nothing
// on the wire had to learn a new notation; only how a client draws and names
// them changes, which is what module.DeckSpanish says.
//
// The eights, nines and tens are simply not in the pack. That is not a
// simplification of a 52-card deck: it is the pack, and it is why a Mexican
// table holding only a French deck plays by throwing those three ranks out.
var (
	ranks = []string{"A", "2", "3", "4", "5", "6", "7", "J", "Q", "K"}
	suits = []string{"H", "C", "D", "S"}
)

// buildDeck is one shuffled forty-card pack.
func buildDeck(seed int64) []string {
	out := make([]string, 0, len(ranks)*len(suits))
	for _, s := range suits {
		for _, r := range ranks {
			out = append(out, r+s)
		}
	}
	r := rand.New(rand.NewSource(seed))
	r.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	return out
}

func rankOf(card string) string {
	if card == "" {
		return ""
	}
	return card[:len(card)-1]
}

// cardHalves is what one card is worth, in halves: the ace one, the numbers
// their number, and each figure a half.
//
// There is no high-low choice anywhere in this game — the as is one and only
// one. Several write-ups say it may count as eleven, which is blackjack
// leaking back into the game it came from, and eleven is not a reachable
// total on a 7½ table in any case.
func cardHalves(card string) int {
	switch rankOf(card) {
	case "J", "Q", "K":
		return 1
	case "A":
		return 2
	case "":
		return 0
	default:
		return int(card[0]-'0') * 2
	}
}

// Total is what a hand is worth, in halves.
func Total(cards []string) int {
	out := 0
	for _, c := range cards {
		out += cardHalves(c)
	}
	return out
}

// IsBust reports a hand gone over 7½ — se pasó.
func IsBust(cards []string) bool { return Total(cards) > Target }

// IsSevenAndAHalf reports exactly 7½, the hand the whole game is named after.
func IsSevenAndAHalf(cards []string) bool { return Total(cards) == Target }

// halvesText writes a total the way a player says it: "7½", "6", "½".
//
// The one place the half is spelled, and a string rather than a number because
// that is what it is — a client is handed the finished figure in Fact.Value
// and never does this arithmetic, which is the whole point of the module
// owning the rules.
func halvesText(h int) string {
	whole := h / 2
	switch {
	case h%2 == 0:
		return itoa(whole)
	case whole == 0:
		return "½"
	default:
		return itoa(whole) + "½"
	}
}

// itoa, rather than strconv in a file with no other need of it.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// Option names. Every one is a house rule the sources disagree about rather
// than a law of the game — see the table in docs/sietemedio-rules.md.
const (
	OptStartingStack = "startingStack"
	OptMinStake      = "minStake"
	OptRounds        = "rounds"
	OptTies          = "ties"
	OptSevenPays     = "sevenAndAHalfPays"
	OptBank          = "bank"
	OptFirstCardDown = "firstCardDown"
)

// Who takes a tie. The sources split almost evenly, which is exactly why this
// is a setting and not a line in the engine.
const (
	tiesToBank = 0
	tiesPush   = 1
)

// What a 7½ collects: twice the stake, or the stake like any other win.
const (
	sevenPaysDouble = 1
	sevenPaysEven   = 0
)

// How the bank moves. "On 7½" is the classic rule — the player who makes it
// takes the bank; "round" passes the deal along every round; "fixed" leaves it
// where it started, and moves it only when the banker can no longer cover.
const (
	bankOnSeven = 0
	bankRound   = 1
	bankFixed   = 2
)

// Variation IDs.
const (
	VarMexicana = "mexicana"
	VarCantina  = "cantina"
	VarAbierta  = "abierta"
)

type variationDefaults struct {
	minStake      int
	startingStack int
	rounds        int
	ties          int
	sevenPays     int
	bank          int
	firstCardDown bool
}

// The three shipped tables are games somebody deals, not sliders at arbitrary
// positions.
var variations = map[string]variationDefaults{
	// The standard game: opening card down, ties to the bank, 7½ pays double
	// and takes the bank.
	VarMexicana: {
		minStake: 10, startingStack: 500, rounds: 12,
		ties: tiesToBank, sevenPays: sevenPaysDouble, bank: bankOnSeven, firstCardDown: true,
	},
	// A friendly table: the deal goes round every round and a tie is a push,
	// so the bank's edge is shared out rather than owned.
	VarCantina: {
		minStake: 10, startingStack: 500, rounds: 12,
		ties: tiesPush, sevenPays: sevenPaysDouble, bank: bankRound, firstCardDown: true,
	},
	// Everything face up: no hidden information at all, which makes it the
	// table to learn the game at.
	VarAbierta: {
		minStake: 10, startingStack: 500, rounds: 12,
		ties: tiesToBank, sevenPays: sevenPaysDouble, bank: bankRound, firstCardDown: false,
	},
}
