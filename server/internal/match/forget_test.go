package match_test

import (
	"testing"
	"time"

	"zolik/server/internal/match"
	"zolik/server/internal/models"
	"zolik/server/internal/module"
	"zolik/server/internal/prsi"
	"zolik/server/internal/ws"
)

const (
	forgetAlice = "65ab000000000000000000a1"
	forgetBob   = "65ab000000000000000000b2"
)

func forgetManager(t *testing.T) (*match.Manager, match.Repository) {
	t.Helper()
	repo := newTestRepository(t)
	hub, err := ws.NewHub(ws.NewConnRegistry(), "")
	if err != nil {
		t.Fatalf("building a hub: %v", err)
	}
	t.Cleanup(func() { _ = hub.Close() })
	return match.NewManager(repo, module.NewRegistry(prsi.New()), hub), repo
}

func account(id, name string) models.Player {
	return models.Player{ID: id, Name: name, UserID: id, Avatar: "fox"}
}

func TestForgetAccountAnonymisesEveryTableTheAccountSatAt(t *testing.T) {
	ctx := t.Context()
	m, repo := forgetManager(t)

	// A game in play, with Alice hosting.
	live, err := m.Create(ctx, "prsi", module.MatchConfig{}, account(forgetAlice, "alice"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := m.Join(ctx, live.ID.Hex(), account(forgetBob, "bob")); err != nil {
		t.Fatalf("join: %v", err)
	}
	if _, err := m.Start(ctx, live.ID.Hex()); err != nil {
		t.Fatalf("start: %v", err)
	}

	// A finished one, where Alice sat second.
	ended := time.Now().UTC().Add(-time.Hour)
	done, err := repo.Insert(ctx, models.Match{
		ModuleID:  "prsi",
		Status:    "completed",
		Players:   []models.Player{account(forgetBob, "bob"), account(forgetAlice, "alice")},
		TurnOrder: []string{forgetBob, forgetAlice},
		HostID:    forgetBob,
		CreatedAt: ended.Add(-time.Hour),
		EndedAt:   &ended,
	})
	if err != nil {
		t.Fatalf("insert finished match: %v", err)
	}

	// A rematch Bob opened, holding a seat for Alice.
	rematch, err := repo.Insert(ctx, models.Match{
		ModuleID:  "prsi",
		Status:    "lobby",
		Players:   []models.Player{account(forgetBob, "bob")},
		TurnOrder: []string{forgetBob},
		HostID:    forgetBob,
		Reserved:  []models.Reservation{{PlayerID: forgetAlice, Name: "alice", UserID: forgetAlice}},
		CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("insert rematch: %v", err)
	}

	if err := m.ForgetAccount(ctx, forgetAlice); err != nil {
		t.Fatalf("ForgetAccount: %v", err)
	}

	cur, err := m.Current(ctx, live.ID.Hex())
	if err != nil {
		t.Fatalf("current: %v", err)
	}
	checkAnonymous(t, "live game", cur, 0, forgetAlice)
	if cur.Status != "active" || len(cur.State) == 0 {
		t.Errorf("live game lost its board or status: status %q, %d bytes of state", cur.Status, len(cur.State))
	}
	if cur.Players[1].Name != "bob" || cur.Players[1].UserID != forgetBob {
		t.Errorf("bob's seat changed: %+v", cur.Players[1])
	}

	stored, err := repo.FindByID(ctx, done.ID)
	if err != nil {
		t.Fatalf("find finished match: %v", err)
	}
	checkAnonymous(t, "finished game", stored, 1, forgetAlice)

	held, err := repo.FindByID(ctx, rematch.ID)
	if err != nil {
		t.Fatalf("find rematch: %v", err)
	}
	if len(held.Reserved) != 0 {
		t.Errorf("the rematch still holds a seat for alice: %+v", held.Reserved)
	}

	if ids, err := repo.MatchIDsForUser(ctx, forgetAlice); err != nil || len(ids) != 0 {
		t.Errorf("matches still naming alice: %v (%v)", ids, err)
	}
	if err := m.ForgetAccount(ctx, forgetAlice); err != nil {
		t.Fatalf("second ForgetAccount: %v", err)
	}
}

func checkAnonymous(t *testing.T, what string, m models.Match, seat int, userID string) {
	t.Helper()
	p := m.Players[seat]
	want := models.AnonymousGuestID(m.ID.Hex(), userID)
	if p.UserID != "" || p.GuestID != want {
		t.Errorf("%s: seat %d is user %q guest %q, want no account and guest %s", what, seat, p.UserID, p.GuestID, want)
	}
	if p.Name != models.AnonymousSeatName(seat) || p.Avatar != "" {
		t.Errorf("%s: seat %d is called %q with avatar %q, want %q and none", what, seat, p.Name, p.Avatar, models.AnonymousSeatName(seat))
	}
	if p.ID != userID {
		t.Errorf("%s: seat id changed to %q; the board refers to it", what, p.ID)
	}
}
