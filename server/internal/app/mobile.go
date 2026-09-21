package app

import (
	"errors"
	"net/http"
	"path/filepath"
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

// NewMobile builds the embedded host's App. Before anything can serve, it
// fixes the signing secrets and turns off the dev-token shortcut. The secrets
// are per install and the caller keeps them, so a player's seat survives the
// app restarting. A host with no secrets refuses to start rather than sign
// with the well-known development ones.
func NewMobile(dataDir, accessSecret, refreshSecret string) (*App, error) {
	if len(accessSecret) < 32 || len(refreshSecret) < 32 {
		return nil, errors.New("mobile host needs per-install signing secrets of at least 32 bytes")
	}
	auth.SetSecrets(accessSecret, refreshSecret)
	auth.DisableDevTokens()
	return New(MobileConfig(dataDir))
}
