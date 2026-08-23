package app

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"zolik/server/internal/auth"
	"zolik/server/internal/db"
	"zolik/server/internal/game"
	"zolik/server/internal/match"
	"zolik/server/internal/module"
	"zolik/server/internal/prsi"
	"zolik/server/internal/scoring"
	"zolik/server/internal/stats"
	userrepo "zolik/server/internal/user"
	"zolik/server/internal/zolikmod"
)

type App struct {
	cfg     Config
	db      *db.Mongo
	hub     *game.Hub
	manager *game.Manager
	auth    *auth.Handlers
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

	registry := game.NewConnRegistry()
	hub, err := game.NewHub(registry, cfg.RedisURL)
	if err != nil {
		_ = m.Close(ctx)
		return nil, err
	}

	repo := game.NewRepository(m)
	manager := game.NewManager(repo, hub)
	// The recorder turns each completed match into a permanent record plus
	// the lifetime updates derived from it. Injected rather than constructed
	// inside the manager so the game package never has to import stats — see
	// game.MatchRecorder.
	statsRepo := stats.NewRepository(m)
	manager.SetMatchRecorder(stats.NewRecorder(statsRepo))
	authHandlers := auth.NewHandlers(m)

	return &App{
		cfg:     cfg,
		db:      m,
		hub:     hub,
		manager: manager,
		auth:    authHandlers,
	}, nil
}

func (a *App) Config() Config { return a.cfg }

func (a *App) Hub() *game.Hub { return a.hub }

func (a *App) Manager() *game.Manager { return a.manager }

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
	gameRest := game.NewGameRestHandlers(
		game.NewRepository(a.db), a.hub, a.manager, statsRepo, a.cfg.TestEndpointsEnabled,
	)

	return []routeGroup{
		{"health", func(r chi.Router) {
			r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("ok"))
			})
		}},
		{"ws", func(r chi.Router) {
			game.NewWebSocketServer(a.manager).RegisterRoutes(r, a.db)
		}},
		{"auth", a.auth.RegisterRoutes},
		{"game", gameRest.RegisterRoutes},
		{"user", userrepo.NewHandlers(userrepo.NewRepository(a.db)).RegisterRoutes},
		{"scoring", scoring.NewHandlers(a.db).RegisterRoutes},
		// The module runtime, mounted alongside the Žolíky path rather than
		// replacing it. Every phase of this migration ships the new shape next
		// to the old and retires the old only once nothing reads it — the
		// existing game routes, documents and clients are untouched by this.
		{"match", func(r chi.Router) {
			modules := module.NewRegistry(zolikmod.New(), prsi.New())
			matchMgr := match.NewManager(match.NewRepository(a.db), modules, a.hub)
			match.NewHandlers(matchMgr).RegisterRoutes(r)
		}},
		{"stats", stats.NewHandlers(statsRepo).RegisterRoutes},
	}
}

func (a *App) RegisterRoutes(r chi.Router) {
	for _, g := range a.routeGroups() {
		g.register(r)
	}
}
