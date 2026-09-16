package match

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/db"
	"zolik/server/internal/models"
)

// These always run: the KDB engine needs nothing external, so the semantic
// contracts the runtime leans on — the version CAS above all — are checked
// on every plain `go test ./...`. The same contracts are exercised against
// Mongo by the invite tests when a Mongo is reachable.

func newKDBRepo(t *testing.T) Repository {
	t.Helper()
	k, err := db.OpenKDB(t.TempDir())
	if err != nil {
		t.Fatalf("opening kdb: %v", err)
	}
	t.Cleanup(func() { _ = k.Close(context.Background()) })
	return NewKDBRepository(k)
}

func TestKDBInsertAndResolve(t *testing.T) {
	r := newKDBRepo(t)
	ctx := context.Background()

	m, err := r.Insert(ctx, models.Match{ModuleID: "prsi", Status: "lobby", JoinCode: "ABC123"})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if m.ID.IsZero() || m.Version != 1 {
		t.Fatalf("insert normalized badly: id=%v version=%d", m.ID, m.Version)
	}

	byID, err := r.Resolve(ctx, m.ID.Hex())
	if err != nil || byID.ID != m.ID {
		t.Fatalf("resolve by id: %v (%v)", err, byID.ID)
	}
	byCode, err := r.Resolve(ctx, "ABC123")
	if err != nil || byCode.ID != m.ID {
		t.Fatalf("resolve by code: %v (%v)", err, byCode.ID)
	}
	if _, err := r.Resolve(ctx, "NOPE99"); err == nil {
		t.Fatal("resolving an unknown code succeeded")
	}
}

func TestKDBUpdateWithVersionIsCAS(t *testing.T) {
	r := newKDBRepo(t)
	ctx := context.Background()

	m, err := r.Insert(ctx, models.Match{ModuleID: "prsi", Status: "lobby", JoinCode: "CAS111"})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	next := m
	next.Status = "active"
	if err := r.UpdateWithVersion(ctx, m.ID, 1, next); err != nil {
		t.Fatalf("first update: %v", err)
	}

	// A writer holding the stale version must lose, and must lose with the
	// sentinel the runtime retries on.
	stale := m
	stale.Status = "completed"
	if err := r.UpdateWithVersion(ctx, m.ID, 1, stale); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("stale update: got %v, want ErrVersionConflict", err)
	}

	got, err := r.FindByID(ctx, m.ID)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got.Status != "active" || got.Version != 2 {
		t.Fatalf("loser overwrote the winner: status=%s version=%d", got.Status, got.Version)
	}

	// And against a document that is gone entirely.
	if err := r.UpdateWithVersion(ctx, bson.NewObjectID(), 1, next); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("update of missing doc: got %v, want ErrVersionConflict", err)
	}
}

func TestKDBMatchStateSurvivesRoundTrip(t *testing.T) {
	r := newKDBRepo(t)
	ctx := context.Background()

	m, err := r.Insert(ctx, models.Match{
		ModuleID: "prsi",
		Status:   "active",
		State:    []byte(`{"drawPile":[1,2,3],"nested":{"deep":true}}`),
		Players:  []models.Player{{ID: "p1", Name: "Ada", GuestID: "0123456789abcdef0123456789abcdef"}},
	})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	got, err := r.FindByID(ctx, m.ID)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if string(got.State) != string(m.State) {
		t.Fatalf("opaque state changed in storage:\n in: %s\nout: %s", m.State, got.State)
	}
	// GuestID is json:"-" but bson-persisted; it must survive storage.
	if got.Players[0].GuestID != "0123456789abcdef0123456789abcdef" {
		t.Fatalf("guest id lost: %+v", got.Players[0])
	}
}

