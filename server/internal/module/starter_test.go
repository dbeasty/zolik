package module_test

import (
	"testing"

	"zolik/server/internal/module"
)

// TestStartingSeat_StableAndInRange guards the property every caller leans
// on: the same seed always names the same seat, and that seat is always one
// that actually exists at the table.
func TestStartingSeat_StableAndInRange(t *testing.T) {
	for _, seats := range []int{2, 3, 4, 5, 8, 9} {
		for _, seed := range []int64{0, 1, 42, -7, 123456789} {
			got := module.StartingSeat(seed, seats)
			if got < 0 || got >= seats {
				t.Fatalf("seats=%d seed=%d: seat %d out of range", seats, seed, got)
			}
			if again := module.StartingSeat(seed, seats); again != got {
				t.Fatalf("seats=%d seed=%d: not stable, got %d then %d", seats, seed, got, again)
			}
		}
	}
}

// TestStartingSeat_SpreadsAcrossSeats guards against a fixed offset that
// happens to always land on the same seat — the whole point of the change is
// that a lobby's host stops being the permanent opener.
func TestStartingSeat_SpreadsAcrossSeats(t *testing.T) {
	const seats = 4
	seen := map[int]bool{}
	for seed := int64(0); seed < 200; seed++ {
		seen[module.StartingSeat(seed, seats)] = true
	}
	if len(seen) != seats {
		t.Fatalf("200 seeds only ever produced seats %v, want all %d seats represented", seen, seats)
	}
}

func TestStartingSeat_NoSeats(t *testing.T) {
	if got := module.StartingSeat(5, 0); got != 0 {
		t.Fatalf("zero seats should not panic or go negative, got %d", got)
	}
}
