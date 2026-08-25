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

// buildDeck returns 108 cards: two standard decks plus four jokers.
//
// Fixed at two decks rather than scaling with the table, because the meld
// arithmetic Canasta is built on — three wilds to four naturals in a seven-card
// canasta — is a property of how many of each rank exist. A third deck is a
// different game (Samba), not a bigger one.
func buildDeck() []string {
	out := make([]string, 0, 108)
	for d := 0; d < 2; d++ {
		for _, r := range ranks {
			for _, s := range suits {
				out = append(out, r+s)
			}
		}
		out = append(out, "JOKER1", "JOKER2")
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

// sortedUnique is used wherever an offer lists cards: a stable, duplicate-free
// list keeps offer content addressable so a client can diff it across pushes.
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
