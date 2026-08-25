package stats

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/auth"
)

type Handlers struct {
	repo Repository
}

func NewHandlers(repo Repository) *Handlers { return &Handlers{repo: repo} }

func (h *Handlers) RegisterRoutes(r chi.Router) {
	r.Get("/leaderboard", h.leaderboard)
	r.Get("/stats/ai", h.aiStats)
	// Keyed on a *game* id, and it lives under /games for that reason: the
	// module runtime owns /matches/{id} (see internal/match), and chi keys a
	// path segment on its position, not on the placeholder's name — so the
	// two registrations were the same route, and whichever registered last
	// silently swallowed the other. This one is a recorded result for a
	// a match, so it belongs here rather than on the runtime.
	r.Get("/matches/{matchId}/result", h.matchResult)
	r.With(auth.AuthMiddleware).Get("/users/me/stats", h.meStats)
	r.With(auth.AuthMiddleware).Get("/users/me/matches", h.meMatches)
	// history is the older name for the same list, kept so shipped clients
	// keep working; both return the same body.
	r.With(auth.AuthMiddleware).Get("/users/me/history", h.meMatches)
	r.With(auth.AuthMiddleware).Get("/users/me/head-to-head", h.meHeadToHead)
}

// requireUserSubject resolves the caller to a durable subject. Guests are
// rejected: they have no lifetime record by design, and inventing one keyed on
// a claimable display name would merge strangers' histories.
func (h *Handlers) requireUserSubject(w http.ResponseWriter, req *http.Request) (Subject, bool) {
	uc, ok := auth.GetUserContext(req)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return Subject{}, false
	}
	if uc.IsGuest {
		http.Error(w, "forbidden for guests", http.StatusForbidden)
		return Subject{}, false
	}
	return Subject{Kind: SubjectUser, ID: uc.UserID, Name: uc.Username}, true
}

// statsResponse is the lifetime record as clients consume it: every tally
// expanded with its derived rates.
//
// The four flat counters at the top are the same names the previous
// statistics endpoint returned, kept so existing clients that read
// gamesPlayed/gamesWon off this response keep rendering — they now show real
// numbers rather than the permanent zeroes of the old, never-written record.
type statsResponse struct {
	GamesPlayed int `json:"gamesPlayed"`
	GamesWon    int `json:"gamesWon"`
	GamesLost   int `json:"gamesLost"`
	GamesDrawn  int `json:"gamesDrawn"`

	Subject Subject   `json:"subject"`
	Overall TallyView `json:"overall"`
	// VsHumans and VsAI overlap: a mixed table counts in both. See
	// PlayerStats for why that is the useful split rather than a partition.
	VsHumans TallyView `json:"vsHumans"`
	VsAI     TallyView `json:"vsAI"`

	ByAIDifficulty map[string]TallyView `json:"byAIDifficulty"`
	ByModule       map[string]TallyView `json:"byModule"`
	ByPlayerCount  map[string]TallyView `json:"byPlayerCount"`

	CurrentStreak     int `json:"currentStreak"`
	LongestWinStreak  int `json:"longestWinStreak"`
	LongestLossStreak int `json:"longestLossStreak"`

	FirstMatchAt  *time.Time `json:"firstMatchAt,omitempty"`
	LastMatchAt   *time.Time `json:"lastMatchAt,omitempty"`
	RecentMatches []MatchRef `json:"recentMatches"`
}

func buildStatsResponse(ps PlayerStats) statsResponse {
	out := statsResponse{
		GamesPlayed:       ps.Overall.Matches,
		GamesWon:          ps.Overall.Wins,
		GamesLost:         ps.Overall.Losses,
		GamesDrawn:        ps.Overall.Draws,
		Subject:           ps.Subject,
		Overall:           ps.Overall.View(),
		VsHumans:          ps.VsHumans.View(),
		VsAI:              ps.VsAI.View(),
		ByAIDifficulty:    viewMap(ps.ByAIDifficulty),
		ByModule:          viewMap(ps.ByModule),
		ByPlayerCount:     viewMap(ps.ByPlayerCount),
		CurrentStreak:     ps.CurrentStreak,
		LongestWinStreak:  ps.LongestWinStreak,
		LongestLossStreak: ps.LongestLossStreak,
		RecentMatches:     ps.RecentMatches,
	}
	if out.RecentMatches == nil {
		out.RecentMatches = []MatchRef{}
	}
	if !ps.FirstMatchAt.IsZero() {
		t := ps.FirstMatchAt
		out.FirstMatchAt = &t
	}
	if !ps.LastMatchAt.IsZero() {
		t := ps.LastMatchAt
		out.LastMatchAt = &t
	}
	return out
}

func viewMap(m map[string]Tally) map[string]TallyView {
	out := map[string]TallyView{}
	for k, v := range m {
		out[k] = v.View()
	}
	return out
}

func (h *Handlers) meStats(w http.ResponseWriter, req *http.Request) {
	subject, ok := h.requireUserSubject(w, req)
	if !ok {
		return
	}
	ps, err := h.repo.FindPlayerStats(req.Context(), subject.Key())
	if errors.Is(err, ErrNotFound) {
		// A player with no finished matches gets a zeroed record rather than
		// a 404, so the profile screen has one shape to render.
		ps = ZeroStats(subject)
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, buildStatsResponse(ps))
}

