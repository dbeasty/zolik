package match_test

import (
	"context"
	"testing"
	"time"

	"zolik/server/internal/match"
	"zolik/server/internal/metrics"
	"zolik/server/internal/models"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Generous enough that nothing in these tests is near a boundary by accident.
var windows = match.RetentionWindows{
	Lobby:     24 * time.Hour,
	Completed: 90 * 24 * time.Hour,
	Abandoned: 30 * 24 * time.Hour,
}

// resolved writes a match in `status`, aged `age` at whichever date that status
// is judged by — createdAt for a lobby, endedAt for anything finished.
func resolved(t *testing.T, repo match.Repository, status string, age time.Duration) models.Match {
	t.Helper()
	at := time.Now().UTC().Add(-age)
	m := models.Match{
		ModuleID:  "prsi",
		Status:    status,
		Players:   []models.Player{{ID: "p1", Name: "Ada"}, bot("bot:a")},
		TurnOrder: []string{"p1", "bot:a"},
		HostID:    "p1",
		CreatedAt: at,
	}
	if status != "lobby" {
		ended := at
		m.EndedAt = &ended
		// Created before it ended, so a lobby window could never be what
		// retires a finished game.
		m.CreatedAt = at.Add(-time.Hour)
	}
	saved, err := repo.Insert(context.Background(), m)
	if err != nil {
		t.Fatalf("inserting a %s match: %v", status, err)
	}
	return saved
}

func exists(t *testing.T, repo match.Repository, id bson.ObjectID) bool {
	t.Helper()
	_, err := repo.FindByID(context.Background(), id)
	return err == nil
}

// The question this feature answers: at what point is the game deleted. Never,
// before this — `matches` was 92% of the production database and nothing had
// ever removed a row from it.
func TestRetentionDeletesResolvedMatchesPastTheirWindow(t *testing.T) {
	ctx := t.Context()
	m, repo, sink := newReaperHarness(t)

	oldLobby := resolved(t, repo, "lobby", 48*time.Hour)
	oldDone := resolved(t, repo, "completed", 120*24*time.Hour)
	oldGone := resolved(t, repo, "abandoned", 60*24*time.Hour)

	if n := m.SweepRetired(ctx, windows); n != 3 {
		t.Fatalf("deleted %d, want 3", n)
	}
	for name, id := range map[string]bson.ObjectID{
		"lobby": oldLobby.ID, "completed": oldDone.ID, "abandoned": oldGone.ID,
	} {
		if exists(t, repo, id) {
			t.Errorf("an over-age %s match survived retention", name)
		}
	}
	if got := sink.counts[metrics.MatchesDeleted]; got != 3 {
		t.Errorf("%s = %d, want 3", metrics.MatchesDeleted, got)
	}
}

// The window is the whole point: a game inside it is not litter yet.
func TestRetentionKeepsResolvedMatchesInsideTheirWindow(t *testing.T) {
	ctx := t.Context()
	m, repo, _ := newReaperHarness(t)

	young := []models.Match{
		resolved(t, repo, "lobby", time.Hour),
		resolved(t, repo, "completed", 24*time.Hour),
		resolved(t, repo, "abandoned", 24*time.Hour),
	}

	if n := m.SweepRetired(ctx, windows); n != 0 {
		t.Fatalf("deleted %d, want 0", n)
	}
	for _, saved := range young {
		if !exists(t, repo, saved.ID) {
			t.Errorf("a %s match inside its window was deleted", saved.Status)
		}
	}
}

// The safety property, and the one worth stating as a test rather than a
// comment: a table somebody could still be sitting at is never a candidate,
// however old it is. `active` and `suspended` are absent from `retired`'s
// switch precisely so that no window can ever apply to them.
func TestRetentionNeverDeletesALiveTable(t *testing.T) {
	ctx := t.Context()
	m, repo, _ := newReaperHarness(t)

	// A year old — far past every window, and still not the sweeper's business.
	live := resolved(t, repo, "active", 365*24*time.Hour)
	paused := resolved(t, repo, "suspended", 365*24*time.Hour)

	if n := m.SweepRetired(ctx, windows); n != 0 {
		t.Fatalf("deleted %d, want 0", n)
	}
	if !exists(t, repo, live.ID) {
		t.Error("an active match was deleted by retention")
	}
	if !exists(t, repo, paused.ID) {
		t.Error("a suspended match was deleted by retention")
	}
}

// A zero window is how an operator says "keep these for ever", and it has to
// mean that rather than "keep them for no time at all".
func TestRetentionZeroWindowKeepsEverything(t *testing.T) {
	ctx := t.Context()
	m, repo, _ := newReaperHarness(t)
	ancient := resolved(t, repo, "completed", 365*24*time.Hour)

	if n := m.SweepRetired(ctx, match.RetentionWindows{}); n != 0 {
		t.Fatalf("deleted %d with every window off, want 0", n)
	}
	if !exists(t, repo, ancient.ID) {
		t.Error("a match was deleted with retention disabled")
	}
}

// Each class is judged on its own window, so switching one on does not
// quietly switch on the others.
func TestRetentionAppliesEachWindowSeparately(t *testing.T) {
	ctx := t.Context()
	m, repo, _ := newReaperHarness(t)

	lobby := resolved(t, repo, "lobby", 48*time.Hour)
	done := resolved(t, repo, "completed", 48*time.Hour)

	// Only the lobby window is on, and the completed game is well past 48h of
	// nothing — but its own window is off.
	if n := m.SweepRetired(ctx, match.RetentionWindows{Lobby: 24 * time.Hour}); n != 1 {
		t.Fatalf("deleted %d, want 1", n)
	}
	if exists(t, repo, lobby.ID) {
		t.Error("the over-age lobby survived its own window")
	}
	if !exists(t, repo, done.ID) {
		t.Error("a completed match was deleted while its window was off")
	}
}

// The race resume created. An abandoned table is a deletion candidate right up
// until somebody picks it back up, and the version guard is what decides it.
// Losing this race must mean the player keeps their game.
func TestRetentionLosesToAPlayerResumingTheTable(t *testing.T) {
	ctx := t.Context()
	m, repo, _ := newReaperHarness(t)
	old := resolved(t, repo, "abandoned", 60*24*time.Hour)

	// The scan has read the row; the player resumes before the delete lands.
	stale := old.Version
	if err := m.ResumeAbandoned(ctx, old.ID.Hex(), "p1"); err != nil {
		t.Fatalf("resuming: %v", err)
	}

	if err := repo.DeleteIfUnchanged(ctx, old.ID, stale); err == nil {
		t.Fatal("a stale delete succeeded against a table that had been resumed")
	}
	if !exists(t, repo, old.ID) {
		t.Fatal("retention deleted a match out from under the player who resumed it")
	}

	// And the sweeper as a whole leaves it alone now, because it is active.
	if n := m.SweepRetired(ctx, windows); n != 0 {
		t.Errorf("deleted %d after the resume, want 0", n)
	}
}

// One pass is bounded, so the first run after this ships — which meets every
// match ever played — works through the backlog over successive ticks instead
// of one long delete burst against a database that is also serving games.
func TestRetentionBoundsOnePass(t *testing.T) {
	ctx := t.Context()
	m, repo, _ := newReaperHarness(t)
	for i := 0; i < 205; i++ {
		resolved(t, repo, "completed", 120*24*time.Hour)
	}

	first := m.SweepRetired(ctx, windows)
	if first != 200 {
		t.Fatalf("first pass deleted %d, want the 200 batch bound", first)
	}
	if second := m.SweepRetired(ctx, windows); second != 5 {
		t.Errorf("second pass deleted %d, want the remaining 5", second)
	}
}
