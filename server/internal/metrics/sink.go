package metrics

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Sink is what the rest of the server sees. Two methods, both of which touch
// memory and return.
//
// An interface rather than the concrete recorder for the usual reason — a
// handler under test should not need a database — but also because a nil Sink
// has to be usable. Recording is not load-bearing: a match must complete, a
// player must be admitted, whether or not anything is counting. Nop() is what
// every caller that has not been given one falls back to.
type Sink interface {
	// Add folds n into a counter for the current UTC day.
	Add(name string, n int64)
	// SeePlayer notes that this subject played today. Idempotent within a
	// day: calling it five times for the same key is one player.
	SeePlayer(subjectKey string)
}

// Nop is a Sink that discards everything, for tests and for any process that
// has no store behind it.
func Nop() Sink { return nopSink{} }

type nopSink struct{}

func (nopSink) Add(string, int64) {}
func (nopSink) SeePlayer(string)  {}

// FlushInterval is how often accumulated deltas are folded into the day's
// document.
//
// Thirty seconds is chosen against the failure it is exposed to: a crash loses
// whatever has not been flushed. That is an accepted, stated cost, and the
// crash itself is not lost with it — Boot rows are written synchronously and
// do not go through here, so "the process died" survives even when "and here
// is the last half-minute of counters" does not.
const FlushInterval = 30 * time.Second

// Recorder accumulates counters in memory and flushes them on a timer.
//
// The accumulator is keyed by day rather than being a single bucket, because a
// flush that straddles midnight UTC must not post yesterday's last few seconds
// against today. Under load that is a handful of counts; on the day an
// operator is reading a graph to explain an incident, it is the difference
// between a spike at 00:00 and the truth.
type Recorder struct {
	store Store
	clock func() time.Time

	mu      sync.Mutex
	pending map[string]*dayDelta
}

type dayDelta struct {
	counters map[string]int64
	players  map[string]struct{}
	// truncated records that this process stopped admitting new player keys
	// for the day. Merged into the document so the flag survives a restart.
	truncated bool
}

func newDayDelta() *dayDelta {
	return &dayDelta{counters: map[string]int64{}, players: map[string]struct{}{}}
}

// NewRecorder builds a recorder over a store. Start must be called to begin
// flushing; until it is, counters accumulate and nothing is written, which is
// what makes a recorder safe to build before the rest of the process is up.
func NewRecorder(store Store) *Recorder {
	return &Recorder{store: store, clock: time.Now, pending: map[string]*dayDelta{}}
}

var _ Sink = (*Recorder)(nil)

func (r *Recorder) Add(name string, n int64) {
	if r == nil || n == 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.todayLocked().counters[name] += n
}

func (r *Recorder) SeePlayer(subjectKey string) {
	if r == nil || subjectKey == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	d := r.todayLocked()
	if _, seen := d.players[subjectKey]; seen {
		return
	}
	// The cap is checked against this process's own pending set, not against
	// the stored day: the store applies the real ceiling when it merges, and
	// duplicating the check against a number this process cannot see would
	// mean reading the document on every player. What this bound is actually
	// for is the pending map between two flushes.
	if len(d.players) >= PlayerSetCap {
		d.truncated = true
		return
	}
	d.players[subjectKey] = struct{}{}
}

func (r *Recorder) todayLocked() *dayDelta {
	key := DayKey(r.clock())
	d, ok := r.pending[key]
	if !ok {
		d = newDayDelta()
		r.pending[key] = d
	}
	return d
}

// Start runs the flush loop until ctx is cancelled, then flushes once more.
//
// The final flush is why this owns the goroutine rather than the caller: a
// graceful shutdown should not throw away the last half-minute, and the only
// place that knows the loop has stopped is the loop.
func (r *Recorder) Start(ctx context.Context) {
	go func() {
		t := time.NewTicker(FlushInterval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				// A cancelled context cannot be used to write, and this write
				// is the point of the shutdown path.
				r.Flush(context.WithoutCancel(ctx))
				return
			case <-t.C:
				r.Flush(ctx)
			}
		}
	}()
}

// Flush writes every pending day and clears what it wrote.
//
// A day whose write fails is put back rather than dropped, so a database
// blip costs a delayed count and not a lost one. It is put back by *adding*
// to whatever accumulated meanwhile, which is why the deltas are increments
// all the way down: replaying an increment twice would be a real error, and
// nothing here can replay one, because the pending bucket is swapped out
// before the write and only merged back on failure.
func (r *Recorder) Flush(ctx context.Context) {
	r.mu.Lock()
	pending := r.pending
	r.pending = map[string]*dayDelta{}
	r.mu.Unlock()

	for day, d := range pending {
		if len(d.counters) == 0 && len(d.players) == 0 && !d.truncated {
			continue
		}
		players := make([]string, 0, len(d.players))
		for k := range d.players {
			players = append(players, k)
		}
		if err := r.store.Merge(ctx, day, d.counters, players, d.truncated); err != nil {
			slog.Warn("metrics flush failed, counters held for the next attempt",
				"day", day, "counters", len(d.counters), "players", len(players), "error", err)
			r.restore(day, d)
		}
	}
}

// restore folds a failed flush back into the pending map.
func (r *Recorder) restore(day string, d *dayDelta) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cur, ok := r.pending[day]
	if !ok {
		r.pending[day] = d
		return
	}
	for name, n := range d.counters {
		cur.counters[name] += n
	}
	for k := range d.players {
		if len(cur.players) >= PlayerSetCap {
			cur.truncated = true
			break
		}
		cur.players[k] = struct{}{}
	}
	cur.truncated = cur.truncated || d.truncated
}
