package ginrummy

import "math/rand"

// Ranks and suits of the standard 52-card deck, in the notation the rest of
// the server already uses for cards ("AH", "TS", "KD") — one rank character,
// one suit character, ace low, ten spelled "T".
var (
	ranks = []string{"A", "2", "3", "4", "5", "6", "7", "8", "9", "T", "J", "Q", "K"}
	suits = []string{"H", "D", "C", "S"}
)

// rankIndex is a rank's position in the ace-low sequence a run walks. There is
// no round-the-corner run, so K never wraps back to A — see the descriptor.
var rankIndex = map[byte]int{
	'A': 1, '2': 2, '3': 3, '4': 4, '5': 5, '6': 6, '7': 7,
	'8': 8, '9': 9, 'T': 10, 'J': 11, 'Q': 12, 'K': 13,
}

func rankOf(card string) string { return card[:1] }
func suitOf(card string) string { return card[1:] }

// cardValue is a card's deadwood weight: ace low at one, face cards at ten,
// everything else its own number.
func cardValue(card string) int {
	switch rankOf(card) {
	case "A":
		return 1
	case "T", "J", "Q", "K":
		return 10
	default:
		return int(card[0] - '0')
	}
}

func handValue(cards []string) int {
	total := 0
	for _, c := range cards {
		total += cardValue(c)
	}
	return total
}

func buildDeck() []string {
	out := make([]string, 0, len(ranks)*len(suits))
	for _, s := range suits {
		for _, r := range ranks {
			out = append(out, r+s)
		}
	}
	return out
}

func shuffle(cards []string, seed int64) []string {
	out := append([]string(nil), cards...)
	r := rand.New(rand.NewSource(seed))
	r.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	return out
}
