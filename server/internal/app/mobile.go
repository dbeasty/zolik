package app

import (
	"errors"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"zolik/server/internal/auth"
	"zolik/server/internal/db"
	"zolik/server/internal/match"
	"zolik/server/internal/replica"
	zsync "zolik/server/internal/sync"
)

// EnvMobile is the Env of a server embedded in the iOS or Android app, where
// a phone hosts a table for the players around it with no internet at all.
const EnvMobile = "mobile"

// IsMobile reports whether this is the copy embedded in the phone app.
func (c Config) IsMobile() bool { return c.Env == EnvMobile }

// MobileConfig is the whole configuration of an embedded host. It is built
// here rather than read from the environment, because the app has no
// environment that anybody sets.
//
// It uses the in-process KDB engine in dataDir, so a table outlives the app
// being killed, with no Redis, no mail and no identity providers. There is no
// public URL, because nobody reaches this host from outside the room it is
// in. It is not local either: debug and test endpoints stay off, because
// other phones on the network can reach the listener.
func MobileConfig(dataDir string) Config {
	return Config{
		Env:      EnvMobile,
		LogLevel: "info",
		DBEngine: db.EngineKDB,
		KDBPath:  filepath.Join(dataDir, "kdb"),

		// A phone keeps every version anyway, and stepping back through a
		// hand is as welcome at a kitchen table as online.
		ReplayEnabled: true,

		Retention: match.RetentionWindows{
			Lobby:     24 * time.Hour,
			Completed: 30 * 24 * time.Hour,
			Abandoned: 30 * 24 * time.Hour,
		},

		// Admission gates disarm themselves where there is no cgroup to
		// read, which is every phone. A handful of players around one table
		// never needs them.
		AdmissionWaitingRoomRatio: 0.8,

		BotThinkMinMS: 900,
		BotThinkMaxMS: 1800,
	}
}

// RegisterMobileRoutes mounts what an offline table needs: health and version,
// a guest seat, and the match runtime with its socket. It leaves out accounts,
// stats, the waiting room, scoring, the operator's endpoints and the web
// bundle. They mean nothing without the internet, and a phone's listener can
// be reached from the local network, so everything left out is surface that
// does not exist.
func (a *App) RegisterMobileRoutes(r chi.Router) {
	for _, g := range a.routeGroups() {
		switch g.name {
		case "health", "match":
			g.register(r)
		}
	}
	a.auth.RegisterLocalRoutes(r)
	r.NotFound(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
	})
}

// DefaultCloudBaseURL is where an embedded host expects to find the cloud.
// It is the canonical production domain rather than one of the mirrors,
// because it is what the signing keys are published under and what an offline
// pass names as its issuer.
const DefaultCloudBaseURL = "https://jokerless.com"

// MobileIdentity is what the app hands the embedded server about itself. It
// is built by zolikcore from host.json, which is the only place an install's
// own secrets live.
type MobileIdentity struct {
	// AccessSecret is the per-install HS256 key. It stays because the tokens
	// handed to guests by earlier builds were signed with it, and they must
	// go on working until they expire.
	AccessSecret string
	// NodeKeySeed is the Ed25519 seed this node signs with: the guest tokens
	// it issues and the seat receipts it hands out. It is what makes a seat
	// taken at this table something another node can later check.
	NodeKeySeed string
	// CloudBaseURL overrides where the cloud's keys are fetched from, for a
	// development build pointed at a local server.
	CloudBaseURL string
	// BundledJWKS is a copy of the cloud's public keys compiled into the app,
	// used until this install has fetched its own. Without one, a phone that
	// has never been online since it was installed cannot verify an offline
	// pass, and will seat guests only.
	//
	// TODO: fill this at build time from the production JWKS, so the very
	// first offline session on a fresh install can seat a real account. It is
	// left empty here rather than pinned to a key checked into the repository:
	// a bundled key that nobody rotates is worse than none.
	BundledJWKS []byte
	// NodeCredential is what this install was given when its owner enrolled
	// it, and is how it authenticates its database sync. Empty means the
	// install has never been enrolled, and the host then plays offline tables
	// without replicating anything.
	NodeCredential string
	// UserHex is the account signed in on this device, whose namespaces this
	// node syncs. Empty alongside a credential means nobody is signed in, and
	// there is nothing to ask the hub for.
	UserHex string
}

// SyncsWithCloud reports whether this install is enrolled and signed in, which
// is what replicating needs.
func (id MobileIdentity) SyncsWithCloud() bool {
	return strings.TrimSpace(id.NodeCredential) != "" && strings.TrimSpace(id.UserHex) != ""
}

