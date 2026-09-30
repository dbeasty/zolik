package botsettings_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"zolik/server/internal/botsettings"
	"zolik/server/internal/db"
)

// Both engines, the same assertions. KDB always runs; Mongo runs against
// ZOLIK_TEST_MONGO_URI (default the dev compose mapping, 127.0.0.1:27018) and
// skips without one, as the other Mongo-backed tests here do.
func engines(t *testing.T) map[string]func(t *testing.T) botsettings.Store {
	return map[string]func(t *testing.T) botsettings.Store{
		"kdb":   kdbStore,
		"mongo": mongoStore,
	}
}

func kdbStore(t *testing.T) botsettings.Store {
	t.Helper()
	k, err := db.OpenKDB(t.TempDir())
	if err != nil {
		t.Fatalf("open kdb: %v", err)
	}
	t.Cleanup(func() { _ = k.Close(context.Background()) })
	return botsettings.NewKDBStore(k)
}

func mongoStore(t *testing.T) botsettings.Store {
	t.Helper()
	uri := strings.TrimSpace(os.Getenv("ZOLIK_TEST_MONGO_URI"))
	if uri == "" {
		uri = "mongodb://127.0.0.1:27018"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		t.Skipf("could not build a mongo client: %v", err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		t.Skipf("no reachable mongo at %s (set ZOLIK_TEST_MONGO_URI): %v", uri, err)
	}
	m := &db.Mongo{Client: client, DB: client.Database(fmt.Sprintf("zolik_botsettings_%d", time.Now().UnixNano()))}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = m.DB.Drop(ctx)
		_ = client.Disconnect(ctx)
	})
	return botsettings.NewMongoStore(m)
}

func TestStoreRoundTrips(t *testing.T) {
	for name, open := range engines(t) {
		t.Run(name, func(t *testing.T) {
			ctx := t.Context()
			s := open(t)

			// Nothing stored: every game is off.
			got, err := s.HardModels(ctx)
			if err != nil {
				t.Fatalf("empty read: %v", err)
			}
			if len(got) != 0 {
				t.Fatalf("empty store has settings: %v", got)
			}

			at := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
			if err := s.SetHardModel(ctx, "zolik", botsettings.HardModel{Enabled: true, UpdatedBy: "operator", UpdatedAt: at}); err != nil {
				t.Fatalf("set zolik: %v", err)
			}
			if err := s.SetHardModel(ctx, "canasta", botsettings.HardModel{Enabled: true, UpdatedBy: "boss@example.com", UpdatedAt: at}); err != nil {
				t.Fatalf("set canasta: %v", err)
			}
			// Changing one game leaves the other alone.
			if err := s.SetHardModel(ctx, "zolik", botsettings.HardModel{Enabled: false, UpdatedBy: "operator", UpdatedAt: at.Add(time.Hour)}); err != nil {
				t.Fatalf("unset zolik: %v", err)
			}

			got, err = s.HardModels(ctx)
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			z, c := got["zolik"], got["canasta"]
			if z.Enabled || z.UpdatedBy != "operator" || !z.UpdatedAt.Equal(at.Add(time.Hour)) {
				t.Errorf("zolik = %+v", z)
			}
			if !c.Enabled || c.UpdatedBy != "boss@example.com" || !c.UpdatedAt.Equal(at) {
				t.Errorf("canasta = %+v", c)
			}
			if _, ok := got["holdem"]; ok {
				t.Error("a game never set has an entry")
			}

			if err := s.SetHardModel(ctx, "a.b", botsettings.HardModel{}); err == nil {
				t.Error("a dotted game name was accepted")
			}
		})
	}
}

// What a restart comes back to: a second store over the same KDB directory
// reads what the first wrote.
func TestKDBSettingSurvivesReopen(t *testing.T) {
	dir := t.TempDir()
	k, err := db.OpenKDB(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := botsettings.NewKDBStore(k).SetHardModel(t.Context(), "holdem", botsettings.HardModel{Enabled: true, UpdatedBy: "op"}); err != nil {
		t.Fatal(err)
	}
	if err := k.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	k, err = db.OpenKDB(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close(context.Background())
	got, err := botsettings.NewKDBStore(k).HardModels(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if !got["holdem"].Enabled {
		t.Errorf("after reopen: %+v", got)
	}
}
