package app

import (
	"cmp"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"runtime/debug"
	"time"

	"github.com/go-chi/chi/v5"

	"zolik/server/internal/admission"
	"zolik/server/internal/auth"
	"zolik/server/internal/buildinfo"
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
	cfg Config
	// closeDB releases whichever storage backend New opened — a Mongo client,
	// or the embedded KDB engine. Repositories below are the only other
	// handles anything holds on it.
	closeDB func(ctx context.Context) error
	hub     *ws.Hub
	auth    *auth.Handlers
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
	matchRepo   match.Repository
	scoringRepo scoring.Repository
	// admission is the one capacity gate every route group shares — built
	// here rather than in routeGroups, which runs once per router
	// registration and would otherwise hand each router an independent
	// counter.
	admission *admission.Controller
}

// repos is every repository the app wires, built in one place so the two
// storage engines stay column-for-column comparable — a repository added to
// one and forgotten in the other is a compile error here, not a nil panic in
// a handler.
type repos struct {
	stats    stats.Repository
	user     userrepo.Repository
	store    auth.Store
	sessions auth.SessionRepository
	match    match.Repository
	scoring  scoring.Repository
	close    func(ctx context.Context) error
}

// mongoRepos connects to MongoDB and builds the Mongo-backed repositories —
// the engine every deployment ran on before the KDB feature flag existed.
func mongoRepos(ctx context.Context, cfg Config) (repos, error) {
	m, err := db.Connect(ctx, db.Config{
		URI: cfg.MongoURI,
		DB:  cfg.MongoDB,
	})
	if err != nil {
		return repos{}, err
	}
	if err := m.EnsureIndexes(ctx); err != nil {
		_ = m.Close(ctx)
		return repos{}, err
	}
	return repos{
		stats:    stats.NewRepository(m),
		user:     userrepo.NewRepository(m),
		store:    auth.NewStore(m),
		sessions: auth.NewSessionRepository(m),
		match:    match.NewRepository(m),
		scoring:  scoring.NewRepository(m),
		close:    m.Close,
	}, nil
}

// kdbRepos opens the embedded KDB engine and builds its repositories. No
// server process, no connection string: the database is a directory, and an
// empty path keeps it all in memory.
func kdbRepos(cfg Config) (repos, error) {
	sc, err := db.KDBStorageFromEnv()
	if err != nil {
		return repos{}, err
	}
	// What an acknowledged write means is the one storage decision an
	// operator makes (KDB_DURABILITY / KDB_SYNC_MODE), so say it out loud.
	log.Printf("kdb durability: %s, sync mode: %s", cmp.Or(sc.Durability, "sync"), cmp.Or(sc.SyncMode, "fast"))
	k, err := db.OpenKDBWithStorage(cfg.KDBPath, sc)
	if err != nil {
		return repos{}, err
	}
	return repos{
		stats:    stats.NewKDBRepository(k),
		user:     userrepo.NewKDBRepository(k),
		store:    auth.NewKDBStore(k),
		sessions: auth.NewKDBSessionRepository(k),
		match:    match.NewKDBRepository(k),
		scoring:  scoring.NewKDBRepository(k),
		close:    k.Close,
	}, nil
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

	var r repos
	var err error
	if cfg.DBEngine == db.EngineKDB {
		r, err = kdbRepos(cfg)
	} else {
		r, err = mongoRepos(ctx, cfg)
	}
	if err != nil {
		return nil, err
	}

	registry := ws.NewConnRegistry()
	hub, err := ws.NewHub(registry, cfg.RedisURL)
	if err != nil {
		_ = r.close(ctx)
		return nil, err
	}

	// The waiting room shares the hub's Redis (or its local-only fallback)
	// rather than dialling a second connection — same instance, same
	// trade-off, same "fine for development without it" story.
	waitingRoom, err := lobby.NewStore(cfg.RedisURL)
	if err != nil {
		_ = r.close(ctx)
		return nil, err
	}

	// Mail is resolved at startup rather than at first use: a deployment that
	// offers email sign-in but cannot send mail should fail to start, not fail
	// silently for the first player who tries it.
	mailer, err := auth.NewMailer(cfg.SMTP, cfg.Env == "" || cfg.Env == "local")
	if err != nil {
		_ = r.close(ctx)
		return nil, err
	}

	authHandlers := auth.NewHandlers(auth.Deps{
		Store:     r.store,
		Sessions:  r.sessions,
		Providers: identity.FromConfig(cfg.Identity),
		Mailer:    mailer,
		// The claimer is injected for the same reason the match recorder is:
		// stats imports auth for its middleware, so auth cannot import stats.
		Claimer:              stats.NewClaimer(r.stats),
		PublicBaseURL:        cfg.PublicBaseURL,
		AllowedReturnURLs:    cfg.AllowedReturnURLs,
		AppName:              "Žolíky",
		TestEndpointsEnabled: cfg.TestEndpointsEnabled,
	})

	return &App{
		cfg:         cfg,
		closeDB:     r.close,
		hub:         hub,
		auth:        authHandlers,
		waitingRoom: waitingRoom,
		statsRepo:   r.stats,
		userRepo:    r.user,
		authStore:   r.store,
		matchRepo:   r.match,
		scoringRepo: r.scoring,
		admission:   newAdmission(cfg),
	}, nil
}

