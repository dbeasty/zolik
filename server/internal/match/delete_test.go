package match_test

import (
	"testing"

	"zolik/server/internal/match"
	"zolik/server/internal/metrics"
	"zolik/server/internal/models"
	"zolik/server/internal/module"
)

// activeMatch writes a match in the ordinary, still-being-played state —
// the case DeleteAsHost is meant to allow, since the rule is deliberately
// blunt: the host may delete in any status, not only a resolved one.
func activeMatch(t *testing.T, repo match.Repository, hostID string, others ...models.Player) models.Match {
	t.Helper()
	players := append([]models.Player{{ID: hostID, Name: "Host"}}, others...)
	order := make([]string, 0, len(players))
	for _, p := range players {
		order = append(order, p.ID)
	}
	m, err := repo.Insert(t.Context(), models.Match{
		ModuleID:  "prsi",
		Status:    "active",
		Players:   players,
		TurnOrder: order,
		HostID:    hostID,
	})
	if err != nil {
		t.Fatalf("inserting an active match: %v", err)
	}
	return m
}

// A host clearing their own table off their list is the ordinary case this
// exists for, and it must actually remove the row rather than just marking
// it — a "my games" list re-reads the collection, not a status field.
func TestDeleteAsHostRemovesTheRow(t *testing.T) {
	ctx := t.Context()
	m, repo, _ := newReaperHarness(t)
	saved := activeMatch(t, repo, "p1", bot("bot:a"))

	if err := m.DeleteAsHost(ctx, saved.ID.Hex(), "p1"); err != nil {
		t.Fatalf("deleting: %v", err)
	}
	if _, err := repo.FindByID(ctx, saved.ID); err == nil {
		t.Fatal("match still readable after DeleteAsHost")
	}
}

// The rule is deliberately blunt in the other direction too: any status may
// be deleted, including one with another human still seated. There is no
// vote and no "only if everyone left" grace period — see DeleteAsHost's own
// comment for why.
func TestDeleteAsHostWorksWithOtherHumansSeated(t *testing.T) {
	ctx := t.Context()
	m, repo, _ := newReaperHarness(t)
	saved := activeMatch(t, repo, "p1", models.Player{ID: "p2", Name: "Bo"})

	if err := m.DeleteAsHost(ctx, saved.ID.Hex(), "p1"); err != nil {
		t.Fatalf("deleting: %v", err)
	}
	if _, err := repo.FindByID(ctx, saved.ID); err == nil {
		t.Fatal("match still readable after DeleteAsHost")
	}
}

// A completed match is the safe end of the range: its permanent record
// already left for match_results, so deleting the board discards nothing
// the statistics need.
func TestDeleteAsHostWorksOnACompletedMatch(t *testing.T) {
	ctx := t.Context()
	m, repo, _ := newReaperHarness(t)
	saved := activeMatch(t, repo, "p1")
	saved.Status = "completed"
	if err := repo.UpdateWithVersion(ctx, saved.ID, saved.Version, saved); err != nil {
		t.Fatalf("marking completed: %v", err)
	}
	saved.Version++

	if err := m.DeleteAsHost(ctx, saved.ID.Hex(), "p1"); err != nil {
		t.Fatalf("deleting a completed match: %v", err)
	}
}

// A table is not a public resource: only the host may end it, and only a
// seated non-host is even told who the host is (NOT_THE_HOST) rather than
// treated as a stranger.
func TestDeleteAsHostRefusesANonHostSeat(t *testing.T) {
	ctx := t.Context()
	m, repo, _ := newReaperHarness(t)
	saved := activeMatch(t, repo, "p1", models.Player{ID: "p2", Name: "Bo"})

	err := m.DeleteAsHost(ctx, saved.ID.Hex(), "p2")
	if module.CodeOf(err) != "NOT_THE_HOST" {
		t.Fatalf("code = %q, want NOT_THE_HOST (err %v)", module.CodeOf(err), err)
	}
	if _, err := repo.FindByID(ctx, saved.ID); err != nil {
		t.Fatalf("a refused delete removed the row: %v", err)
	}
}

// A stranger gets a different refusal from a seated non-host: NOT_AT_THIS_TABLE
// says they were never in this game at all, which NOT_THE_HOST does not.
func TestDeleteAsHostRefusesAStranger(t *testing.T) {
	ctx := t.Context()
	m, repo, _ := newReaperHarness(t)
	saved := activeMatch(t, repo, "p1")

	err := m.DeleteAsHost(ctx, saved.ID.Hex(), "stranger")
	if module.CodeOf(err) != "NOT_AT_THIS_TABLE" {
		t.Fatalf("code = %q, want NOT_AT_THIS_TABLE (err %v)", module.CodeOf(err), err)
	}
}

func TestDeleteAsHostUnknownMatch(t *testing.T) {
	ctx := t.Context()
	m, _, _ := newReaperHarness(t)

	err := m.DeleteAsHost(ctx, "60b1f0f0f0f0f0f0f0f0f0f0", "p1")
	if module.CodeOf(err) != "MATCH_NOT_FOUND" {
		t.Fatalf("code = %q, want MATCH_NOT_FOUND (err %v)", module.CodeOf(err), err)
	}
}

// The metric this feature adds is its own: folding a player's own tidy-up
// into the sweeper's counter would make a busy day of deletes look like a
// misconfigured retention window.
func TestDeleteAsHostCountsItsOwnMetric(t *testing.T) {
	ctx := t.Context()
	m, repo, sink := newReaperHarness(t)
	saved := activeMatch(t, repo, "p1")

	if err := m.DeleteAsHost(ctx, saved.ID.Hex(), "p1"); err != nil {
		t.Fatalf("deleting: %v", err)
	}
	if got := sink.counts[metrics.MatchesDeletedByHost]; got != 1 {
		t.Errorf("%s = %d, want 1", metrics.MatchesDeletedByHost, got)
	}
	if got := sink.counts[metrics.MatchesDeleted]; got != 0 {
		t.Errorf("a host delete incremented retention's own counter (%d); it should not", got)
	}
}

// A double press deletes once. The second finds nothing to resolve, the same
// refusal a stale client following the same link a moment later would get.
func TestDeleteAsHostTwiceDeletesOnce(t *testing.T) {
	ctx := t.Context()
	m, repo, sink := newReaperHarness(t)
	saved := activeMatch(t, repo, "p1")

	if err := m.DeleteAsHost(ctx, saved.ID.Hex(), "p1"); err != nil {
		t.Fatalf("first delete: %v", err)
	}
	if err := m.DeleteAsHost(ctx, saved.ID.Hex(), "p1"); module.CodeOf(err) != "MATCH_NOT_FOUND" {
		t.Fatalf("second delete: code = %q, want MATCH_NOT_FOUND", module.CodeOf(err))
	}
	if got := sink.counts[metrics.MatchesDeletedByHost]; got != 1 {
		t.Errorf("%s = %d, want 1", metrics.MatchesDeletedByHost, got)
	}
}
