package module

import "math/rand"

// StartingSeat picks which seat opens a match, deterministically from the
// match seed, so a lobby's host stops being its match's permanent first
// player without a match losing the "same seed deals the same match"
// property replay, self-play and reproduction depend on.
//
// Offset from the seed rather than reusing it outright: a module derives its
// own deck shuffle from the same seed (typically seed, or seed plus a small
// per-deal multiple), and correlating "who opens" with "what is dealt" would
// make the opening seat a function of the shuffle instead of an independent
// choice. The offset is arbitrary but fixed, so it only has to avoid the
// small multiples the modules already use.
func StartingSeat(seed int64, seats int) int {
	if seats <= 0 {
		return 0
	}
	r := rand.New(rand.NewSource(seed + startingSeatOffset))
	return r.Intn(seats)
}

const startingSeatOffset = 104729 // an arbitrary prime, clear of per-deal shuffle offsets
