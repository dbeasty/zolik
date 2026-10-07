package match_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/canasta"
	"zolik/server/internal/klondike"
	"zolik/server/internal/match"
	"zolik/server/internal/models"
	"zolik/server/internal/module"
	zsync "zolik/server/internal/sync"
	"zolik/server/internal/ws"
)

// Solitaire is played against the deck, so the deal is the whole game: it
// must never reach a device, nor be chosen by one (docs/solitaire-plan.md §4a).

func soloManager(t *testing.T) (*match.Manager, match.Repository, *module.Registry) {
	t.Helper()
	repo := newTestRepository(t)
	hub, err := ws.NewHub(ws.NewConnRegistry(), "")
	if err != nil {
		t.Fatalf("building a hub: %v", err)
	}
	t.Cleanup(func() { _ = hub.Close() })
	mods := module.NewRegistry(canasta.New(), klondike.New())
	return match.NewManager(repo, mods, hub), repo, mods
}

const soloUser = "0123456789abcdef01234567"

func TestADeviceIsNeverSeatedAtAServerDealtMatch(t *testing.T) {
	m, _, _ := soloManager(t)
	ctx := context.Background()
	host := models.Player{ID: "p1", Name: "Solo", UserID: soloUser}

	solo, err := m.Create(ctx, "klondike", module.MatchConfig{}, host)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := m.Start(ctx, solo.ID.Hex()); err != nil {
		t.Fatalf("start: %v", err)
	}
	seated, err := m.Seated(ctx, solo.ID.Hex(), soloUser)
	if err != nil {
		t.Fatal(err)
	}
	if seated {
		t.Fatal("the solitaire player's device may pull the match namespace, and every face-down card with it")
	}

	// A game with other players at it is unchanged.
	duo, err := m.Create(ctx, "canasta", module.MatchConfig{}, host)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if seated, _ := m.Seated(ctx, duo.ID.Hex(), soloUser); !seated {
		t.Fatal("a canasta host is no longer seated at their own table")
	}
}

func TestTwoMatchesCreatedTogetherDealDifferently(t *testing.T) {
	m, _, _ := soloManager(t)
	ctx := context.Background()
	host := models.Player{ID: "p1", Name: "Solo"}
	seen := map[int64]bool{}
	for i := 0; i < 8; i++ {
		created, err := m.Create(ctx, "klondike", module.MatchConfig{}, host)
		if err != nil {
			t.Fatal(err)
		}
		if seen[created.Seed] {
			t.Fatalf("seed %d dealt twice", created.Seed)
		}
		seen[created.Seed] = true
	}
}

func TestAServerDealtBundleIsRefused(t *testing.T) {
	_, repo, mods := soloManager(t)
	imp := match.NewImporter(repo, mods, nil, nil)
	m := models.Match{ModuleID: "klondike", Status: "completed", Players: []models.Player{{ID: "p1"}}}
	m.ID = bson.NewObjectID()
	env, _ := json.Marshal(m)
	err := imp.ImportBundle(context.Background(), zsync.Bundle{
		Match: m.ID.Hex(), Module: "klondike", Seed: 1, Envelope: env,
	})
	if err == nil || !strings.Contains(err.Error(), "server only") {
		t.Fatalf("a phone-dealt solitaire game was accepted: %v", err)
	}
}

// A solitaire table left behind is a saved game: suspended to wait for its
// player for a month, never swept up as abandoned two minutes later.
func TestALeftSolitaireGameIsSaved(t *testing.T) {
	m, repo, _ := soloManager(t)
	ctx := context.Background()
	solo, err := m.Create(ctx, "klondike", module.MatchConfig{}, models.Player{ID: "p1", Name: "Solo"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Start(ctx, solo.ID.Hex()); err != nil {
		t.Fatal(err)
	}
	m.SuspendOnDisconnect(ctx, solo.ID.Hex(), "p1", "test")
	got, err := repo.FindByID(ctx, solo.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "suspended" || got.AbandonAt == nil {
		t.Fatalf("not suspended: %s", got.Status)
	}
	if wait := got.AbandonAt.Sub(*got.SuspendedAt); wait < match.SavedGameWindow {
		t.Fatalf("a saved game waits only %v", wait)
	}
	if n := m.ReapAbandoned(ctx); n != 0 {
		t.Fatalf("the reaper abandoned %d", n)
	}
	m.ResumeIfReturning(ctx, solo.ID.Hex(), "p1")
	if back, _ := repo.FindByID(ctx, solo.ID); back.Status != "active" {
		t.Fatalf("the player came back to a %s game", back.Status)
	}
}

// "Play this deal again" copies the deal on the server: the new table is dealt
// the very same cards, and nobody is ever shown the seed that did it.
func TestADealPlayedAgainIsTheSameDeal(t *testing.T) {
	m, repo, _ := soloManager(t)
	ctx := context.Background()
	host := models.Player{ID: "p1", Name: "Solo"}
	first, err := m.Create(ctx, "klondike", module.MatchConfig{Variation: "draw3"}, host)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Start(ctx, first.ID.Hex()); err != nil {
		t.Fatal(err)
	}
	opening := tableauOf(t, m, first.ID.Hex())

	// Not while it is still being played.
	if _, err := m.DealAgain(ctx, first.ID.Hex(), "p1"); module.CodeOf(err) != "MATCH_NOT_OVER" {
		t.Fatalf("dealt again mid-game: %v", err)
	}
	if err := m.HandleAction(ctx, first.ID.Hex(), "p1", module.Action{Verb: "giveup"}); err != nil {
		t.Fatal(err)
	}
	// Not for somebody who never sat there.
	if _, err := m.DealAgain(ctx, first.ID.Hex(), "stranger"); module.CodeOf(err) != "NOT_AT_THIS_TABLE" {
		t.Fatalf("a stranger was dealt somebody else's game: %v", err)
	}

	again, err := m.DealAgain(ctx, first.ID.Hex(), "p1")
	if err != nil {
		t.Fatal(err)
	}
	if again.Status != "active" || again.ID == first.ID {
		t.Fatalf("the deal again is %s, id %s", again.Status, again.ID.Hex())
	}
	if again.Variation != "draw3" || again.DealFrom != first.ID.Hex() {
		t.Fatalf("variation %q, dealFrom %q", again.Variation, again.DealFrom)
	}
	if got := tableauOf(t, m, again.ID.Hex()); got != opening {
		t.Fatalf("dealt differently:\n%s\n%s", got, opening)
	}
	stored, _ := repo.FindByID(ctx, again.ID)
	if stored.Seed != first.Seed {
		t.Fatal("the seed was not carried over")
	}

	// Played again from the repeat, it still names the first table.
	if err := m.HandleAction(ctx, again.ID.Hex(), "p1", module.Action{Verb: "giveup"}); err != nil {
		t.Fatal(err)
	}
	third, err := m.DealAgain(ctx, again.ID.Hex(), "p1")
	if err != nil {
		t.Fatal(err)
	}
	if third.DealFrom != first.ID.Hex() {
		t.Fatalf("a repeat of a repeat names %q", third.DealFrom)
	}
}

// tableauOf is the face-up board as the player sees it: the same deal reads
// the same.
func tableauOf(t *testing.T, m *match.Manager, matchID string) string {
	t.Helper()
	cur, err := m.Current(context.Background(), matchID)
	if err != nil {
		t.Fatal(err)
	}
	vm, err := klondike.New().View(module.State(cur.State), "p1")
	if err != nil {
		t.Fatal(err)
	}
	blob, _ := json.Marshal(vm.Zones)
	return string(blob)
}
