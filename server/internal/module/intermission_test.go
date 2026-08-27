package module_test

import (
	"testing"

	"zolik/server/internal/module"
)

// The ready-up, checked once so three games do not each answer these
// differently.

func TestAPausedTableWaitsForEverySeat(t *testing.T) {
	order := []string{"p1", "p2", "p3"}
	var i module.Intermission
	i.Begin(2)

	if got := i.Waiting(order); len(got) != 3 {
		t.Fatalf("a fresh intermission waits on %v, want all three", got)
	}
	if i.Settled(order) {
		t.Fatal("settled before anybody said so")
	}

	for n, id := range order {
		if err := i.Mark(order, id); err != nil {
			t.Fatalf("%s could not ready: %v", id, err)
		}
		want := len(order) - n - 1
		if got := i.Waiting(order); len(got) != want {
			t.Errorf("after %s readied the table waits on %v, want %d seats", id, got, want)
		}
		// The round must not advance early: settled only once the last one is in.
		if settled := i.Settled(order); settled != (want == 0) {
			t.Errorf("after %s readied, settled=%v with %d still to go", id, settled, want)
		}
	}
}

// TestReadyingTwiceIsRefused — pressing a button twice is a thing people do,
// and the second press must not be able to carry the table.
func TestReadyingTwiceIsRefused(t *testing.T) {
	order := []string{"p1", "p2"}
	var i module.Intermission
	i.Begin(1)

	if err := i.Mark(order, "p1"); err != nil {
		t.Fatalf("first ready: %v", err)
	}
	err := i.Mark(order, "p1")
	if module.CodeOf(err) != module.ErrAlreadyReady {
		t.Errorf("readying twice gave %v, want %s", err, module.ErrAlreadyReady)
	}
	if i.Settled(order) {
		t.Error("one seat readying twice settled a two-seat table")
	}
}

func TestOnlySeatsMayContinue(t *testing.T) {
	order := []string{"p1", "p2"}
	var i module.Intermission
	i.Begin(1)

	if code := module.CodeOf(i.Mark(order, "someone-else")); code != module.ErrNotSeated {
		t.Errorf("a stranger readying gave %q, want %s", code, module.ErrNotSeated)
	}

	var closed module.Intermission
	if code := module.CodeOf(closed.Mark(order, "p1")); code != module.ErrNotPaused {
		t.Errorf("readying a running table gave %q, want %s", code, module.ErrNotPaused)
	}
}

// TestTheContinueOfferIsAnOrdinaryOffer — the whole claim that an intermission
// is not a new primitive. If SubmissionFor can build the action, then every bot
// and the conformance driver can take it without knowing what it is for.
func TestTheContinueOfferIsAnOrdinaryOffer(t *testing.T) {
	order := []string{"p1", "p2"}
	var i module.Intermission
	i.Begin(1)

	offers := i.Offers(order, "p1")
	if len(offers) != 1 {
		t.Fatalf("a paused seat is offered %d controls, want 1", len(offers))
	}
	// The control is the instruction, and carries nothing else. It used to
	// ship a "waiting for n" fact, which read as a status line under a button
	// that was actually asking to be pressed — and said the same thing the
	// shell already says to a player with nothing to do.
	if len(offers[0].Facts) != 0 {
		t.Errorf("the offer to go on carries %d facts; the label is the whole message", len(offers[0].Facts))
	}

	a, ok := module.SubmissionFor(offers[0])
	if !ok {
		t.Fatal("the continue offer describes no submission anything could send")
	}
	if a.Verb != module.VerbContinue || a.OfferID != module.OfferContinue {
		t.Errorf("submission is %+v, want the continue verb and offer id", a)
	}

	// And a bot with no preferences at all finds it.
	if got, ok := module.ChooseAction(offers, nil); !ok || got.Verb != module.VerbContinue {
		t.Errorf("an unprompted bot chose %+v (ok=%v); it must be able to press continue", got, ok)
	}
}

// TestAReadiedSeatIsOfferedTheReasonWhyNot — an omitted offer is
// indistinguishable from a client bug, so the control stays and explains itself.
func TestAReadiedSeatIsOfferedTheReasonWhyNot(t *testing.T) {
	order := []string{"p1", "p2"}
	var i module.Intermission
	i.Begin(1)
	if err := i.Mark(order, "p1"); err != nil {
		t.Fatalf("ready: %v", err)
	}

	offers := i.Offers(order, "p1")
	if len(offers) != 1 {
		t.Fatalf("a readied seat is offered %d controls, want 1", len(offers))
	}
	if offers[0].Enabled {
		t.Error("a seat that has already readied is still offered the button")
	}
	if offers[0].WhyNot != module.ErrAlreadyReady {
		t.Errorf("disabled with %q, want %s", offers[0].WhyNot, module.ErrAlreadyReady)
	}
}

// TestAPausedTableIsStillSeated — the runtime works out who it is waiting on
// from the seat list, so an intermission that emitted none would look like a
// table with nobody to act.
func TestAPausedTableIsStillSeated(t *testing.T) {
	order := []string{"p1", "p2", "p3"}
	var i module.Intermission
	i.Begin(1)
	if err := i.Mark(order, "p2"); err != nil {
		t.Fatalf("ready: %v", err)
	}

	seats := i.Seats(order)
	if len(seats) != len(order) {
		t.Fatalf("%d seats for %d players", len(seats), len(order))
	}
	active := map[string]bool{}
	for _, s := range seats {
		if s.Active {
			active[s.PlayerID] = true
		}
	}
	// The awaited set is exactly the set still to ready.
	if len(active) != 2 || !active["p1"] || !active["p3"] {
		t.Errorf("active seats are %v, want p1 and p3", active)
	}
}
