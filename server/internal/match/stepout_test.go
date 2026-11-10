package match_test

import (
	"testing"
	"time"

	"zolik/server/internal/models"
	"zolik/server/internal/module"
)

var stepOutOn = module.Options{module.OptStepOut: module.OptOn}

// Off unless the lobby turned it on: a table that did not agree to it is not
// handed a bot because one player asked.
func TestSteppingOutNeedsTheOption(t *testing.T) {
	ctx := t.Context()
	m, id, me, _ := standInTable(t, nil)
	sitDown(t, m, id, me)
	// Answered as before the option existed: the host is here, a guest is
	// not the host.
	if _, err := m.SetStandIn(ctx, id, me, me, true, ""); err == nil {
		t.Fatal("stepping out was allowed at a table that did not turn it on")
	}
	if seat(t, m, id, me).StandIn != nil {
		t.Fatal("a stand-in went in anyway")
	}
}

// The point of it: the player is still connected — the tab is open on the
// table — and the bot plays their seat anyway, and keeps playing it through a
// reconnect, until they say they are back.
func TestASteppedOutSeatIsPlayedWhileItsPlayerIsHere(t *testing.T) {
	ctx := t.Context()
	m, id, me, other := standInTable(t, stepOutOn)
	sitDown(t, m, id, other)
	sitDown(t, m, id, me)

	if _, err := m.SetStandIn(ctx, id, me, me, true, ""); err != nil {
		t.Fatalf("step out: %v", err)
	}
	p := seat(t, m, id, me)
	if p.StandIn == nil || p.StandIn.By != models.StandInSelf || p.IsAI {
		t.Fatalf("seat = %+v, want the person's seat with a self stand-in", p)
	}
	eventually(t, "the stand-in to play my turn", func() bool { return m.Awaits(ctx, id, other) })

	// A reload is not coming back.
	m.ResumeIfReturning(ctx, id, me)
	if seat(t, m, id, me).StandIn == nil {
		t.Fatal("reconnecting ended the step-out; only the player may")
	}

	if _, err := m.SetStandIn(ctx, id, me, me, false, ""); err != nil {
		t.Fatalf("step back in: %v", err)
	}
	if seat(t, m, id, me).StandIn != nil {
		t.Fatal("still stood in for after stepping back in")
	}
	if cur, _ := m.Current(ctx, id); len(cur.StoodIn) != 1 || cur.StoodIn[0] != me {
		t.Errorf("stoodIn = %v, want [%s]: the result must say a bot played", cur.StoodIn, me)
	}
}

// Playing a card is stepping back in: a player who forgot the button is not
// left fighting their own bot for the seat.
func TestAMoveOfYourOwnTakesTheSeatBack(t *testing.T) {
	ctx := t.Context()
	m, id, me, other := standInTable(t, stepOutOn)
	// Slow enough that the stand-in has not moved before the player does.
	m.SetBotPace(10*time.Second, 11*time.Second)
	sitDown(t, m, id, other)
	sitDown(t, m, id, me)
	if !m.Awaits(ctx, id, me) {
		t.Fatal("the table is not waiting on the first seat")
	}
	if _, err := m.SetStandIn(ctx, id, me, me, true, ""); err != nil {
		t.Fatalf("step out: %v", err)
	}

	cur, _ := m.Current(ctx, id)
	mod := m.Registry().Get(cur.ModuleID)
	offers, err := mod.LegalActions(module.State(cur.State), me)
	if err != nil {
		t.Fatalf("offers: %v", err)
	}
	played := false
	for _, a := range module.ChooseActions(offers, nil) {
		if m.HandleAction(ctx, id, me, a) == nil {
			played = true
			break
		}
	}
	if !played {
		t.Fatal("no legal move went through")
	}
	if seat(t, m, id, me).StandIn != nil {
		t.Fatal("the stand-in kept the seat after its player moved")
	}
}

// A host about their own seat steps out like anybody else, rather than being
// refused for being at the table.
func TestTheHostStepsOutToo(t *testing.T) {
	ctx := t.Context()
	m, id, _, _ := standInTable(t, stepOutOn)
	sitDown(t, m, id, "p2")
	sitDown(t, m, id, "p1")
	if _, err := m.SetStandIn(ctx, id, "p1", "p1", true, ""); err != nil {
		t.Fatalf("host steps out: %v", err)
	}
	if p := seat(t, m, id, "p1"); p.StandIn == nil || p.StandIn.By != models.StandInSelf {
		t.Fatalf("host seat = %+v, want a self stand-in", p.StandIn)
	}
}

// Nobody else may step out for you.
func TestStepOutIsYourOwnSeatOnly(t *testing.T) {
	ctx := t.Context()
	m, id, _, _ := standInTable(t, stepOutOn)
	sitDown(t, m, id, "p1")
	sitDown(t, m, id, "p2")
	_, err := m.SetStandIn(ctx, id, "p2", "p1", true, "")
	if codeOf(err) != "NOT_THE_HOST" {
		t.Fatalf("err = %v, want NOT_THE_HOST", err)
	}
}
