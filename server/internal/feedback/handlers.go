package feedback

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"zolik/server/internal/auth"
	"zolik/server/internal/ratelimit"
)

// perReporterWindow and perReporterLimit throttle submissions.
//
// The endpoint is reachable by guests, which is deliberate — most players
// never sign in, and the reports worth having come from exactly those people.
// That openness is also why it needs a ceiling: without one, a stuck client
// retrying in a loop would fill the collection on its own, no malice required.
const (
	perReporterWindow = time.Hour
	perReporterLimit  = 10
)

// tooManyReports is worded the same however the ceiling was reached, so the
// response never reveals which of the two throttles a caller tripped.
const tooManyReports = "you have sent a lot of reports just now — please try again later"

type Handlers struct {
	repo Repository
	// anon covers reports that arrive with no session, which the repository's
	// per-reporter count cannot see.
	anon *ratelimit.Limiter
}

func NewHandlers(repo Repository) *Handlers {
	return &Handlers{repo: repo, anon: ratelimit.New(perReporterWindow, perReporterLimit)}
}

func (h *Handlers) RegisterRoutes(r chi.Router) {
	// Optional auth, not required: a guest hitting a bug is the commonest
	// reporter there is, and demanding an account first would lose the
	// reports most worth reading. Whatever session *is* presented gets
	// recorded, so a signed-in report can be traced back.
	r.With(auth.OptionalAuthMiddleware).Post("/feedback", h.submit)
}

type submitReq struct {
	Kind         string `json:"kind"`
	Message      string `json:"message"`
	ContactEmail string `json:"contactEmail"`
	AppVersion   string `json:"appVersion"`
	Platform     string `json:"platform"`
	MatchID      string `json:"matchId"`
}

func (h *Handlers) submit(w http.ResponseWriter, req *http.Request) {
	var body submitReq
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	message := strings.TrimSpace(body.Message)
	if message == "" {
		http.Error(w, "a message is required", http.StatusBadRequest)
		return
	}
	if len([]rune(message)) > MaxMessageLen {
		http.Error(w, "that message is too long", http.StatusBadRequest)
		return
	}

	kind := strings.TrimSpace(body.Kind)
	if kind == "" {
		kind = KindOther
	}
	if !ValidKind(kind) {
		http.Error(w, "unknown kind", http.StatusBadRequest)
		return
	}

	report := Report{
		Kind:    kind,
		Message: message,
		// Context is clipped rather than rejected: a client sending an
		// over-long platform string is a client bug, and losing the whole
		// report over it would be the wrong trade.
		ContactEmail: clip(body.ContactEmail, maxContextLen),
		AppVersion:   clip(body.AppVersion, maxContextLen),
		Platform:     clip(body.Platform, maxContextLen),
		MatchID:      clip(body.MatchID, maxContextLen),
		Status:       StatusNew,
	}

	if uc, ok := auth.GetUserContext(req); ok {
		report.Username = uc.Username
		report.UserID = uc.PlayerUserID()
		report.GuestID = uc.PlayerGuestID()
	}

	ctx := req.Context()
	now := time.Now().UTC()

	// Two throttles, because neither covers the other's case. A signed-in or
	// guest reporter is counted in the database, which holds across restarts
	// and instances. A reporter with no session at all has nothing to count,
	// so those fall back to an in-process ceiling keyed by address.
	if report.UserID == "" && report.GuestID == "" {
		if !h.anon.Allow(ratelimit.ClientIP(req), now) {
			http.Error(w, tooManyReports, http.StatusTooManyRequests)
			return
		}
	} else {
		recent, err := h.repo.CountRecentFrom(ctx, report.UserID, report.GuestID, now.Add(-perReporterWindow))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if recent >= perReporterLimit {
			http.Error(w, tooManyReports, http.StatusTooManyRequests)
			return
		}
	}

	saved, err := h.repo.Insert(ctx, report)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	// The id and nothing else: the reporter has no way to read a report back,
	// so echoing its contents would only be a way to confirm what was stored.
	_ = json.NewEncoder(w).Encode(map[string]any{"id": saved.ID.Hex()})
}
