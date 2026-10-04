package botgov

import (
	"fmt"
	"testing"
	"time"

	"zolik/server/internal/capacity"
)

var (
	t0         = time.Unix(1_000_000, 0)
	mariasHard = Class{"marias", "hard", EngineRule}
	mariasMed  = Class{"marias", "medium", EngineRule}
	zolikNet   = Class{"zolik", "hard", EngineNet}
	zolikHard  = Class{"zolik", "hard", EngineRule}
	prsiHard   = Class{"prsi", "hard", EngineRule}
)

// A governor with room for exactly n Mariáš Hard seats at green.
func roomFor(n int, mode Mode) *Governor {
	think := 1350 * time.Millisecond
	w := float64(54*time.Millisecond) / float64(think)
	return New(Config{Mode: mode, Cores: float64(n) * w / (0.5 * 0.7), Think: think})
}

func TestCheapClassesNeverNeedALease(t *testing.T) {
	g := roomFor(1, Enforce)
	g.SetLevel(capacity.Red)
	if d := g.Decide("m", "b", prsiHard, 0, false, t0); d.Reduced || d.Class != prsiHard {
		t.Fatalf("a cheap class was reduced: %v", d)
	}
}

func TestSeatsBeyondCapacityPlayTheFallback(t *testing.T) {
	g := roomFor(2, Enforce)
	for i := 0; i < 2; i++ {
		if d := g.Decide(fmt.Sprint("m", i), "b", mariasHard, 0, false, t0); d.Reduced {
			t.Fatalf("seat %d of 2 was reduced", i)
		}
	}
	d := g.Decide("m2", "b", mariasHard, 0, false, t0)
	if !d.Reduced || d.Class != mariasMed {
		t.Fatalf("third seat: %v, want marias/medium reduced", d)
	}
	if !g.Reduced("m2", "b") || g.Reduced("m0", "b") {
		t.Fatal("Reduced disagrees with the decisions")
	}
}

func TestAModelFallsBackToTheRuleBotAtTheSameSkill(t *testing.T) {
	g := New(Config{Mode: Enforce, Cores: 16})
	g.SetLevel(capacity.Red)
	d := g.Decide("m", "b", zolikNet, 0, false, t0)
	if d.Class != zolikHard {
		t.Fatalf("got %v, want zolik/hard/rule: the model's fallback is the heuristic, which is cheap", d)
	}
}

// Downgrades wait for the seat's next turn. A seat in the middle of a turn —
// it made the table's last move — finishes it with the engine it started.
func TestARevokedSeatFinishesItsTurnFirst(t *testing.T) {
	g := roomFor(1, Enforce)
	g.Decide("m", "b", mariasHard, 0, true, t0)
	g.Moved("m", "b") // the seat played; its turn continues

	g.SetLevel(capacity.Red)
	if d := g.Decide("m", "b", mariasHard, 0, true, t0); d.Reduced {
		t.Fatal("downgraded mid-turn")
	}
	g.Moved("m", "p1") // someone else moved: the bot's next decision opens a turn
	if d := g.Decide("m", "b", mariasHard, 0, true, t0); !d.Reduced {
		t.Fatal("not downgraded at its next turn")
	}
	if st := g.Status(); st.Leases != 0 || st.LeasedCores != 0 {
		t.Fatalf("the lease was not given back: %+v", st)
	}
}

// Upgrades wait for a round boundary, so an opponent never gets stronger in
// the middle of a hand.
func TestAnUpgradeWaitsForTheNextRound(t *testing.T) {
	g := roomFor(1, Enforce)
	g.SetLevel(capacity.Red)
	if d := g.Decide("m", "b", mariasHard, 3, true, t0); !d.Reduced {
		t.Fatal("not reduced at red")
	}
	g.SetLevel(capacity.Green)
	g.Moved("m", "p1")
	if d := g.Decide("m", "b", mariasHard, 3, true, t0); !d.Reduced {
		t.Fatal("upgraded mid-round")
	}
	g.Moved("m", "p1")
	if d := g.Decide("m", "b", mariasHard, 4, true, t0); d.Reduced {
		t.Fatal("not upgraded when the round moved on")
	}
}

func TestRevocationTakesBotOnlyTablesFirstThenTheNewest(t *testing.T) {
	g := roomFor(3, Enforce)
	g.Decide("old-human", "b", mariasHard, 0, true, t0)
	g.Decide("bots", "b", mariasHard, 0, false, t0.Add(time.Second))
	g.Decide("new-human", "b", mariasHard, 0, true, t0.Add(2*time.Second))

	g.SetLevel(capacity.Amber) // room for 1.5 seats: two must go
	for _, m := range []string{"old-human", "bots", "new-human"} {
		g.Moved(m, "someone-else")
	}
	got := map[string]bool{}
	for _, m := range []string{"old-human", "bots", "new-human"} {
		got[m] = g.Decide(m, "b", mariasHard, 0, m != "bots", t0.Add(3*time.Second)).Reduced
	}
	want := map[string]bool{"bots": true, "new-human": true, "old-human": false}
	for m, w := range want {
		if got[m] != w {
			t.Errorf("%s reduced=%v, want %v", m, got[m], w)
		}
	}
}

