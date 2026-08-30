package admission

import (
	"errors"
	"sync"
	"testing"
)

func mustAdmit(t *testing.T, c *Controller, class Class) *Release {
	t.Helper()
	rel, err := c.Admit(class)
	if err != nil {
		t.Fatalf("Admit(%v) refused unexpectedly: %v", class, err)
	}
	return rel
}

func refusedWith(t *testing.T, c *Controller, class Class) Reason {
	t.Helper()
	_, err := c.Admit(class)
	if err == nil {
		t.Fatalf("Admit(%v) was admitted, want refusal", class)
	}
	var rej *Rejection
	if !errors.As(err, &rej) {
		t.Fatalf("Admit(%v) error = %T, want *Rejection", class, err)
	}
	return rej.Reason
}

func TestAdmitsUpToCeilingThenRefuses(t *testing.T) {
	c := newWithGauge(Limits{MaxConnections: 3}, nil)
	for i := 0; i < 3; i++ {
		mustAdmit(t, c, ClassGameplay)
	}
	if got := refusedWith(t, c, ClassGameplay); got != ReasonAtCapacity {
		t.Fatalf("reason = %q, want %q", got, ReasonAtCapacity)
	}
	if live := c.Snapshot().Live; live != 3 {
		t.Fatalf("live = %d after a refusal, want 3 — a refused arrival must not keep its slot", live)
	}
}

func TestReleaseFreesTheSlot(t *testing.T) {
	c := newWithGauge(Limits{MaxConnections: 1}, nil)
	rel := mustAdmit(t, c, ClassGameplay)
	refusedWith(t, c, ClassGameplay)

	rel.Release()
	mustAdmit(t, c, ClassGameplay)
}

// A handler releases from a defer that also runs on paths where the slot was
// never taken. Double-counting a release would slowly convince the server it
// has room it does not have — the bug this guards is a gradual drift back
// towards the OOM the package exists to prevent.
func TestDoubleReleaseIsANoOp(t *testing.T) {
	c := newWithGauge(Limits{MaxConnections: 2}, nil)
	rel := mustAdmit(t, c, ClassGameplay)
	rel.Release()
	rel.Release()
	rel.Release()

	if live := c.Snapshot().Live; live != 0 {
		t.Fatalf("live = %d, want 0", live)
	}
}

func TestReleasingARefusedHandleIsANoOp(t *testing.T) {
	c := newWithGauge(Limits{MaxConnections: 1}, nil)
	held := mustAdmit(t, c, ClassGameplay)

	rel, err := c.Admit(ClassGameplay)
	if err == nil {
		t.Fatal("second Admit should have been refused")
	}
	rel.Release() // the caller deferred this before checking err

	if live := c.Snapshot().Live; live != 1 {
		t.Fatalf("live = %d, want 1 — releasing a refused handle must not free someone else's slot", live)
	}
	held.Release()
}

// The waiting room is what gets shed first: a player idling in the lobby has
// no game to lose, and someone with a match under way does.
func TestWaitingRoomIsRefusedBeforeGameplay(t *testing.T) {
	// Ceiling 10, waiting room closes at 8.
	c := newWithGauge(Limits{MaxConnections: 10, WaitingRoomRatio: 0.8}, nil)
	for i := 0; i < 8; i++ {
		mustAdmit(t, c, ClassGameplay)
	}

	if got := refusedWith(t, c, ClassWaiting); got != ReasonWaitingRoomClosed {
		t.Fatalf("reason = %q, want %q", got, ReasonWaitingRoomClosed)
	}
	// ...while gameplay still gets the reserved slice.
	mustAdmit(t, c, ClassGameplay)
	mustAdmit(t, c, ClassGameplay)
	if got := refusedWith(t, c, ClassGameplay); got != ReasonAtCapacity {
		t.Fatalf("reason = %q, want %q", got, ReasonAtCapacity)
	}
}

func TestMemoryPressureRefusesEvenWithCountHeadroom(t *testing.T) {
	// Well under the count ceiling, but 90% of memory is gone.
	c := newWithGauge(
		Limits{MaxConnections: 1000, MemoryHighWatermark: 0.85},
		newStubGauge(900, 1000, true),
	)
	if got := refusedWith(t, c, ClassGameplay); got != ReasonMemoryPressure {
		t.Fatalf("reason = %q, want %q", got, ReasonMemoryPressure)
	}
}

func TestBelowWatermarkAdmits(t *testing.T) {
	c := newWithGauge(
		Limits{MaxConnections: 1000, MemoryHighWatermark: 0.85},
		newStubGauge(500, 1000, true),
	)
	mustAdmit(t, c, ClassGameplay)
}

