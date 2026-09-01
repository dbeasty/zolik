package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"zolik/server/internal/auth"
	"zolik/server/internal/buildinfo"
	"zolik/server/internal/db"
	"zolik/server/internal/lobby"
	"zolik/server/internal/match"
	"zolik/server/internal/scoring"
	"zolik/server/internal/stats"
	userrepo "zolik/server/internal/user"
	"zolik/server/internal/ws"
)

// mustWaitingRoom builds a local-only waiting room. No Redis, which is the
// supported single-instance mode, so this needs nothing running.
func mustWaitingRoom(t *testing.T) lobby.Store {
	t.Helper()
	s, err := lobby.NewStore("")
	if err != nil {
		t.Fatalf("building a waiting room: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// TestRouteGroupsSurviveMounting checks that no group of routes silently
// swallows another's.
//
// chi resolves a path segment by position, not by the name in the braces, so
// /matches/{id} and /matches/{gameId} are the same route to it. Registering
// both is not an error and not a panic: the second simply replaces the first,
// and the first's handler becomes unreachable. The module runtime and the
// stats handlers collided that way, and every symptom appeared a layer away —
// a live match answering "no recorded result for this game" — so only the
// browser suite caught it.
//
// Mounting each group alone and comparing against the combined table turns
// that into a unit-test failure naming the group that lost.
func TestRouteGroupsSurviveMounting(t *testing.T) {
	a := offlineApp(t)

	combined := map[string]bool{}
	for _, route := range routesOf(t, func(r chi.Router) {
		for _, g := range a.routeGroups() {
			g.register(r)
		}
	}) {
		combined[route] = true
	}

	for _, g := range a.routeGroups() {
		for _, route := range routesOf(t, g.register) {
			if !combined[route] {
				t.Errorf(
					"%s registers %s, but it is missing from the combined table — "+
						"another group claims the same path shape and replaced it",
					g.name, route,
				)
			}
		}
	}
}

// TestMatchRuntimeOwnsMatchesPath pins the specific pair that collided, so the
// reason the stats route lives under /games reads as deliberate rather than
// arbitrary.
func TestMatchRuntimeOwnsMatchesPath(t *testing.T) {
	a := offlineApp(t)

	got := map[string]bool{}
	for _, route := range routesOf(t, func(r chi.Router) {
		for _, g := range a.routeGroups() {
			g.register(r)
		}
	}) {
		got[route] = true
	}

	for _, want := range []string{
		"GET /matches/{id}",             // the runtime's live match
		"GET /matches/{matchId}/result", // the recorded result of a finished match
		"POST /matches/{id}/join",       // and the runtime's own sub-routes
	} {
		if !got[want] {
			t.Errorf("route %q is not reachable", want)
		}
	}
}

// TestVersionRouteIsRegistered pins /version into the health group's route
// table, and TestVersionRouteReportsBuildInfo pins its exact wire shape —
// both the RN and TUI clients parse "version"/"commit" directly.
func TestVersionRouteIsRegistered(t *testing.T) {
	a := offlineApp(t)

	got := map[string]bool{}
	for _, g := range a.routeGroups() {
		for _, route := range routesOf(t, g.register) {
			got[route] = true
		}
	}

	if !got["GET /version"] {
		t.Error(`"GET /version" is not reachable`)
	}
}

func TestVersionRouteReportsBuildInfo(t *testing.T) {
	origVersion, origCommit := buildinfo.Version, buildinfo.Commit
	t.Cleanup(func() { buildinfo.Version, buildinfo.Commit = origVersion, origCommit })
	buildinfo.Version, buildinfo.Commit = "1.1.1.2", "7feb025"

	a := offlineApp(t)
	r := chi.NewRouter()
	a.RegisterRoutes(r)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/version", nil)
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /version status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var body struct {
		Version string `json:"version"`
		Commit  string `json:"commit"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if body.Version != "1.1.1.2" {
		t.Errorf("version = %q, want 1.1.1.2", body.Version)
	}
	if body.Commit != "7feb025" {
		t.Errorf("commit = %q, want 7feb025", body.Commit)
	}
}

func TestCapacityRouteIsRegistered(t *testing.T) {
	a := offlineApp(t)

	got := map[string]bool{}
	for _, g := range a.routeGroups() {
		for _, route := range routesOf(t, g.register) {
			got[route] = true
		}
	}
	if !got["GET /healthz/capacity"] {
		t.Error(`"GET /healthz/capacity" is not reachable`)
	}
}

func TestCapacityRouteReportsSnapshot(t *testing.T) {
	a := offlineApp(t)
	r := chi.NewRouter()
	a.RegisterRoutes(r)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz/capacity", nil)
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /healthz/capacity status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var body struct {
		Accepting         bool `json:"accepting"`
		WaitingRoomOpen   bool `json:"waitingRoomOpen"`
		StartingMatches   bool `json:"startingMatches"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if !body.Accepting || !body.WaitingRoomOpen || !body.StartingMatches {
		t.Fatalf("nil admission snapshot = %+v, want all gates open", body)
	}
}

// routesOf mounts one registrar on a router of its own and returns its routes
// as "METHOD /pattern".
func routesOf(t *testing.T, register func(chi.Router)) []string {
	t.Helper()
	r := chi.NewRouter()
	register(r)

	var out []string
	err := chi.Walk(r, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		out = append(out, method+" "+route)
		return nil
	})
	if err != nil {
		t.Fatalf("walking routes: %v", err)
	}
	return out
}

// offlineApp builds an App wired up exactly as New does, but against a Mongo
// client that has never dialled anything. Registering routes touches no
// collection, so this needs no database — which is what keeps this a unit test.
func offlineApp(t *testing.T) *App {
	t.Helper()

	client, err := mongo.Connect(options.Client().ApplyURI("mongodb://127.0.0.1:27017"))
	if err != nil {
		t.Fatalf("building an offline mongo client: %v", err)
	}
	t.Cleanup(func() { _ = client.Disconnect(t.Context()) })
	m := &db.Mongo{Client: client, DB: client.Database("route_test")}

	hub, err := ws.NewHub(ws.NewConnRegistry(), "")
	if err != nil {
		t.Fatalf("building a hub: %v", err)
	}
	t.Cleanup(func() { _ = hub.Close() })

	return &App{
		closeDB: m.Close,
		hub:     hub,
		auth: auth.NewHandlers(auth.Deps{
			Store:    auth.NewStore(m),
			Sessions: auth.NewSessionRepository(m),
		}),
		// The waiting room is part of the route table now: /lobby/waiting and
		// the invite path both need it, and a nil store would panic on mount
		// rather than fail a test with something readable.
		waitingRoom: mustWaitingRoom(t),
		statsRepo:   stats.NewRepository(m),
		userRepo:    userrepo.NewRepository(m),
		authStore:   auth.NewStore(m),
		matchRepo:   match.NewRepository(m),
		scoringRepo: scoring.NewRepository(m),
	}
}

// TestPublicVhostReachesEveryRoute keeps the deployed nginx vhost in step with
// the router it sits in front of.
//
// The vhost hands a fixed list of top-level prefixes to the Go server and
// serves everything else from the Expo export, falling back to index.html for
// client-side routes. That fallback is why a missing prefix is worse than a
// 404: the request does not fail, it succeeds with HTML, and the client gets a
// parse error somewhere far away from the cause. /lobby/waiting shipped that
// way — the host's waiting-room panel asked for JSON and was handed the page
// it was rendered on.
//
// Enumerating the real router rather than grepping the source, so a route
// added through any group is caught by the same test.
func TestPublicVhostReachesEveryRoute(t *testing.T) {
	a := offlineApp(t)

	conf, err := os.ReadFile(filepath.Join("..", "..", "..", "deploy", "nginx", "play-limidus.conf"))
	if err != nil {
		t.Fatalf("reading the deployed vhost: %v", err)
	}
	covered := vhostPrefixes(string(conf))

	seen := map[string]bool{}
	for _, g := range a.routeGroups() {
		for _, route := range routesOf(t, g.register) {
			// "GET /matches/{id}/join" -> "matches"
			path := route[strings.IndexByte(route, '/'):]
			top := strings.SplitN(strings.TrimPrefix(path, "/"), "/", 2)[0]
			if top == "" || seen[top] {
				continue
			}
			seen[top] = true
			if !covered[top] {
				t.Errorf(
					"the server registers /%s but deploy/nginx/play-limidus.conf does not send it "+
						"to the API — in production that path returns index.html instead", top,
				)
			}
		}
	}
}

// vhostPrefixes reads the top-level paths the vhost forwards to the Go server:
// the alternation in the API location's regex, plus any plain prefix location
// (which is how the WebSocket route is matched).
func vhostPrefixes(conf string) map[string]bool {
	out := map[string]bool{}

	if m := regexp.MustCompile(`location\s+~\s+\^/\(([^)]*)\)`).FindStringSubmatch(conf); m != nil {
		for _, name := range strings.Split(m[1], "|") {
			out[strings.TrimSpace(name)] = true
		}
	}
	for _, m := range regexp.MustCompile(`(?m)^\s*location\s+(?:\^~\s+)?/([a-z0-9_-]+)/`).FindAllStringSubmatch(conf, -1) {
		out[m[1]] = true
	}
	return out
}
