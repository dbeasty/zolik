package match_test

import (
	"context"
	"testing"

	"zolik/server/internal/match"
	"zolik/server/internal/models"
	"zolik/server/internal/module"
	"zolik/server/internal/prsi"
	"zolik/server/internal/ws"
)

// A lesson's table is an ordinary match with two differences, and only two:
// it is dealt from the lesson's own seed, and it says it is a lesson so that
// statistics, the player's games and the hand-up to the cloud can leave it
// out. Everything else — the engine, the bots, the socket — is the path every
// other table takes.
func TestLessonTableIsDealtFromTheLessonAndMarked(t *testing.T) {
	bothEngines(t, func(t *testing.T, repo match.Repository) {
		hub, err := ws.NewHub(ws.NewConnRegistry(), "")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = hub.Close() })
		mod := prsi.New()
		m := match.NewManager(repo, module.NewRegistry(mod), hub)
		ctx := context.Background()
		host := models.Player{ID: "p1", Name: "Ada"}

		lesson, err := module.FindLesson(mod, module.MatchConfig{}, "basics")
		if err != nil {
			t.Fatal(err)
		}

		got, err := m.CreateLesson(ctx, "prsi", module.MatchConfig{}, host, "basics")
		if err != nil {
			t.Fatalf("CreateLesson: %v", err)
		}
		stored, err := repo.Resolve(ctx, got.ID.Hex())
		if err != nil {
			t.Fatal(err)
		}
		if stored.Seed != lesson.Seed {
			t.Errorf("seed = %d, want the lesson's %d", stored.Seed, lesson.Seed)
		}
		if stored.Tutorial == nil || stored.Tutorial.LessonID != "basics" {
			t.Errorf("tutorial mark = %+v, want the basics lesson", stored.Tutorial)
		}

		plain, err := m.Create(ctx, "prsi", module.MatchConfig{}, host)
		if err != nil {
			t.Fatal(err)
		}
		if p, _ := repo.Resolve(ctx, plain.ID.Hex()); p.Tutorial != nil {
			t.Errorf("an ordinary table carries a tutorial mark: %+v", p.Tutorial)
		}
	})
}

func TestLessonTableRefusesALessonTheGameDoesNotShip(t *testing.T) {
	bothEngines(t, func(t *testing.T, repo match.Repository) {
		hub, err := ws.NewHub(ws.NewConnRegistry(), "")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = hub.Close() })
		m := match.NewManager(repo, module.NewRegistry(prsi.New()), hub)

		_, err = m.CreateLesson(context.Background(), "prsi", module.MatchConfig{}, models.Player{ID: "p1", Name: "Ada"}, "nope")
		if module.CodeOf(err) != "UNKNOWN_LESSON" {
			t.Errorf("error = %v, want UNKNOWN_LESSON", err)
		}
	})
}
