package match_test

import (
	"context"
	"testing"
	"time"

	"zolik/server/internal/db"
	"zolik/server/internal/match"
	"zolik/server/internal/models"
	"zolik/server/internal/module"
	"zolik/server/internal/ws"
	"zolik/server/internal/zolikmod"
)

// A stand-in put in and taken out again before it moved played nobody's
// game: the result stays its player's.
func TestAStandInThatNeverMovedLeavesTheResultAlone(t *testing.T) {
	ctx := t.Context()
	m, id, onTurn, me := standInTable(t, stepOutOn)
	sitDown(t, m, id, onTurn)
	sitDown(t, m, id, me)

	// Not my turn, so my bot has nothing to do while it is in.
	if _, err := m.SetStandIn(ctx, id, me, me, true, ""); err != nil {
		t.Fatalf("step out: %v", err)
	}
	if seat(t, m, id, me).StandIn == nil {
		t.Fatal("no stand-in after stepping out")
	}
	time.Sleep(100 * time.Millisecond)
	if _, err := m.SetStandIn(ctx, id, me, me, false, ""); err != nil {
		t.Fatalf("step back in: %v", err)
	}
	if cur, _ := m.Current(ctx, id); len(cur.StoodIn) != 0 {
		t.Errorf("stoodIn = %v, want none: the stand-in never moved", cur.StoodIn)
	}
}

// The case that cost a player two wins: they left a long table at the results
// screen, a stand-in pressed Continue for them, and they came back and played
// every card of every deal themselves. Pressing Continue is not playing.
func TestAStandInPressingContinueLeavesTheResultAlone(t *testing.T) {
	ctx := t.Context()
	k, err := db.OpenKDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = k.Close(context.Background()) })
	hub, err := ws.NewHub(ws.NewConnRegistry(), "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = hub.Close() })
	mod := zolikmod.New()
	m := match.NewManager(match.NewKDBRepository(k), module.NewRegistry(mod), hub)
	m.SetBotPace(time.Millisecond, 2*time.Millisecond)

	cfg := module.MatchConfig{Options: module.Options{
		module.OptPauseBetweenRounds: module.OptOn,
		module.OptStandInAfter:       120,
	}}
	created, err := m.Create(ctx, "zolik", cfg, models.Player{ID: "p1", Name: "Ada"})
	if err != nil {
		t.Fatal(err)
	}
	id := created.ID.Hex()
	if _, err := m.Join(ctx, id, models.Player{ID: "p2", Name: "Bo"}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Start(ctx, id); err != nil {
		t.Fatal(err)
	}
	sitDown(t, m, id, "p1")
	leave := sitDown(t, m, id, "p2")

	// Both play their own cards, with the game's bot choosing, until the
	// first deal ends and the table asks both of them to go on.
	bot := module.BotFor(mod)
	continuing := func(pid string) bool {
		cur, _ := m.Current(ctx, id)
		offers, _ := mod.LegalActions(module.State(cur.State), pid)
		for _, o := range offers {
			if o.Verb == module.VerbContinue && o.Live() {
				return true
			}
		}
		return false
	}
	for moves := 0; !continuing("p1") || !continuing("p2"); moves++ {
		if moves > 2000 {
			t.Fatal("the first deal did not end")
		}
		cur, err := m.Current(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if cur.Status != "active" {
			t.Fatalf("status = %q before the first deal ended", cur.Status)
		}
		played := false
		for _, pid := range []string{"p1", "p2"} {
			if !m.Awaits(ctx, id, pid) {
				continue
			}
			offers, err := mod.LegalActions(module.State(cur.State), pid)
			if err != nil {
				t.Fatal(err)
			}
			candidates := module.ChooseActions(offers, nil)
			if a, ok := bot.Act(module.State(cur.State), module.BotSeat{PlayerID: pid, Seed: cur.Seed}, offers); ok {
				candidates = append([]module.Action{a}, candidates...)
			}
			for _, a := range candidates {
				if m.HandleAction(ctx, id, pid, a) == nil {
					played = true
					break
				}
			}
			break
		}
		if !played {
			time.Sleep(2 * time.Millisecond)
		}
	}

	// Bo leaves at the results screen; the host puts a bot in, and it goes
	// on for him.
	leave()
	if _, err := m.SetStandIn(ctx, id, "p1", "p2", true, ""); err != nil {
		t.Fatalf("stand-in: %v", err)
	}
	eventually(t, "the stand-in to press Continue", func() bool { return !continuing("p2") })

	// Bo comes back before the next deal asks anything of him.
	sitDown(t, m, id, "p2")
	m.ResumeIfReturning(ctx, id, "p2")
	if seat(t, m, id, "p2").StandIn != nil {
		t.Fatal("the stand-in kept the seat after its player came back")
	}
	if cur, _ := m.Current(ctx, id); len(cur.StoodIn) != 0 {
		t.Errorf("stoodIn = %v, want none: the stand-in only pressed Continue", cur.StoodIn)
	}
}
