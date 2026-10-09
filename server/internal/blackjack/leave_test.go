package blackjack

import (
	"testing"

	"zolik/server/internal/module"
)

func leave(t *testing.T, raw module.State, playerID string) (module.State, error) {
	t.Helper()
	next, _, err := New().Apply(raw, playerID, module.Action{Verb: module.VerbLeave})
	return next, err
}

// Before the cards are out a player goes at once, with any stake handed back,
// and the deal they were holding up goes ahead.
func TestLeavingBeforeTheDealHandsTheStakeBack(t *testing.T) {
	raw := newMatch(t, 3, module.MatchConfig{})
	raw, err := bet(t, raw, "p1", 50)
	if err != nil {
		t.Fatal(err)
	}
	s := stateOf(t, raw)
	start := seatOf(t, s, "p2").Stack
	// p2 had not bet: p1's stake was waiting on them.
	raw, err = leave(t, raw, "p2")
	if err != nil {
		t.Fatalf("leave: %v", err)
	}
	s = stateOf(t, raw)
	p2 := seatOf(t, s, "p2")
	if !p2.Left || !p2.Out || p2.Stack != start {
		t.Fatalf("p2 = %+v, want left with %d", p2, start)
	}
	if s.Phase == phaseBets {
		t.Error("the deal is still waiting on a player who got up")
	}

	// And a stake put up but not yet dealt to comes back.
	raw = newMatch(t, 4, module.MatchConfig{})
	raw, _ = bet(t, raw, "p1", 50)
	before := seatOf(t, stateOf(t, raw), "p1").Stack + 50
	raw, err = leave(t, raw, "p1")
	if err != nil {
		t.Fatalf("leave with a stake down: %v", err)
	}
	if got := seatOf(t, stateOf(t, raw), "p1").Stack; got != before {
		t.Errorf("stack after leaving = %d, want the stake back: %d", got, before)
	}
}

// With cards in front of them, a player plays the round out and gets up when
// it settles: a hand walked away from would be a stake nobody could settle.
func TestLeavingMidRoundWaitsForTheRoundToSettle(t *testing.T) {
	raw := newMatch(t, 5, module.MatchConfig{Options: module.Options{module.OptPauseBetweenRounds: module.OptOff}})
	raw, _ = bet(t, raw, "p1", 50)
	raw, _ = bet(t, raw, "p2", 50)
	s := stateOf(t, raw)
	if s.Phase == phaseBets {
		t.Fatal("not dealt")
	}
	raw, err := leave(t, raw, "p2")
	if err != nil {
		t.Fatalf("leave mid-round: %v", err)
	}
	if p2 := seatOf(t, stateOf(t, raw), "p2"); !p2.Leaving || p2.Left {
		t.Fatalf("p2 = %+v, want leaving and not yet gone", p2)
	}
	if _, err := leave(t, raw, "p2"); module.CodeOf(err) != ErrAlreadyLeft {
		t.Errorf("asking twice: %v, want %s", err, ErrAlreadyLeft)
	}
	offers, _ := New().LegalActions(raw, "p2")
	for _, o := range offers {
		if o.Verb == module.VerbLeave {
			t.Error("a seat already leaving is offered to leave")
		}
	}
	// Play the round out by standing on everything.
	for i := 0; i < 20; i++ {
		s = stateOf(t, raw)
		if s.Phase == phaseBets || s.Status != "active" {
			break
		}
		if s.Break.Open {
			for _, id := range order(s) {
				raw, _ = apply(t, raw, id, module.VerbContinue)
			}
			continue
		}
		for _, id := range []string{"p1", "p2"} {
			if next, err := apply(t, raw, id, VerbStand); err == nil {
				raw = next
			}
			if next, err := apply(t, raw, id, VerbDecline); err == nil {
				raw = next
			}
		}
	}
	s = stateOf(t, raw)
	if p2 := seatOf(t, s, "p2"); !p2.Left || !p2.Out {
		t.Fatalf("after the round p2 = %+v, want gone", p2)
	}
	if s.Status == "active" && s.Phase != phaseBets {
		t.Errorf("phase %q, want the next round's betting", s.Phase)
	}
}

// The last person at a table of bots getting up ends it.
func TestTheLastPersonLeavingEndsTheTable(t *testing.T) {
	raw, err := New().NewMatch(module.MatchConfig{}, []module.PlayerRef{
		{ID: "ann", Name: "Ann"}, {ID: "b1", Name: "Bot", IsAI: true},
	}, 6)
	if err != nil {
		t.Fatal(err)
	}
	raw, err = leave(t, raw, "ann")
	if err != nil {
		t.Fatalf("leave: %v", err)
	}
	if s := stateOf(t, raw); s.Status != "completed" {
		t.Fatalf("status = %q, want completed", s.Status)
	}
}

// Leave is the last offer, and nobody but the person makes it.
func TestTheLeaveOfferIsLastAndManual(t *testing.T) {
	raw := newMatch(t, 7, module.MatchConfig{})
	offers, _ := New().LegalActions(raw, "p1")
	last := offers[len(offers)-1]
	if last.Verb != module.VerbLeave || !last.Manual || !last.Enabled {
		t.Fatalf("last offer = %+v, want an enabled manual leave", last)
	}
	if a, ok := module.ChooseAction(offers, []string{module.VerbLeave}); ok && a.Verb == module.VerbLeave {
		t.Error("a driver chose to leave")
	}
}