// An unreadable gauge must not be able to wedge the server shut. Dev machines
// and unconstrained hosts have no cgroup limit to read, and the server has to
// keep working there.
func TestUnreadableGaugeLeavesTheGateOpen(t *testing.T) {
	c := newWithGauge(
		Limits{MemoryHighWatermark: 0.85},
		newStubGauge(0, 0, false),
	)
	for i := 0; i < 50; i++ {
		mustAdmit(t, c, ClassGameplay)
	}
	if !c.Snapshot().Accepting {
		t.Fatal("Accepting = false with no readable limit, want true")
	}
}

func TestZeroWatermarkDisablesTheMemoryGate(t *testing.T) {
	c := newWithGauge(
		Limits{MemoryHighWatermark: 0},
		newStubGauge(999, 1000, true),
	)
	mustAdmit(t, c, ClassGameplay)
}

// The reservation is what stops two arrivals from both reading "one slot
// left" and both taking it.
func TestConcurrentAdmitsNeverExceedTheCeiling(t *testing.T) {
	const ceiling = 50
	c := newWithGauge(Limits{MaxConnections: ceiling}, nil)

	var wg sync.WaitGroup
	var mu sync.Mutex
	admitted := 0
	for i := 0; i < 500; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := c.Admit(ClassGameplay); err == nil {
				mu.Lock()
				admitted++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if admitted != ceiling {
		t.Fatalf("admitted = %d, want exactly %d", admitted, ceiling)
	}
	if live := c.Snapshot().Live; live != ceiling {
		t.Fatalf("live = %d, want %d", live, ceiling)
	}
}

func TestSnapshotReportsPressure(t *testing.T) {
	c := newWithGauge(
		Limits{MaxConnections: 2, MemoryHighWatermark: 0.85},
		newStubGauge(500, 1000, true),
	)
	mustAdmit(t, c, ClassGameplay)
	mustAdmit(t, c, ClassGameplay)
	refusedWith(t, c, ClassGameplay)

	s := c.Snapshot()
	if s.Accepting {
		t.Fatal("Accepting = true at the ceiling, want false")
	}
	if s.WaitingRoomOpen {
		t.Fatal("WaitingRoomOpen = true at the ceiling, want false")
	}
	if s.Refused[ReasonAtCapacity] != 1 {
		t.Fatalf("Refused[%s] = %d, want 1", ReasonAtCapacity, s.Refused[ReasonAtCapacity])
	}
	if s.MemoryFraction != 0.5 {
		t.Fatalf("MemoryFraction = %v, want 0.5", s.MemoryFraction)
	}
}

// CPU pressure sheds only growth. A gameplay socket is someone joining or
// returning to a table that already exists; refusing it saves too little CPU
// to justify stranding them.
func TestCPUPressureRefusesWaitingButNotGameplay(t *testing.T) {
	c := newWithGauges(
		Limits{CPUHighWatermark: 0.25},
		nil,
		newStubCPUGauge(0.40, true),
	)
	if got := refusedWith(t, c, ClassWaiting); got != ReasonCPUPressure {
		t.Fatalf("reason = %q, want %q", got, ReasonCPUPressure)
	}
	s := c.Snapshot()
	if s.WaitingRoomOpen {
		t.Fatal("WaitingRoomOpen = true under CPU pressure, want false")
	}
	if !s.Accepting {
		t.Fatal("Accepting = false under CPU pressure, want true — gameplay is not shed")
	}
	mustAdmit(t, c, ClassGameplay)
}

func TestCPUBelowWatermarkAdmitsWaiting(t *testing.T) {
	c := newWithGauges(
		Limits{CPUHighWatermark: 0.25},
		nil,
		newStubCPUGauge(0.10, true),
	)
	mustAdmit(t, c, ClassWaiting)
}

// PSI is Linux-only; a dev Mac or an old kernel reads nothing, and that must
// leave the gate open — the same rule the memory gauge follows.
func TestUnreadableCPUGaugeLeavesTheGateOpen(t *testing.T) {
	c := newWithGauges(
		Limits{CPUHighWatermark: 0.25},
		nil,
		newStubCPUGauge(0, false),
	)
	mustAdmit(t, c, ClassWaiting)
	if err := c.AllowMatchStart(); err != nil {
		t.Fatalf("AllowMatchStart refused with unreadable PSI: %v", err)
	}
}

func TestMatchStartRefusedUnderCPUPressure(t *testing.T) {
	c := newWithGauges(
		Limits{CPUHighWatermark: 0.25},
		nil,
		newStubCPUGauge(0.40, true),
	)
	err := c.AllowMatchStart()
	var rej *Rejection
	if !errors.As(err, &rej) || rej.Reason != ReasonCPUPressure {
		t.Fatalf("AllowMatchStart error = %v, want %s rejection", err, ReasonCPUPressure)
	}
}

func TestMatchStartRefusedUnderMemoryPressure(t *testing.T) {
	c := newWithGauge(
		Limits{MemoryHighWatermark: 0.85},
		newStubGauge(900, 1000, true),
	)
	err := c.AllowMatchStart()
	var rej *Rejection
	if !errors.As(err, &rej) || rej.Reason != ReasonMemoryPressure {
		t.Fatalf("AllowMatchStart error = %v, want %s rejection", err, ReasonMemoryPressure)
	}
}

// New matches stop at the waiting-room ceiling, not the hard one: the
// reserved tail of capacity is for finishing games, not starting more.
func TestMatchStartStopsAtTheWaitingCeiling(t *testing.T) {
	c := newWithGauge(Limits{MaxConnections: 10, WaitingRoomRatio: 0.8}, nil)
	for i := 0; i < 8; i++ {
		mustAdmit(t, c, ClassGameplay)
	}
	if err := c.AllowMatchStart(); err == nil {
		t.Fatal("AllowMatchStart allowed at the waiting ceiling, want refusal")
	}
	// The refusal reserved nothing: gameplay sockets still fit.
	mustAdmit(t, c, ClassGameplay)
}

// AllowMatchStart holds no slot — asking many times must not consume
// capacity the way Admit does.
func TestMatchStartReservesNothing(t *testing.T) {
	c := newWithGauge(Limits{MaxConnections: 5}, nil)
	for i := 0; i < 100; i++ {
		if err := c.AllowMatchStart(); err != nil {
			t.Fatalf("AllowMatchStart refused on iteration %d: %v", i, err)
		}
	}
	if live := c.Snapshot().Live; live != 0 {
		t.Fatalf("live = %d after AllowMatchStart calls, want 0", live)
	}
}

func TestNilControllerAllowsMatchStart(t *testing.T) {
	var c *Controller
	if err := c.AllowMatchStart(); err != nil {
		t.Fatalf("nil controller refused a match start: %v", err)
	}
}

// Snapshot answers "would a match start be allowed" without a probe counting
// as a refused player.
func TestSnapshotDoesNotCountProbesAsRefusals(t *testing.T) {
	c := newWithGauge(
		Limits{MemoryHighWatermark: 0.85},
		newStubGauge(900, 1000, true),
	)
	s := c.Snapshot()
	if s.StartingMatches {
		t.Fatal("StartingMatches = true under memory pressure, want false")
	}
	if n := len(s.Refused); n != 0 {
		t.Fatalf("Refused has %d entries after a Snapshot, want 0 — probes are not players", n)
	}
}

// A reconnect is never refused, but it is counted: the displaced handler
// releases its own slot on the way out, and without the reconnect taking one
// the ledger would drift downward until an occupied server read as empty.
func TestReconnectIsCountedNotRefused(t *testing.T) {
	c := newWithGauge(Limits{MaxConnections: 1}, nil)
	old := mustAdmit(t, c, ClassGameplay)

	// At the ceiling, a reconnect still gets in...
	fresh := c.AdmitReconnect()
	if live := c.Snapshot().Live; live != 2 {
		t.Fatalf("live = %d during the handover, want 2", live)
	}
	// ...and once the displaced handler unwinds, the count settles back to
	// one slot per open socket.
	old.Release()
	if live := c.Snapshot().Live; live != 1 {
		t.Fatalf("live = %d after the handover, want 1", live)
	}
	fresh.Release()
	if live := c.Snapshot().Live; live != 0 {
		t.Fatalf("live = %d after the socket closed, want 0", live)
	}
}

func TestNilControllerAdmitReconnect(t *testing.T) {
	var c *Controller
	c.AdmitReconnect().Release() // must not panic
}

func TestZeroLimitsAdmitEverything(t *testing.T) {
	c := newWithGauge(Limits{}, nil)
	for i := 0; i < 1000; i++ {
		mustAdmit(t, c, ClassWaiting)
	}
}

func TestDeriveMaxConnections(t *testing.T) {
	// 512 MiB, 45 MiB baseline, 65 KiB per connection, 85% watermark — the
	// shape of the box these numbers were measured on. The answer has to sit
	// meaningfully below the ~7,000 where that box was OOM-killed.
	got := DeriveMaxConnections(512<<20, 45<<20, 65<<10, 0.85)
	if got < 3000 || got > 6500 {
		t.Fatalf("DeriveMaxConnections = %d, want a ceiling between 3000 and 6500", got)
	}
}

func TestDeriveMaxConnectionsWithoutALimitIsUnbounded(t *testing.T) {
	if got := DeriveMaxConnections(0, 45<<20, 65<<10, 0.85); got != 0 {
		t.Fatalf("DeriveMaxConnections = %d with no limit, want 0 (no ceiling)", got)
	}
}

// A box too small to fit even its own baseline must report "no ceiling"
// rather than a negative one that would wrap into a huge positive.
func TestDeriveMaxConnectionsWhenBaselineExceedsLimit(t *testing.T) {
	if got := DeriveMaxConnections(32<<20, 45<<20, 65<<10, 0.85); got != 0 {
		t.Fatalf("DeriveMaxConnections = %d, want 0", got)
	}
}
