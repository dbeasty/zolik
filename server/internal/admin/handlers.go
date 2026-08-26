package admin

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/v2/bson"
	"golang.org/x/crypto/bcrypt"

	"zolik/server/internal/auth"
	"zolik/server/internal/feedback"
	"zolik/server/internal/models"
	"zolik/server/internal/stats"
	userrepo "zolik/server/internal/user"
)

// Users is the slice of the account repository the admin API needs.
type Users interface {
	UserLookup
	UpdateByID(ctx context.Context, id bson.ObjectID, update bson.M) error
	DeleteByID(ctx context.Context, id bson.ObjectID) error
	ListUsers(ctx context.Context, q userrepo.Query) ([]models.User, error)
	CountUsers(ctx context.Context, q userrepo.Query) (int64, error)
	CountUsersSeenSince(ctx context.Context, since time.Time) (int64, error)
}

// Identities is the slice of auth.Store the admin API needs: listing an
// account's sign-in methods, and removing them all when it is deleted.
type Identities interface {
	ListIdentities(ctx context.Context, userID string) ([]models.Identity, error)
	DeleteIdentitiesForUser(ctx context.Context, userID string) (int64, error)
}

// Sessions is the slice of auth.SessionRepository the admin API needs.
type Sessions interface {
	DeleteByUserID(ctx context.Context, userID string) (int64, error)
	CountActiveSessions(ctx context.Context, now time.Time) (users, guests int64, err error)
}

// Usage supplies the match-record aggregates behind the overview.
type Usage interface {
	UsageSummary(ctx context.Context, days int) (stats.Usage, error)
}

// Live reports this instance's currently-held websocket connections, by room.
type Live interface {
	Totals() (rooms, conns int)
	CountRoom(roomID string) int
}

// Feedback is the slice of the report repository the console triages with.
type Feedback interface {
	List(ctx context.Context, q feedback.Query) ([]feedback.Report, error)
	Count(ctx context.Context, q feedback.Query) (int64, error)
	CountByStatus(ctx context.Context) (map[string]int64, error)
	Update(ctx context.Context, id bson.ObjectID, set bson.M) error
	Delete(ctx context.Context, id bson.ObjectID) error
}

type Deps struct {
	Guard      *Guard
	Users      Users
	Identities Identities
	Sessions   Sessions
	Usage      Usage
	Live       Live
	Feedback   Feedback
	// WaitingRoomID is the reserved room the lobby's waiting players occupy.
	// It shares the connection registry with real matches, so without it the
	// overview reports an idle server as having a game in progress. Injected
	// rather than imported so this package need not know what a lobby is.
	WaitingRoomID string
}

type Handlers struct {
	deps Deps
}

func NewHandlers(d Deps) *Handlers { return &Handlers{deps: d} }

