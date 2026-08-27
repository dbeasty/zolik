package match

import (
	"context"
	"errors"
	"testing"

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
