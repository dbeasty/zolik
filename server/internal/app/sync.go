package app

import (
	"context"
	"fmt"
	"log"
	"time"

	"zolik/server/internal/auth"
	"zolik/server/internal/db"
	"zolik/server/internal/match"
	zsync "zolik/server/internal/sync"

	"github.com/limidus/kdb/go/kdb/syncnode"
)

// Joining the distributed database is a deployment decision, not a code path
// every server takes: FEATURE_FLAG_SYNC says whether this process is the hub
// other nodes replicate with, a spoke that replicates with one, or neither.
//
// It needs the embedded engine. Replication is the engine's, not this
// application's: a Mongo deployment has no commit history to exchange, no
// namespace to grant and nothing to merge, and pretending otherwise would mean
// writing a second, worse database inside the server.

// openSyncNode builds this process's node, or returns nil when it is not one.
func openSyncNode(cfg Config, k *db.KDB, seating zsync.MatchSeating) (*zsync.Node, error) {
	if !cfg.Sync.Enabled() {
		return nil, nil
	}
	if cfg.DBEngine != db.EngineKDB {
		return nil, fmt.Errorf("FEATURE_FLAG_SYNC=%s needs FEATURE_FLAG_DB_ENGINE=kdb: replication is the embedded engine's, and Mongo has no history to replicate", cfg.Sync.Role)
	}
	if k == nil {
		return nil, fmt.Errorf("FEATURE_FLAG_SYNC=%s needs the embedded engine to be open", cfg.Sync.Role)
	}

	nodeCfg := zsync.Config{
		Role:       zsync.Role(cfg.Sync.Role),
		DataDir:    cfg.KDBPath,
		HubURL:     cfg.Sync.HubURL,
		Token:      cfg.Sync.Token,
		Advertise:  cfg.Sync.Advertise,
		Namespaces: cfg.Sync.Namespaces,
	}
	if nodeCfg.Role == zsync.RoleHub {
		// The hub holds a namespace per person and per match, which is far
		// more than it can keep open. They cost nothing closed and reopen on
		// their next use, so the only thing this tuning decides is how much
		// of a busy evening stays resident.
		nodeCfg.IdleClose = &syncnode.IdleConfig{
			MaxOpen:   2048,
			IdleAfter: 15 * time.Minute,
			MinIdle:   time.Minute,
			Interval:  time.Minute,
		}
	}
	return zsync.Open(k, nodeCfg, syncVerifier{}, seating)
}

// syncVerifier turns the bearer token a peer presents into who it is.
//
// The two kinds of peer are a person's own device, which presents the node
// credential it was given when its owner enrolled it, and another server of
// ours, which presents one issued for a server. What the token says is what
// the peer may sync, and internal/sync decides the rest from the namespace's
// name alone.
type syncVerifier struct{}

func (syncVerifier) VerifySyncToken(_ context.Context, token string) (zsync.Identity, error) {
	claims, err := auth.VerifyNodeCredential(token, auth.LocalKeys())
	if err != nil {
		return zsync.Identity{}, err
	}
	return zsync.Identity{
		NodeID:   claims.NodeID(),
		OwnerHex: claims.Owner,
		// A self-hosted server holds whoever is playing on it, which is not a
		// set anything here can name in advance, so it is trusted with the
		// namespaces of the people it serves. A phone is trusted with one
		// person's: its owner's.
		Server: claims.Kind == auth.NodeKindLAN,
	}, nil
}

// matchSeating answers the question every match namespace's authorization
// comes down to: is this person sitting at this table.
type matchSeating struct{ mgr *match.Manager }

func (s matchSeating) Seated(ctx context.Context, matchHex, userHex string) (bool, error) {
	return s.mgr.Seated(ctx, matchHex, userHex)
}

// startSync begins replicating, once everything the node needs to answer
// questions about matches exists.
func (a *App) startSync() {
	if a.sync == nil {
		return
	}
	a.sync.SetMatchHome(a.matchManager())
	if err := a.sync.Start(); err != nil {
		log.Printf("sync: this node is not replicating: %v", err)
		return
	}
	log.Printf("sync: node %s is a %s of the distributed database", a.sync.NodeID(), a.cfg.Sync.Role)
}
