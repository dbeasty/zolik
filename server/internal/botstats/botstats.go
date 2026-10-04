// Package botstats measures what bot decisions cost on a live server.
//
// It exists to replace an estimate with a number. The plan for gating deep
// agents on available CPU (docs/bot-compute-gating-plan.md) sizes a lease pool
// from "CPU per decision", and every figure for that so far has been read off
// the code rather than off a running process. This records one: how long each
// Bot.Act took, per game and per skill, how many are running right now, how
// many were abandoned by the runtime's timeout and are still burning a core,
// and what share of the process's CPU all of that adds up to.
//
// Wall time stands in for CPU time. Go has no per-goroutine CPU clock, and a
// bot decision is pure computation with no I/O, so the two differ only by how
// long the goroutine sat runnable waiting for a core — which is itself the
// contention this package is here to notice. Read a p99 that climbs while the
// process's CPU share does not as exactly that.
//
// Recording is not load-bearing. A nil *Recorder accepts every call and keeps
// nothing, so a Manager built without one (every test) behaves as before.
package botstats

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// Source is where a decision was asked for.
type Source string

const (
	// SourceLoop is a seat's own move, from the runtime's bot loop.
	SourceLoop Source = "loop"
	// SourceHint is a suggestion computed for a person, inside their request.
	SourceHint Source = "hint"
)

// Key names one series.
type Key struct {
	Module string `json:"module"`
	Skill  string `json:"skill"`
	Source Source `json:"source"`
}

// window is how far back Recent looks. Two generations of this length are
// kept, so the recent view always covers between one and two windows.
const window = time.Minute

// Recorder accumulates decision timings. Safe for concurrent use.
type Recorder struct {
	clock func() time.Time
	start time.Time

	mu      sync.Mutex
	total   map[Key]*hist
	cur     map[Key]*hist
	prev    map[Key]*hist
	rotated time.Time

	inFlight atomic.Int64
	orphans  atomic.Int64
	timeouts atomic.Int64
	loops    atomic.Int64
	actNanos atomic.Int64
}

// New builds a recorder.
func New() *Recorder { return newAt(time.Now) }

func newAt(clock func() time.Time) *Recorder {
	now := clock()
	return &Recorder{
		clock:   clock,
		start:   now,
		total:   map[Key]*hist{},
		cur:     map[Key]*hist{},
		prev:    map[Key]*hist{},
		rotated: now,
	}
}

// Begin marks a decision as started and returns the function that ends it.
// The returned func must be called exactly once, from wherever Act actually
// returned — which, for a call the runtime has given up on, is long after the
// runtime stopped waiting. That is the point: the cost is recorded as it was,
// not as the runtime saw it.
func (r *Recorder) Begin(k Key) (end func()) {
	if r == nil {
		return func() {}
	}
	r.inFlight.Add(1)
	t0 := r.clock()
	return func() {
		d := r.clock().Sub(t0)
		r.inFlight.Add(-1)
		r.actNanos.Add(int64(d))
		r.observe(k, d)
	}
}

// Abandoned notes that the runtime stopped waiting for a decision that is
// still running. Call the returned func when that decision finally returns.
func (r *Recorder) Abandoned() (finished func()) {
	if r == nil {
		return func() {}
	}
	r.timeouts.Add(1)
	r.orphans.Add(1)
	return func() { r.orphans.Add(-1) }
}

// LoopStarted and LoopEnded track how many bot loops are alive. Loops spend
// almost all their time asleep; the count is here to relate to the others, not
// because a loop costs anything by itself.
func (r *Recorder) LoopStarted() {
	if r != nil {
		r.loops.Add(1)
	}
}

func (r *Recorder) LoopEnded() {
	if r != nil {
		r.loops.Add(-1)
	}
}

func (r *Recorder) observe(k Key, d time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rotateLocked()
	for _, m := range []map[Key]*hist{r.total, r.cur} {
		h, ok := m[k]
		if !ok {
			h = &hist{}
			m[k] = h
		}
		h.add(d)
	}
}

func (r *Recorder) rotateLocked() {
	now := r.clock()
	switch el := now.Sub(r.rotated); {
	case el >= 2*window:
		// Nothing in either generation is recent any more.
		r.prev, r.cur, r.rotated = map[Key]*hist{}, map[Key]*hist{}, now
	case el >= window:
		r.prev, r.cur, r.rotated = r.cur, map[Key]*hist{}, now
	}
}

// Series is one key's summary.
type Series struct {
	Key
	Count int64   `json:"count"`
	MeanM float64 `json:"meanMs"`
	P50M  float64 `json:"p50Ms"`
	P95M  float64 `json:"p95Ms"`
	P99M  float64 `json:"p99Ms"`
	MaxM  float64 `json:"maxMs"`
}

