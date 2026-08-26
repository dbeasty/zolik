package app

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"

	"zolik/server/internal/admin"
	"zolik/server/internal/auth"
	"zolik/server/internal/canasta"
	"zolik/server/internal/db"
	"zolik/server/internal/holdem"
	"zolik/server/internal/identity"
	"zolik/server/internal/lobby"
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
	// waitingRoom is the pool of human players waiting to be picked up into
	// a match — see internal/lobby. Rides the same Hub as match rooms do, so
	// it needs no database of its own and no separate scaling story.
	waitingRoom lobby.Store
	// statsRepo, userRepo and authStore are built once here rather than
	// re-constructed per route group — a repository handle is cheap to hold,
	// and building it twice was never anything but two names for the same
	// backend.
	statsRepo   stats.Repository
	userRepo    userrepo.Repository
	authStore   auth.Store
	sessionRepo auth.SessionRepository
}

func New(cfg Config) (*App, error) {
	// Covers Mongo connect + EnsureIndexes + the lobby waiting room's Redis
	// ping. 30s rather than a tighter figure gives real headroom for a cold
	// start — Mongo initializing an empty data volume for the first time, or
	// a container runtime under load — where the connection itself succeeds
	// within a couple of seconds but the very first ping/index-build calls
	// land before mongod has fully warmed up. docker-compose.yml's own
	// healthchecks (condition: service_healthy) are the primary defense
	// against starting this too early at all; this is the fallback for
	// running outside Compose (`go run ./cmd/server` against a Mongo that
	// happens to still be starting).
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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

	// Resolved once, before either consumer is built. The hub and the waiting
	// room are documented as sharing one Redis instance, so deciding this
	// separately in each would let them disagree — a hub gossiping across
	// instances while the waiting room quietly kept its pool to itself.
	redisURL := resolveRedisURL(ctx, cfg)

	registry := ws.NewConnRegistry()
	hub, err := ws.NewHub(registry, redisURL)
	if err != nil {
		_ = m.Close(ctx)
		return nil, err
	}

	// The waiting room shares the hub's Redis (or its local-only fallback)
	// rather than dialling a second connection — same instance, same
	// trade-off, same "fine for development without it" story.
	waitingRoom, err := lobby.NewStore(redisURL)
	if err != nil {
		_ = m.Close(ctx)
		return nil, err
	}

	statsRepo := stats.NewRepository(m)
	userRepo := userrepo.NewRepository(m)
	// Built once and shared: auth.Handlers and the user route group both need
	// it, and a repository handle has no reason to exist twice.
	authStore := auth.NewStore(m)
	sessionRepo := auth.NewSessionRepository(m)

	// Mail is resolved at startup rather than at first use: a deployment that
	// offers email sign-in but cannot send mail should fail to start, not fail
	// silently for the first player who tries it.
	mailer, err := auth.NewMailer(cfg.SMTP, cfg.Env == "" || cfg.Env == "local")
	if err != nil {
		_ = m.Close(ctx)
		return nil, err
	}

	authHandlers := auth.NewHandlers(auth.Deps{
		Store:     authStore,
		Sessions:  sessionRepo,
		Providers: identity.FromConfig(cfg.Identity),
		Mailer:    mailer,
		// The claimer is injected for the same reason the match recorder is:
		// stats imports auth for its middleware, so auth cannot import stats.
		Claimer:              stats.NewClaimer(statsRepo),
		PublicBaseURL:        cfg.PublicBaseURL,
		AllowedReturnURLs:    cfg.AllowedReturnURLs,
		AppName:              "Žolíky",
		TestEndpointsEnabled: cfg.TestEndpointsEnabled,
	})

	return &App{
		cfg:         cfg,
		db:          m,
		hub:         hub,
		auth:        authHandlers,
		waitingRoom: waitingRoom,
		statsRepo:   statsRepo,
		userRepo:    userRepo,
		authStore:   authStore,
		sessionRepo: sessionRepo,
	}, nil
}

