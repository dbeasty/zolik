package match_test

import (
	"context"
	"testing"
	"time"

	"zolik/server/internal/db"
	"zolik/server/internal/match"
	"zolik/server/internal/models"
	"zolik/server/internal/module"
	"zolik/server/internal/prsi"
	"zolik/server/internal/ws"
)

// bothEngines runs a test against the embedded KDB engine and against the dev
// stack's Mongo, which newTestRepository skips when none is reachable. The
// two engines answer EachFinished and FindRetired with different code — a
// scan and a predicate against a query document — so each must be asked.
func bothEngines(t *testing.T, fn func(t *testing.T, repo match.Repository)) {
	t.Run("kdb", func(t *testing.T) {
		k, err := db.OpenKDB(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = k.Close(context.Background()) })
		fn(t, match.NewKDBRepository(k))
	})
	t.Run("mongo", func(t *testing.T) {
		t.Setenv("ZOLIK_TEST_DB_ENGINE", "")
		fn(t, newTestRepository(t))
	})
}

// ended writes a resolved match of one game, ended at a given time.
func ended(t *testing.T, repo match.Repository, game, status string, at time.Time) models.Match {
	t.Helper()
	m := models.Match{
		ModuleID:  game,
		Status:    status,
		Players:   []models.Player{{ID: "p1", Name: "Ada"}, bot("bot:a")},
		TurnOrder: []string{"p1", "bot:a"},
		HostID:    "p1",
		CreatedAt: at.Add(-time.Hour),
	}
	if status != "lobby" && status != "active" {
		e := at
		m.EndedAt = &e
	}
	saved, err := repo.Insert(t.Context(), m)
	if err != nil {
		t.Fatalf("inserting a %s %s match: %v", game, status, err)
	}
	return saved
}

func TestEachFinishedListsOneGamesFinishedMatchesOldestFirst(t *testing.T) {
	bothEngines(t, func(t *testing.T, repo match.Repository) {
		day := func(d int) time.Time { return time.Date(2026, 9, d, 12, 0, 0, 0, time.UTC) }
		third := ended(t, repo, "holdem", "completed", day(3))
		first := ended(t, repo, "holdem", "completed", day(1))
		second := ended(t, repo, "holdem", "completed", day(2))
		gone := ended(t, repo, "holdem", "abandoned", day(2))
		ended(t, repo, "canasta", "completed", day(2))
		ended(t, repo, "holdem", "active", day(2))

		list := func(f match.FinishedFilter) []string {
			var out []string
			if err := repo.EachFinished(t.Context(), f, func(m models.Match) error {
				out = append(out, m.ID.Hex())
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			return out
		}
		same := func(label string, got []string, want ...models.Match) {
			t.Helper()
			if len(got) != len(want) {
				t.Fatalf("%s: listed %d, want %d", label, len(got), len(want))
			}
			for i := range want {
				if got[i] != want[i].ID.Hex() {
					t.Fatalf("%s: position %d is not the match that ended %v", label, i, want[i].EndedAt)
				}
			}
		}

		same("completed hold'em", list(match.FinishedFilter{ModuleID: "holdem"}), first, second, third)
		same("a window", list(match.FinishedFilter{ModuleID: "holdem", EndedFrom: day(2), EndedBefore: day(3)}), second)
		same("a limit", list(match.FinishedFilter{ModuleID: "holdem", Limit: 2}), first, second)
		got := list(match.FinishedFilter{ModuleID: "holdem", Statuses: []string{"completed", "abandoned"}, EndedFrom: day(2), EndedBefore: day(3)})
		if len(got) != 2 || (got[0] != gone.ID.Hex() && got[1] != gone.ID.Hex()) {
			t.Fatalf("asking for abandoned too listed %v", got)
		}
	})
}

// A game with a window of its own keeps its completed matches for that long,
// and a zero window keeps them for ever; every other game is unaffected.
func TestRetentionWindowPerGame(t *testing.T) {
	bothEngines(t, func(t *testing.T, repo match.Repository) {
		hub, err := ws.NewHub(ws.NewConnRegistry(), "")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = hub.Close() })
		m := match.NewManager(repo, module.NewRegistry(prsi.New()), hub)

		now := time.Now().UTC()
		age := func(days int) time.Time { return now.Add(-time.Duration(days) * 24 * time.Hour) }
		w := match.RetentionWindows{
			Completed: 90 * 24 * time.Hour,
			CompletedByGame: map[string]time.Duration{
				"holdem":  365 * 24 * time.Hour,
				"canasta": 0,
			},
		}
		oldPrsi := ended(t, repo, "prsi", "completed", age(120))
		keptHoldem := ended(t, repo, "holdem", "completed", age(120))
		oldHoldem := ended(t, repo, "holdem", "completed", age(400))
		ancientCanasta := ended(t, repo, "canasta", "completed", age(2000))

		if n := m.SweepRetired(t.Context(), w); n != 2 {
			t.Fatalf("deleted %d, want 2", n)
		}
		for _, c := range []struct {
			label string
			m     models.Match
			kept  bool
		}{
			{"prsi past the general window", oldPrsi, false},
			{"hold'em inside its own window", keptHoldem, true},
			{"hold'em past its own window", oldHoldem, false},
			{"canasta, kept for ever", ancientCanasta, true},
		} {
			if exists(t, repo, c.m.ID) != c.kept {
				t.Errorf("%s: kept = %v, want %v", c.label, !c.kept, c.kept)
			}
		}
	})
}

// A per-game window is enough on its own to start the sweeper.
func TestRetentionPerGameWindowAloneIsOn(t *testing.T) {
	if !(match.RetentionWindows{CompletedByGame: map[string]time.Duration{"holdem": time.Hour}}).Any() {
		t.Fatal("a per-game window alone reads as retention off")
	}
	if (match.RetentionWindows{CompletedByGame: map[string]time.Duration{"holdem": 0}}).Any() {
		t.Fatal("keeping one game for ever reads as retention on")
	}
}
