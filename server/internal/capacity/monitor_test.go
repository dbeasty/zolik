package capacity

import (
	"testing"
	"time"
)

type script struct{ r Reading }

func (s *script) read() Reading { return s.r }

func newScripted() (*Monitor, *script) {
	s := &script{}
	return New(s.read, DefaultThresholds(), time.Second), s
}

var t0 = time.Unix(1_000_000, 0)

func TestWorseIsAppliedAtOnce(t *testing.T) {
	m, s := newScripted()
	events := m.Subscribe()

	s.r = Reading{CPUOK: true, CPUStall: 0.25}
	m.Step(t0)
	if m.Level() != Red {
		t.Fatalf("level %s, want red", m.Level())
	}
	ev := <-events
	if ev.Previous != Green || ev.Level != Red || ev.Reason != "cpu_pressure" {
		t.Fatalf("event %+v", ev)
	}
}

func TestBetterWaitsForCalmAndStepsOneLevelAtATime(t *testing.T) {
	m, s := newScripted()
	s.r = Reading{MemOK: true, MemFrac: 0.9}
	m.Step(t0)
	if m.Level() != Red {
		t.Fatal("not red")
	}

	s.r = Reading{MemOK: true, MemFrac: 0.1}
	now := t0
	for i := 0; i < 14; i++ { // 28 s of calm, under RecoverAfter
		now = now.Add(2 * time.Second)
		m.Step(now)
	}
	if m.Level() != Red {
		t.Fatalf("recovered after %s of calm, want %s", now.Sub(t0), RecoverAfter)
	}
	now = now.Add(4 * time.Second)
	m.Step(now)
	if m.Level() != Amber {
		t.Fatalf("level %s, want amber: red steps down through amber", m.Level())
	}
	for i := 0; i < 16; i++ {
		now = now.Add(2 * time.Second)
		m.Step(now)
	}
	if m.Level() != Green {
		t.Fatalf("level %s, want green", m.Level())
	}
}

// A signal hovering around a line must not flap the level: one bad sample
// in the recovery period restarts it.
func TestASpikeDuringRecoveryRestartsTheClock(t *testing.T) {
	m, s := newScripted()
	s.r = Reading{CPUOK: true, CPUStall: 0.15}
	m.Step(t0)
	if m.Level() != Amber {
		t.Fatal("not amber")
	}
	now := t0
	for i := 0; i < 60; i++ {
		now = now.Add(2 * time.Second)
		if i%10 == 9 {
			s.r.CPUStall = 0.15
		} else {
			s.r.CPUStall = 0.01
		}
		m.Step(now)
		if m.Level() != Amber {
			t.Fatalf("flapped to %s at step %d", m.Level(), i)
		}
	}
	if st := m.Status(); st.Transitions != 1 {
		t.Fatalf("transitions %d, want 1", st.Transitions)
	}
}

func TestUnreadableGaugesDoNotMoveTheLevel(t *testing.T) {
	m, s := newScripted()
	s.r = Reading{CPUOK: false, CPUStall: 1, MemOK: false, MemFrac: 1}
	m.Step(t0)
	if m.Level() != Green {
		t.Fatalf("level %s from unreadable gauges", m.Level())
	}
}

func TestOverrunNeedsEnoughDecisions(t *testing.T) {
	m, s := newScripted() // think window 1 s
	s.r = Reading{ActP95: 2 * time.Second, ActCount: 3}
	m.Step(t0)
	if m.Level() != Green {
		t.Fatal("three slow decisions are not a trend")
	}
	s.r.ActCount = 50
	m.Step(t0.Add(time.Second))
	if m.Level() != Red {
		t.Fatalf("level %s, want red at 2x the think window", m.Level())
	}
	s.r.ActP95 = 600 * time.Millisecond
	m.Step(t0.Add(2 * time.Second))
	m.Step(t0.Add(2*time.Second + RecoverAfter))
	if m.Level() != Amber {
		t.Fatalf("level %s, want amber at 0.6 of the window", m.Level())
	}
}

// A subscriber that never reads must not stall the monitor, and finds the
// latest event when it does.
func TestASlowSubscriberGetsTheLatest(t *testing.T) {
	m, s := newScripted()
	events := m.Subscribe()
	s.r = Reading{CPUOK: true, CPUStall: 0.15}
	m.Step(t0)
	s.r.CPUStall = 0.3
	m.Step(t0.Add(time.Second))
	ev := <-events
	if ev.Level != Red || ev.Previous != Amber {
		t.Fatalf("got %+v, want the amber->red event", ev)
	}
	select {
	case extra := <-events:
		t.Fatalf("stale event left behind: %+v", extra)
	default:
	}
}

func TestTheDevicesOwnHeatMovesTheLevel(t *testing.T) {
	m, s := newScripted()
	s.r = Reading{Thermal: 1}
	m.Step(t0)
	if m.Level() != Amber {
		t.Fatalf("serious thermal state: %s, want amber", m.Level())
	}
	s.r.Thermal = 2
	m.Step(t0.Add(time.Second))
	if m.Level() != Red {
		t.Fatalf("critical thermal state: %s, want red", m.Level())
	}
}

func TestAPinnedLevelHoldsWhateverTheReadings(t *testing.T) {
	m, s := newScripted()
	m.Pin(Red)
	m.Step(t0)
	if m.Level() != Red {
		t.Fatalf("pinned red read %s", m.Level())
	}
	s.r = Reading{CPUOK: true, CPUStall: 0.5}
	m.Pin(Green)
	m.Step(t0.Add(time.Second))
	if m.Level() != Green {
		t.Fatalf("pinned green read %s", m.Level())
	}
}
