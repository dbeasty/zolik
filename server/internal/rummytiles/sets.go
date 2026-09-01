package rummytiles

import "sort"

// Set validity, and where a joker's identity comes from.
//
// A joker carries no identity of its own — what it "is" is entirely a
// function of the set it sits in. So validation and identity-resolution are
// the same pass: a set validates by finding *some* assignment of numbers to
// its jokers that makes it a legal group or run, and that assignment is what
// setValueOf and swap_joker read back afterward. There is no separate step
// that "declares" what a joker means; the shape it is placed into is the only
// declaration there ever is.

func splitJokers(cards []string) (reals, jokers []string) {
	for _, c := range cards {
		if isJoker(c) {
			jokers = append(jokers, c)
		} else {
			reals = append(reals, c)
		}
	}
	return reals, jokers
}

// validateGroup checks whether cards can be three or four tiles of one
// number, one of each colour, with jokers filling any colours not present.
// The canonical order is the reals sorted by colour, jokers last.
func validateGroup(cards []string) ([]string, bool) {
	if len(cards) < 3 || len(cards) > 4 {
		return nil, false
	}
	reals, jokers := splitJokers(cards)
	if len(reals) == 0 {
		return nil, false
	}
	number := numberOf(reals[0])
	seenColour := map[string]bool{}
	for _, r := range reals {
		if numberOf(r) != number {
			return nil, false
		}
		c := colourOf(r)
		if seenColour[c] {
			return nil, false
		}
		seenColour[c] = true
	}
	out := append([]string(nil), reals...)
	sort.Strings(out)
	out = append(out, jokers...)
	return out, true
}

// validateRun checks whether cards can be three or more consecutive numbers
// in one colour, with jokers filling internal gaps or extending either end —
// never wrapping past 13 or below 1. The canonical order is strictly
// ascending by the number each position represents.
func validateRun(cards []string) ([]string, bool) {
	if len(cards) < 3 || len(cards) > 13 {
		return nil, false
	}
	reals, jokers := splitJokers(cards)
	if len(reals) == 0 {
		return nil, false
	}
	colour := colourOf(reals[0])
	seenNumber := map[int]bool{}
	minN, maxN := 14, 0
	for _, r := range reals {
		if colourOf(r) != colour {
			return nil, false
		}
		n := numberOf(r)
		if seenNumber[n] {
			return nil, false
		}
		seenNumber[n] = true
		if n < minN {
			minN = n
		}
		if n > maxN {
			maxN = n
		}
	}

	span := maxN - minN + 1
	internalGaps := span - len(reals)
	if len(jokers) < internalGaps {
		return nil, false
	}
	extra := len(jokers) - internalGaps
	room := (minN - 1) + (13 - maxN)
	if extra > room {
		return nil, false
	}

	// Extend the low end first, deterministically — which specific joker
	// lands where does not affect validity, only rendering.
	lowExtra := extra
	if lowExtra > minN-1 {
		lowExtra = minN - 1
	}
	startN := minN - lowExtra
	endN := maxN + (extra - lowExtra)

	realByNumber := make(map[int]string, len(reals))
	for _, r := range reals {
		realByNumber[numberOf(r)] = r
	}
	jokerQueue := append([]string(nil), jokers...)
	out := make([]string, 0, endN-startN+1)
	for n := startN; n <= endN; n++ {
		if r, ok := realByNumber[n]; ok {
			out = append(out, r)
			continue
		}
		out = append(out, jokerQueue[0])
		jokerQueue = jokerQueue[1:]
	}
	return out, true
}

// validateSet tries group first, then run — a single real tile plus two
// jokers can structurally satisfy either, and which one is reported does not
// change its value (three tiles are three tiles) or its legality.
func validateSet(cards []string) (kind string, canonical []string, ok bool) {
	if g, ok := validateGroup(cards); ok {
		return "group", g, true
	}
	if r, ok := validateRun(cards); ok {
		return "run", r, true
	}
	return "", nil, false
}

// setValueOf is a valid, canonicalized set's point total — a joker counts as
// the number it was resolved to, not its 30-point hand penalty.
func setValueOf(kind string, canonical []string) int {
	if kind == "group" {
		number := numberOf(realTileIn(canonical))
		return number * len(canonical)
	}
	// run: canonical is strictly ascending, so one real tile fixes every
	// position's implied number.
	startN := runStartNumber(canonical)
	total := 0
	for i := range canonical {
		total += startN + i
	}
	return total
}

// realTileIn is the first non-joker tile in a set — enough to read a group's
// number or a run's colour, since every real tile in a valid set agrees.
func realTileIn(cards []string) string {
	for _, c := range cards {
		if !isJoker(c) {
			return c
		}
	}
	return ""
}

// runStartNumber is the number a canonical run's first position represents,
// read back from wherever its first real tile happens to sit.
func runStartNumber(canonical []string) int {
	for i, c := range canonical {
		if !isJoker(c) {
			return numberOf(c) - i
		}
	}
	return 0
}
