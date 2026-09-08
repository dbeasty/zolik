package metrics_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"zolik/server/internal/metrics"
)

// fakeStore records what was merged, and can be made to fail.
type fakeStore struct {
	metrics.Store

	mu     sync.Mutex
	merges []merge
	fail   error
}

type merge struct {
	day       string
	counters  map[string]int64
	players   []string
	truncated bool
}

func (f *fakeStore) Merge(_ context.Context, day string, counters map[string]int64, players []string, truncated bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail != nil {
		return f.fail
	}
	f.merges = append(f.merges, merge{day, counters, players, truncated})
	return nil
}

func (f *fakeStore) total(name string) int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	var n int64
	for _, m := range f.merges {
		n += m.counters[name]
	}
	return n
}

func TestRecorderFlushesCounters(t *testing.T) {
	store := &fakeStore{}
	r := metrics.NewRecorder(store)

	r.Add(metrics.MatchesCompleted, 1)
	r.Add(metrics.MatchesCompleted, 2)
	r.Add(metrics.WSConnected, 5)
	r.Flush(t.Context())

	if got := store.total(metrics.MatchesCompleted); got != 3 {
		t.Errorf("matches.completed = %d, want 3", got)
	}
	if got := store.total(metrics.WSConnected); got != 5 {
		t.Errorf("ws.connected = %d, want 5", got)
	}
}

// Flushing twice must not post the same counts again. The store's Merge is an
// increment, so a recorder that failed to clear what it wrote would double
// every number on every tick.
func TestRecorderClearsWhatItWrote(t *testing.T) {
	store := &fakeStore{}
	r := metrics.NewRecorder(store)

	r.Add(metrics.MatchesCompleted, 4)
	r.Flush(t.Context())
	r.Flush(t.Context())

	if got := store.total(metrics.MatchesCompleted); got != 4 {
		t.Errorf("matches.completed = %d after two flushes, want 4", got)
	}
}

// A database blip should cost a delayed count, not a lost one.
func TestRecorderHoldsCountersWhenTheStoreFails(t *testing.T) {
	store := &fakeStore{fail: errors.New("database is away")}
	r := metrics.NewRecorder(store)

	r.Add(metrics.MatchesCompleted, 7)
	r.Flush(t.Context())

	store.mu.Lock()
	store.fail = nil
	store.mu.Unlock()

	// Something else happened while the store was down; the retry must carry
	// both, added together rather than either overwriting the other.
	r.Add(metrics.MatchesCompleted, 1)
	r.Flush(t.Context())

	if got := store.total(metrics.MatchesCompleted); got != 8 {
		t.Errorf("matches.completed = %d, want 8 (7 held + 1 new)", got)
	}
}

func TestRecorderDeduplicatesPlayersWithinAFlush(t *testing.T) {
	store := &fakeStore{}
	r := metrics.NewRecorder(store)

	r.SeePlayer("user:a")
	r.SeePlayer("user:a")
	r.SeePlayer("user:b")
	r.SeePlayer("")
	r.Flush(t.Context())

	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.merges) != 1 {
		t.Fatalf("got %d merges, want 1", len(store.merges))
	}
	if got := len(store.merges[0].players); got != 2 {
		t.Errorf("players = %d (%v), want 2", got, store.merges[0].players)
	}
}

func TestRecorderIgnoresZeroAndNil(t *testing.T) {
	store := &fakeStore{}
	r := metrics.NewRecorder(store)

	r.Add(metrics.MatchesCompleted, 0)
	r.Flush(t.Context())

	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.merges) != 0 {
		t.Errorf("a zero-delta flush wrote %d merges, want none", len(store.merges))
	}

	// Recording is not load-bearing: a nil recorder is a working Sink, so
	// nothing in the game path has to check before counting.
	var nilRec *metrics.Recorder
	nilRec.Add(metrics.MatchesCompleted, 1)
	nilRec.SeePlayer("user:a")

	metrics.Nop().Add(metrics.MatchesCompleted, 1)
	metrics.Nop().SeePlayer("user:a")
}

// The shutdown path must not throw away the last half-minute of counters.
func TestRecorderFlushesOnShutdown(t *testing.T) {
	store := &fakeStore{}
	r := metrics.NewRecorder(store)

	ctx, cancel := context.WithCancel(t.Context())
	r.Start(ctx)
	r.Add(metrics.MatchesCompleted, 2)
	cancel()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if store.total(metrics.MatchesCompleted) == 2 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("shutdown flush never arrived: matches.completed = %d", store.total(metrics.MatchesCompleted))
}

func TestRecorderIsConcurrencySafe(t *testing.T) {
	store := &fakeStore{}
	r := metrics.NewRecorder(store)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				r.Add(metrics.WSConnected, 1)
				r.SeePlayer("user:a")
			}
		}()
	}
	wg.Wait()
	r.Flush(t.Context())

	if got := store.total(metrics.WSConnected); got != 1000 {
		t.Errorf("ws.connected = %d, want 1000", got)
	}
}

func TestDayKeyIsUTC(t *testing.T) {
	// 23:30 in a zone twelve hours ahead is already the next day in UTC. The
	// bucket must follow UTC, because that is what every other timestamp in
	// the statistics stack is bucketed by.
	tz := time.FixedZone("UTC+12", 12*3600)
	local := time.Date(2026, 9, 9, 11, 30, 0, 0, tz)
	if got := metrics.DayKey(local); got != "2026-09-08" {
		t.Errorf("DayKey = %s, want 2026-09-08", got)
	}
}

// brokenStore fails every operation, standing in for a database that is away.
type brokenStore struct{}

func (brokenStore) Merge(context.Context, string, map[string]int64, []string, bool) error {
	return errors.New("store is broken")
}
func (brokenStore) Days(context.Context, string, string) ([]metrics.Day, error) {
	return nil, errors.New("store is broken")
}
func (brokenStore) InsertBoot(context.Context, metrics.Boot) error {
	return errors.New("store is broken")
}
func (brokenStore) LatestOpenBoot(context.Context, string) (metrics.Boot, bool, error) {
	return metrics.Boot{}, false, errors.New("store is broken")
}
func (brokenStore) CloseBoot(context.Context, string, time.Time, string, bool) error {
	return errors.New("store is broken")
}
func (brokenStore) Boots(context.Context, time.Time, time.Time) ([]metrics.Boot, error) {
	return nil, errors.New("store is broken")
}
