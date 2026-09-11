package canasta

import (
	"math/rand"
	"sort"
)

// Card notation matches the rest of the server: rank letter plus suit letter
// ("AS", "TD", "2C"), and jokers as "JOKER1"/"JOKER2". Two decks means every
// card appears twice and every joker name appears twice, so cards are a
// multiset everywhere — the same assumption `rules.BuildDeck` already makes.
var (
	ranks = []string{"2", "3", "4", "5", "6", "7", "8", "9", "T", "J", "Q", "K", "A"}
	suits = []string{"H", "C", "D", "S"}
)

const (
	rankJoker = "JOKER"
	rankTwo   = "2"
	rankThree = "3"
)

// buildDeck returns the variation's deck: 108 cards at two decks and four
// jokers, 162 at three and six.
//
// The count is a property of the variation rather than of the table, because the
// meld arithmetic a game is built on — three wilds to four naturals in a
// seven-card canasta — is a property of how many of each rank exist. A third deck
// is a different game (Samba), not a bigger one, which is why it arrives with a
// ruleset rather than with a fifth player.
func buildDeck(r ruleset) []string {
	out := make([]string, 0, r.Decks*(len(ranks)*len(suits)+r.JokersPerDeck))
	for d := 0; d < r.Decks; d++ {
		for _, rank := range ranks {
			for _, s := range suits {
				out = append(out, rank+s)
			}
		}
		for j := 0; j < r.JokersPerDeck; j++ {
			out = append(out, "JOKER"+string(rune('1'+j)))
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

// rankOf is the meld-relevant identity of a card. Jokers collapse to a single
// rank so "JOKER1" and "JOKER2" are never treated as two different wilds.
func rankOf(card string) string {
	if len(card) >= 5 && card[:5] == rankJoker {
		return rankJoker
	}
	if card == "" {
		return ""
	}
	return card[:len(card)-1]
}

// runRanks are the ranks a sequence can be built from, low to high.
//
// It starts at four and the ace is high, and both ends are consequences rather
// than choices: twos are wild and threes are never an ordinary meld, so there is
// nothing below the four to run from, and an ace that could also be low would
// make A-2-3 a sequence in a game where the 2 is a joker.
var runRanks = []string{"4", "5", "6", "7", "8", "9", "T", "J", "Q", "K", "A"}

var runRankIndex = func() map[string]int {
	m := make(map[string]int, len(runRanks))
	for i, r := range runRanks {
		m[r] = i
	}
	return m
}()

// runIndexOf is a card's position in a sequence, and whether it can be in one at
// all. Wilds and threes cannot, which is what the second return says.
func runIndexOf(card string) (int, bool) {
	if isWild(card) {
		return 0, false
	}
	i, ok := runRankIndex[rankOf(card)]
	return i, ok
}

// runSpan is the lowest and highest positions a sequence covers. Callers have
// already validated that it is one, so the gaps are not re-checked here.
func runSpan(cards []string) (low, high int) {
	low, high = len(runRanks), -1
	for _, c := range cards {
		i, ok := runIndexOf(c)
		if !ok {
			continue
		}
		if i < low {
			low = i
		}
		if i > high {
			high = i
		}
	}
	return low, high
}

// sortRun puts a sequence in rank order, which is how it is stored and how it is
// drawn. A run whose cards arrive shuffled is still the same meld, but a client
// showing 9-7-8 would look like a bug in the client.
func sortRun(cards []string) []string {
	out := append([]string(nil), cards...)
	sort.SliceStable(out, func(i, j int) bool {
		a, _ := runIndexOf(out[i])
		b, _ := runIndexOf(out[j])
		return a < b
	})
	return out
}

func suitOf(card string) string {
	if rankOf(card) == rankJoker || card == "" {
		return ""
	}
	return card[len(card)-1:]
}

// isWild reports whether a card can stand in for another rank: jokers and 2s.
func isWild(card string) bool {
	r := rankOf(card)
	return r == rankJoker || r == rankTwo
}

// isRedThree is the card that plays itself: it never sits in a hand, never
// gets melded and never gets discarded.
func isRedThree(card string) bool {
	if rankOf(card) != rankThree {
		return false
	}
	s := suitOf(card)
	return s == "H" || s == "D"
}

// isBlackThree blocks the pile when discarded and may only be melded on the
// way out.
func isBlackThree(card string) bool {
	if rankOf(card) != rankThree {
		return false
	}
	s := suitOf(card)
	return s == "C" || s == "S"
}

// cardValue is a card's point value, the number that feeds both the initial
// meld minimum and the deal score.
//
// Red threes are excluded here and scored separately: their 100 is a bonus
// that flips sign with the partnership's canasta count, so folding it into a
// per-card value would make it wrong in one of the two cases.
func cardValue(card string) int {
	switch rankOf(card) {
	case rankJoker:
		return 50
	case rankTwo:
		return 20
	case "A":
		return 20
	case "K", "Q", "J", "T", "9", "8":
		return 10
	case "7", "6", "5", "4":
		return 5
	case rankThree:
		if isRedThree(card) {
			return redThreeValue
		}
		return 5 // black three
	}
	return 0
}

func handValue(cards []string) int {
	total := 0
	for _, c := range cards {
		total += cardValue(c)
	}
	return total
}

// removeCards takes one copy of each named card out of a hand, and reports
// whether every one of them was actually there.
//
// One copy per request, not all matching copies: with two decks a player can
// hold two identical cards and mean only one of them.
func removeCards(hand []string, cards []string) ([]string, bool) {
	out := append([]string(nil), hand...)
	for _, want := range cards {
		found := -1
		for i, have := range out {
			if have == want {
				found = i
				break
			}
		}
		if found < 0 {
			return hand, false
		}
		out = append(out[:found], out[found+1:]...)
	}
	return out, true
}

func hasCards(hand []string, cards []string) bool {
	_, ok := removeCards(hand, cards)
	return ok
}

// sortedCards is used wherever an offer lists the cards in a hand it would
// take: a stable order keeps offer content addressable so a client can diff it
// across pushes, and every copy is kept, because a hand is a multiset. Two
// decks are in play, so a player holding two 7H holds two cards, and an offer
// that named one of them would be describing a hand nobody has — see
// `removeCards`, which has taken one copy per request all along.
func sortedCards(cards []string) []string {
	out := append([]string(nil), cards...)
	sort.Strings(out)
	return out
}

// sortedUnique is the same for a list of *choices* rather than of cards: which
// single card to discard, which rank to meld. One entry per distinct card is
// the whole list there, and a second copy would offer the same choice twice.
func sortedUnique(cards []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(cards))
	for _, c := range cards {
		if seen[c] {
			continue
		}
		seen[c] = true
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}

// countByRank groups a hand by meld rank. Wilds and threes are grouped like
// anything else; callers decide what they are allowed to do with them.
func countByRank(cards []string) map[string][]string {
	out := map[string][]string{}
	for _, c := range cards {
		out[rankOf(c)] = append(out[rankOf(c)], c)
	}
	for r := range out {
		sort.Strings(out[r])
	}
	return out
}