// resolveRedisURL decides which Redis, if any, this process will use.
//
// An optional URL — one this process guessed rather than was told (see
// Config.RedisOptional) — is probed, and dropped if nothing answers, so that
// `go run ./cmd/server` works on a machine with no Redis while still picking
// one up automatically when it is there. A URL that was configured is returned
// untouched even if it is unreachable, leaving the consumers to fail startup
// on it: a deployment that asked for Redis and cannot have it is broken, and
// should say so rather than come up quietly as a single instance.
func resolveRedisURL(ctx context.Context, cfg Config) string {
	if cfg.RedisURL == "" || !cfg.RedisOptional {
		return cfg.RedisURL
	}
	if err := redisReachable(ctx, cfg.RedisURL); err != nil {
		log.Printf("redis at %s is unreachable (%v) — continuing local-only; set REDIS_URL to require it", cfg.RedisURL, err)
		return ""
	}
	return cfg.RedisURL
}

// redisReachable dials a Redis URL and lets it go again, so an optional
// default can be tested without either consumer having to be built first.
func redisReachable(ctx context.Context, url string) error {
	opt, err := redis.ParseURL(url)
	if err != nil {
		return err
	}
	client := redis.NewClient(opt)
	defer func() { _ = client.Close() }()
	return client.Ping(ctx).Err()
}

func (a *App) Config() Config { return a.cfg }

func (a *App) Hub() *ws.Hub { return a.hub }

func (a *App) Auth() *auth.Handlers { return a.auth }

func (a *App) Close(ctx context.Context) error {
	if a.hub != nil {
		_ = a.hub.Close()
		a.hub = nil
	}
	if a.waitingRoom != nil {
		_ = a.waitingRoom.Close()
		a.waitingRoom = nil
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
	// One runtime, hosting every game. The registry is the only place a game
	// is named: register a module and it appears in /modules, in the lobby's
	// picker, and on the one screen that plays all of them.
	modules := module.NewRegistry(zolikmod.New(), prsi.New(), canasta.New(), holdem.New())
	matchMgr := match.NewManager(match.NewRepository(a.db), modules, a.hub)
	// The recorder turns each completed match into a permanent record plus the
	// lifetime updates derived from it. Injected rather than constructed
	// inside the manager, so the runtime never has to import stats.
	matchMgr.SetRecorder(stats.NewRecorder(a.statsRepo))
	// And the waiting room, so a host can seat a specific player out of the
	// pool. Wired through a narrow interface rather than an import, so the
	// runtime does not learn what a waiting room is.
	matchMgr.SetWaitingRoom(a.waitingRoom, lobby.RoomID)

	lobbyHandlers := lobby.NewHandlers(a.hub, a.waitingRoom)

	return []routeGroup{
		{"health", func(r chi.Router) {
			r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("ok"))
			})
		}},
		{"auth", a.auth.RegisterRoutes},
		{"lobby", lobbyHandlers.RegisterRoutes},
		{"user", userrepo.NewHandlers(a.userRepo, a.authStore).RegisterRoutes},
		{"scoring", scoring.NewHandlers(a.db).RegisterRoutes},
		// The runtime, and now the only gameplay path there is. It replaced
		// the Žolíky-specific one rather than sitting beside it: /games, its
		// documents, its socket and its 24-field wire message are gone.
		{"match", func(r chi.Router) {
			match.NewHandlers(matchMgr, a.cfg.TestEndpointsEnabled).RegisterRoutes(r)
		}},
		{"stats", stats.NewHandlers(a.statsRepo).RegisterRoutes},
		// The operator's console and the API behind it. Registered
		// unconditionally: with no ADMIN_EMAILS configured the guard rejects
		// every request, which is a better failure than a route table that
		// changes shape depending on the environment.
		{"admin", admin.NewHandlers(admin.Deps{
			Guard:         admin.NewGuard(a.userRepo, a.cfg.AdminEmails),
			Users:         a.userRepo,
			Identities:    a.authStore,
			Sessions:      a.sessionRepo,
			Usage:         a.statsRepo,
			Live:          a.hub.Registry(),
			WaitingRoomID: lobby.RoomID,
		}).RegisterRoutes},
	}
}

func (a *App) RegisterRoutes(r chi.Router) {
	for _, g := range a.routeGroups() {
		g.register(r)
	}
}
