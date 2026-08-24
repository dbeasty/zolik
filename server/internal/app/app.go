package app

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"zolik/server/internal/auth"
	"zolik/server/internal/canasta"
	"zolik/server/internal/db"
	"zolik/server/internal/holdem"
	"zolik/server/internal/match"
	"zolik/server/internal/module"
	"zolik/server/internal/prsi"
	"zolik/server/internal/scoring"
	"zolik/server/internal/stats"
	userrepo "zolik/server/internal/user"
	"zolik/server/internal/ws"
	"zolik/server/internal/zolikmod"
)

type App struct {
	cfg  Config
	db   *db.Mongo
	hub  *ws.Hub
	auth *auth.Handlers
}

func New(cfg Config) (*App, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	m, err := db.Connect(ctx, db.Config{
		URI: cfg.MongoURI,
		DB:  cfg.MongoDB,
	})
	if err != nil {
		return nil, err
	}
	if err := m.EnsureIndexes(ctx); err != nil {
		_ = m.Close(ctx)
		return nil, err
	}

	registry := ws.NewConnRegistry()
	hub, err := ws.NewHub(registry, cfg.RedisURL)
	if err != nil {
		_ = m.Close(ctx)
		return nil, err
	}

	authHandlers := auth.NewHandlers(m)

	return &App{
		cfg:  cfg,
		db:   m,
		hub:  hub,
		auth: authHandlers,
	}, nil
}

func (a *App) Config() Config { return a.cfg }

func (a *App) Hub() *ws.Hub { return a.hub }

func (a *App) Auth() *auth.Handlers { return a.auth }

func (a *App) Close(ctx context.Context) error {
	if a.hub != nil {
		_ = a.hub.Close()
		a.hub = nil
	}
	if a.db != nil {
		return a.db.Close(ctx)
	}
	return nil
}

// routeGroup is one package's worth of routes, named so a failure can say
// which one lost.
type routeGroup struct {
	name     string
	register func(chi.Router)
}

// routeGroups is every group of routes this server exposes.
//
// A list rather than a run of inline calls so a test can mount each group on a
// router of its own and check it still appears in the combined table. chi keys
// a path segment on its position, not on the placeholder's name, so
// /matches/{id} and /matches/{gameId} are one route to it: registering both
// leaves only whichever came last, with no panic and no warning. That is
// exactly what happened between the module runtime and the stats handlers, and
// nothing but the browser suite noticed.
func (a *App) routeGroups() []routeGroup {
	statsRepo := stats.NewRepository(a.db)

	// One runtime, hosting every game. The registry is the only place a game
	// is named: register a module and it appears in /modules, in the lobby's
	// picker, and on the one screen that plays all of them.
	modules := module.NewRegistry(zolikmod.New(), prsi.New(), canasta.New(), holdem.New())
	matchMgr := match.NewManager(match.NewRepository(a.db), modules, a.hub)
	// The recorder turns each completed match into a permanent record plus the
	// lifetime updates derived from it. Injected rather than constructed
	// inside the manager, so the runtime never has to import stats.
	matchMgr.SetRecorder(stats.NewRecorder(statsRepo))

	return []routeGroup{
		{"health", func(r chi.Router) {
			r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("ok"))
			})
		}},
		{"auth", a.auth.RegisterRoutes},
		{"user", userrepo.NewHandlers(userrepo.NewRepository(a.db)).RegisterRoutes},
		{"scoring", scoring.NewHandlers(a.db).RegisterRoutes},
		// The runtime, and now the only gameplay path there is. It replaced
		// the Žolíky-specific one rather than sitting beside it: /games, its
		// documents, its socket and its 24-field wire message are gone.
		{"match", func(r chi.Router) {
			match.NewHandlers(matchMgr, a.cfg.TestEndpointsEnabled).RegisterRoutes(r)
		}},
		{"stats", stats.NewHandlers(statsRepo).RegisterRoutes},
	}
}

func (a *App) RegisterRoutes(r chi.Router) {
	for _, g := range a.routeGroups() {
		g.register(r)
	}
}
