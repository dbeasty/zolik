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

// hand is a set of cards as four rank masks, one per suit: everything score7
// needs, in eight bytes. It is a value on purpose — equity tallies the board
// once per trial and copies it for each player's hole cards, rather than
// counting the same five cards again for every seat.
//
// A set, not a multiset: the same card added twice counts once. Every caller
// ranks distinct cards, as a deck deals them.
type hand struct {
	suits [4]uint16
}

func (h *hand) add(c code) { h.suits[c&3] |= 1 << (c >> 2) }

// score7 ranks five to seven cards: the category in the top bits, then up to
// five tiebreak ranks, four bits each, in the order Best compares them. Two
// scores compare the way the two HandRanks would.
func score7(cards []code) uint32 {
	var h hand
	for _, c := range cards {
		h.add(c)
	}
	return h.score()
}

// score is score7 for the cards already in h.
func (h *hand) score() uint32 {
	for s := 0; s < 4; s++ {
		if bits.OnesCount16(h.suits[s]) >= 5 {
			if hi, ok := straightIn(h.suits[s]); ok {
				return pack(straightFlush, hi)
			}
			// Otherwise a flush, unless a full house or quads beats it; those
			// are found below and returned first, so hold this until then.
			return h.bestOf(h.suits[s])
		}
	}
	return h.bestOf(0)
}

// bestOf ranks everything but a straight flush. flushMask, when set, is the
// ranks of a flush suit.
//
// Every count it needs is a mask, read off the four suit masks with bit
// arithmetic: a rank held at least twice is one present in two suits, and so
// on. Highest rank first is the highest set bit, so nothing is sorted or
// collected and nothing is allocated.
func (h *hand) bestOf(flushMask uint16) uint32 {
	a, b, c, d := h.suits[0], h.suits[1], h.suits[2], h.suits[3]
	all := a | b | c | d
	fours := a & b & c & d
	atLeast3 := a&b&(c|d) | c&d&(a|b)
	atLeast2 := a&(b|c|d) | b&(c|d) | c&d
	threes := atLeast3 &^ fours
	twos := atLeast2 &^ atLeast3

	switch {
	case fours != 0:
		q := top(fours)
		return pack(quads, q, kicker(all&^bit(q)))
	case threes != 0 && (threes&(threes-1) != 0 || twos != 0):
		// The best pair to go with the trips may be a second set of trips.
		t := top(threes)
		pr := -1
		if rest := threes &^ bit(t); rest != 0 {
			pr = top(rest)
		}
		if twos != 0 && top(twos) > pr {
			pr = top(twos)
		}
		return pack(fullHouse, t, pr)
	case flushMask != 0:
		return packTop(flush, flushMask)
	}
	if hi, ok := straightIn(all); ok {
		return pack(straight, hi)
	}
	switch {
	case threes != 0:
		t := top(threes)
		rest := all &^ bit(t)
		k1 := kicker(rest)
		return pack(trips, t, k1, kicker(rest&^bit(k1)))
	case twos&(twos-1) != 0:
		p1 := top(twos)
		p2 := top(twos &^ bit(p1))
		return pack(twoPair, p1, p2, kicker(all&^bit(p1)&^bit(p2)))
	case twos != 0:
		p := top(twos)
		rest := all &^ bit(p)
		k1 := kicker(rest)
		rest &^= bit(k1)
		k2 := kicker(rest)
		return pack(pair, p, k1, k2, kicker(rest&^bit(k2)))
	default:
		return packTop(highCard, all)
	}
}

func bit(r int) uint16 { return 1 << r }

// top is the highest rank in a mask that is not empty.
func top(mask uint16) int { return bits.Len16(mask) - 1 }

// kicker is the highest rank left in a mask, or the deuce when none is.
func kicker(mask uint16) int {
	if mask == 0 {
		return 0
	}
	return top(mask)
}

// packTop is pack with the highest five ranks of a mask, or all of them when
// there are fewer.
func packTop(category int, mask uint16) uint32 {
	s := uint32(category) << 20
	for i := 0; i < 5 && mask != 0; i++ {
		r := top(mask)
		s |= uint32(r+1) << (16 - 4*i)
		mask &^= bit(r)
	}
	return s
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