func TestObserveDecidesButChangesNothing(t *testing.T) {
	g := roomFor(1, Observe)
	g.SetLevel(capacity.Red)
	d := g.Decide("m", "b", mariasHard, 0, false, t0)
	if d.Reduced || d.Class != mariasHard {
		t.Fatalf("observe mode changed a move: %v", d)
	}
	st := g.Status()
	if st.ReducedSeats != 1 || st.ReducedTurns != 1 {
		t.Fatalf("observe mode did not count what it would have done: %+v", st)
	}
	if g.Reduced("m", "b") {
		t.Fatal("observe mode reported a seat as reduced")
	}
}

func TestOffIsInert(t *testing.T) {
	var nilGov *Governor
	if d := nilGov.Decide("m", "b", mariasHard, 0, false, t0); d.Reduced {
		t.Fatal("nil governor reduced")
	}
	g := roomFor(1, Off)
	g.SetLevel(capacity.Red)
	if d := g.Decide("m", "b", mariasHard, 0, false, t0); d.Reduced {
		t.Fatal("off governor reduced")
	}
}

// The operator switching a game's model on or off changes what a seat asks
// for; the old lease is returned rather than leaked.
func TestAChangedRequestReturnsTheOldLease(t *testing.T) {
	g := New(Config{Mode: Enforce, Cores: 16})
	g.Decide("m", "b", zolikNet, 0, false, t0)
	if g.Status().Leases != 1 {
		t.Fatal("no lease for the model seat")
	}
	g.Moved("m", "x")
	g.Decide("m", "b", zolikHard, 0, false, t0)
	if st := g.Status(); st.Leases != 0 || st.LeasedCores != 0 {
		t.Fatalf("lease leaked across a changed request: %+v", st)
	}
}

func TestIdleSeatsAndFinishedMatchesGiveBackTheirLeases(t *testing.T) {
	g := roomFor(2, Enforce)
	g.Decide("done", "b", mariasHard, 0, false, t0)
	g.Decide("quiet", "b", mariasHard, 0, false, t0)
	g.ReleaseMatch("done")
	if st := g.Status(); st.Leases != 1 {
		t.Fatalf("ReleaseMatch: %+v", st)
	}
	if n := g.Sweep(t0.Add(3 * time.Minute)); n != 1 {
		t.Fatalf("Sweep released %d, want 1", n)
	}
	if st := g.Status(); st.Leases != 0 || st.Seats != 0 || st.LeasedCores != 0 {
		t.Fatalf("after sweep: %+v", st)
	}
	// And the room is usable again.
	if d := g.Decide("next", "b", mariasHard, 0, false, t0.Add(4*time.Minute)); d.Reduced {
		t.Fatal("capacity not recovered after the sweep")
	}
}

// A slower machine leases more of a core per seat, and can bring a class that
// is cheap here over the floor.
func TestSpeedScalesCosts(t *testing.T) {
	fast := New(Config{Mode: Enforce, Cores: 1})
	slow := New(Config{Mode: Enforce, Cores: 1, Speed: 4})
	if fast.weight(Class{"canasta", "hard", EngineNet}) != 0 {
		t.Fatal("canasta's model needs a lease at speed 1")
	}
	if slow.weight(Class{"canasta", "hard", EngineNet}) == 0 {
		t.Fatal("canasta's model is still free on a machine four times slower")
	}
}

func TestCalibrateIsClampedAndPlausible(t *testing.T) {
	f := Calibrate()
	if f < 0.5 || f > 20 {
		t.Fatalf("Calibrate = %v outside its clamp", f)
	}
}

func TestHintsAreCheapUnderPressure(t *testing.T) {
	g := roomFor(10, Enforce)
	if got := g.ForHint(mariasHard); got != mariasHard {
		t.Fatalf("green: %v", got)
	}
	g.SetLevel(capacity.Amber)
	if got := g.ForHint(mariasHard); got != mariasMed {
		t.Fatalf("amber: %v, want marias/medium", got)
	}
	if got := g.ForHint(prsiHard); got != prsiHard {
		t.Fatalf("a cheap hint was reduced: %v", got)
	}
	obs := roomFor(10, Observe)
	obs.SetLevel(capacity.Red)
	if got := obs.ForHint(mariasHard); got != mariasHard {
		t.Fatalf("observe changed a hint: %v", got)
	}
}
