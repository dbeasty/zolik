package botstats

import (
	"sync"
	"testing"
	"time"
)

type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	c.mu.Unlock()
}

var k = Key{Module: "holdem", Skill: "hard", Source: SourceLoop}

func TestQuantilesAreUpperBoundsNeverLow(t *testing.T) {
	c := &fakeClock{now: time.Unix(0, 0)}
	r := newAt(c.Now)
	for i := 1; i <= 100; i++ {
		end := r.Begin(k)
		c.Advance(time.Duration(i) * time.Millisecond)
		end()
	}
	s := r.Snapshot().Total[0]
	if s.Count != 100 {
		t.Fatalf("count %d", s.Count)
	}
	for name, got := range map[string][2]float64{"p50": {s.P50M, 50}, "p95": {s.P95M, 95}, "p99": {s.P99M, 99}} {
		if got[0] < got[1] || got[0] > 2*got[1] {
			t.Errorf("%s = %.1f ms, want within [%.0f, %.0f]", name, got[0], got[1], 2*got[1])
		}
	}
	if s.MaxM != 100 {
		t.Errorf("max %.1f", s.MaxM)
	}
}

func TestASingleSampleReadsAsItself(t *testing.T) {
	c := &fakeClock{now: time.Unix(0, 0)}
	r := newAt(c.Now)
	end := r.Begin(k)
	c.Advance(3 * time.Millisecond)
	end()
	if s := r.Snapshot().Total[0]; s.P99M != 3 {
		t.Errorf("p99 of one 3 ms sample = %.2f", s.P99M)
	}
}

func TestRecentForgetsWhatTotalKeeps(t *testing.T) {
	c := &fakeClock{now: time.Unix(0, 0)}
	r := newAt(c.Now)
	end := r.Begin(k)
	c.Advance(time.Millisecond)
	end()

	c.Advance(window + time.Second)
	if got := r.Snapshot().Recent; len(got) != 1 {
		t.Fatalf("one window later the sample should still be recent, got %v", got)
	}
	c.Advance(2 * window)
	s := r.Snapshot()
	if len(s.Recent) != 0 {
		t.Errorf("two windows later nothing should be recent, got %v", s.Recent)
	}
	if len(s.Total) != 1 || s.Total[0].Count != 1 {
		t.Errorf("total lost the sample: %v", s.Total)
	}
}

func TestOrphansAreCountedUntilTheyReturn(t *testing.T) {
	r := New()
	end := r.Begin(k)
	done := r.Abandoned()
	if s := r.Snapshot(); s.Orphans != 1 || s.InFlight != 1 || s.Timeouts != 1 {
		t.Fatalf("while running: %+v", s)
	}
	end()
	done()
	if s := r.Snapshot(); s.Orphans != 0 || s.InFlight != 0 || s.Timeouts != 1 {
		t.Fatalf("after return: %+v", s)
	}
}

func TestANilRecorderIsInert(t *testing.T) {
	var r *Recorder
	r.Begin(k)()
	r.Abandoned()()
	r.LoopStarted()
	r.LoopEnded()
	if s := r.Snapshot(); s.Loops != 0 || len(s.Total) != 0 {
		t.Fatalf("%+v", s)
	}
}
