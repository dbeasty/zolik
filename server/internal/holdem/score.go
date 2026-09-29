package holdem

import "math/bits"

// A second way to rank a hand, for the one caller that ranks hundreds of
// thousands of them.
//
// equity (bot.go) deals a few hundred trials per decision and ranks every
// hand in each, and Best was written to run once per showdown: it tries all
// twenty-one five-card subsets of seven, and each try builds maps and sorts.
// That made a single rollout cost ten to twenty milliseconds — invisible
// behind a bot's thinking pause, and the whole budget of a training run, which
// asks the encoder for an equity at every decision it sees.
//
// So equity ranks with this instead: seven cards in, one integer out, whose
// order is Best's order. It is a second implementation of the hand rankings,
// which is exactly what equity's own comment warns against, and what keeps it
// honest is that the showdown never uses it — the pot is still awarded by
// Best — and TestScoreAgreesWithBest, which compares the two on hundreds of
// thousands of hands, every category included, and requires the same winner
// and the same category every time. equity's answer does not change by a
// single trial; TestEquityIsUnchanged pins that against the old arithmetic.

// A card as a small integer: rank (0 = two .. 12 = ace) times four plus suit.
type code uint8

func codeOf(card string) code {
	r, s, _ := cardIndex(card)
	return code(r*4 + s)
}

// scoreCategory reads the category back out of a score.
func scoreCategory(score uint32) int { return int(score >> 20) }

// score7 ranks five to seven cards: the category in the top bits, then up to
// five tiebreak ranks, four bits each, in the order Best compares them. Two
// scores compare the way the two HandRanks would.
func score7(cards []code) uint32 {
	var counts [13]uint8
	var suitMask [4]uint16
	var rankMask uint16
	for _, c := range cards {
		r, s := c>>2, c&3
		counts[r]++
		suitMask[s] |= 1 << r
		rankMask |= 1 << r
	}

	for s := 0; s < 4; s++ {
		if bits.OnesCount16(suitMask[s]) >= 5 {
			if hi, ok := straightIn(suitMask[s]); ok {
				return pack(straightFlush, hi)
			}
			// Otherwise a flush, unless a full house or quads beats it; those
			// are found below and returned first, so hold this until then.
			return bestOf(counts, rankMask, suitMask[s])
		}
	}
	return bestOf(counts, rankMask, 0)
}

// bestOf ranks everything but a straight flush. flushMask, when set, is the
// ranks of a flush suit.
func bestOf(counts [13]uint8, rankMask, flushMask uint16) uint32 {
	// Ranks by how many of each, highest rank first.
	var fours, threes, twos, ones []int
	for r := 12; r >= 0; r-- {
		switch counts[r] {
		case 4:
			fours = append(fours, r)
		case 3:
			threes = append(threes, r)
		case 2:
			twos = append(twos, r)
		case 1:
			ones = append(ones, r)
		}
	}
	// kicker is the highest rank not already used.
	kicker := func(used ...int) int {
		for r := 12; r >= 0; r-- {
			if counts[r] == 0 {
				continue
			}
			taken := false
			for _, u := range used {
				if u == r {
					taken = true
				}
			}
			if !taken {
				return r
			}
		}
		return 0
	}

	switch {
	case len(fours) > 0:
		return pack(quads, fours[0], kicker(fours[0]))
	case len(threes) > 0 && (len(threes) > 1 || len(twos) > 0):
		// The best pair to go with the trips may be a second set of trips.
		pr := -1
		if len(threes) > 1 {
			pr = threes[1]
		}
		if len(twos) > 0 && twos[0] > pr {
			pr = twos[0]
		}
		return pack(fullHouse, threes[0], pr)
	case flushMask != 0:
		top := make([]int, 0, 5)
		for r := 12; r >= 0 && len(top) < 5; r-- {
			if flushMask&(1<<r) != 0 {
				top = append(top, r)
			}
		}
		return pack(flush, top...)
	}
	if hi, ok := straightIn(rankMask); ok {
		return pack(straight, hi)
	}
	switch {
	case len(threes) > 0:
		k1 := kicker(threes[0])
		return pack(trips, threes[0], k1, kicker(threes[0], k1))
	case len(twos) >= 2:
		return pack(twoPair, twos[0], twos[1], kicker(twos[0], twos[1]))
	case len(twos) == 1:
		k1 := kicker(twos[0])
		k2 := kicker(twos[0], k1)
		return pack(pair, twos[0], k1, k2, kicker(twos[0], k1, k2))
	default:
		return pack(highCard, ones[:min(5, len(ones))]...)
	}
}

// straightIn finds the highest straight in a set of ranks, reporting its top
// card as a rank index — with the wheel's top card the five (index 3).
func straightIn(mask uint16) (int, bool) {
	for hi := 12; hi >= 4; hi-- {
		want := uint16(0x1f) << (hi - 4)
		if mask&want == want {
			return hi, true
		}
	}
	// A-2-3-4-5: the ace (12) plays low.
	const wheel = 1<<12 | 1<<0 | 1<<1 | 1<<2 | 1<<3
	if mask&wheel == wheel {
		return 3, true
	}
	return 0, false
}

// pack builds a score. Ranks go in as indices plus one, so a present rank is
// never zero and a hand with fewer tiebreaks sorts as Best's shorter slice
// does — though within one category the counts always agree.
func pack(category int, ranks ...int) uint32 {
	s := uint32(category) << 20
	for i, r := range ranks {
		if i >= 5 {
			break
		}
		s |= uint32(r+1) << (16 - 4*i)
	}
	return s
}
