package ginrummy

import (
	"fmt"
	"sort"
)

// Deadwood, and why exhaustive search is the right amount of cleverness.
//
// The best arrangement of a ten-card hand into melds is a small combinatorial
// search, not a player's decision — see the package doc on why that is what
// makes `knock:<card>` an enumerable offer at all. A hand this size has a
// bounded number of candidate melds (at most four sets, one per rank present
// three or four times, and a handful of run fragments per suit), so packing
// them for maximum value is exhaustive search over a few dozen candidates with
// a used-cards bitmask for memoization — microseconds, not an algorithm this
// package should apologize for. BenchmarkDeadwood guards that claim rather
// than leaving it as a comment.

type meldCandidate struct {
	Cards []string
	Value int
	Kind  string // "set" | "run"
}

// candidateMelds lists every set and run a hand could lay — every subset of
// same-rank cards of size three or four, and every contiguous same-suit
// sub-run of length three or more. Not just the maximal ones: a shorter run
// can matter when it leaves a better card free for a set elsewhere, and the
// packing search is what decides that trade-off, not this function.
func candidateMelds(hand []string) []meldCandidate {
	var out []meldCandidate

	byRank := map[string][]string{}
	for _, c := range hand {
		byRank[rankOf(c)] = append(byRank[rankOf(c)], c)
	}
	for _, cards := range byRank {
		if len(cards) < 3 {
			continue
		}
		sorted := append([]string(nil), cards...)
		sort.Strings(sorted)
		for _, subset := range combinations(sorted, 3) {
			out = append(out, meldCandidate{Cards: subset, Value: handValue(subset), Kind: "set"})
		}
		if len(sorted) == 4 {
			out = append(out, meldCandidate{Cards: append([]string(nil), sorted...), Value: handValue(sorted), Kind: "set"})
		}
	}

	bySuit := map[string][]string{}
	for _, c := range hand {
		bySuit[suitOf(c)] = append(bySuit[suitOf(c)], c)
	}
	for _, cards := range bySuit {
		sorted := append([]string(nil), cards...)
		sort.Slice(sorted, func(i, j int) bool { return rankIndex[sorted[i][0]] < rankIndex[sorted[j][0]] })
		i := 0
		for i < len(sorted) {
			j := i
			for j+1 < len(sorted) && rankIndex[sorted[j+1][0]] == rankIndex[sorted[j][0]]+1 {
				j++
			}
			segLen := j - i + 1
			if segLen >= 3 {
				for start := i; start <= j-2; start++ {
					for end := start + 2; end <= j; end++ {
						sub := append([]string(nil), sorted[start:end+1]...)
						out = append(out, meldCandidate{Cards: sub, Value: handValue(sub), Kind: "run"})
					}
				}
			}
			i = j + 1
		}
	}
	return out
}

// combinations lists every k-sized subset of items, preserving order.
func combinations(items []string, k int) [][]string {
	var out [][]string
	var pick func(start int, cur []string)
	pick = func(start int, cur []string) {
		if len(cur) == k {
			out = append(out, append([]string(nil), cur...))
			return
		}
		for i := start; i < len(items); i++ {
			pick(i+1, append(cur, items[i]))
		}
	}
	pick(0, nil)
	return out
}

// Deadwood finds the arrangement of hand into melds that leaves the least
// value uncovered — the minimum, not any, cover. It returns that value and the
// melds themselves, each already sorted for display (a run ascending by rank,
// a set alphabetically by suit).
func Deadwood(hand []string) (int, []Meld) {
	idx := make(map[string]int, len(hand))
	for i, c := range hand {
		idx[c] = i
	}
	candidates := candidateMelds(hand)

	type cand struct {
		mask  uint32
		value int
	}
	cs := make([]cand, len(candidates))
	for i, mc := range candidates {
		var m uint32
		for _, c := range mc.Cards {
			m |= 1 << uint(idx[c])
		}
		cs[i] = cand{mask: m, value: mc.Value}
	}

	type memoKey struct {
		i    int
		used uint32
	}
	type memoVal struct {
		val    int
		choice []int
	}
	memo := map[memoKey]memoVal{}

	var rec func(i int, used uint32) (int, []int)
	rec = func(i int, used uint32) (int, []int) {
		if i == len(cs) {
			return 0, nil
		}
		key := memoKey{i, used}
		if v, ok := memo[key]; ok {
			return v.val, v.choice
		}
		bestVal, bestChoice := rec(i+1, used)
		if cs[i].mask&used == 0 {
			v2, c2 := rec(i+1, used|cs[i].mask)
			v2 += cs[i].value
			if v2 > bestVal {
				bestVal = v2
				bestChoice = append([]int{i}, c2...)
			}
		}
		memo[key] = memoVal{bestVal, bestChoice}
		return bestVal, bestChoice
	}

	bestVal, choice := rec(0, 0)

	var melds []Meld
	for i, ci := range choice {
		c := candidates[ci]
		cards := append([]string(nil), c.Cards...)
		if c.Kind == "run" {
			sort.Slice(cards, func(a, b int) bool { return rankIndex[cards[a][0]] < rankIndex[cards[b][0]] })
		} else {
			sort.Strings(cards)
		}
		melds = append(melds, Meld{ID: fmt.Sprintf("m%d", i), Kind: c.Kind, Cards: cards})
	}
	return handValue(hand) - bestVal, melds
}

func isSet(m Meld) bool { return m.Kind == "set" }

// runBounds is the rank range a run meld spans, assuming Cards is sorted
// ascending — the invariant every constructor and mutator in this package
// keeps.
func runBounds(cards []string) (lo, hi int) {
	lo = rankIndex[cards[0][0]]
	hi = rankIndex[cards[len(cards)-1][0]]
	return
}

// extendsMeld reports whether card may be laid onto m: any card of a set's
// rank while it has room for a fourth, or a run's card continuing either end —
// there is no round-the-corner run and no bridging a gap, since a valid meld
// never has one.
func extendsMeld(m Meld, card string) bool {
	if len(m.Cards) == 0 {
		return false
	}
	if isSet(m) {
		return rankOf(card) == rankOf(m.Cards[0]) && len(m.Cards) < 4 && !hasCard(m.Cards, card)
	}
	if suitOf(card) != suitOf(m.Cards[0]) {
		return false
	}
	lo, hi := runBounds(m.Cards)
	r := rankIndex[card[0]]
	return r == lo-1 || r == hi+1
}

// insertIntoMeld returns m's cards with card added at the correct place: a run
// keeps its ascending order, a set is re-sorted alphabetically by suit.
func insertIntoMeld(m Meld, card string) []string {
	if isSet(m) {
		out := append(append([]string(nil), m.Cards...), card)
		sort.Strings(out)
		return out
	}
	lo, _ := runBounds(m.Cards)
	if rankIndex[card[0]] == lo-1 {
		return append([]string{card}, m.Cards...)
	}
	return append(append([]string(nil), m.Cards...), card)
}

func meldIndex(melds []Meld, id string) int {
	for i, m := range melds {
		if m.ID == id {
			return i
		}
	}
	return -1
}
