// Package capacity watches whether this server still has room to think, and
// tells whoever subscribes when the answer changes.
//
// It is the resource monitor of docs/bot-compute-gating-plan.md §3.4. The
// admission gate asks "is there room?" at the moment somebody arrives; nothing
// was told when the answer changed for the people already playing. This
// samples the same gauges on a timer, folds them into one level — green,
// amber, red — and publishes a transition, so a later phase can move bot
// seats between engines without polling anything itself.
//
// For now nothing acts on a transition. The app subscribes and logs, and the
// level is readable at /debug/capacity: observe mode, so the thresholds are
// tuned against real load before anything depends on them.
package capacity

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"
)

// Level is how much headroom the server has.
type Level int

const (
	Green Level = iota
	Amber
	Red
)

func (l Level) String() string {
	switch l {
	case Amber:
		return "amber"
	case Red:
		return "red"
	default:
		return "green"
	}
}

func (l Level) MarshalText() ([]byte, error) { return []byte(l.String()), nil }

// Reading is one sample of the signals the level is decided from. A signal
// whose OK is false could not be read here (PSI on macOS, a memory limit on an
// unconstrained host) and is ignored rather than read as zero pressure or as
// full pressure: an unreadable gauge must not move the level either way.
type Reading struct {
	CPUStall float64 `json:"cpuStallFraction"`
	CPUOK    bool    `json:"cpuReadable"`
	MemFrac  float64 `json:"memoryFraction"`
	MemOK    bool    `json:"memoryReadable"`
	// ActP95 is the recent 95th percentile of a bot decision, and ActCount
	// how many decisions it was taken over.
	ActP95   time.Duration `json:"actP95"`
	ActCount int64         `json:"actCount"`
}

// Thresholds are where each level begins. A level is entered when any one
// signal reaches its line; it is left only when every signal has been back
// under the lines for the recovery period.
type Thresholds struct {
	AmberCPU float64 `json:"amberCpuStall"`
	RedCPU   float64 `json:"redCpuStall"`
	AmberMem float64 `json:"amberMemoryFraction"`
	RedMem   float64 `json:"redMemoryFraction"`
	// Overrun is the decision p95 as a fraction of the bot's think window:
	// at 1.0 a bot is spending its whole cosmetic pause actually thinking,
	// and the next one is late.
	AmberOverrun float64 `json:"amberOverrun"`
	RedOverrun   float64 `json:"redOverrun"`
	// MinActs is how many recent decisions the overrun needs before it
	// counts, so one slow decision on an idle server is not a trend.
	MinActs int64 `json:"minActs"`
}

// DefaultThresholds sit below admission's own lines (CPU stall 0.25, memory
// 0.85) on purpose: bots should feel pressure before players are turned away.
func DefaultThresholds() Thresholds {
	return Thresholds{
		AmberCPU: 0.10, RedCPU: 0.20,
		AmberMem: 0.75, RedMem: 0.85,
		AmberOverrun: 0.5, RedOverrun: 1.0,
		MinActs: 20,
	}
}

// Event is a level change.
type Event struct {
	Level    Level     `json:"level"`
	Previous Level     `json:"previous"`
	Reason   string    `json:"reason"`
	At       time.Time `json:"at"`
	Reading  Reading   `json:"reading"`
}

// Monitor folds readings into a level. Safe for concurrent use.
type Monitor struct {
	sample  func() Reading
	th      Thresholds
	think   time.Duration
	recover time.Duration

	mu          sync.Mutex
	level       Level
	since       time.Time
	calmSince   time.Time
	last        Reading
	lastAt      time.Time
	transitions int64
	subs        []chan Event
}

// RecoverAfter is how long every signal must stay under a level's lines
// before the monitor steps back down from it. Long next to the two-second
// sample: a monitor that flapped would move seats back and forth.
const RecoverAfter = 30 * time.Second

// New builds a monitor. think is the shortest pause a bot takes before
// answering (the overrun is measured against it); sample is called once per
// Step.
func New(sample func() Reading, th Thresholds, think time.Duration) *Monitor {
	return &Monitor{sample: sample, th: th, think: think, recover: RecoverAfter}
}