// Snapshot is the whole recorder at one moment.
type Snapshot struct {
	Uptime float64 `json:"uptimeSeconds"`
	// InFlight counts Act calls running now, orphans included.
	InFlight int64 `json:"inFlight"`
	// Orphans counts calls the runtime timed out on that have not returned
	// yet: CPU nobody is waiting for.
	Orphans  int64 `json:"orphans"`
	Timeouts int64 `json:"timeoutsTotal"`
	Loops    int64 `json:"botLoops"`
	// ActSeconds is the summed wall time of every finished decision.
	ActSeconds float64 `json:"actSeconds"`
	// ProcessCPUSeconds is user+system CPU of the whole process since it
	// started, so ActSeconds / ProcessCPUSeconds is roughly the share of
	// this server's CPU that went on bots.
	ProcessCPUSeconds float64 `json:"processCpuSeconds"`
	BotCPUShare       float64 `json:"botCpuShare"`
	// Total is since the process started; Recent covers the last one to two
	// minutes, which is what a live gate would read.
	Total  []Series `json:"total"`
	Recent []Series `json:"recent"`
}

// Snapshot summarises the recorder. A nil recorder yields an empty snapshot.
func (r *Recorder) Snapshot() Snapshot {
	if r == nil {
		return Snapshot{}
	}
	r.mu.Lock()
	r.rotateLocked()
	total := summarise(r.total)
	recent := map[Key]*hist{}
	for _, m := range []map[Key]*hist{r.prev, r.cur} {
		for k, h := range m {
			acc, ok := recent[k]
			if !ok {
				acc = &hist{}
				recent[k] = acc
			}
			acc.merge(h)
		}
	}
	r.mu.Unlock()

	s := Snapshot{
		Uptime:     r.clock().Sub(r.start).Seconds(),
		InFlight:   r.inFlight.Load(),
		Orphans:    r.orphans.Load(),
		Timeouts:   r.timeouts.Load(),
		Loops:      r.loops.Load(),
		ActSeconds: time.Duration(r.actNanos.Load()).Seconds(),
		Total:      total,
		Recent:     summarise(recent),
	}
	s.ProcessCPUSeconds = ProcessCPU().Seconds()
	if s.ProcessCPUSeconds > 0 {
		s.BotCPUShare = math.Min(1, s.ActSeconds/s.ProcessCPUSeconds)
	}
	return s
}

func summarise(m map[Key]*hist) []Series {
	out := make([]Series, 0, len(m))
	for k, h := range m {
		out = append(out, h.series(k))
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i].Key, out[j].Key
		if a.Module != b.Module {
			return a.Module < b.Module
		}
		if a.Source != b.Source {
			return a.Source < b.Source
		}
		return a.Skill < b.Skill
	})
	return out
}

// LogEvery writes one summary line per interval in which any decision
// finished, until ctx is done. Production runs with the debug endpoints off,
// so this is how its numbers reach anyone: the log is the one channel an
// operator already reads.
func (r *Recorder) LogEvery(ctx context.Context, interval time.Duration) {
	if r == nil || interval <= 0 {
		return
	}
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		var last map[Key]int64
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				s := r.Snapshot()
				now := map[Key]int64{}
				var parts []any
				for _, x := range s.Total {
					now[x.Key] = x.Count
					if x.Count == last[x.Key] {
						continue
					}
					parts = append(parts, x.Module+"/"+string(x.Source)+"/"+x.Skill,
						fmt.Sprintf("n=%d p95=%.1fms p99=%.1fms max=%.1fms", x.Count-last[x.Key], x.P95M, x.P99M, x.MaxM))
				}
				last = now
				if len(parts) == 0 {
					continue
				}
				slog.Info("bot decisions",
					append([]any{"loops", s.Loops, "inFlight", s.InFlight, "orphans", s.Orphans,
						"timeouts", s.Timeouts, "botCpuShare", fmt.Sprintf("%.3f", s.BotCPUShare)}, parts...)...)
			}
		}
	}()
}

// RecentQuantile is the q-th quantile of every recent decision from one
// source, across games and skills, and how many decisions it was taken over.
// It is what a resource monitor reads: one number for "how long are bots
// taking right now", not a table.
func (r *Recorder) RecentQuantile(src Source, q float64) (time.Duration, int64) {
	if r == nil {
		return 0, 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rotateLocked()
	var acc hist
	for _, m := range []map[Key]*hist{r.prev, r.cur} {
		for k, h := range m {
			if k.Source == src {
				acc.merge(h)
			}
		}
	}
	return acc.quantile(q), acc.count
}
