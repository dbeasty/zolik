package app

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"zolik/server/internal/auth"
	"zolik/server/internal/db"
	"zolik/server/internal/models"
	"zolik/server/internal/module"
)

// Starting a match in one place and carrying on with it in another.
//
// A match is written by one node at a time, because turn order is the one
// thing eventual consistency cannot give you. So picking it up somewhere else
// is not two writers agreeing: it is the match itself moving, with everything
// it has so far, to the device the person is now holding.
func TestAMatchStartedOnTheWebIsPickedUpOnAPhone(t *testing.T) {
	signingKeyForTest(t, "https://cloud.test")
	phoneKey := nodeKeyForTest(t)

	hub := newNode(t, Config{Sync: SyncConfig{Role: syncRoleHub, Advertise: "hub"}})
	hub.Start(context.Background())
	router := chi.NewRouter()
	hub.RegisterRoutes(router)
	cloud := httptest.NewServer(router)
	t.Cleanup(cloud.Close)

	ctx := context.Background()
	player, err := hub.authStore.InsertUser(ctx, models.User{Username: "ada"})
	if err != nil {
		t.Fatalf("creating the account: %v", err)
	}
	nodeID := auth.NodeIDFor(phoneKey)
	if err := hub.nodes.SaveNode(ctx, auth.Node{
		ID: nodeID, OwnerID: player.ID.Hex(), Kind: auth.NodeKindPhone,
		PublicKey: auth.EncodeNodePublicKey(phoneKey), EnrolledAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("enrolling the phone: %v", err)
	}
	credential, err := auth.CreateNodeCredential(nodeID, player.ID.Hex(), auth.NodeKindPhone, time.Hour)
	if err != nil {
		t.Fatalf("credential: %v", err)
	}

	// The match starts in the browser, which is to say on the cloud.
	mgr := hub.matchManager()
	seat := models.Player{ID: "seat-ada", Name: "Ada", UserID: player.ID.Hex()}
	created, err := mgr.Create(ctx, "prsi", module.MatchConfig{}, seat)
	if err != nil {
		t.Fatalf("opening the table: %v", err)
	}
	for _, bot := range []models.Player{
		{ID: "seat-bot1", Name: "Bot One", IsAI: true, AIDifficulty: "medium", AIPersona: "medium:bot-one"},
		{ID: "seat-bot2", Name: "Bot Two", IsAI: true, AIDifficulty: "medium", AIPersona: "medium:bot-two"},
	} {
		if _, err := mgr.Join(ctx, created.ID.Hex(), bot); err != nil {
			t.Fatalf("seating a bot: %v", err)
		}
	}
	if _, err := mgr.Start(ctx, created.ID.Hex()); err != nil {
		t.Fatalf("starting: %v", err)
	}
	playOneTurn(t, hub, created.ID.Hex(), seat.ID)
	if err := hub.sync.ClaimMatch(created.ID.Hex()); err != nil {
		t.Fatalf("claiming the match for the cloud: %v", err)
	}

	// The person picks their phone up.
	phone := newNode(t, Config{Sync: SyncConfig{
		Role:   syncRoleSpoke,
		HubURL: "ws://" + strings.TrimPrefix(cloud.URL, "http://") + "/kdb/sync",
		Token:  credential,
		Namespaces: []string{
			db.UserNS(player.ID.Hex()), db.UserReadOnlyNS(player.ID.Hex()),
		},
	}})
	phone.Start(context.Background())

	if err := phone.sync.Follow(db.MatchNS(created.ID.Hex())); err != nil {
		t.Fatalf("following the match: %v", err)
	}
	if err := phone.sync.SyncNow(); err != nil {
		t.Fatalf("first sync: %v", err)
	}
	if err := phone.sync.TakeMatch(ctx, created.ID.Hex(), "opened on the phone"); err != nil {
		t.Fatalf("taking the match over: %v", err)
	}
	if !phone.sync.WritesMatches(created.ID.Hex()) {
		t.Fatal("the phone took the match but does not think it writes it")
	}
	if hub.sync.WritesMatches(created.ID.Hex()) {
		t.Fatal("the cloud still thinks it writes a match it handed over")
	}

	// Everything played so far came with it, and the game carries on here.
	before, err := phone.matchManager().Current(ctx, created.ID.Hex())
	if err != nil {
		t.Fatalf("reading the match on the phone: %v", err)
	}
	if before.Status != "active" {
		t.Fatalf("the match arrived as %q", before.Status)
	}
	moves, err := phone.matchRepo.Moves(ctx, created.ID, 0, -1)
	if err != nil {
		t.Fatalf("reading its moves: %v", err)
	}
	if len(moves) == 0 {
		t.Fatal("the match came across with none of its moves")
	}
	playOneTurn(t, phone, created.ID.Hex(), seat.ID)

	// And the cloud sees the move that was played on the phone.
	waitFor(t, "the phone's move to reach the cloud", func() bool {
		if err := phone.sync.SyncNow(); err != nil {
			return false
		}
		after, err := hub.matchRepo.Moves(ctx, created.ID, 0, -1)
		return err == nil && len(after) > len(moves)
	})
}

// playOneTurn plays the person's seat once, waiting for their turn if the
// bots are still going.
func playOneTurn(t *testing.T, a *App, matchID, seat string) {
	t.Helper()
	mgr := a.matchManager()
	ctx := context.Background()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		m, err := mgr.Current(ctx, matchID)
		if err != nil {
			t.Fatalf("reading the match: %v", err)
		}
		if m.Status != "active" {
			t.Fatalf("the match is %q", m.Status)
		}
		mod := mgr.Registry().Get(m.ModuleID)
		if !containsString(module.AwaitedSeats(mod, module.State(m.State), seat, playerRefsForTest(m)), seat) {
			time.Sleep(5 * time.Millisecond)
			continue
		}
		offers, err := mod.LegalActions(module.State(m.State), seat)
		if err != nil {
			t.Fatalf("legal actions: %v", err)
		}
		for _, action := range module.ChooseActions(offers, nil) {
			if err := mgr.HandleAction(ctx, matchID, seat, action); err == nil {
				return
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("never got a turn")
}
