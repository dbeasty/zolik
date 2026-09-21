package match

import (
	"context"
	"errors"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/models"
	"zolik/server/internal/module"
	"zolik/server/internal/prsi"
)

// Whether this deployment can offer a replay at all.
//
// Three separate questions, and the reason they are pinned together is that
// getting any one of them wrong produces the same visible fault: a "Replay"
// button on a list that the endpoint behind it then refuses. So every case
// here asserts both answers — ReplayAvailable, and that nothing quietly
// disagrees with it.

// plainStore is a repository that replaces documents in place, which is every
// Mongo deployment: it can say what a match is now and nothing about what it
// was. The embedded Repository supplies the rest of the interface and is nil,
// which is exactly right — calling any of it here would be the bug.
type plainStore struct{ Repository }

// historyStore keeps versions, and says whether it is still keeping them.
type historyStore struct {
	Repository
	retains error
}

func (h historyStore) BoardAfter(context.Context, bson.ObjectID, int) (models.Match, error) {
	return models.Match{}, errors.New("not called")
}
func (h historyStore) RetainsHistory(context.Context) error { return h.retains }

func managerWith(repo Repository, on bool) *Manager {
	m := NewManager(repo, module.NewRegistry(prsi.New()), nil)
	m.SetReplayEnabled(on)
	return m
}

func TestReplayNeedsTheFlagTheEngineAndTheHistory(t *testing.T) {
	reclaimed := errors.New(
		`kdb: namespace "zolik/matches" runs with history=none, which reclaims commits ` +
			`on a retention window, so stepping back through a game that has stopped is not ` +
			`available: Convert the namespace with kdb-inspect migrate-history`)

	cases := []struct {
		name string
		mgr  *Manager
		want string // a fragment of the refusal, or "" for available
	}{
		{
			// The operator's switch, and it is first because it is the one
			// that costs nothing to check — a deployment with replay off
			// never touches the engine to find that out.
			name: "the flag is off, on an engine that could",
			mgr:  managerWith(historyStore{}, false),
			want: "FEATURE_FLAG_MATCH_REPLAY",
		},
		{
			// The flag alone is not enough, and this is the case the flag
			// would otherwise turn into a broken screen: Mongo replaces the
			// document in place, so there is no earlier board to show.
			name: "the flag is on, but the store keeps no versions",
			mgr:  managerWith(plainStore{}, true),
			want: "FEATURE_FLAG_DB_ENGINE=kdb",
		},
		{
			// And keeping versions is not the same as still having them. This
			// is the gate that exists because the code path *works* under
			// history=none, right up until the window sweeps it.
			name: "the store keeps versions but reclaims them",
			mgr:  managerWith(historyStore{retains: reclaimed}, true),
			want: "migrate-history",
		},
		{
			name: "the flag is on and the store keeps its history",
			mgr:  managerWith(historyStore{}, true),
			want: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.mgr.ReplayAvailable(context.Background())
			if tc.want == "" {
				if err != nil {
					t.Fatalf("ReplayAvailable = %v, want available", err)
				}
				return
			}
			if err == nil {
				t.Fatal("ReplayAvailable said yes")
			}
			if code := module.CodeOf(err); code != "REPLAY_UNAVAILABLE" {
				t.Errorf("code = %q, want REPLAY_UNAVAILABLE", code)
			}
			// The message is for whoever has to fix it, so it has to name the
			// thing they would change. A bare code names nothing.
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("refusal %q does not mention %q", err.Error(), tc.want)
			}
		})
	}
}

// A deployment that cannot replay does not pay for replay.
//
// Checkpoints are the whole of what the feature costs the write path, and
// checkpointFor is the function that path calls — so the bytes follow the
// flag rather than piling up against a feature nobody in this deployment can
// reach. Asserted on both sides, because "never checkpoints" would pass this
// just as well as the guard does.
func TestCheckpointsFollowTheSameGateAsTheEndpoint(t *testing.T) {
	ctx := context.Background()
	// A round has closed, twelve moves in: the input every case here shares.
	played := models.Match{ActionCount: 11}
	rounds := &module.RoundLog{Rounds: []module.RoundResult{{}}}

	if _, _, ok := managerWith(historyStore{}, true).checkpointFor(ctx, played, 12, rounds); !ok {
		t.Fatal("a closed round earned no checkpoint where replay is available")
	}
	for _, tc := range []struct {
		name string
		mgr  *Manager
	}{
		{"flag off", managerWith(historyStore{}, false)},
		{"no history in the store", managerWith(plainStore{}, true)},
		{"history reclaimed", managerWith(historyStore{retains: errors.New("history=none")}, true)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, ok := tc.mgr.checkpointFor(ctx, played, 12, rounds); ok {
				t.Error("stored a checkpoint for a replay this deployment will not serve")
			}
		})
	}
}