func (h *Handlers) meMatches(w http.ResponseWriter, req *http.Request) {
	subject, ok := h.requireUserSubject(w, req)
	if !ok {
		return
	}
	q := req.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))

	var before time.Time
	if s := q.Get("before"); s != "" {
		t, err := time.Parse(time.RFC3339Nano, s)
		if err != nil {
			http.Error(w, "before must be an RFC3339 timestamp", http.StatusBadRequest)
			return
		}
		before = t
	}

	matches, err := h.repo.ListMatchesForSubject(req.Context(), subject.Key(), before, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// nextBefore is the cursor for the following page; absent on the last
	// page so a client knows to stop without an extra empty request.
	var nextBefore *time.Time
	if len(matches) > 0 {
		t := matches[len(matches)-1].CompletedAt
		nextBefore = &t
	}

	writeJSON(w, map[string]any{
		"matches": matches,
		// matchHistory is the key the previous endpoint used; kept as an
		// alias so shipped clients do not break on the rename.
		"matchHistory": matches,
		"nextBefore":   nextBefore,
	})
}

// headToHeadRow is one opponent record, flattened for rendering.
type headToHeadRow struct {
	HeadToHead
	// AheadRate is the share of shared matches finished ahead of them.
	AheadRate float64 `json:"aheadRate"`
	// ScoreMargin is negative when this player scores fewer penalty points
	// than the opponent — which, in a low-score-wins game, is the good side.
	ScoreMargin int `json:"pointsMargin"`
}

func (h *Handlers) meHeadToHead(w http.ResponseWriter, req *http.Request) {
	subject, ok := h.requireUserSubject(w, req)
	if !ok {
		return
	}
	ps, err := h.repo.FindPlayerStats(req.Context(), subject.Key())
	if errors.Is(err, ErrNotFound) {
		ps = ZeroStats(subject)
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	rows := make([]headToHeadRow, 0, len(ps.HeadToHead))
	for _, rec := range ps.HeadToHead {
		row := headToHeadRow{
			HeadToHead:  rec,
			ScoreMargin: rec.ScoreFor - rec.ScoreAgainst,
		}
		if rec.Matches > 0 {
			row.AheadRate = float64(rec.Ahead) / float64(rec.Matches)
		}
		rows = append(rows, row)
	}
	// Most-played first: the opponents a player cares about are the ones they
	// actually face, not whoever sorts first alphabetically.
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Matches != rows[j].Matches {
			return rows[i].Matches > rows[j].Matches
		}
		return rows[i].Subject.Key() < rows[j].Subject.Key()
	})

	writeJSON(w, map[string]any{"opponents": rows})
}

func (h *Handlers) leaderboard(w http.ResponseWriter, req *http.Request) {
	q := req.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	minMatches, _ := strconv.Atoi(q.Get("minMatches"))

	kind := SubjectKind(q.Get("kind"))
	switch kind {
	case "", SubjectUser, SubjectAI:
	default:
		http.Error(w, "kind must be user or ai", http.StatusBadRequest)
		return
	}

	rows, err := h.repo.Leaderboard(req.Context(), LeaderboardQuery{
		Kind:       kind,
		Scope:      q.Get("scope"),
		MinMatches: minMatches,
		Limit:      limit,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{
		"scope":   scopeName(q.Get("scope")),
		"kind":    string(kindOrDefault(kind)),
		"entries": rows,
	})
}

// aiStats reports the bots' own lifetime records, difficulty by difficulty.
// Publishing it is the honest counterpart to counting matches against bots in
// a player's record: if a bot's results shape your statistics, you can see how
// that bot actually performs.
func (h *Handlers) aiStats(w http.ResponseWriter, req *http.Request) {
	rows, err := h.repo.Leaderboard(req.Context(), LeaderboardQuery{
		Kind:  SubjectAI,
		Scope: req.URL.Query().Get("scope"),
		Limit: 100,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Difficulty order, not rank order — a client rendering an easy/medium/hard
	// table wants them in that order even though hard presumably wins most.
	sort.SliceStable(rows, func(i, j int) bool {
		return difficultyOrder(rows[i].Subject.ID) < difficultyOrder(rows[j].Subject.ID)
	})
	writeJSON(w, map[string]any{"difficulties": rows})
}

func difficultyOrder(d string) int {
	switch d {
	case "easy":
		return 0
	case "medium":
		return 1
	case "hard":
		return 2
	default:
		return 3
	}
}

func (h *Handlers) matchResult(w http.ResponseWriter, req *http.Request) {
	oid, err := bson.ObjectIDFromHex(chi.URLParam(req, "matchId"))
	if err != nil {
		http.Error(w, "invalid match id", http.StatusBadRequest)
		return
	}
	m, err := h.repo.FindMatchByMatchID(req.Context(), oid)
	if errors.Is(err, ErrNotFound) {
		// Either the match is still running or it predates recording. The
		// live standings on match_state answer for both.
		http.Error(w, "no recorded result for this match", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, m)
}

func scopeName(s string) string {
	switch s {
	case "vs_humans", "vs_ai":
		return s
	default:
		return "overall"
	}
}

func kindOrDefault(k SubjectKind) SubjectKind {
	if k == "" {
		return SubjectUser
	}
	return k
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
