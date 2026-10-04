package botstats

import (
	"math"
	"math/bits"
	"time"
)

// Buckets are powers of two of a microsecond, so bucket i holds durations in
// [2^(i-1), 2^i) µs and the last one everything from about 67 s up. A quantile
// read off them is the bucket's upper edge — at most 2x high, never low, which
// is the right direction to be wrong in when the number sizes a budget.
const buckets = 28

type hist struct {
	n      [buckets]int64
	count  int64
	sum    time.Duration
	maxDur time.Duration
}

func bucketOf(d time.Duration) int {
	us := d.Microseconds()
	if us <= 0 {
		return 0
	}
	i := 64 - bits.LeadingZeros64(uint64(us))
	if i >= buckets {
		return buckets - 1
	}
	return i
}

// upper is the largest duration bucket i can hold.
func upper(i int) time.Duration {
	return time.Duration(1<<uint(i)) * time.Microsecond
}

func (h *hist) add(d time.Duration) {
	h.n[bucketOf(d)]++
	h.count++
	h.sum += d
	if d > h.maxDur {
		h.maxDur = d
	}
}

func (h *hist) merge(o *hist) {
	for i := range h.n {
		h.n[i] += o.n[i]
	}
	h.count += o.count
	h.sum += o.sum
	if o.maxDur > h.maxDur {
		h.maxDur = o.maxDur
	}
}

// quantile returns an upper bound on the q-th quantile, capped at the observed
// maximum so a single sample does not read as twice itself.
func (h *hist) quantile(q float64) time.Duration {
	if h.count == 0 {
		return 0
	}
	rank := int64(math.Ceil(q * float64(h.count)))
	if rank < 1 {
		rank = 1
	}
	var seen int64
	for i, c := range h.n {
		seen += c
		if seen >= rank {
			if u := upper(i); u < h.maxDur {
				return u
			}
			return h.maxDur
		}
	}
	return h.maxDur
}

func ms(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }

func (h *hist) series(k Key) Series {
	s := Series{Key: k, Count: h.count, MaxM: ms(h.maxDur)}
	if h.count > 0 {
		s.MeanM = ms(h.sum / time.Duration(h.count))
		s.P50M = ms(h.quantile(0.50))
		s.P95M = ms(h.quantile(0.95))
		s.P99M = ms(h.quantile(0.99))
	}
	return s
}
