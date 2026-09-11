package app

import (
	"cmp"
	"context"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"os"
	"runtime/debug"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"zolik/server/internal/admission"
	"zolik/server/internal/auth"
	"zolik/server/internal/blackjack"
	"zolik/server/internal/buildinfo"
	"zolik/server/internal/canasta"
	"zolik/server/internal/db"
	"zolik/server/internal/ginrummy"
	"zolik/server/internal/holdem"
	"zolik/server/internal/identity"
	"zolik/server/internal/lobby"
	"zolik/server/internal/match"
	"zolik/server/internal/metrics"
	"zolik/server/internal/module"
	"zolik/server/internal/prsi"
	"zolik/server/internal/rummytiles"
	"zolik/server/internal/scoring"
	"zolik/server/internal/stats"
	userrepo "zolik/server/internal/user"
	"zolik/server/internal/webui"
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
	// matchMgr is the game runtime, built once — see matchManager.
	matchOnce sync.Once
	matchMgr  *match.Manager
	// metrics is the in-memory counter sink every instrumented path writes
	// to; recorder flushes it, reporter reads it back, and boots records
	// this process's lifetime. See internal/metrics.
	metrics  metrics.Sink
	recorder *metrics.Recorder
	reporter *metrics.Reporter
	boots    *metrics.BootRecorder
	// web is the Expo bundle compiled into this binary, served for anything
	// the API does not claim. Absent in every development build and every
	// test, which is why nothing here may assume it is there.
	web *webui.Handler
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
	// metrics is the daily counter store behind the operator console. Built
	// alongside every other repository so the two engines stay
	// column-for-column comparable.
	metrics metrics.Store
	close   func(ctx context.Context) error
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
		metrics:  metrics.NewMongoStore(m),
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
	// What a namespace keeps is not otherwise visible from outside: it is
	// recorded per namespace in meta.json inside the data volume, and reading
	// it there means stopping to mount the volume. Say it at startup instead.
	switch {
	case sc.HistoryRetention:
		log.Printf("kdb history retention: on, mode %s",
			cmp.Or(sc.HistoryMode, "unset (namespaces keep the mode they were built with)"))
	case sc.HistoryMode != "":
		log.Printf("kdb history retention: off — KDB_HISTORY_MODE=%s ignored "+
			"(set FEATURE_FLAG_KDB_HISTORY_RETENTION=true to honour it)", sc.HistoryMode)
	}
	// Same cgroup reading the connection/CPU admission gate already uses
	// (newAdmission, below) — one source of truth for "how much memory does
	// this process actually have". Degrades to off with it: no cgroup limit
	// means no derived budget, same as the rest of admission on a dev
	// machine. Without this, KDB's own per-write admission control
	// (kdb-spec-layer13 Component 48) never turns on for the embedded
	// engine, and nothing sheds load from matches already in progress before
	// the process's total memory crosses the real ceiling.
	if limitBytes, haveLimit := admission.MemoryLimit(); haveLimit {
		sc.MemoryBudgetBytes = limitBytes
		log.Printf("kdb: memory admission on, budget %d MiB", limitBytes>>20)
	}
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
		metrics:  metrics.NewKDBStore(k),
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

	// Built before anything that counts, and Start()ed later from Run: until
	// then counters accumulate in memory and nothing is written, which is
	// what makes a recorder safe to hand out during construction.
	recorder := metrics.NewRecorder(r.metrics)
	gate := newAdmission(cfg)
	gate.SetSink(recorder)
	authHandlers.SetMetrics(recorder)

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
		admission:   gate,
		metrics:     recorder,
		recorder:    recorder,
		reporter:    metrics.NewReporter(r.metrics),
		boots:       metrics.NewBootRecorder(r.metrics, recorder),
		web:         webui.NewHandler(webui.Embedded()),
	}, nil
}

