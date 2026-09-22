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

	a, err := New(MobileConfig(dataDir))
	if err != nil {
		return nil, nil, err
	}
	a.Auth().SetOfflineKeys(keys)
	return a, keys, nil
}
