package app

import (
	"strings"
	"testing"

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
