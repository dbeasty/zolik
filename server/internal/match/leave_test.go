package match_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"zolik/server/internal/db"
	"zolik/server/internal/holdem"
	"zolik/server/internal/match"
	"zolik/server/internal/models"
	"zolik/server/internal/module"
	"zolik/server/internal/ws"
)

// pokerTable deals a three-person poker table in the given format, with a
// short cash-out wait, and reports its id.
func pokerTable(t *testing.T, format int) (*match.Manager, string) {
	t.Helper()
	ctx := t.Context()
	k, err := db.OpenKDB(t.TempDir())
	if err != nil {
		t.Fatalf("kdb: %v", err)
	}
	t.Cleanup(func() { _ = k.Close(context.Background()) })
	hub, err := ws.NewHub(ws.NewConnRegistry(), "")
	if err != nil {
		t.Fatalf("hub: %v", err)
	}
	t.Cleanup(func() { _ = hub.Close() })
	m := match.NewManager(match.NewKDBRepository(k), module.NewRegistry(holdem.New()), hub)
	m.SetBotPace(time.Millisecond, 2*time.Millisecond)
	m.SetLeaveWait(150 * time.Millisecond)

	created, err := m.Create(ctx, "holdem", module.MatchConfig{Options: module.Options{holdem.OptFormat: format}}, models.Player{ID: "p1", Name: "Ada"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	id := created.ID.Hex()
	for _, p := range []string{"p2", "p3"} {
		if _, err := m.Join(ctx, id, models.Player{ID: p, Name: p}); err != nil {
			t.Fatalf("join: %v", err)
		}
	}
	if _, err := m.Start(ctx, id); err != nil {
		t.Fatalf("start: %v", err)
	}
	return m, id
}

// left reports whether the board shows a seat as got up from the table.
func left(t *testing.T, m *match.Manager, id, playerID string) bool {
	t.Helper()
	cur, err := m.Current(context.Background(), id)
	if err != nil {
		t.Fatalf("current: %v", err)
	}
	vm, err := holdem.New().View(module.State(cur.State), playerID)
	if err != nil {
		t.Fatalf("view: %v", err)
	}
	for _, s := range vm.Seats {
		if s.PlayerID == playerID {
			return slices.Contains(s.LabelKeys, "seat.left")
		}
	}
	return false
}

// A player whose connection is gone longer than the table allows is got up
// from a cash table with their chips — not left posting blinds all evening.
func TestAnAwayPlayerIsCashedOutOfACashTable(t *testing.T) {
	ctx := t.Context()
	m, id := pokerTable(t, holdem.FormatCash)
	sitDown(t, m, id, "p1")
	sitDown(t, m, id, "p3")
	leave := sitDown(t, m, id, "p2")
	leave()
	m.SuspendOnDisconnect(ctx, id, "p2", "test")

	eventually(t, "p2 to be cashed out", func() bool { return left(t, m, id, "p2") })
	if cur, _ := m.Current(ctx, id); cur.Status != "active" {
		t.Errorf("status = %q, want the other two still playing", cur.Status)
	}
}

// In a tournament the chips are what everyone is playing for: an away seat is
// sat out and blinded off, never cashed out.
func TestATournamentNeverCashesAnybodyOut(t *testing.T) {
	ctx := t.Context()
	m, id := pokerTable(t, holdem.FormatTournament)
	sitDown(t, m, id, "p1")
	sitDown(t, m, id, "p3")
	leave := sitDown(t, m, id, "p2")
	leave()
	m.SuspendOnDisconnect(ctx, id, "p2", "test")

	time.Sleep(500 * time.Millisecond)
	if left(t, m, id, "p2") {
		t.Fatal("a tournament seat was cashed out")
	}
}

// Back in time, nothing happens.
func TestComingBackInTimeKeepsTheSeat(t *testing.T) {
	ctx := t.Context()
	m, id := pokerTable(t, holdem.FormatCash)
	m.SetLeaveWait(400 * time.Millisecond)
	sitDown(t, m, id, "p1")
	leave := sitDown(t, m, id, "p2")
	leave()
	m.SuspendOnDisconnect(ctx, id, "p2", "test")
	time.Sleep(100 * time.Millisecond)
	sitDown(t, m, id, "p2")
	m.ResumeIfReturning(ctx, id, "p2")

	time.Sleep(700 * time.Millisecond)
	if left(t, m, id, "p2") {
		t.Fatal("a player who came back in time was cashed out")
	}
}
