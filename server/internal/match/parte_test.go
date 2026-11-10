package match_test

import (
	"context"
	"encoding/json"
	"slices"
	"testing"

	"zolik/server/internal/db"
	"zolik/server/internal/holdem"
	"zolik/server/internal/lastcard"
	"zolik/server/internal/match"
	"zolik/server/internal/models"
	"zolik/server/internal/module"
	"zolik/server/internal/ws"
)

// A cash table takes somebody who arrives after the deal; a tournament does
// not.
func TestSomebodyJoinsACashTableUnderWay(t *testing.T) {
	ctx := t.Context()
	m, id := pokerTable(t, holdem.FormatCash)
	joined, err := m.Join(ctx, id, models.Player{ID: "p4", Name: "Dee"})
	if err != nil {
		t.Fatalf("join a cash table under way: %v", err)
	}
	if !slices.ContainsFunc(joined.Players, func(p models.Player) bool { return p.ID == "p4" }) {
		t.Fatal("p4 is not seated")
	}
	cur, _ := m.Current(ctx, id)
	vm, err := holdem.New().View(module.State(cur.State), "p4")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, s := range vm.Seats {
		if s.PlayerID == "p4" {
			found = slices.Contains(s.LabelKeys, "seat.joining")
		}
	}
	if !found {
		t.Error("the new seat does not say it joins at the next hand")
	}

	m2, id2 := pokerTable(t, holdem.FormatTournament)
	if _, err := m2.Join(ctx, id2, models.Player{ID: "p4", Name: "Dee"}); module.CodeOf(err) != "LATE_JOIN_TOURNAMENT" {
		t.Errorf("join a tournament under way: %v, want LATE_JOIN_TOURNAMENT", err)
	}
}

// The host hands an away player's seat to somebody else: they take it under
// their own name, the first player's own sign-in no longer sits there, and
// the match is marked as nobody's whole game.
func TestTheHostHandsASeatToSomebodyElse(t *testing.T) {
	ctx := t.Context()
	m, id, _, _ := standInTable(t, module.Options{module.OptStandInAfter: 0})
	sitDown(t, m, id, "p1")

	if _, err := m.HandOverSeat(ctx, id, "p2", "p1"); codeOf(err) != "NOT_THE_HOST" {
		t.Errorf("a guest handing over a seat: %v", err)
	}
	if _, err := m.HandOverSeat(ctx, id, "p1", "p1"); err == nil {
		t.Error("the host handed over their own seat")
	}
	secret, err := m.HandOverSeat(ctx, id, "p1", "p2")
	if err != nil {
		t.Fatalf("HandOverSeat: %v", err)
	}
	_, linked, err := m.SeatByLink(ctx, id, secret)
	if err != nil || linked.ID != "p2" || !linked.PendingHandover {
		t.Fatalf("the link opens %+v, %v", linked, err)
	}
	if _, err := m.TakeOverSeat(ctx, id, "p2", "  "); codeOf(err) != "NAME_REQUIRED" {
		t.Errorf("taking a seat with no name: %v", err)
	}
	if _, err := m.TakeOverSeat(ctx, id, "p2", "Zed"); err != nil {
		t.Fatalf("TakeOverSeat: %v", err)
	}
	p := seat(t, m, id, "p2")
	if p.Name != "Zed" || p.FormerName != "Bo" || !p.HandedOver || p.PendingHandover {
		t.Fatalf("seat after the takeover = %+v", p)
	}
	if !m.SeatHandedOver(ctx, id, "p2") {
		t.Error("the seat is not marked as handed over")
	}
	cur, _ := m.Current(ctx, id)
	if !slices.Contains(cur.HandedOver, "p2") {
		t.Errorf("handedOver = %v", cur.HandedOver)
	}
	if _, err := m.TakeOverSeat(ctx, id, "p2", "Yan"); codeOf(err) != "SEAT_LINK_EXPIRED" {
		t.Errorf("a second takeover with the same link: %v", err)
	}
}

// At a Last Card table, a player a bot stands in for is dealt out of the
// deals after the one in play, and dealt back in when they return.
func TestAStoodInPlayerIsDealtAroundAndBackIn(t *testing.T) {
	ctx := t.Context()
	k, err := db.OpenKDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = k.Close(context.Background()) })
	hub, _ := ws.NewHub(ws.NewConnRegistry(), "")
	t.Cleanup(func() { _ = hub.Close() })
	m := match.NewManager(match.NewKDBRepository(k), module.NewRegistry(lastcard.New()), hub)
	m.SetBotPace(1e6, 2e6)
	created, err := m.Create(ctx, "lastcard", module.MatchConfig{Options: module.Options{module.OptStandInAfter: 0}}, models.Player{ID: "p1", Name: "Ada"})
	if err != nil {
		t.Fatal(err)
	}
	id := created.ID.Hex()
	for _, p := range []string{"p2", "p3"} {
		if _, err := m.Join(ctx, id, models.Player{ID: p, Name: p}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := m.Start(ctx, id); err != nil {
		t.Fatal(err)
	}
	sitDown(t, m, id, "p1")
	sitDown(t, m, id, "p3")

	sittingOut := func() []string {
		cur, _ := m.Current(ctx, id)
		var s struct {
			SittingOut []string `json:"sittingOut"`
		}
		_ = json.Unmarshal(cur.State, &s)
		return s.SittingOut
	}
	if _, err := m.SetStandIn(ctx, id, "p1", "p2", true, ""); err != nil {
		t.Fatalf("SetStandIn: %v", err)
	}
	if got := sittingOut(); !slices.Contains(got, "p2") {
		t.Fatalf("sittingOut = %v after a stand-in began, want p2", got)
	}
	sitDown(t, m, id, "p2")
	m.ResumeIfReturning(ctx, id, "p2")
	if got := sittingOut(); slices.Contains(got, "p2") {
		t.Fatalf("sittingOut = %v after p2 came back", got)
	}
}