// Start begins the background work that describes the server: flushing
// counters, recording this process's boot, and sweeping abandoned matches.
//
// Separate from New because all three want a context that lives as long as the
// process, and because a test that builds an App has no use for any of them.
// Everything here is best-effort by design: none of it may stop the server
// from serving.
func (a *App) Start(ctx context.Context) {
	a.recorder.Start(ctx)
	version, _ := buildinfo.Resolved()
	a.boots.Start(ctx, version)
	// The sweeper that turns a table nobody came back to into an abandoned
	// one. Started here rather than in routeGroups so there is exactly one of
	// it — see matchManager.
	a.matchManager().StartReaper(ctx)
	// And the sweeper that reclaims the ones it resolved, long afterwards.
	// Separate from the reaper on purpose: that one decides what a table
	// *became* and runs in seconds, this one decides when the row stops being
	// worth keeping and runs in days. Folding them together is how a deadline
	// the runtime reasons about turns into a deadline the store deletes on,
	// which is the bug retention.go exists not to repeat.
	a.matchManager().StartRetention(ctx, a.cfg.Retention)
}

// Stop closes this process's boot record, and flushes whatever counters have
// not been written yet.
//
// Called from the shutdown path *before* Close, so a slow database teardown
// cannot be what loses the fact that this was a clean exit — which is the
// difference between the next start reporting a deploy and reporting a crash.
func (a *App) Stop(ctx context.Context) {
	a.boots.Stop(ctx, metrics.ReasonSignal)
	a.recorder.Flush(ctx)
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
		softLimit := goMemLimitFor(limitBytes, cfg.AdmissionMemoryWatermark)
		debug.SetMemoryLimit(softLimit)
		log.Printf("admission: GOMEMLIMIT set to %d MiB, below the %.0f%% watermark at %d MiB "+
			"(cgroup limit %d MiB)",
			softLimit>>20, cfg.AdmissionMemoryWatermark*100,
			int64(float64(limitBytes)*cfg.AdmissionMemoryWatermark)>>20, limitBytes>>20)
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

// goMemLimitMargin is how far below the admission watermark GOMEMLIMIT sits.
//
// Which side of the watermark it sits on is the whole question, and it used to
// be the wrong one: a flat 90% against a default 85% watermark meant the
// collector was told it could grow *past* the point where the gate starts
// turning players away. Under sustained allocation that is exactly what it did
// — the soak runs show the process climbing to 91% while refusing arrivals,
// and a single collection then giving 280 MiB back. Players were refused to
// protect memory that was garbage.
//
// So: collector first, gate second. The runtime is pushed to work as the heap
// approaches a line *below* the one the gate defends, and a player is refused
// only when collecting could not get under it — which is the only reason
// refusing someone is the right answer.
//
// It is a margin rather than a guarantee. GOMEMLIMIT governs the Go runtime's
// own accounting, and the gate reads the cgroup, which also counts file pages
// the runtime knows nothing about; the two can still disagree by whatever the
// page cache is holding. Ordering them correctly is not the same as making them
// agree, and only the first is in this function's gift.
const goMemLimitMargin = 0.05

// goMemLimitFallbackFraction applies where there is no watermark to sit below
// — the gate can be turned off (watermark <= 0) while the cgroup limit is still
// real, and the collector should still be told about the ceiling.
const goMemLimitFallbackFraction = 0.9

// goMemLimitMinFraction floors the result, so a watermark set very low
// misconfigures the gate rather than also strangling the collector.
const goMemLimitMinFraction = 0.5

func goMemLimitFor(limitBytes uint64, watermark float64) int64 {
	fraction := goMemLimitFallbackFraction
	if watermark > 0 {
		fraction = watermark - goMemLimitMargin
	}
	if fraction < goMemLimitMinFraction {
		fraction = goMemLimitMinFraction
	}
	return int64(float64(limitBytes) * fraction)
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
// matchManager builds the runtime once and hands back the same instance.
//
// Memoised because routeGroups runs per router registration and the manager is
// no longer only a bundle of handlers: it owns the bot-loop flags, and the
// abandon reaper Start runs against it. Two managers would mean two reapers
// racing each other for the same rows, and bot state split across instances
// that cannot see each other's flags.
func (a *App) matchManager() *match.Manager {
	a.matchOnce.Do(func() {
		// One runtime, hosting every game. The registry is the only place a
		// game is named: register a module and it appears in /modules, in the
		// lobby's picker, and on the one screen that plays all of them.
		modules := module.NewRegistry(zolikmod.New(), prsi.New(), canasta.New(), holdem.New(), ginrummy.New(), rummytiles.New(), blackjack.New())
		a.matchMgr = a.configureManager(match.NewManager(a.matchRepo, modules, a.hub))
	})
	return a.matchMgr
}

func (a *App) configureManager(matchMgr *match.Manager) *match.Manager {
	// The recorder turns each completed match into a permanent record plus the
	// lifetime updates derived from it. Injected rather than constructed
	// inside the manager, so the runtime never has to import stats.
	statsRecorder := stats.NewRecorder(a.statsRepo)
	statsRecorder.SetMetrics(a.metrics)
	matchMgr.SetRecorder(statsRecorder)
	// The runtime counts what it does — lobbies opened, games started,
	// tables abandoned — for the operator's console.
	matchMgr.SetMetrics(a.metrics)
	// And the waiting room, so a host can seat a specific player out of the
	// pool. Wired through a narrow interface rather than an import, so the
	// runtime does not learn what a waiting room is.
	matchMgr.SetWaitingRoom(a.waitingRoom, lobby.RoomID)
	// And where the outside world reaches us, so a host can share a link
	// instead of dictating a join code. The same configured value the OAuth
	// redirect is built from, and for the same reason — see
	// match.Manager.SetInviteBaseURL.
	matchMgr.SetInviteBaseURL(a.cfg.PublicBaseURL)
	// And how long a bot pauses before answering, which is a pace question
	// rather than a rules one — see Manager.SetBotPace.
	matchMgr.SetBotPace(
		time.Duration(a.cfg.BotThinkMinMS)*time.Millisecond,
		time.Duration(a.cfg.BotThinkMaxMS)*time.Millisecond,
	)

	return matchMgr
}

func (a *App) routeGroups() []routeGroup {
	matchMgr := a.matchManager()

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
		// Where the memory went. Off unless APP_ENV is local — see
		// debugmem.go for why it is not something a public listener carries.
		{"debug", func(r chi.Router) {
			if a.cfg.DebugEndpointsEnabled {
				a.registerDebugRoutes(r)
			}
		}},
	}
}

func (a *App) RegisterRoutes(r chi.Router) {
	for _, g := range a.routeGroups() {
		g.register(r)
	}
	a.registerWebUI(r)
}

// SetWebUI replaces the compiled-in bundle. Only tests call it: the real
// bundle arrives through //go:embed at image build time, and there is no
// deployment in which it is chosen at runtime.
func (a *App) SetWebUI(fsys fs.FS) { a.web = webui.NewHandler(fsys) }

// registerWebUI hangs the web client off the router's leftovers.
//
// Deliberately NotFound/MethodNotAllowed rather than a `r.Get("/*")` catch-all:
// a catch-all is a route, and chi resolves routes by specificity in a way that
// makes "did the API or the SPA win this path" a question with a surprising
// answer. Off the leftovers, the API always wins, and a route group added
// later cannot be shadowed by the client no matter what it is called.
//
// MethodNotAllowed is not belt-and-braces. The client has screens at
// /auth/login, /auth/register and /auth/guest; the server registers those
// three as POST endpoints. A browser opening one of those URLs sends a GET,
// chi matches the path, finds no GET, and answers 405 — so before this
// existed, following a link to the sign-in page produced "Method Not Allowed"
// rather than the sign-in page. The old nginx vhost had the same bug for the
// same reason, one layer further out.
func (a *App) registerWebUI(r chi.Router) {
	r.NotFound(a.serveWebUI(http.StatusNotFound))
	r.MethodNotAllowed(a.serveWebUI(http.StatusMethodNotAllowed))
}

// serveWebUI answers with the client, and falls back to the API's own status
// when it cannot.
//
// The order matters. An existing file is served whatever the request's Accept
// header says, because a browser fetching /_expo/static/js/entry-<hash>.js
// sends `Accept: */*` and a rule keyed on text/html would 404 the entire
// JavaScript bundle. Only when the path names no file does Accept decide, and
// then it decides the right thing: a browser navigating to /lobby/games gets
// the app, and a mistyped API call gets the 404 it deserves instead of a page
// it cannot parse.
func (a *App) serveWebUI(status int) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if a.web.Available() && (req.Method == http.MethodGet || req.Method == http.MethodHead) {
			if name, ok := a.web.Lookup(req.URL.Path); ok {
				a.web.ServeFile(w, req, name)
				return
			}
			if webui.WantsHTML(req) {
				a.web.ServeIndex(w, req)
				return
			}
		}
		http.Error(w, http.StatusText(status), status)
	}
}
