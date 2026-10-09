package botgov

import (
	"time"
)

// DefaultCosts are the mean CPU per decision of every class that measured
// above the floor, from `cmd/botcost -learned` (docs/bot-compute-gating-plan.md
// §5.4 and §5.6, 2026-10-04, one Apple-silicon core, CPU columns). Each is the worst configuration of the
// class — the largest table, the dearest variation — because a lease has to
// cover the seat wherever it sits.
//
// Rerun botcost whenever an engine changes and update these: the Hold'em rule
// bot moved 75x in four days once, and a stale table would lease the wrong
// seats. Classes not listed are cheap (under 1 ms) and never need a lease.
func DefaultCosts() map[Class]time.Duration {
	return map[Class]time.Duration{
		// Mariáš Hard is a rule bot, and still the dearest class at most
		// tables: the allocation-free trick search (#247) took it from 54 ms
		// to 15 ms, with p95 near 100 ms.
		{"marias", "hard", EngineRule}: 15 * time.Millisecond,
		// Žolíky's AI seat (the network) at an eight-seat classic table, the
		// one configuration whose tail runs to a quarter of a second.
		{"zolik", "ai", EngineNet}: 12 * time.Millisecond,
		// Lóra Hard plays every choice out over 24 sampled deals, and the
		// maturant's choice plays out each game in turn.
		{"lora", "hard", EngineRule}: 3 * time.Millisecond,
		// Below the floor today, listed so a slower machine (Speed) or a
		// lower floor brings them in without a code change.
		{"canasta", "ai", EngineNet}:    1340 * time.Microsecond,
		{"holdem", "ai", EngineNet}:     480 * time.Microsecond,
		{"zolik", "hard", EngineRule}:   890 * time.Microsecond,
		{"zolik", "medium", EngineRule}: 750 * time.Microsecond,
		{"zolik", "easy", EngineRule}:   700 * time.Microsecond,
		// Ferbl Hard samples the hidden cards, weighted by how each player
		// has bet; six seats is the dearest table.
		{"ferbl", "hard", EngineRule}: 1310 * time.Microsecond,
		// Sedma Hard plays every choice out over 24 sampled deals; the
		// two-seat table, with the longest stock, is the dearest.
		{"sedma", "hard", EngineRule}: 1220 * time.Microsecond,
		// Šnaps Hard solves the endgame over sampled hands; Šedesát šest,
		// with six cards a hand, is the dearer variation (p99 6 ms).
		{"snaps", "hard", EngineRule}: 990 * time.Microsecond,
	}
}

// Calibrate measures this machine against the one DefaultCosts was measured
// on, as a multiplier for Config.Speed: 1 there, 3 on a phone three times
// slower. It times a fixed integer workload shaped like a bot's inner loop —
// hashing, branching, small-slice writes — so it reads the core's speed
// rather than its memory system, takes a few milliseconds, and is clamped to
// [0.5, 20] so one descheduled sample cannot make the server think it is a
// toaster.
func Calibrate() float64 {
	best := time.Duration(1<<63 - 1)
	for i := 0; i < 5; i++ {
		t0 := time.Now()
		calibrationSink += workload()
		if d := time.Since(t0); d < best {
			best = d
		}
	}
	f := float64(best) / float64(referenceWorkload)
	switch {
	case f < 0.5:
		return 0.5
	case f > 20:
		return 20
	}
	return f
}

// referenceWorkload is workload's best-of-five time on the machine that
// measured DefaultCosts.
const referenceWorkload = 1450 * time.Microsecond

var calibrationSink uint64

func workload() uint64 {
	var buf [64]uint32
	x := uint64(0x9E3779B97F4A7C15)
	for i := 0; i < 400_000; i++ {
		x ^= x << 13
		x ^= x >> 7
		x ^= x << 17
		j := x & 63
		if x&1 == 0 {
			buf[j] += uint32(x)
		} else {
			buf[j] ^= uint32(x >> 32)
		}
	}
	var s uint64
	for _, v := range buf {
		s += uint64(v)
	}
	return s
}
