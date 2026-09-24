package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"zolik/server/internal/db"
)

func TestSyncNeedsTheEmbeddedEngine(t *testing.T) {
	// Replication is the engine's. A Mongo deployment has no commit history
	// to exchange, so asking for it is a configuration mistake worth refusing
	// at startup rather than a silent half-join.
	cfg := Config{DBEngine: db.EngineMongo}
	cfg.Sync.Role = syncRoleHub
	_, err := openSyncNode(cfg, nil, nil)
	if err == nil {
		t.Fatal("a Mongo deployment was allowed to join the distributed database")
	}
	if !strings.Contains(err.Error(), "kdb") {
		t.Fatalf("refusal = %v, want it to name the engine it needs", err)
	}
}

func TestSyncIsOffUnlessAskedFor(t *testing.T) {
	cfg := Config{DBEngine: db.EngineKDB}
	cfg.Sync.Role = syncRoleOff
	node, err := openSyncNode(cfg, nil, nil)
	if err != nil {
		t.Fatalf("a server that replicates with nobody must still start: %v", err)
	}
	if node != nil {
		t.Fatal("a node was built for a server that was not asked to be one")
	}
}

func TestAnUnrecognisedSyncRoleIsOff(t *testing.T) {
	// A misspelled flag keeps the deployment exactly as it was.
	t.Setenv("FEATURE_FLAG_SYNC", "hubb")
	if got := syncConfig().Role; got != syncRoleOff {
		t.Fatalf("role = %q, want %q", got, syncRoleOff)
	}
	t.Setenv("FEATURE_FLAG_SYNC", "SPOKE")
	if got := syncConfig().Role; got != syncRoleSpoke {
		t.Fatalf("role = %q, want %q", got, syncRoleSpoke)
	}
}

func TestOnlyTheCloudMintsAccounts(t *testing.T) {
	// A self-hosted node verifies the tokens the cloud signed and refuses to
	// issue its own. A username has to name one person, and a node deciding
	// that for itself is a node that will eventually disagree with another.
	for _, tc := range []struct {
		role string
		want int
	}{
		{syncRoleOff, http.StatusBadRequest},
		{syncRoleHub, http.StatusBadRequest},
		{syncRoleSpoke, http.StatusConflict},
	} {
		t.Run(tc.role, func(t *testing.T) {
			a := newNode(t, Config{Sync: SyncConfig{Role: tc.role, HubURL: "ws://localhost:1/kdb/sync", Token: "t"}})
			r := chi.NewRouter()
			a.RegisterRoutes(r)
			server := httptest.NewServer(r)
			t.Cleanup(server.Close)

			// An empty body: a node that is allowed to create accounts gets
			// as far as refusing the request itself, and one that is not says
			// so before looking at it.
			res, err := http.Post(server.URL+"/auth/register", "application/json", strings.NewReader(`{}`))
			if err != nil {
				t.Fatalf("register: %v", err)
			}
			defer res.Body.Close()
			if res.StatusCode != tc.want {
				t.Fatalf("register on a %s = %d, want %d", tc.role, res.StatusCode, tc.want)
			}
		})
	}
}