// classify is the level a reading alone calls for, and the signal that called
// for it.
func (m *Monitor) classify(r Reading) (Level, string) {
	overrun := 0.0
	if m.think > 0 && r.ActCount >= m.th.MinActs {
		overrun = float64(r.ActP95) / float64(m.think)
	}
	switch {
	case r.CPUOK && r.CPUStall >= m.th.RedCPU:
		return Red, "cpu_pressure"
	case r.MemOK && r.MemFrac >= m.th.RedMem:
		return Red, "memory"
	case overrun >= m.th.RedOverrun && m.th.RedOverrun > 0:
		return Red, "overrun"
	case r.CPUOK && r.CPUStall >= m.th.AmberCPU:
		return Amber, "cpu_pressure"
	case r.MemOK && r.MemFrac >= m.th.AmberMem:
		return Amber, "memory"
	case overrun >= m.th.AmberOverrun && m.th.AmberOverrun > 0:
		return Amber, "overrun"
	}
	return Green, "recovered"
}

// Step takes one sample and moves the level if it should. Worse is applied at
// once; better only after RecoverAfter of calm, and one level at a time, so a
// red server passes through amber on its way back.
func (m *Monitor) Step(now time.Time) {
	r := m.sample()
	target, reason := m.classify(r)

	m.mu.Lock()
	m.last, m.lastAt = r, now
	if m.since.IsZero() {
		m.since = now
	}
	var ev *Event
	switch {
	case target > m.level:
		ev = m.moveLocked(target, reason, now, r)
	case target < m.level:
		if m.calmSince.IsZero() {
			m.calmSince = now
		} else if now.Sub(m.calmSince) >= m.recover {
			next := m.level - 1
			if next == Green {
				reason = "recovered"
			}
			ev = m.moveLocked(next, reason, now, r)
		}
	default:
		m.calmSince = time.Time{}
	}
	subs := m.subs
	m.mu.Unlock()

	if ev != nil {
		for _, ch := range subs {
			publish(ch, *ev)
		}
	}
}

func (m *Monitor) moveLocked(to Level, reason string, now time.Time, r Reading) *Event {
	ev := &Event{Level: to, Previous: m.level, Reason: reason, At: now, Reading: r}
	m.level, m.since, m.calmSince = to, now, time.Time{}
	m.transitions++
	return ev
}

// publish hands a subscriber the latest event without ever blocking the
// monitor: a subscriber that has not read the previous one gets this one in
// its place, which is all a level-follower needs.
func publish(ch chan Event, ev Event) {
	for {
		select {
		case ch <- ev:
			return
		default:
		}
		select {
		case <-ch:
		default:
		}
	}
}

// Subscribe returns a channel that receives every level change from now on,
// coalesced to the latest if the reader falls behind.
func (m *Monitor) Subscribe() <-chan Event {
	ch := make(chan Event, 1)
	m.mu.Lock()
	m.subs = append(m.subs, ch)
	m.mu.Unlock()
	return ch
}

// Level is the current level.
func (m *Monitor) Level() Level {
	if m == nil {
		return Green
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.level
}

// Run steps every interval until ctx is done.
func (m *Monitor) Run(ctx context.Context, interval time.Duration) {
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-t.C:
				m.Step(now)
			}
		}
	}()
}

// Status is the monitor at one moment, for /debug/capacity.
type Status struct {
	Level       Level      `json:"level"`
	Since       time.Time  `json:"since"`
	Transitions int64      `json:"transitions"`
	Last        Reading    `json:"last"`
	LastAt      time.Time  `json:"lastAt"`
	Thresholds  Thresholds `json:"thresholds"`
	ThinkWindow string     `json:"thinkWindow"`
	// GOMAXPROCS is how many cores Go schedules onto. Since Go 1.25 the
	// runtime sizes it from the container's CPU limit and follows changes,
	// so this is the quota as the scheduler sees it.
	GOMAXPROCS int `json:"gomaxprocs"`
	NumCPU     int `json:"numCPU"`
}

// Status reports the monitor.
func (m *Monitor) Status() Status {
	s := Status{GOMAXPROCS: runtime.GOMAXPROCS(0), NumCPU: runtime.NumCPU()}
	if m == nil {
		return s
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	s.Level, s.Since, s.Transitions = m.level, m.since, m.transitions
	s.Last, s.LastAt, s.Thresholds = m.last, m.lastAt, m.th
	s.ThinkWindow = m.think.String()
	return s
}

// String renders an event for a log line.
func (e Event) String() string {
	return fmt.Sprintf("%s -> %s (%s)", e.Previous, e.Level, e.Reason)
}
