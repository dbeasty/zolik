package klondike

import "math/rand"

// Card notation matches the rest of the server: rank letter plus suit letter
// ("AS", "TD", "2C"). Klondike needs three facts about a card — its rank in
// order, its colour, and its suit — and nothing else.
var (
	ranks = []string{"A", "2", "3", "4", "5", "6", "7", "8", "9", "T", "J", "Q", "K"}
	// suits is also the order of the foundations: f-S, f-H, f-D, f-C.
	suits = []string{"S", "H", "D", "C"}
)

const (
	numColumns = 7
	rankKing   = 13
)

func newDeck(seed int64) []string {
	out := make([]string, 0, 52)
	for _, s := range suits {
		for _, r := range ranks {
			out = append(out, r+s)
		}
	}
	r := rand.New(rand.NewSource(seed))
	r.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	return out
}

// rankOf is 1 (ace) to 13 (king); 0 for something that is not a card.
func rankOf(card string) int {
	if len(card) < 2 {
		return 0
	}
	r := card[:len(card)-1]
	for i, x := range ranks {
		if x == r {
			return i + 1
		}
	}
	return 0
}

func suitOf(card string) string {
	if len(card) < 2 {
		return ""
	}
	return card[len(card)-1:]
}

func isRed(card string) bool {
	s := suitOf(card)
	return s == "H" || s == "D"
}

func suitIndex(suit string) int {
	for i, s := range suits {
		if s == suit {
			return i
		}
	}
	return -1
}

// buildsOn reports whether card may be laid on top of base in a column: one
// rank lower, the other colour.
func buildsOn(card, base string) bool {
	return rankOf(card) == rankOf(base)-1 && isRed(card) != isRed(base)
}
