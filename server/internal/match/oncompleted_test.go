package match_test

import (
	"context"
	"sync"
	"testing"

	"zolik/server/internal/blackjack"
	"zolik/server/internal/match"
	"zolik/server/internal/models"
	"zolik/server/internal/module"
	"zolik/server/internal/ws"
)

// The store lets go of a match's namespace when the manager reports it
// finished, so the report has to come exactly once, for a match that really
// ended, and not for one that is still being played.
func TestManagerReportsAMatchWhenItCompletes(t *testing.T) {
	repo := newTestRepository(t)
	hub, err := ws.NewHub(ws.NewConnRegistry(), "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = hub.Close() })
	mods := module.NewRegistry(blackjack.New())
	m := match.NewManager(repo, mods, hub)

	var mu sync.Mutex
	var told []string
	m.SetOnCompleted(func(id string) {
		mu.Lock()
		told = append(told, id)
		mu.Unlock()
	})

	ctx := context.Background()
	cfg := module.MatchConfig{Variation: "atlantic", Options: module.Options{
		blackjack.OptRounds: 5, blackjack.OptStartingStack: 200, blackjack.OptMinBet: 5}}
	created, err := m.Create(ctx, "blackjack", cfg, models.Player{ID: "p1", Name: "Host"})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"p2", "p3"} {
		if _, err := m.Join(ctx, created.ID.Hex(), models.Player{ID: id, Name: id}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := m.Start(ctx, created.ID.Hex()); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	if len(told) != 0 {
		t.Fatalf("told %v about a match that has only started", told)
	}
	mu.Unlock()

	mod := mods.Get("blackjack")
	for step := 0; step < 50; step++ {
		cur, err := m.Current(ctx, created.ID.Hex())
		if err != nil {
			t.Fatal(err)
		}
		if cur.Status != "active" {
			break
		}
		for _, p := range []string{"p1", "p2", "p3"} {
			offers, _ := mod.LegalActions(module.State(cur.State), p)
			for _, o := range offers {
				if o.Enabled && o.Verb == "leave" {
					_ = m.HandleAction(ctx, created.ID.Hex(), p, module.Action{OfferID: o.ID, Verb: o.Verb})
					break
				}
			}
		}
	}
	final, err := m.Current(ctx, created.ID.Hex())
	if err != nil || final.Status != "completed" {
		t.Fatalf("not completed: %v %s", err, final.Status)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(told) != 1 || told[0] != created.ID.Hex() {
		t.Fatalf("told %v, want exactly [%s]", told, created.ID.Hex())
	}
}
