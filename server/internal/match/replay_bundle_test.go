package match_test

import (
	"context"
	"strconv"
	"testing"

	"zolik/server/internal/blackjack"
	"zolik/server/internal/match"
	"zolik/server/internal/models"
	"zolik/server/internal/module"
	zsync "zolik/server/internal/sync"
	"zolik/server/internal/ws"
)

// A blackjack table played to its end by three people at a phone must replay
// to the same end on the cloud that receives its bundle.
func TestAFinishedBlackjackTableReplaysToItsEnd(t *testing.T) { replaysToItsEnd(t, false) }

// ...including one where every seat got up: it is over, and there is nobody
// left to rank. The cloud used to refuse it for having no standings.
func TestABlackjackTableEveryoneLeftReplaysToItsEnd(t *testing.T) { replaysToItsEnd(t, true) }

func replaysToItsEnd(t *testing.T, everyoneLeaves bool) {
	repo := newTestRepository(t)
	hub, err := ws.NewHub(ws.NewConnRegistry(), "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = hub.Close() })
	mods := module.NewRegistry(blackjack.New())
	m := match.NewManager(repo, mods, hub)
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
	mod := mods.Get("blackjack")
	order := []string{"bet", "decline_insurance", "stand", "hit", "double", "split", "surrender", "continue"}
	if everyoneLeaves {
		order = []string{"leave"}
	}
	for step := 0; step < 2000; step++ {
		cur, err := m.Current(ctx, created.ID.Hex())
		if err != nil {
			t.Fatal(err)
		}
		if cur.Status != "active" {
			break
		}
		acted := false
	seats:
		for _, p := range []string{"p1", "p2", "p3"} {
			offers, err := mod.LegalActions(module.State(cur.State), p)
			if err != nil {
				t.Fatal(err)
			}
			for _, verb := range order {
				for _, o := range offers {
					if !o.Enabled || o.Verb != verb {
						continue
					}
					a := module.Action{OfferID: o.ID, Verb: o.Verb}
					for _, ps := range o.Params {
						if ps.Kind == module.ParamKindInt {
							v := ps.Default
							if v < ps.Min || v > ps.Max {
								v = ps.Min
							}
							if a.Params == nil {
								a.Params = map[string]string{}
							}
							a.Params[ps.Name] = strconv.Itoa(v)
						} else if len(ps.Choices) > 0 {
							if a.Params == nil {
								a.Params = map[string]string{}
							}
							a.Params[ps.Name] = ps.Choices[0].Value
						}
					}
					if err := m.HandleAction(ctx, created.ID.Hex(), p, a); err == nil {
						acted = true
						break seats
					}
				}
			}
		}
		if !acted {
			t.Fatalf("stuck at step %d", step)
		}
	}
	final, err := m.Current(ctx, created.ID.Hex())
	if err != nil || final.Status != "completed" {
		t.Fatalf("not completed: %v %s", err, final.Status)
	}

	b, err := match.BundleOf(ctx, repo, final)
	if err != nil {
		t.Fatal(err)
	}
	cloud := newTestRepository(t)
	imp := match.NewImporter(cloud, mods, noRecorder{}, nil)
	if err := imp.ImportBundle(ctx, zsync.Bundle(b)); err != nil {
		t.Fatalf("the cloud refused a finished table: %v", err)
	}
}

type noRecorder struct{}

func (noRecorder) RecordMatchAsync(models.Match, module.Outcome) {}
