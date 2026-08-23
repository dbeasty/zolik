package app

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"zolik/server/internal/auth"
	"zolik/server/internal/db"
	"zolik/server/internal/game"
	"zolik/server/internal/scoring"
	"zolik/server/internal/stats"
	userrepo "zolik/server/internal/user"
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

func (a *App) RegisterRoutes(r chi.Router) {
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	ws := game.NewWebSocketServer(a.manager)
	ws.RegisterRoutes(r, a.db)

	a.auth.RegisterRoutes(r)

	statsRepo := stats.NewRepository(a.db)

	repo := game.NewRepository(a.db)
	gameRest := game.NewGameRestHandlers(repo, a.hub, a.manager, statsRepo, a.cfg.TestEndpointsEnabled)
	gameRest.RegisterRoutes(r)

	userHandlers := userrepo.NewHandlers(userrepo.NewRepository(a.db))
	userHandlers.RegisterRoutes(r)

	scoringHandlers := scoring.NewHandlers(a.db)
	scoringHandlers.RegisterRoutes(r)

	statsHandlers := stats.NewHandlers(statsRepo)
	statsHandlers.RegisterRoutes(r)
}
