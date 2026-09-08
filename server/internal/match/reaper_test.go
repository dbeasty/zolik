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

// reaperHarness is a manager over a real KDB repository, with a counting sink.
func newReaperHarness(t *testing.T) (*match.Manager, match.Repository, *reaperSink) {
	t.Helper()

	k, err := db.OpenKDB(t.TempDir())
	if err != nil {
		t.Fatalf("opening kdb: %v", err)
	}
	t.Cleanup(func() { _ = k.Close(context.Background()) })
	repo := match.NewKDBRepository(k)

	hub, err := ws.NewHub(ws.NewConnRegistry(), "")
	if err != nil {
		t.Fatalf("building a hub: %v", err)
	}
	t.Cleanup(func() { _ = hub.Close() })

	m := match.NewManager(repo, module.NewRegistry(prsi.New()), hub)
	sink := &reaperSink{counts: map[string]int64{}}
	m.SetMetrics(sink)
	return m, repo, sink
}

type reaperSink struct{ counts map[string]int64 }

func (s *reaperSink) Add(name string, n int64) { s.counts[name] += n }
func (s *reaperSink) SeePlayer(string)         {}

// suspendedMatch writes a match sitting in the state SuspendOnDisconnect
// leaves behind, with an abandon deadline the caller chooses.
func suspendedMatch(t *testing.T, repo match.Repository, abandonAt time.Time) models.Match {
	t.Helper()
	now := time.Now().UTC().Add(-5 * time.Minute)
	m, err := repo.Insert(context.Background(), models.Match{
		ModuleID:        "prsi",
		Status:          "suspended",
		Players:         []models.Player{{ID: "p1", Name: "Ada"}, {ID: "p2", Name: "Bo"}},
		TurnOrder:       []string{"p1", "p2"},
		HostID:          "p1",
		SuspendedAt:     &now,
		AbandonAt:       &abandonAt,
		SuspendedPlayer: "p2",
		CreatedAt:       now,
	})
	if err != nil {
		t.Fatalf("inserting a suspended match: %v", err)
	}
	return m
}

// The bug this closes: AbandonAt was written by SuspendOnDisconnect and read
// by nothing, so a table whose player closed the tab stayed suspended for
// ever — not finished, not abandoned, not counted.
func TestReaperAbandonsAnExpiredMatch(t *testing.T) {
	ctx := t.Context()
	m, repo, sink := newReaperHarness(t)

	stranded := suspendedMatch(t, repo, time.Now().UTC().Add(-time.Minute))

	if n := m.ReapAbandoned(ctx); n != 1 {
		t.Fatalf("reaped %d matches, want 1", n)
	}

	after, err := repo.FindByID(ctx, stranded.ID)
	if err != nil {
		t.Fatalf("reloading: %v", err)
	}
	if after.Status != "abandoned" {
		t.Errorf("status = %q, want abandoned", after.Status)
	}
	if after.EndedAt == nil {
		t.Error("an abandoned match has no EndedAt")
	}
	// Cleared, so a row that somehow stays in this state is not swept again on
	// every subsequent tick.
	if after.AbandonAt != nil {
		t.Error("AbandonAt was not cleared")
	}
	if got := sink.counts["matches.abandoned"]; got != 1 {
		t.Errorf("matches.abandoned = %d, want 1", got)
	}
	if got := sink.counts["matches.abandoned.prsi"]; got != 1 {
		t.Errorf("matches.abandoned.prsi = %d, want 1", got)
	}
}

// The window is the promise: a player has AbandonWindow to come back, and a
// sweep that ran early would take the game away from someone still inside it.
func TestReaperLeavesAMatchInsideItsWindow(t *testing.T) {
	ctx := t.Context()
	m, repo, sink := newReaperHarness(t)

	waiting := suspendedMatch(t, repo, time.Now().UTC().Add(time.Minute))

	if n := m.ReapAbandoned(ctx); n != 0 {
		t.Fatalf("reaped %d matches, want 0", n)
	}
	after, _ := repo.FindByID(ctx, waiting.ID)
	if after.Status != "suspended" {
		t.Errorf("status = %q, want suspended", after.Status)
	}
	if got := sink.counts["matches.abandoned"]; got != 0 {
		t.Errorf("matches.abandoned = %d, want 0", got)
	}
}

// An active match is never touched, whatever stale deadline it happens to
// carry — a player who came back has their game, and ResumeIfReturning does
// not clear AbandonAt.
func TestReaperIgnoresActiveMatches(t *testing.T) {
	ctx := t.Context()
	m, repo, _ := newReaperHarness(t)

	resumed := suspendedMatch(t, repo, time.Now().UTC().Add(-time.Minute))
	loaded, _ := repo.FindByID(ctx, resumed.ID)
	loaded.Status = "active"
	if err := repo.UpdateWithVersion(ctx, loaded.ID, loaded.Version, loaded); err != nil {
		t.Fatalf("resuming: %v", err)
	}

	if n := m.ReapAbandoned(ctx); n != 0 {
		t.Fatalf("reaped %d matches, want 0 — the player came back", n)
	}
	after, _ := repo.FindByID(ctx, resumed.ID)
	if after.Status != "active" {
		t.Errorf("status = %q, want active", after.Status)
	}
}

// Abandonment is a runtime fact, not a result. The lifetime aggregates are
// rebuilt from stats.MatchResult rows, so a half-played table folded into
// them would corrupt every average it touched — for people who never agreed
// to play it out.
func TestReaperWritesNoMatchResult(t *testing.T) {
	ctx := t.Context()
	m, repo, _ := newReaperHarness(t)
	recorder := &countingRecorder{}
	m.SetRecorder(recorder)

	suspendedMatch(t, repo, time.Now().UTC().Add(-time.Minute))
	m.ReapAbandoned(ctx)

	if recorder.calls != 0 {
		t.Errorf("the recorder was called %d times for an abandoned match, want 0", recorder.calls)
	}
}

type countingRecorder struct{ calls int }

func (r *countingRecorder) RecordMatchAsync(models.Match, module.Outcome) { r.calls++ }

func TestReaperIsIdempotent(t *testing.T) {
	ctx := t.Context()
	m, repo, sink := newReaperHarness(t)

	suspendedMatch(t, repo, time.Now().UTC().Add(-time.Minute))
	m.ReapAbandoned(ctx)
	// A second sweep must find nothing: the count an operator reads is one
	// game abandoned, not one per tick for the rest of the day.
	m.ReapAbandoned(ctx)

	if got := sink.counts["matches.abandoned"]; got != 1 {
		t.Errorf("matches.abandoned = %d after two sweeps, want 1", got)
	}
}

func TestReaperHandlesABacklog(t *testing.T) {
	ctx := t.Context()
	m, repo, sink := newReaperHarness(t)

	// The first run after this ships meets every table ever stranded.
	for i := 0; i < 5; i++ {
		suspendedMatch(t, repo, time.Now().UTC().Add(-time.Duration(i+1)*time.Minute))
	}
	if n := m.ReapAbandoned(ctx); n != 5 {
		t.Fatalf("reaped %d, want 5", n)
	}
	if got := sink.counts["matches.abandoned"]; got != 5 {
		t.Errorf("matches.abandoned = %d, want 5", got)
	}
}
