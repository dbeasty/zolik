package blackjack

import "math/rand"

// Card notation matches the rest of the server: rank letter plus suit letter
// ("AS", "TD", "2C"). No jokers, and — unlike every other module here — no
// wild card and no rank order either. Blackjack cares about one thing a card
// can be worth, which is why this file is so much smaller than holdem/cards.go
// despite both games dealing the same 52 cards.
var (
	ranks = []string{"2", "3", "4", "5", "6", "7", "8", "9", "T", "J", "Q", "K", "A"}
	suits = []string{"H", "C", "D", "S"}
)

// buildShoe is n decks, shuffled together — the shoe a modern table deals
// from. One deck is a real ruleset rather than a degenerate case, so nothing
// here special-cases it.
func buildShoe(decks int, seed int64) []string {
	out := make([]string, 0, decks*52)
	for d := 0; d < decks; d++ {
		for _, s := range suits {
			for _, r := range ranks {
				out = append(out, r+s)
			}
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

// cardValue is what one card is worth with an ace counted low. The high ace
// is handled in Total, because "eleven when it fits" is a property of a hand,
// not of a card.
func cardValue(card string) int {
	switch rankOf(card) {
	case "A":
		return 1
	case "T", "J", "Q", "K":
		return 10
	case "":
		return 0
	default:
		return int(card[0] - '0')
	}
}

// Total is the best total a hand can make, and whether it is soft — an ace
// still counted as eleven.
//
// One ace at most is ever raised: two elevens is already twenty-two, so the
// question "how many aces do I promote" has no interesting answer. Soft is
// returned rather than derived by a caller because it is the whole difference
// between a hand that can be hit freely and one that can bust, and it is
// exactly the sort of rule a client must never be left to work out.
func Total(cards []string) (total int, soft bool) {
	aces := 0
	for _, c := range cards {
		total += cardValue(c)
		if rankOf(c) == "A" {
			aces++
		}
	}
	if aces > 0 && total+10 <= 21 {
		return total + 10, true
	}
	return total, false
}

// IsBust reports a hand over twenty-one.
func IsBust(cards []string) bool {
	t, _ := Total(cards)
	return t > 21
}

// isNatural is twenty-one on the first two cards. Split hands are excluded by
// the caller: twenty-one made from a split ace is twenty-one, not a blackjack,
// and paying it 3:2 is the single most common way to get this game wrong.
func isNatural(cards []string) bool {
	if len(cards) != 2 {
		return false
	}
	t, _ := Total(cards)
	return t == 21
}

// samePairValue reports two cards a table would let you split.
//
// By value, not by rank: a king and a queen are two ten-value cards and every
// casino splits them. It is a house rule rather than a law of the game, which
// is why it is stated here in one place and pointed at from the written rules.
func samePairValue(a, b string) bool { return cardValue(a) == cardValue(b) }
