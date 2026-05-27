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
	userrepo "zolik/server/internal/user"
)

type App struct {
	cfg Config
	db  *db.Mongo
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

	return &App{cfg: cfg, db: m}, nil
}

func (a *App) Close(ctx context.Context) error {
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

	registry := game.NewConnRegistry()
	repo := game.NewRepository(a.db)
	manager := game.NewManager(repo, registry)
	ws := game.NewWebSocketServer(manager)
	ws.RegisterRoutes(r, a.db)

	authHandlers := auth.NewHandlers(a.db)
	authHandlers.RegisterRoutes(r)

	gameRest := game.NewGameRestHandlers(repo, registry, manager)
	gameRest.RegisterRoutes(r)

	userHandlers := userrepo.NewHandlers(userrepo.NewRepository(a.db))
	userHandlers.RegisterRoutes(r)

	scoringHandlers := scoring.NewHandlers(a.db)
	scoringHandlers.RegisterRoutes(r)
}

