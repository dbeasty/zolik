package app

import "testing"

// The ordering these two thresholds are in is the whole point, and it is the
// kind of thing that gets swapped back by a well-meaning edit to one constant.
//
// Collector first, gate second: the runtime must be pushed to work at a line
// below the one the admission gate defends, so that a player is refused only
// when collecting could not get under it. The other way round — which is how
// this started — the gate turns people away to protect memory the collector was
// explicitly told it could still use, and has not got to yet.
func TestGoMemLimitSitsBelowTheAdmissionWatermark(t *testing.T) {
	limit := uint64(2) << 30

	for _, watermark := range []float64{0.7, 0.85, 0.9, 0.95} {
		gate := int64(float64(limit) * watermark)
		soft := goMemLimitFor(limit, watermark)
		if soft >= gate {
			t.Errorf(
				"watermark %.2f: GOMEMLIMIT is %d MiB and the gate refuses at %d MiB. The "+
					"collector must be told to work before players are turned away, not after.",
				watermark, soft>>20, gate>>20,
			)
		}
	}
}

// A gate that has been turned off does not mean a ceiling that has gone away.
// The cgroup limit is still real and the collector should still know about it.
func TestGoMemLimitStillSetWithNoWatermark(t *testing.T) {
	// A variable, not a constant: `int64(float64(const) * const)` is a
	// constant expression, and Go refuses to truncate one silently.
	limit := uint64(1) << 30

	soft := goMemLimitFor(limit, 0)
	if soft <= 0 || soft >= int64(limit) {
		t.Fatalf("GOMEMLIMIT is %d with the gate off; want a real fraction of the %d MiB limit",
			soft, limit>>20)
	}
	if want := int64(float64(limit) * goMemLimitFallbackFraction); soft != want {
		t.Errorf("GOMEMLIMIT is %d MiB with the gate off, want the fallback %d MiB", soft>>20, want>>20)
	}
}

// A watermark set absurdly low is a misconfigured gate. It should not also
// become a collector that can never let the heap breathe.
func TestGoMemLimitIsFloored(t *testing.T) {
	limit := uint64(1) << 30

	soft := goMemLimitFor(limit, 0.1)
	if want := int64(float64(limit) * goMemLimitMinFraction); soft != want {
		t.Errorf("GOMEMLIMIT is %d MiB under a 0.10 watermark, want the floor at %d MiB",
			soft>>20, want>>20)
	}
}
