package user

import (
	"context"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/db"
	"zolik/server/internal/models"
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

func TestKDBCreateAndFindUser(t *testing.T) {
	r := newKDBRepo(t)
	ctx := context.Background()

	u, err := r.CreateUser(ctx, models.User{Username: "ada", Preferences: models.UserPreferences{Language: "cs", CardStyle: "classic"}})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if u.ID.IsZero() {
		t.Fatal("create assigned no id")
	}

	byName, err := r.FindByUsername(ctx, "ada")
	if err != nil || byName.ID != u.ID {
		t.Fatalf("find by name: %v", err)
	}
	byID, err := r.FindByID(ctx, u.ID)
	if err != nil || byID.Username != "ada" {
		t.Fatalf("find by id: %v", err)
	}
	if byID.Preferences.Language != "cs" {
		t.Fatalf("preferences lost: %+v", byID.Preferences)
	}

	if _, err := r.CreateUser(ctx, models.User{Username: "ada"}); !db.IsDuplicateKey(err) {
		t.Fatalf("duplicate create: got %v, want duplicate-key", err)
	}
}

func TestKDBUpdateByIDPatchesAndGuardsUsername(t *testing.T) {
	r := newKDBRepo(t)
	ctx := context.Background()

	ada, err := r.CreateUser(ctx, models.User{Username: "ada"})
	if err != nil {
		t.Fatalf("create ada: %v", err)
	}
	if _, err := r.CreateUser(ctx, models.User{Username: "grace"}); err != nil {
		t.Fatalf("create grace: %v", err)
	}

	// Renaming onto a taken name must surface as the duplicate the handler
	// turns into a 409.
	if err := r.UpdateByID(ctx, ada.ID, bson.M{"username": "grace"}); !db.IsDuplicateKey(err) {
		t.Fatalf("rename onto taken name: got %v, want duplicate-key", err)
	}

	// A partial patch must only touch its fields.
	if err := r.UpdateByID(ctx, ada.ID, bson.M{"preferences": models.UserPreferences{Language: "en", CardStyle: "modern"}}); err != nil {
		t.Fatalf("patch preferences: %v", err)
	}
	got, err := r.FindByID(ctx, ada.ID)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got.Username != "ada" {
		t.Fatalf("username changed by an unrelated patch: %q", got.Username)
	}
	if got.Preferences.CardStyle != "modern" {
		t.Fatalf("patch did not land: %+v", got.Preferences)
	}
}