// NewMobile builds the embedded host's App, and the cache of cloud keys it
// verifies offline passes against.
//
// Before anything can serve, it fixes both of the install's keys. The access
// secret is per install and the caller keeps it, so a player's seat survives
// the app restarting. The node key is what this host signs as itself with,
// and takes over the signing of new tokens: a host with neither refuses to
// start rather than sign with the well-known development key.
func NewMobile(dataDir string, id MobileIdentity) (*App, *auth.JWKSCache, error) {
	if len(id.AccessSecret) < 32 {
		return nil, nil, errors.New("mobile host needs a per-install signing secret of at least 32 bytes")
	}
	if strings.TrimSpace(id.NodeKeySeed) == "" {
		return nil, nil, errors.New("mobile host needs a per-install node key")
	}
	auth.SetAccessSecret(id.AccessSecret)
	if err := auth.SetNodeKey(id.NodeKeySeed); err != nil {
		return nil, nil, err
	}

	base := strings.TrimRight(strings.TrimSpace(id.CloudBaseURL), "/")
	if base == "" {
		base = DefaultCloudBaseURL
	}
	// Beside host.json and the database, because it is part of what this
	// install owns: a player who clears the app's data starts again from the
	// bundled copy rather than keeping keys nothing else agrees with.
	keys := auth.NewJWKSCache(auth.JWKSCacheConfig{
		URL:     base + "/.well-known/jwks.json",
		Path:    filepath.Join(dataDir, "jwks.json"),
		Bundled: id.BundledJWKS,
	})

	cfg := MobileConfig(dataDir)
	if id.SyncsWithCloud() {
		// The phone dials the cloud; the cloud never dials a phone, which has
		// no address anybody could dial. It asks for its own account's two
		// namespaces and for its own outbox, and picks up a match's namespace
		// when its player opens one.
		namespaces := []string{
			db.UserNS(id.UserHex),
			db.UserReadOnlyNS(id.UserHex),
		}
		// The outbox is where a finished offline match waits, and a phone
		// that does not replicate it hands nothing up: the bundle is written,
		// the match looks recorded, and it never leaves the device. It is
		// named after the node key rather than the database's node id,
		// exactly as App.outbox names it.
		if node := auth.NodeID(); node != "" {
			namespaces = append(namespaces, db.NodeOutboxNS(node))
		}
		cfg.Sync = SyncConfig{
			Role:       syncRoleSpoke,
			HubURL:     cloudSyncURL(base),
			Token:      strings.TrimSpace(id.NodeCredential),
			Namespaces: namespaces,
		}
	}
	a, err := New(cfg)
	if err != nil {
		return nil, nil, err
	}
	a.Auth().SetOfflineKeys(keys)
	a.replicaUser = strings.TrimSpace(id.UserHex)
	return a, keys, nil
}

// cloudSyncURL turns the cloud's base URL into its peer-sync endpoint. The
// scheme has to change with it: the sync is a WebSocket upgrade on the same
// origin, port and certificate as every other call the app makes.
func cloudSyncURL(base string) string {
	switch {
	case strings.HasPrefix(base, "https://"):
		return "wss://" + strings.TrimPrefix(base, "https://") + "/kdb/sync"
	case strings.HasPrefix(base, "http://"):
		return "ws://" + strings.TrimPrefix(base, "http://") + "/kdb/sync"
	default:
		return base + "/kdb/sync"
	}
}

// RegisterNodeRoutes mounts what a phone that has synced can answer for
// itself: everything RegisterMobileRoutes serves, plus the account's own data
// out of the copy this device holds.
//
// The two are separate because they are different situations rather than
// different configurations of one. A phone hosting a table in a room with no
// internet is serving the people around it; a phone serving its own player
// their history on a train is serving one person, from data nobody else at
// any table may see. What stays out of both is anything about other people -
// the waiting room, the leaderboard, looking somebody up by friend code -
// because this device does not hold the answer and should not pretend to.
func (a *App) RegisterNodeRoutes(r chi.Router) {
	for _, g := range a.routeGroups() {
		switch g.name {
		case "health", "match":
			g.register(r)
		}
	}
	a.auth.RegisterLocalRoutes(r)
	// The person's own data, read from this device's copy and written back to
	// it: a setting changed on a train takes now and reaches the cloud when
	// there is a connection, rather than being refused because there is not.
	local := replica.NewHandlers(a.Replica)
	local.SetWriter(a.ReplicaWriter)
	local.RegisterRoutes(r)
	r.NotFound(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
	})
}

// Replica is this device's copy of the signed-in account's data.
func (a *App) Replica() *replica.Reader {
	return replica.NewReader(a.kdb, a.replicaUser)
}

// ReplicaWriter applies the person's own changes to that copy.
func (a *App) ReplicaWriter() *replica.Writer {
	return replica.NewWriter(a.kdb, a.replicaUser)
}

// Sync is this process as a node of the distributed database, for the app to
// ask for a sync when it comes back to the foreground or the network returns.
// Nil when this install replicates with nobody.
func (a *App) Sync() *zsync.Node { return a.sync }