// Per-connection footprint on the measured 512 MiB box: ~45 MiB idle
// baseline, ~65 KiB per connected player. Used only to derive a count
// ceiling when none is configured — the memory gate is what actually holds
// the line.
const (
	admissionBaselineBytes = 45 << 20
	admissionPerConnBytes  = 65 << 10
)

// newAdmission builds the capacity gate from config. Everything about it
// degrades to "off" where there is nothing to read: no cgroup limit means no
// derived ceiling and no memory gate, no PSI means no CPU gate — a dev
// machine behaves exactly as if this did not exist.
func newAdmission(cfg Config) *admission.Controller {
	maxConns := cfg.AdmissionMaxConnections
	limitBytes, haveLimit := admission.MemoryLimit()
	switch {
	case maxConns < 0:
		maxConns = 0
	case maxConns == 0 && haveLimit:
		maxConns = admission.DeriveMaxConnections(
			limitBytes, admissionBaselineBytes, admissionPerConnBytes, cfg.AdmissionMemoryWatermark)
	}

	// GOMEMLIMIT tells the garbage collector about the same wall the
	// admission gate defends, so the runtime works harder as it approaches
	// instead of finding out from the OOM killer. The environment variable
	// wins if the operator set one — the runtime already honoured it at
	// startup, and overriding an explicit choice here would be rude.
	if haveLimit && os.Getenv("GOMEMLIMIT") == "" {
		softLimit := int64(float64(limitBytes) * 0.9)
		debug.SetMemoryLimit(softLimit)
		log.Printf("admission: GOMEMLIMIT set to %d MiB (90%% of the %d MiB cgroup limit)",
			softLimit>>20, limitBytes>>20)
	}

	c := admission.New(admission.Limits{
		MaxConnections:      maxConns,
		WaitingRoomRatio:    cfg.AdmissionWaitingRoomRatio,
		MemoryHighWatermark: cfg.AdmissionMemoryWatermark,
		CPUHighWatermark:    cfg.AdmissionCPUWatermark,
	})
	if haveLimit || maxConns > 0 {
		log.Printf("admission: gating on (max connections %d, memory watermark %.2f, cpu watermark %.2f)",
			maxConns, cfg.AdmissionMemoryWatermark, cfg.AdmissionCPUWatermark)
	}
	return c
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
	if a.closeDB != nil {
		return a.closeDB(ctx)
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
	matchMgr := match.NewManager(a.matchRepo, modules, a.hub)
	// The recorder turns each completed match into a permanent record plus the
	// lifetime updates derived from it. Injected rather than constructed
	// inside the manager, so the runtime never has to import stats.
	matchMgr.SetRecorder(stats.NewRecorder(a.statsRepo))
	// And the waiting room, so a host can seat a specific player out of the
	// pool. Wired through a narrow interface rather than an import, so the
	// runtime does not learn what a waiting room is.
	matchMgr.SetWaitingRoom(a.waitingRoom, lobby.RoomID)
	// And how long a bot pauses before answering, which is a pace question
	// rather than a rules one — see Manager.SetBotPace.
	matchMgr.SetBotPace(
		time.Duration(a.cfg.BotThinkMinMS)*time.Millisecond,
		time.Duration(a.cfg.BotThinkMaxMS)*time.Millisecond,
	)

	lobbyHandlers := lobby.NewHandlers(a.hub, a.waitingRoom)
	lobbyHandlers.SetAdmission(a.admission)

	return []routeGroup{
		{"health", func(r chi.Router) {
			r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("ok"))
			})
			// The capacity snapshot, for operators and for clients. A client
			// cannot read the body of a refused WebSocket handshake — the
			// browser API hides everything before the upgrade — so this is
			// where it learns "the server is full" rather than "the server is
			// gone", and words the difference for the player.
			r.Get("/healthz/capacity", func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(a.admission.Snapshot())
			})
			// Both clients render this beside their own build, so a bug
			// report says which server the reporter was actually talking to.
			r.Get("/version", func(w http.ResponseWriter, _ *http.Request) {
				version, commit := buildinfo.Resolved()
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{
					"version": version,
					"commit":  commit,
				})
			})
		}},
		{"auth", a.auth.RegisterRoutes},
		{"lobby", lobbyHandlers.RegisterRoutes},
		{"user", userrepo.NewHandlers(a.userRepo, a.authStore).RegisterRoutes},
		{"scoring", scoring.NewHandlers(a.scoringRepo).RegisterRoutes},
		// The runtime, and now the only gameplay path there is. It replaced
		// the Žolíky-specific one rather than sitting beside it: /games, its
		// documents, its socket and its 24-field wire message are gone.
		{"match", func(r chi.Router) {
			h := match.NewHandlers(matchMgr, a.cfg.TestEndpointsEnabled)
			h.SetAdmission(a.admission)
			h.RegisterRoutes(r)
		}},
		{"stats", stats.NewHandlers(a.statsRepo).RegisterRoutes},
	}
}

func (a *App) RegisterRoutes(r chi.Router) {
	for _, g := range a.routeGroups() {
		g.register(r)
	}
}
