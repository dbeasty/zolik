package holdem

import (
	"math/rand"
	"testing"
)

func codes(cards []string) []code {
	out := make([]code, len(cards))
	for i, c := range cards {
		out[i] = codeOf(c)
	}
	return out
}

func sign(n int) int {
	switch {
	case n < 0:
		return -1
	case n > 0:
		return 1
	}
	return 0
}

// TestScoreAgreesWithBest is what licenses score7 to exist: on every pair of
// hands it must pick the winner Best picks and name the category Best names.
//
// Random seven-card hands reach a straight flush about once in three thousand
// and quads once in six hundred, so half the hands here are dealt from two
// suits and a handful of ranks, which is where both live.
func TestScoreAgreesWithBest(t *testing.T) {
	n := 200000
	if testing.Short() {
		n = 40000
	}
	deck := buildDeck()
	rnd := rand.New(rand.NewSource(7))
	deal := func() []string {
		pool := deck
		if rnd.Intn(2) == 0 {
			// Two suits, eight ranks from a random start: flushes, straights,
			// straight flushes, boats and quads all turn up often.
			lo := rnd.Intn(6)
			var narrow []string
			for _, c := range deck {
				r, s, _ := cardIndex(c)
				if s < 2 && (r-lo+13)%13 < 8 {
					narrow = append(narrow, c)
				}
				if s >= 2 && r >= lo && r < lo+3 {
					narrow = append(narrow, c)
				}
			}
			pool = narrow
		}
		n := 5 + rnd.Intn(3)
		perm := rnd.Perm(len(pool))
		out := make([]string, n)
		for i := range out {
			out[i] = pool[perm[i]]
		}
		return out
	}
	seen := map[int]int{}
	for i := 0; i < n; i++ {
		a, b := deal(), deal()
		ra, rb := Best(a), Best(b)
		sa, sb := score7(codes(a)), score7(codes(b))
		if scoreCategory(sa) != ra.Category {
			t.Fatalf("%v: category %d, Best says %d", a, scoreCategory(sa), ra.Category)
		}
		want := ra.Compare(rb)
		got := sign(int(int64(sa) - int64(sb)))
		if got != want {
			t.Fatalf("%v (%v) vs %v (%v): score says %d, Best says %d", a, ra.Tiebreak, b, rb.Tiebreak, got, want)
		}
		seen[ra.Category]++
	}
	for c := highCard; c <= straightFlush; c++ {
		if seen[c] == 0 {
			t.Errorf("category %d never dealt", c)
		}
	}
	t.Logf("categories dealt: %v", seen)
}

// equityByBest is equity as it was before score7: every trial ranked by Best.
// Kept as the reference TestEquityIsUnchanged holds the fast version to.
func equityByBest(hole, board []string, opponents, trials, floor int, bluffShare float64, rnd *rand.Rand) float64 {
	if len(hole) < 2 || opponents < 1 {
		return 1
	}
	seen := map[string]bool{}
	for _, c := range append(append([]string(nil), hole...), board...) {
		seen[c] = true
	}
	var deck []string
	for _, c := range buildDeck() {
		if !seen[c] {
			deck = append(deck, c)
		}
	}
	runout := max(5-len(board), 0)
	needed := runout + 2*opponents
	if needed > len(deck) {
		return 0.5
	}
	won, counted := 0.0, 0
	anyhow, dealt := 0.0, 0
	for attempt := 0; attempt < trials*6 && counted < trials; attempt++ {
		for j := 0; j < needed; j++ {
			k := j + rnd.Intn(len(deck)-j)
			deck[j], deck[k] = deck[k], deck[j]
		}
		run := deck[:runout]
		best := Best(append(append(append([]string(nil), hole...), board...), run...))
		ahead, split, claims := true, 1, floor <= highCard
		for o := 0; o < opponents; o++ {
			at := runout + 2*o
			rank := Best(append(append([]string{deck[at], deck[at+1]}, board...), run...))
			if rank.Category >= floor {
				claims = true
			}
			switch best.Compare(rank) {
			case -1:
				ahead = false
			case 0:
				split++
			}
			if !ahead && claims {
				break
			}
		}
		score := 0.0
		if ahead {
			score = 1 / float64(split)
		}
		anyhow, dealt = anyhow+score, dealt+1
		if claims {
			won, counted = won+score, counted+1
		}
	}
	if dealt == 0 {
		return 0.5
	}
	unfiltered := anyhow / float64(dealt)
	if counted == 0 {
		return unfiltered
	}
	filtered := won / float64(counted)
	if bluffShare <= 0 {
		return filtered
	}
	if bluffShare >= 1 {
		return unfiltered
	}
	return (1-bluffShare)*filtered + bluffShare*unfiltered
}

// TestEquityIsUnchanged: moving the rollout onto score7 changed its speed and
// nothing else — the same coin gives the same number, to the last bit, so
// every bot decision is the decision it was.
func TestEquityIsUnchanged(t *testing.T) {
	rnd := rand.New(rand.NewSource(3))
	deck := buildDeck()
	for i := 0; i < 300; i++ {
		perm := rnd.Perm(52)
		boardLen := []int{0, 3, 4, 5}[i%4]
		hole := []string{deck[perm[0]], deck[perm[1]]}
		var board []string
		for j := 0; j < boardLen; j++ {
			board = append(board, deck[perm[2+j]])
		}
		opponents := 1 + i%5
		floor := []int{highCard, pair, twoPair}[i%3]
		share := []float64{0, 0.3, 1}[i%3]
		seed := int64(i)
		got := equity(hole, board, opponents, 60, floor, share, rand.New(rand.NewSource(seed)))
		want := equityByBest(hole, board, opponents, 60, floor, share, rand.New(rand.NewSource(seed)))
		if got != want {
			t.Fatalf("%v on %v against %d (floor %d): %v, was %v", hole, board, opponents, floor, got, want)
		}
	}
}
