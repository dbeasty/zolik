package holdem

import (
	"math/rand"
	"sort"
)

// Card notation matches the rest of the server: rank letter plus suit letter
// ("AS", "TD", "2C"). One deck, no jokers — the first module here with no wild
// card of any kind.
var (
	ranks = []string{"2", "3", "4", "5", "6", "7", "8", "9", "T", "J", "Q", "K", "A"}
	suits = []string{"H", "C", "D", "S"}
)

// rankValue orders ranks for hand comparison. Aces are high here; the one
// place they play low (the wheel, A-2-3-4-5) is handled where straights are
// found, because that is the only rule that knows about it.
var rankValue = map[string]int{
	"2": 2, "3": 3, "4": 4, "5": 5, "6": 6, "7": 7, "8": 8,
	"9": 9, "T": 10, "J": 11, "Q": 12, "K": 13, "A": 14,
}

func buildDeck() []string {
	out := make([]string, 0, 52)
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

func rankOf(card string) string {
	if card == "" {
		return ""
	}
	return card[:len(card)-1]
}

func suitOf(card string) string {
	if card == "" {
		return ""
	}
	return card[len(card)-1:]
}

// --- hand evaluation --------------------------------------------------------

// Hand categories, worst to best.
const (
	highCard = iota
	pair
	twoPair
	trips
	straight
	flush
	fullHouse
	quads
	straightFlush
)

// HandRank is a comparable score for a five-card hand.
//
// Category first, then tiebreakers in the order they are compared at a table —
// so comparison is a plain lexicographic walk and there is no per-category
// special case anywhere else.
type HandRank struct {
	Category int      `json:"category"`
	Tiebreak []int    `json:"tiebreak"`
	Cards    []string `json:"cards"`
}

// Compare returns -1, 0 or 1. Zero means a genuine tie, which is how split
// pots arise — and split pots are why this module made the runtime stop
// assuming a single winner.
func (h HandRank) Compare(other HandRank) int {
	if h.Category != other.Category {
		if h.Category < other.Category {
			return -1
		}
		return 1
	}
	for i := 0; i < len(h.Tiebreak) && i < len(other.Tiebreak); i++ {
		if h.Tiebreak[i] != other.Tiebreak[i] {
			if h.Tiebreak[i] < other.Tiebreak[i] {
				return -1
			}
			return 1
		}
	}
	return 0
}

// Best returns the best five-card hand from any number of cards.
//
// Brute force over the 21 five-card subsets of seven. A clever evaluator would
// be faster and much harder to be sure of, and this runs once per showdown.
func Best(cards []string) HandRank {
	if len(cards) < 5 {
		return HandRank{Category: highCard}
	}
	var best HandRank
	first := true
	n := len(cards)
	for a := 0; a < n-4; a++ {
		for b := a + 1; b < n-3; b++ {
			for c := b + 1; c < n-2; c++ {
				for d := c + 1; d < n-1; d++ {
					for e := d + 1; e < n; e++ {
						hand := []string{cards[a], cards[b], cards[c], cards[d], cards[e]}
						r := evaluateFive(hand)
						if first || r.Compare(best) > 0 {
							best, first = r, false
						}
					}
				}
			}
		}
	}
	return best
}

// evaluateFive scores exactly five cards.
func evaluateFive(hand []string) HandRank {
	values := make([]int, 0, 5)
	suitCount := map[string]int{}
	byValue := map[int]int{}
	for _, c := range hand {
		v := rankValue[rankOf(c)]
		values = append(values, v)
		suitCount[suitOf(c)]++
		byValue[v]++
	}
	sort.Sort(sort.Reverse(sort.IntSlice(values)))

	isFlush := false
	for _, n := range suitCount {
		if n == 5 {
			isFlush = true
		}
	}

	straightHigh, isStraight := straightHighOf(values)

	// Group by count, then by value: this ordering is what makes the
	// tiebreakers fall out in the right order for every paired category
	// without naming any of them.
	type group struct{ count, value int }
	groups := make([]group, 0, 5)
	for v, n := range byValue {
		groups = append(groups, group{count: n, value: v})
	}
	sort.Slice(groups, func(i, j int) bool {
		if groups[i].count != groups[j].count {
			return groups[i].count > groups[j].count
		}
		return groups[i].value > groups[j].value
	})
	tiebreak := make([]int, 0, 5)
	for _, g := range groups {
		tiebreak = append(tiebreak, g.value)
	}

	switch {
	case isStraight && isFlush:
		return HandRank{Category: straightFlush, Tiebreak: []int{straightHigh}, Cards: hand}
	case groups[0].count == 4:
		return HandRank{Category: quads, Tiebreak: tiebreak, Cards: hand}
	case groups[0].count == 3 && len(groups) > 1 && groups[1].count == 2:
		return HandRank{Category: fullHouse, Tiebreak: tiebreak, Cards: hand}
	case isFlush:
		return HandRank{Category: flush, Tiebreak: values, Cards: hand}
	case isStraight:
		return HandRank{Category: straight, Tiebreak: []int{straightHigh}, Cards: hand}
	case groups[0].count == 3:
		return HandRank{Category: trips, Tiebreak: tiebreak, Cards: hand}
	case groups[0].count == 2 && len(groups) > 1 && groups[1].count == 2:
		return HandRank{Category: twoPair, Tiebreak: tiebreak, Cards: hand}
	case groups[0].count == 2:
		return HandRank{Category: pair, Tiebreak: tiebreak, Cards: hand}
	default:
		return HandRank{Category: highCard, Tiebreak: values, Cards: hand}
	}
}

// straightHighOf reports the top card of a straight, if these five values are
// one.
//
// The wheel is the whole reason this is a function rather than a subtraction:
// A-2-3-4-5 is a straight to the *five*, so the ace that sorted to the top has
// to be re-read as a one.
func straightHighOf(desc []int) (int, bool) {
	uniq := make([]int, 0, 5)
	seen := map[int]bool{}
	for _, v := range desc {
		if !seen[v] {
			seen[v] = true
			uniq = append(uniq, v)
		}
	}
	if len(uniq) != 5 {
		return 0, false
	}
	if uniq[0]-uniq[4] == 4 {
		return uniq[0], true
	}
	if uniq[0] == 14 && uniq[1] == 5 && uniq[4] == 2 {
		return 5, true
	}
	return 0, false
}

// categoryKey names a hand for the client's locale bundle. A key, never a
// rendered sentence — the same contract every other module keeps.
func categoryKey(category int) string {
	switch category {
	case straightFlush:
		return "holdem.hand.straightFlush"
	case quads:
		return "holdem.hand.fourOfAKind"
	case fullHouse:
		return "holdem.hand.fullHouse"
	case flush:
		return "holdem.hand.flush"
	case straight:
		return "holdem.hand.straight"
	case trips:
		return "holdem.hand.threeOfAKind"
	case twoPair:
		return "holdem.hand.twoPair"
	case pair:
		return "holdem.hand.pair"
	default:
		return "holdem.hand.highCard"
	}
}
