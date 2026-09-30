package app

import (
	"context"
	"fmt"
	"log"
	"net/http"
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
//
// It reaches the runtime through the app rather than holding it, because of an
// order this cannot get out of otherwise: the sync node has to exist before
// the match runtime is built, since the runtime's recorder depends on what
// kind of node this is, and the node needs somebody to ask about matches. The
// question is only ever asked by a peer that has already connected, which is
// long after both exist.
type matchSeating struct{ app *App }

func (s matchSeating) Seated(ctx context.Context, matchHex, userHex string) (bool, error) {
	return s.app.matchManager().Seated(ctx, matchHex, userHex)
}

// attendSigner is the middleware a self-hosted node runs: whoever is signed in
// and using it is somebody whose data it should be holding.
//
// It is only mounted on a spoke. The cloud holds everyone by definition, and a
// phone holds its owner, so neither has a question to answer here.
func (a *App) attendSigner(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if uc, ok := auth.GetUserContext(req); ok && !uc.IsGuest {
			a.sync.Attend(uc.UserID)
		}
		next.ServeHTTP(w, req)
	})
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
	if a.importer != nil {
		a.importer.Start()
	}
	if a.cfg.Sync.Role == syncRoleSpoke {
		go a.forgetIdleVisitors()
	}
	log.Printf("sync: node %s is a %s of the distributed database", a.sync.NodeID(), a.cfg.Sync.Role)
}

// forgetIdleVisitors stops a self-hosted node from accumulating everybody who
// has ever signed in on it.
func (a *App) forgetIdleVisitors() {
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	for range t.C {
		if a.sync == nil {
			return
		}
		if n := a.sync.ForgetIdle(time.Now()); n > 0 {
			log.Printf("sync: stopped following %d people who have not played here for a day", n)
		}
	}
}

// outbox is where a node that is not the cloud puts a finished match. Nil on
// the hub, which has nobody to hand anything to.
//
// It is named after this install's node key rather than after the database's
// own node id, because the node key is what the cloud enrolled and what the
// bundles are signed with: the two ids would otherwise have to be kept in step
// for no gain, and a mismatch means a phone pushing an empty namespace and
// wondering where its matches went.
func (a *App) outbox() *zsync.Outbox {
	if a.sync == nil || a.cfg.Sync.Role != syncRoleSpoke {
		return nil
	}
	node := auth.NodeID()
	if node == "" {
		return nil
	}
	return zsync.NewOutbox(a.kdb, node, auth.NodeSigner{})
}

// configureOfflineFlow decides what happens to a match when it finishes, which
// is the one thing that differs most between the kinds of node.
//
// On a spoke the match is handed up rather than recorded: a phone has no
// leaderboard and no lifetime figures of its own, and keeping its own would be
// keeping a second set of numbers that quietly disagreed with the ones the
// player sees everywhere else.
//
// On the hub, matches handed up by other nodes are replayed, checked and
// recorded exactly as a match played here would be.
func (a *App) configureOfflineFlow(matchMgr *match.Manager, statsRecorder match.Recorder) match.Recorder {
	if a.sync == nil {
		return statsRecorder
	}
	if out := a.outbox(); out != nil {
		return match.NewHandUp(a.matchRepo, out)
	}
	if a.cfg.Sync.Role != syncRoleHub {
		return statsRecorder
	}
	claims := auth.NewKDBGuestClaims(a.kdb)
	importer := match.NewImporter(a.matchRepo, matchMgr.Registry(), statsRecorder, claims)
	a.importer = zsync.NewImporter(a.kdb, auth.BundleVerifier{Nodes: a.nodes}, importer, time.Minute)
	return statsRecorder
}
