package match_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"zolik/server/internal/match"
	"zolik/server/internal/models"
	"zolik/server/internal/module"
)

// standInTable deals a two-person Prší table — Ada hosts, Bo joins — with
// fast bots and a short stand-in wait, and reports who the table is waiting
// on first and who the other player is.
func standInTable(t *testing.T, opts module.Options) (m *match.Manager, matchID, waitedOn, other string) {
	t.Helper()
	ctx := t.Context()
	m, _, _ = newReaperHarness(t)
	m.SetBotPace(time.Millisecond, 2*time.Millisecond)
	m.SetStandInWait(150 * time.Millisecond)

	created, err := m.Create(ctx, "prsi", module.MatchConfig{Options: opts}, models.Player{ID: "p1", Name: "Ada"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	matchID = created.ID.Hex()
	if _, err := m.Join(ctx, matchID, models.Player{ID: "p2", Name: "Bo"}); err != nil {
		t.Fatalf("join: %v", err)
	}
	if _, err := m.Start(ctx, matchID); err != nil {
		t.Fatalf("start: %v", err)
	}
	waitedOn, other = "p1", "p2"
	if !m.Awaits(ctx, matchID, "p1") {
		waitedOn, other = "p2", "p1"
	}
	return m, matchID, waitedOn, other
}

func seat(t *testing.T, m *match.Manager, matchID, playerID string) models.Player {
	t.Helper()
	cur, err := m.Current(context.Background(), matchID)
	if err != nil {
		t.Fatalf("current: %v", err)
	}
	for _, p := range cur.Players {
		if p.ID == playerID {
			return p
		}
	}
	t.Fatalf("no seat %s", playerID)
	return models.Player{}
}

// eventually polls cond for up to five seconds.
func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// The whole point: somebody drops, the table waits, a bot plays their seat
// while the other player is still there, and they get it back on return.
func TestAStandInPlaysForAnAwaySeatAndHandsItBack(t *testing.T) {
	ctx := t.Context()
	m, id, gone, stays := standInTable(t, nil)
	sitDown(t, m, id, stays)
	leave := sitDown(t, m, id, gone)

	leave()
	m.SuspendOnDisconnect(ctx, id, gone, "test")
	if cur, _ := m.Current(ctx, id); cur.Status != "suspended" {
		t.Fatalf("status = %q, want suspended — the table waits first", cur.Status)
	}

	eventually(t, "the stand-in to play and pass the turn", func() bool {
		return seat(t, m, id, gone).StandIn != nil && m.Awaits(ctx, id, stays)
	})
	cur, _ := m.Current(ctx, id)
	if cur.Status != "active" {
		t.Errorf("status = %q, want active once a bot plays the seat", cur.Status)
	}
	if len(cur.StoodIn) != 1 || cur.StoodIn[0] != gone {
		t.Errorf("stoodIn = %v, want [%s]", cur.StoodIn, gone)
	}
	p := seat(t, m, id, gone)
	if p.IsAI || p.Name == "" {
		t.Errorf("the seat stopped being the person's: %+v", p)
	}
	if p.StandIn.By != models.StandInTimeout || p.StandIn.Skill != string(module.SkillMedium) {
		t.Errorf("stand-in = %+v, want a medium bot put in by the timeout", p.StandIn)
	}
	msg := m.BuildStateMsg(cur, stays)
	for _, pm := range msg.Players {
		if pm.ID == gone && pm.StandIn == nil {
			t.Error("the state message does not say a bot is playing for them")
		}
	}

	sitDown(t, m, id, gone)
	m.ResumeIfReturning(ctx, id, gone)
	if seat(t, m, id, gone).StandIn != nil {
		t.Fatal("the stand-in kept the seat after its player came back")
	}
	if cur, _ := m.Current(ctx, id); len(cur.StoodIn) != 1 {
		t.Errorf("stoodIn = %v after the return, want it kept for the result", cur.StoodIn)
	}
}

// Bots never play to an empty room: with nobody else at the table, the wait
// runs out and nothing happens — the reaper decides, as it always has.
func TestNoStandInWhenNobodyElseIsThere(t *testing.T) {
	ctx := t.Context()
	m, id, gone, _ := standInTable(t, nil)
	leave := sitDown(t, m, id, gone)
	leave()
	m.SuspendOnDisconnect(ctx, id, gone, "test")

	time.Sleep(500 * time.Millisecond)
	if seat(t, m, id, gone).StandIn != nil {
		t.Fatal("a bot stood in at a table nobody is sitting at")
	}
	if cur, _ := m.Current(ctx, id); cur.Status != "suspended" {
		t.Errorf("status = %q, want suspended", cur.Status)
	}
}

// "Wait for them" is today's behaviour, kept: no bot, however long it takes.
func TestWaitingForThemMeansNoStandIn(t *testing.T) {
	ctx := t.Context()
	m, id, gone, stays := standInTable(t, module.Options{module.OptStandInAfter: 0})
	sitDown(t, m, id, stays)
	leave := sitDown(t, m, id, gone)
	leave()
	m.SuspendOnDisconnect(ctx, id, gone, "test")

	time.Sleep(500 * time.Millisecond)
	if seat(t, m, id, gone).StandIn != nil {
		t.Fatal("a bot stood in at a table that chose to wait")
	}
	if cur, _ := m.Current(ctx, id); cur.Status != "suspended" {
		t.Errorf("status = %q, want suspended", cur.Status)
	}
}

// The host decides last: a bot at once, at a chosen strength, even at a table
// that chose to wait; and never for a player who is here.
func TestTheHostPutsAStandInIn(t *testing.T) {
	ctx := t.Context()
	m, id, _, _ := standInTable(t, module.Options{module.OptStandInAfter: 0})
	sitDown(t, m, id, "p1")

	if _, err := m.SetStandIn(ctx, id, "p2", "p2", true, ""); codeOf(err) != "NOT_THE_HOST" {
		t.Errorf("a guest deciding: err = %v, want NOT_THE_HOST", err)
	}
	if _, err := m.SetStandIn(ctx, id, "p1", "p1", true, ""); codeOf(err) != "STAND_IN_PLAYER_HERE" {
		t.Errorf("a bot for somebody who is here: err = %v, want STAND_IN_PLAYER_HERE", err)
	}
	if _, err := m.SetStandIn(ctx, id, "p1", "p2", true, "nonsense"); codeOf(err) != "UNKNOWN_SKILL" {
		t.Errorf("an unknown strength: err = %v, want UNKNOWN_SKILL", err)
	}

	if _, err := m.SetStandIn(ctx, id, "p1", "p2", true, "hard"); err != nil {
		t.Fatalf("host puts a bot in: %v", err)
	}
	bo := seat(t, m, id, "p2")
	if bo.StandIn == nil || bo.StandIn.Skill != "hard" || bo.StandIn.By != models.StandInHost {
		t.Fatalf("stand-in = %+v, want a hard bot the host put in", bo.StandIn)
	}
	eventually(t, "the host's bot to play Bo's turn", func() bool { return m.Awaits(ctx, id, "p1") })
}

// Taking the bot out means waiting for them: the table pauses if it is their
// turn, and no bot comes back on its own until they have been back.
func TestTakingTheStandInOutWaitsForThem(t *testing.T) {
	ctx := t.Context()
	m, id, gone, stays := standInTable(t, nil)
	sitDown(t, m, id, stays)
	leave := sitDown(t, m, id, gone)
	leave()
	m.SuspendOnDisconnect(ctx, id, gone, "test")
	eventually(t, "the timeout's stand-in", func() bool { return seat(t, m, id, gone).StandIn != nil })

	host := "p1"
	if _, err := m.SetStandIn(ctx, id, host, gone, false, ""); err != nil {
		t.Fatalf("host takes the bot out: %v", err)
	}
	if seat(t, m, id, gone).StandIn != nil {
		t.Fatal("the bot is still in after the host took it out")
	}
	time.Sleep(500 * time.Millisecond)
	if seat(t, m, id, gone).StandIn != nil {
		t.Fatal("a bot came back for a seat the host is holding")
	}
	if m.Awaits(ctx, id, gone) {
		if cur, _ := m.Current(ctx, id); cur.Status != "suspended" {
			t.Errorf("status = %q while waiting on %s, want suspended", cur.Status, gone)
		}
	}
}

func codeOf(err error) string {
	var me module.Error
	if errors.As(err, &me) {
		return me.Code
	}
	return ""
}
