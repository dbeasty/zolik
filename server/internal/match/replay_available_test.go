package match

import (
	"context"
	"testing"

	"zolik/server/internal/module"
)

// Whether this deployment offers replay at all.
//
// Only the flag decides now. Replay used to lean on kdb's own document
// history, so it asked the store too; it folds the stored moves from a stored
// snapshot instead, which every engine keeps, so a Mongo deployment replays
// exactly as a kdb one does.

// plainStore is a repository with nothing but the interface: the embedded
// Repository is nil, so if the gate ever asked the store again this would
// panic rather than pass.
type plainStore struct{ Repository }

func managerWith(repo Repository, on bool) *Manager {
	m := &Manager{repo: repo, registry: module.NewRegistry()}
	m.SetReplayEnabled(on)
	return m
}

func TestReplayNeedsOnlyTheFlag(t *testing.T) {
	ctx := context.Background()
	if err := managerWith(plainStore{}, true).ReplayAvailable(ctx); err != nil {
		t.Errorf("flag on: %v, want replay available", err)
	}
	err := managerWith(plainStore{}, false).ReplayAvailable(ctx)
	if module.CodeOf(err) != "REPLAY_UNAVAILABLE" {
		t.Errorf("flag off: %v, want REPLAY_UNAVAILABLE", err)
	}
}