func (h *Handlers) RegisterRoutes(r chi.Router) {
	ui := uiHandler()
	r.Route("/admin", func(r chi.Router) {
		// The API is registered before the console's catch-all. chi matches a
		// static segment ahead of a wildcard regardless of registration order,
		// so /admin/api/... never reaches the file server — but keeping them
		// in this order makes that the obvious reading rather than a fact you
		// have to know about chi.
		r.Route("/api", func(r chi.Router) {
			// Public: signing in is by definition what someone with no token
			// does. It is throttled per address — see login.go.
			r.Post("/login", h.login)
			// Whether a password sign-in exists at all, so the console knows
			// which forms to offer without probing /login.
			r.Get("/methods", h.methods)

			r.Group(func(r chi.Router) {
				r.Use(h.deps.Guard.Require)
				r.Get("/session", h.session)
				r.Get("/usage", h.usage)
				r.Get("/users", h.listUsers)
				r.Delete("/users/{id}", h.deleteUser)
				r.Post("/users/{id}/password", h.setPassword)
				r.Post("/users/{id}/revoke-sessions", h.revokeSessions)
				r.Get("/feedback", h.listFeedback)
				r.Patch("/feedback/{id}", h.patchFeedback)
				r.Delete("/feedback/{id}", h.deleteFeedback)
			})
		})
		r.Handle("/", ui)
		r.Handle("/*", ui)
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// methods tells the console which sign-in forms to offer. It is public
// because it has to be — it is read before anyone is signed in — and says only
// whether each door exists, never anything about who may come through it.
func (h *Handlers) methods(w http.ResponseWriter, _ *http.Request) {
	g := h.deps.Guard
	writeJSON(w, map[string]any{
		"password": g.password.Enabled(),
		"email":    len(g.admins) > 0,
	})
}

// session tells the UI who it is signed in as. It carries little of its own —
// reaching it at all is the answer, since the guard rejects everyone else.
func (h *Handlers) session(w http.ResponseWriter, r *http.Request) {
	a, _ := Caller(r)
	res := map[string]any{
		"username":    a.Username,
		"email":       a.Email,
		"viaPassword": a.ViaPassword,
	}
	if !a.User.ID.IsZero() {
		res["id"] = a.User.ID.Hex()
	}
	writeJSON(w, res)
}

func (h *Handlers) usage(w http.ResponseWriter, r *http.Request) {
	days := 30
	if v := strings.TrimSpace(r.URL.Query().Get("days")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			days = n
		}
	}
	ctx := r.Context()
	now := time.Now().UTC()

	matches, err := h.deps.Usage.UsageSummary(ctx, days)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	totalUsers, err := h.deps.Users.CountUsers(ctx, userrepo.Query{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	activeDay, err := h.deps.Users.CountUsersSeenSince(ctx, now.Add(-24*time.Hour))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	activeWeek, err := h.deps.Users.CountUsersSeenSince(ctx, now.AddDate(0, 0, -7))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sessionUsers, sessionGuests, err := h.deps.Sessions.CountActiveSessions(ctx, now)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	rooms, conns := h.deps.Live.Totals()
	// The waiting room is one of those rooms but is not a match, so it is
	// reported on its own rather than inflating the game count.
	waiting := h.deps.Live.CountRoom(h.deps.WaitingRoomID)
	matchRooms := rooms
	if waiting > 0 {
		matchRooms--
	}

	writeJSON(w, map[string]any{
		"users": map[string]any{
			"total":        totalUsers,
			"activeDay":    activeDay,
			"activeWeek":   activeWeek,
			"openSessions": sessionUsers,
		},
		"guests":  map[string]any{"openSessions": sessionGuests},
		"matches": matches,
		// Labelled as this instance's because the registry holds only the
		// sockets this process is serving — behind a load balancer it is a
		// share of the fleet, not the whole of it.
		"live": map[string]any{
			"instanceMatches":     matchRooms,
			"instanceConnections": conns,
			"instanceWaiting":     waiting,
		},
	})
}

// userRow is the account shape the roster renders. PasswordHash is not in it,
// and must never be: models.User already hides it from JSON, and this makes
// the omission deliberate rather than inherited.
type userRow struct {
	ID            string    `json:"id"`
	Username      string    `json:"username"`
	Email         string    `json:"email,omitempty"`
	EmailVerified bool      `json:"emailVerified"`
	AuthProvider  string    `json:"authProvider"`
	HasPassword   bool      `json:"hasPassword"`
	CreatedAt     time.Time `json:"createdAt"`
	LastSeenAt    time.Time `json:"lastSeenAt"`
	Identities    []string  `json:"identities"`
	IsAdmin       bool      `json:"isAdmin"`
}

func (h *Handlers) listUsers(w http.ResponseWriter, r *http.Request) {
	q := userrepo.Query{Search: r.URL.Query().Get("search")}
	if v, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil {
		q.Limit = v
	}
	if v, err := strconv.Atoi(r.URL.Query().Get("skip")); err == nil {
		q.Skip = v
	}

	ctx := r.Context()
	users, err := h.deps.Users.ListUsers(ctx, q)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	total, err := h.deps.Users.CountUsers(ctx, q)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	rows := make([]userRow, 0, len(users))
	for _, u := range users {
		id := u.ID.Hex()
		providers := []string{}
		if list, err := h.deps.Identities.ListIdentities(ctx, id); err == nil {
			for _, ident := range list {
				providers = append(providers, ident.Provider)
			}
		}
		_, isAdmin := h.deps.Guard.admins[strings.ToLower(strings.TrimSpace(u.Email))]
		rows = append(rows, userRow{
			ID:            id,
			Username:      u.Username,
			Email:         u.Email,
			EmailVerified: u.EmailVerified,
			AuthProvider:  u.AuthProvider,
			HasPassword:   u.PasswordHash != "",
			CreatedAt:     u.CreatedAt,
			LastSeenAt:    u.LastSeenAt,
			Identities:    providers,
			IsAdmin:       isAdmin && u.EmailVerified,
		})
	}
	writeJSON(w, map[string]any{"users": rows, "total": total})
}

// target resolves the {id} path parameter to an account, reporting the failure
// itself so callers only handle the happy path.
func (h *Handlers) target(w http.ResponseWriter, r *http.Request) (models.User, bool) {
	oid, err := bson.ObjectIDFromHex(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid user id", http.StatusBadRequest)
		return models.User{}, false
	}
	u, err := h.deps.Users.FindByID(r.Context(), oid)
	if err != nil {
		http.Error(w, "no such user", http.StatusNotFound)
		return models.User{}, false
	}
	return u, true
}

func (h *Handlers) deleteUser(w http.ResponseWriter, r *http.Request) {
	target, ok := h.target(w, r)
	if !ok {
		return
	}
	// A password administrator has no account of their own, so there is
	// nothing for them to delete by accident and the zero id never matches.
	if caller, _ := Caller(r); !caller.User.ID.IsZero() && target.ID == caller.User.ID {
		// Deleting the account you are signed in as would revoke the sessions
		// mid-request and leave the allow-list pointing at nothing.
		http.Error(w, "you cannot delete your own account", http.StatusConflict)
		return
	}

	ctx := r.Context()
	id := target.ID.Hex()
	// Identities and sessions go first, and the account last. In that order a
	// failure part-way leaves an account that can still be signed into and
	// deleted again; the reverse order would leave orphaned identities holding
	// the (provider, subject) unique index against an account that no longer
	// exists, locking that person out of ever signing up again.
	if _, err := h.deps.Identities.DeleteIdentitiesForUser(ctx, id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if _, err := h.deps.Sessions.DeleteByUserID(ctx, id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := h.deps.Users.DeleteByID(ctx, target.ID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Match records are deliberately left alone. They are the immutable
	// history every other player's statistics are derived from, so deleting
	// this account's rows would silently rewrite opponents' records too.
	writeJSON(w, map[string]any{"deleted": true, "id": id})
}

type setPasswordReq struct {
	Password string `json:"password"`
}

// minPasswordLen is the floor for an administrator-set password. It is longer
// than the sign-up path's own minimum on purpose: a password set by someone
// other than its owner has to survive being sent to them out-of-band.
const minPasswordLen = 12

func (h *Handlers) setPassword(w http.ResponseWriter, r *http.Request) {
	target, ok := h.target(w, r)
	if !ok {
		return
	}

	var body setPasswordReq
	// An empty body is valid and means "generate one for me", so a decode
	// failure on no content is not an error.
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && r.ContentLength > 0 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	password := strings.TrimSpace(body.Password)
	generated := false
	if password == "" {
		p, err := auth.NewRandomToken(12)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		password, generated = p, true
	} else if len([]rune(password)) < minPasswordLen {
		http.Error(w, "password must be at least 12 characters", http.StatusBadRequest)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	ctx := r.Context()
	if err := h.deps.Users.UpdateByID(ctx, target.ID, bson.M{"passwordHash": string(hash)}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Every existing session is revoked along with the change. A password
	// reset that left the old sessions alive would not actually lock anyone
	// out, which is the whole point of resetting it.
	revoked, err := h.deps.Sessions.DeleteByUserID(ctx, target.ID.Hex())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	res := map[string]any{"updated": true, "revokedSessions": revoked}
	if generated {
		// Returned exactly once, and never stored in the clear — there is no
		// second chance to read it.
		res["password"] = password
	}
	writeJSON(w, res)
}

/* ------------------------------------------------------------------ feedback */

func (h *Handlers) listFeedback(w http.ResponseWriter, r *http.Request) {
	q := feedback.Query{
		Status: strings.TrimSpace(r.URL.Query().Get("status")),
		Kind:   strings.TrimSpace(r.URL.Query().Get("kind")),
	}
	// An unrecognised filter is refused rather than ignored. Silently
	// returning everything would read to the operator as "no reports match",
	// which is the opposite of the truth.
	if q.Status != "" && !feedback.ValidStatus(q.Status) {
		http.Error(w, "unknown status", http.StatusBadRequest)
		return
	}
	if q.Kind != "" && !feedback.ValidKind(q.Kind) {
		http.Error(w, "unknown kind", http.StatusBadRequest)
		return
	}
	if v, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil {
		q.Limit = v
	}
	if v, err := strconv.Atoi(r.URL.Query().Get("skip")); err == nil {
		q.Skip = v
	}

	ctx := r.Context()
	reports, err := h.deps.Feedback.List(ctx, q)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	total, err := h.deps.Feedback.Count(ctx, q)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	counts, err := h.deps.Feedback.CountByStatus(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"reports": reports, "total": total, "counts": counts})
}

type patchFeedbackReq struct {
	Status *string `json:"status,omitempty"`
	Note   *string `json:"note,omitempty"`
}

func (h *Handlers) patchFeedback(w http.ResponseWriter, r *http.Request) {
	id, err := bson.ObjectIDFromHex(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid report id", http.StatusBadRequest)
		return
	}
	var body patchFeedbackReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	set := bson.M{}
	if body.Status != nil {
		status := strings.TrimSpace(*body.Status)
		if !feedback.ValidStatus(status) {
			http.Error(w, "unknown status", http.StatusBadRequest)
			return
		}
		set["status"] = status
	}
	if body.Note != nil {
		set["note"] = strings.TrimSpace(*body.Note)
	}
	if len(set) == 0 {
		writeJSON(w, map[string]any{"updated": false})
		return
	}

	if err := h.deps.Feedback.Update(r.Context(), id, set); err != nil {
		if errors.Is(err, feedback.ErrNotFound) {
			http.Error(w, "no such report", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"updated": true})
}

func (h *Handlers) deleteFeedback(w http.ResponseWriter, r *http.Request) {
	id, err := bson.ObjectIDFromHex(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid report id", http.StatusBadRequest)
		return
	}
	if err := h.deps.Feedback.Delete(r.Context(), id); err != nil {
		if errors.Is(err, feedback.ErrNotFound) {
			http.Error(w, "no such report", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"deleted": true})
}

func (h *Handlers) revokeSessions(w http.ResponseWriter, r *http.Request) {
	target, ok := h.target(w, r)
	if !ok {
		return
	}
	revoked, err := h.deps.Sessions.DeleteByUserID(r.Context(), target.ID.Hex())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"revokedSessions": revoked})
}
