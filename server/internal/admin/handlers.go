// Package admin is the operator's console: the screen that says how much was
// played, by how many people, and how often the server said no.
//
// It is not registered on the public listener. Not guarded there — absent.
// nginx proxies one catch-all to the game server by design, so anything the
// public router registers is on the internet by construction; the console
// lives on a second listener that nginx does not proxy and Compose publishes
// to host loopback only. See app.RegisterAdminRoutes and
// docs/logging-and-reporting-plan.md §7.
//
// That is the control that matters. The guard below is the second layer, and
// a good one, but "the route is not on the public listener" is the one that
// does not depend on a guard being right.
package admin

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"zolik/server/internal/metrics"
)

// Reports supplies the operator's numbers.
type Reports interface {
	Build(ctx context.Context, from, to time.Time, bucket metrics.Bucket) (metrics.Report, error)
}

// Live reports this instance's currently-held websocket connections, by room.
// An interface rather than the concrete registry so the console can be tested
// without a hub.
type Live interface {
	Totals() (rooms, conns int)
	CountRoom(roomID string) int
}

type Deps struct {
	Guard   *Guard
	Reports Reports
	Live    Live
	// WaitingRoomID is the reserved room the lobby's waiting players occupy.
	// It shares the connection registry with real matches, so without it the
	// overview reports an idle server as having a game in progress. Injected
	// rather than imported so this package need not know what a lobby is.
	WaitingRoomID string
	// Capacity is how close to full the box is right now, as opposed to how
	// often it has already said no. The two belong on one screen: a refusal
	// count means something different at 40% of capacity than at 95%. A
	// func rather than an interface so app can hand over
	// admission.Controller.Snapshot without this package importing admission.
	// Optional — nil shows no capacity panel rather than an empty one.
	Capacity func() any
	// Version is what this build calls itself, shown in the console footer so
	// an operator can tell which release the numbers came from.
	Version string
}

type Handlers struct {
	deps Deps
}

func NewHandlers(d Deps) *Handlers { return &Handlers{deps: d} }

// RegisterRoutes mounts the console and its API.
//
// Mounted by the admin listener only. A test that finds these on the public
// router is a test reporting a serious regression — see
// TestPublicRouterHasNoAdminRoutes.
func (h *Handlers) RegisterRoutes(r chi.Router) {
	ui := uiHandler()
	r.Route("/admin", func(r chi.Router) {
		// The API is registered before the console's catch-all, because chi
		// resolves by specificity and a catch-all that won the match would
		// serve index.html to a request for JSON — a failure that surfaces as
		// a parse error somewhere else entirely.
		r.Route("/api", func(r chi.Router) {
			// Unauthenticated by necessity: this is where a token comes from.
			// Rate-limited by address inside, before the password is looked at.
			r.Post("/login", h.login)
			// What doors are configured, so the console can render the right
			// form without an operator guessing. It names methods, never
			// whether a given username exists.
			r.Get("/methods", h.methods)

			r.Group(func(r chi.Router) {
				r.Use(h.deps.Guard.Require)
				r.Get("/session", h.session)
				r.Get("/report", h.report)
				r.Get("/status", h.status)
			})
		})
		r.Handle("/*", ui)
		r.Handle("/", ui)
	})
}

// methods says which sign-in doors exist. Deliberately not which accounts do.
func (h *Handlers) methods(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{
		"password":  h.deps.Guard.password.Enabled(),
		"allowList": h.deps.Guard.HasAllowList(),
		"version":   h.deps.Version,
	})
}

func (h *Handlers) session(w http.ResponseWriter, r *http.Request) {
	admin, _ := Caller(r)
	writeJSON(w, map[string]any{
		"username":    admin.Username,
		"email":       admin.Email,
		"viaPassword": admin.ViaPassword,
	})
}

// report answers the operator's question for a range of UTC days.
//
// Defaults to the last 30 days bucketed by day, because that is the question
// somebody opening the console without choosing anything is asking.
func (h *Handlers) report(w http.ResponseWriter, r *http.Request) {
	if h.deps.Reports == nil {
		http.Error(w, "reporting is not configured", http.StatusServiceUnavailable)
		return
	}

	q := r.URL.Query()
	bucket := metrics.ParseBucket(q.Get("bucket"))
	to := parseDay(q.Get("to"), time.Now().UTC())
	from := parseDay(q.Get("from"), to.AddDate(0, 0, -29))

	rep, err := h.deps.Reports.Build(r.Context(), from, to, bucket)
	if err != nil {
		slog.Warn("building the operator report failed", "error", err)
		http.Error(w, "could not build the report", http.StatusInternalServerError)
		return
	}

	if q.Get("format") == "csv" {
		writeReportCSV(w, rep)
		return
	}
	writeJSON(w, rep)
}

// status is the right-now view: what this process is holding, and how close to
// its ceiling it is.
func (h *Handlers) status(w http.ResponseWriter, _ *http.Request) {
	out := map[string]any{"version": h.deps.Version}

	if h.deps.Live != nil {
		rooms, conns := h.deps.Live.Totals()
		waiting := 0
		if h.deps.WaitingRoomID != "" {
			waiting = h.deps.Live.CountRoom(h.deps.WaitingRoomID)
			// The waiting room is a room in the registry like any other, so
			// without this an idle server with one person in the lobby reports
			// a game in progress. Discounted only when it is actually
			// occupied, because an empty room is not registered at all.
			if waiting > 0 {
				rooms--
			}
		}
		if rooms < 0 {
			rooms = 0
		}
		out["live"] = map[string]any{
			"matches":     rooms,
			"connections": conns,
			"waiting":     waiting,
		}
	}
	if h.deps.Capacity != nil {
		out["capacity"] = h.deps.Capacity()
	}
	writeJSON(w, out)
}

// parseDay reads a YYYY-MM-DD query parameter, falling back to a default.
// A malformed date is the default rather than an error: the console's own
// controls cannot produce one, so a bad value is somebody experimenting with
// the URL and a 400 would teach them nothing the fallback does not.
func parseDay(s string, fallback time.Time) time.Time {
	if s == "" {
		return fallback
	}
	t, err := time.Parse(metrics.DayFormat, s)
	if err != nil {
		return fallback
	}
	return t
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	// The console holds a token in local storage and these responses describe
	// the deployment; neither belongs in a shared cache or a search index.
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	_ = json.NewEncoder(w).Encode(v)
}
