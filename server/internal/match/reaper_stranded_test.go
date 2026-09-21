package match_test

import (
	"testing"
	"time"

	"zolik/server/internal/match"
	"zolik/server/internal/models"
)

// idleActiveMatch writes an active match whose last activity the caller chooses,
// the way one is left when its sockets never existed or died with the process.
func idleActiveMatch(t *testing.T, repo match.Repository, updatedAt time.Time) models.Match {
	t.Helper()
	created := updatedAt.Add(-time.Minute)
	m, err := repo.Insert(t.Context(), models.Match{
		ModuleID:  "prsi",
		Status:    "active",
		Players:   []models.Player{{ID: "p1", Name: "Ada"}, {ID: "p2", Name: "Bo"}},
		TurnOrder: []string{"p1", "p2"},
		HostID:    "p1",
		CreatedAt: created,
		StartedAt: &created,
		UpdatedAt: updatedAt,
	})
	if err != nil {
		t.Fatalf("inserting an active match: %v", err)
	}
	return m
}

// The bug this closes: SuspendOnDisconnect is the only way out of "active",
// and it needs a socket to drop. The dev stack held 509 active matches nobody
// was playing, none of which anything would ever resolve.
func TestReaperAbandonsAStrandedActiveMatch(t *testing.T) {
	ctx := t.Context()
	m, repo, sink := newReaperHarness(t)

	stranded := idleActiveMatch(t, repo, time.Now().UTC().Add(-match.StrandedAfter-time.Minute))

	if n := m.ReapAbandoned(ctx); n != 1 {
		t.Fatalf("reaped %d matches, want 1", n)
	}
	after, _ := repo.FindByID(ctx, stranded.ID)
	if after.Status != "abandoned" {
		t.Errorf("status = %q, want abandoned", after.Status)
	}
	if after.EndedAt == nil {
		t.Error("an abandoned match has no EndedAt")
	}
	if got := sink.counts["matches.abandoned.prsi"]; got != 1 {
		t.Errorf("matches.abandoned.prsi = %d, want 1", got)
	}
	// Once, not once per tick.
	if n := m.ReapAbandoned(ctx); n != 0 {
		t.Errorf("a second sweep reaped %d, want 0", n)
	}
}

// A quiet table is not a stranded one while it is inside the window: a player
// whose socket died in a restart is back long before it runs out.
func TestReaperLeavesARecentlyActiveMatch(t *testing.T) {
	ctx := t.Context()
	m, repo, _ := newReaperHarness(t)

	recent := idleActiveMatch(t, repo, time.Now().UTC().Add(-match.StrandedAfter+time.Minute))

	if n := m.ReapAbandoned(ctx); n != 0 {
		t.Fatalf("reaped %d matches, want 0", n)
	}
	if after, _ := repo.FindByID(ctx, recent.ID); after.Status != "active" {
		t.Errorf("status = %q, want active", after.Status)
	}
}

// Somebody at the table is playing it, however long they have been thinking.
func TestReaperLeavesAnIdleMatchSomebodyIsAt(t *testing.T) {
	ctx := t.Context()
	m, repo, _ := newReaperHarness(t)

	thinking := idleActiveMatch(t, repo, time.Now().UTC().Add(-time.Hour))
	leave := sitDown(t, m, thinking.ID.Hex(), "p2")

	if n := m.ReapAbandoned(ctx); n != 0 {
		t.Fatalf("reaped %d matches, want 0 — p2 is at the table", n)
	}
	leave()
	if n := m.ReapAbandoned(ctx); n != 1 {
		t.Fatalf("reaped %d matches after p2 left, want 1", n)
	}
}

// A row written before UpdatedAt existed is judged by when it started, not
// treated as untouched since the epoch.
func TestReaperJudgesARowWithoutUpdatedAtByItsStart(t *testing.T) {
	ctx := t.Context()
	m, repo, _ := newReaperHarness(t)

	started := time.Now().UTC().Add(-time.Minute)
	fresh, err := repo.Insert(ctx, models.Match{
		ModuleID:  "prsi",
		Status:    "active",
		Players:   []models.Player{{ID: "p1", Name: "Ada"}, {ID: "p2", Name: "Bo"}},
		TurnOrder: []string{"p1", "p2"},
		HostID:    "p1",
		CreatedAt: started.Add(-time.Hour),
		StartedAt: &started,
	})
	if err != nil {
		t.Fatalf("inserting: %v", err)
	}

	if n := m.ReapAbandoned(ctx); n != 0 {
		t.Fatalf("reaped %d matches, want 0 — it started a minute ago", n)
	}
	if after, _ := repo.FindByID(ctx, fresh.ID); after.Status != "active" {
		t.Errorf("status = %q, want active", after.Status)
	}
}

// A lobby or a finished match is not "active", whatever its age.
func TestReaperLeavesOldMatchesInOtherStatuses(t *testing.T) {
	ctx := t.Context()
	m, repo, _ := newReaperHarness(t)

	old := time.Now().UTC().Add(-24 * time.Hour)
	for _, status := range []string{"lobby", "completed", "abandoned"} {
		if _, err := repo.Insert(ctx, models.Match{
			ModuleID: "prsi", Status: status, HostID: "p1", CreatedAt: old, UpdatedAt: old,
			Players: []models.Player{{ID: "p1", Name: "Ada"}},
		}); err != nil {
			t.Fatalf("inserting a %s match: %v", status, err)
		}
	}
	if n := m.ReapAbandoned(ctx); n != 0 {
		t.Fatalf("reaped %d matches, want 0", n)
	}
}
