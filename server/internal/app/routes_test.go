package app

import (
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"zolik/server/internal/auth"
	"zolik/server/internal/db"
	"zolik/server/internal/lobby"
	"zolik/server/internal/ws"
)

// mustWaitingRoom builds a local-only waiting room. No Redis, which is the
// supported single-instance mode, so this needs nothing running.
func mustWaitingRoom(t *testing.T) *lobby.Store {
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
		db:   m,
		hub:  hub,
		auth: auth.NewHandlers(auth.Deps{Mongo: m}),
		// The waiting room is part of the route table now: /lobby/waiting and
		// the invite path both need it, and a nil store would panic on mount
		// rather than fail a test with something readable.
		waitingRoom: mustWaitingRoom(t),
	}
}