func TestKDBFindForPlayerFiltersOrdersAndStripsState(t *testing.T) {
	r := newKDBRepo(t)
	ctx := context.Background()

	// Newest activity first: the KDB path derives this from UpdatedAt, which
	// only UpdateWithVersion sets, so it is stamped directly here to control
	// the ordering rather than relying on timing between two Inserts.
	older, err := r.Insert(ctx, models.Match{
		ModuleID: "prsi", Status: "lobby",
		Players: []models.Player{{ID: "p1", Name: "Ada"}},
		State:   []byte(`{"secret":true}`),
	})
	if err != nil {
		t.Fatalf("insert older: %v", err)
	}
	older.UpdatedAt = time.Now().UTC().Add(-time.Hour)
	if err := r.UpdateWithVersion(ctx, older.ID, older.Version, older); err != nil {
		t.Fatalf("backdating older: %v", err)
	}

	newer, err := r.Insert(ctx, models.Match{
		ModuleID: "prsi", Status: "active",
		Players: []models.Player{{ID: "p1", Name: "Ada"}},
	})
	if err != nil {
		t.Fatalf("insert newer: %v", err)
	}
	// Insert alone leaves UpdatedAt at its zero value — only UpdateWithVersion
	// stamps it — so this writes it forward to "now" the same way any real
	// action on the match would, rather than leaving the comparison against
	// `older` decided by an unset field.
	if err := r.UpdateWithVersion(ctx, newer.ID, newer.Version, newer); err != nil {
		t.Fatalf("touching newer: %v", err)
	}

	// Belongs to a different seat entirely; must never come back for p1.
	if _, err := r.Insert(ctx, models.Match{
		ModuleID: "prsi", Status: "lobby",
		Players: []models.Player{{ID: "someone-else", Name: "Bo"}},
	}); err != nil {
		t.Fatalf("insert other player's match: %v", err)
	}

	// Finished, and excluded by the default (unfinished) filter below.
	if _, err := r.Insert(ctx, models.Match{
		ModuleID: "prsi", Status: "completed",
		Players: []models.Player{{ID: "p1", Name: "Ada"}},
	}); err != nil {
		t.Fatalf("insert completed match: %v", err)
	}

	got, err := r.FindForPlayer(ctx, "p1", PlayerMatchFilter{Statuses: UnfinishedStatuses})
	if err != nil {
		t.Fatalf("FindForPlayer: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d matches, want 2 (completed and the other player's excluded): %+v", len(got), got)
	}
	if got[0].ID != newer.ID || got[1].ID != older.ID {
		t.Fatalf("order = [%s, %s], want newest activity first: [%s, %s]",
			got[0].ID.Hex(), got[1].ID.Hex(), newer.ID.Hex(), older.ID.Hex())
	}
	// The whole point of the projection: a list row must never carry the
	// bytes that make this collection expensive, or the module's state along
	// with them.
	for _, m := range got {
		if m.State != nil {
			t.Errorf("match %s: State leaked into a list row: %s", m.ID.Hex(), m.State)
		}
	}

	finished, err := r.FindForPlayer(ctx, "p1", PlayerMatchFilter{Statuses: FinishedStatuses})
	if err != nil {
		t.Fatalf("FindForPlayer(finished): %v", err)
	}
	if len(finished) != 1 || finished[0].Status != "completed" {
		t.Fatalf("finished tab = %+v, want exactly the one completed match", finished)
	}
}

func TestKDBFindForPlayerLimitsAndDefaults(t *testing.T) {
	r := newKDBRepo(t)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		if _, err := r.Insert(ctx, models.Match{
			ModuleID: "prsi", Status: "lobby",
			Players: []models.Player{{ID: "p1", Name: "Ada"}},
		}); err != nil {
			t.Fatalf("insert %d: %v", i, err)
		}
	}

	// No statuses named at all: the zero filter is "every unfinished status",
	// which is what a fresh "my games" screen opens on.
	got, err := r.FindForPlayer(ctx, "p1", PlayerMatchFilter{})
	if err != nil {
		t.Fatalf("FindForPlayer: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d matches with a zero filter, want all 3", len(got))
	}

	limited, err := r.FindForPlayer(ctx, "p1", PlayerMatchFilter{Limit: 2})
	if err != nil {
		t.Fatalf("FindForPlayer(limit 2): %v", err)
	}
	if len(limited) != 2 {
		t.Fatalf("got %d matches with Limit: 2, want 2", len(limited))
	}
}
