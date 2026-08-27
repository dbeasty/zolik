package scoring

import (
	"context"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/db"
)

func newKDBRepo(t *testing.T) Repository {
	t.Helper()
	k, err := db.OpenKDB(t.TempDir())
	if err != nil {
		t.Fatalf("opening kdb: %v", err)
	}
	t.Cleanup(func() { _ = k.Close(context.Background()) })
	return NewKDBRepository(k)
}

func TestKDBScoringSessionLifecycle(t *testing.T) {
	r := newKDBRepo(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Millisecond)
	s := ScoringSession{
		ID:        bson.NewObjectID(),
		Players:   []PlayerScore{{Name: "Ada", Scores: make([]int, 7)}, {Name: "Grace", Scores: make([]int, 7)}},
		Rounds:    7,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := r.Insert(ctx, s); err != nil {
		t.Fatalf("insert: %v", err)
	}

	got, err := r.FindByID(ctx, s.ID)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if len(got.Players) != 2 || got.Players[0].Name != "Ada" {
		t.Fatalf("players round-trip: %+v", got.Players)
	}

	got.Players[0].Scores[0] = 42
	got.UpdatedAt = now.Add(time.Minute)
	if err := r.Replace(ctx, got); err != nil {
		t.Fatalf("replace: %v", err)
	}
	again, err := r.FindByID(ctx, s.ID)
	if err != nil || again.Players[0].Scores[0] != 42 {
		t.Fatalf("replace did not land: %+v (%v)", again.Players, err)
	}

	if _, err := r.FindByID(ctx, bson.NewObjectID()); !db.IsNotFound(err) {
		t.Fatalf("missing session: got %v, want not-found", err)
	}
}
