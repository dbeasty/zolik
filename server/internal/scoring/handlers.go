package scoring

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sort"
	"time"

	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/auth"
)

// maxBody bounds what a caller can make the server read and store: a
// scorepad of eight short names and seven rounds is well under 2 KiB.
const maxBody = 16 << 10

// maxScore bounds one round's score. Real scores are tens or hundreds; the cap
// exists so that adding seven of them can never overflow and make the lowest
// total the highest.
const maxScore = 1_000_000

func clampScore(v int) int {
	if v > maxScore {
		return maxScore
	}
	if v < -maxScore {
		return -maxScore
	}
	return v
}

type Handlers struct {
	repo Repository
}

// NewHandlers creates scoring session REST handlers.
func NewHandlers(repo Repository) *Handlers {
	return &Handlers{repo: repo}
}

type createReq struct {
	Players []string `json:"players"`
}

type patchReq struct {
	Round int `json:"round"` // 1..7
	// Version, when sent, is the version the client last read; a patch
	// against an older one is refused with 409 rather than undoing a newer
	// write. Optional, because a client that sends none still gets the
	// server-side check between its own read and write.
	Version   *int           `json:"version,omitempty"`
	Scores    map[string]int `json:"scores,omitempty"`
	ScoresArr []int          `json:"scoresArr,omitempty"`
}

type playerOut struct {
	Name      string `json:"name"`
	Scores    []int  `json:"scores"`
	Total     int    `json:"total"`
	RoundsWon int    `json:"roundsWon"`
}

type getResp struct {
	ID        bson.ObjectID `json:"id"`
	Version   int           `json:"version"`
	Players   []playerOut   `json:"players"`
	Winner    *string       `json:"winner,omitempty"`
	CreatedAt time.Time     `json:"createdAt"`
	UpdatedAt time.Time     `json:"updatedAt"`
}

// RegisterRoutes mounts the scorepad. Every route needs an account: a pad
// belongs to whoever made it, nobody else can read or write it (the ids are
// time-ordered and guessable, so an id is no secret), and deleting the
// account deletes its pads. A guest has nowhere for one to be kept, which is
// what the clients already tell them.
func (h *Handlers) RegisterRoutes(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(auth.AuthMiddleware)
		r.Post("/scoring-sessions", h.create)
		r.Get("/scoring-sessions/{id}", h.get)
		r.Patch("/scoring-sessions/{id}", h.patch)
		r.Get("/scoring-sessions/{id}/export", h.export)
	})
}

// owner is the signed-in account a request acts for, or "" (having answered)
// for a guest.
func owner(w http.ResponseWriter, req *http.Request) string {
	uc, ok := auth.GetUserContext(req)
	if !ok || uc.IsGuest || uc.UserID == "" {
		http.Error(w, "sign in with an account to keep a score table", http.StatusForbidden)
		return ""
	}
	return uc.UserID
}

// load reads the pad named in the URL for its owner. Somebody else's pad is
// answered exactly like a missing one.
func (h *Handlers) load(w http.ResponseWriter, req *http.Request) (ScoringSession, bool) {
	userID := owner(w, req)
	if userID == "" {
		return ScoringSession{}, false
	}
	oid, err := bson.ObjectIDFromHex(chi.URLParam(req, "id"))
	if err != nil {
		http.Error(w, "invalid session id", http.StatusBadRequest)
		return ScoringSession{}, false
	}
	s, err := h.repo.FindByID(req.Context(), oid)
	if err != nil || s.OwnerUser != userID {
		http.Error(w, "not found", http.StatusNotFound)
		return ScoringSession{}, false
	}
	return s, true
}

func (h *Handlers) create(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	userID := owner(w, req)
	if userID == "" {
		return
	}
	var body createReq
	if err := json.NewDecoder(http.MaxBytesReader(w, req.Body, maxBody)).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if len(body.Players) < 2 || len(body.Players) > 8 {
		http.Error(w, "players must be 2..8", http.StatusBadRequest)
		return
	}
	now := time.Now().UTC()

	players := make([]PlayerScore, 0, len(body.Players))
	for i, name := range body.Players {
		// A name is drawn on every screen that opens the sheet; see
		// auth.CleanDisplayName for what that rules out.
		name = auth.CleanDisplayName(name, auth.MaxGuestNameRunes)
		if name == "" {
			name = fmt.Sprintf("Player %d", i+1)
		}
		players = append(players, PlayerScore{
			Name:   name,
			Scores: make([]int, 7),
		})
	}

	s := ScoringSession{
		ID:        bson.NewObjectID(),
		OwnerUser: userID,
		Players:   players,
		Rounds:    7,
		Version:   1,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := h.repo.Insert(ctx, s); err != nil {
		log.Printf("scoring: insert: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// "id" is what the clients read; "sessionId" is kept for anything that
	// learned the older name.
	_ = json.NewEncoder(w).Encode(map[string]any{"id": s.ID.Hex(), "sessionId": s.ID.Hex(), "version": s.Version})
}

func (h *Handlers) get(w http.ResponseWriter, req *http.Request) {
	s, ok := h.load(w, req)
	if !ok {
		return
	}
	_ = json.NewEncoder(w).Encode(buildGetResp(s))
}

func (h *Handlers) patch(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	var body patchReq
	if err := json.NewDecoder(http.MaxBytesReader(w, req.Body, maxBody)).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if body.Round < 1 || body.Round > 7 {
		http.Error(w, "round must be 1..7", http.StatusBadRequest)
		return
	}
	idx := body.Round - 1

	s, ok := h.load(w, req)
	if !ok {
		return
	}
	read := s.Version
	if body.Version != nil && *body.Version != read {
		conflict(w, s)
		return
	}

	if len(body.ScoresArr) > 0 {
		// scoresArr aligns with players order.
		for i := 0; i < len(s.Players) && i < len(body.ScoresArr); i++ {
			s.Players[i].Scores[idx] = clampScore(body.ScoresArr[i])
		}
	} else {
		// Keyed by the name the client typed, which create cleaned; so match on
		// the cleaned form of each key too, or "Ann " would silently score nobody.
		byName := make(map[string]int, len(body.Scores))
		for name, v := range body.Scores {
			byName[name] = v
			byName[auth.CleanDisplayName(name, auth.MaxGuestNameRunes)] = v
		}
		for i := range s.Players {
			if v, ok := byName[s.Players[i].Name]; ok {
				s.Players[i].Scores[idx] = clampScore(v)
			}
		}
	}

	s.UpdatedAt = time.Now().UTC()
	s.Version = read + 1
	err := h.repo.Replace(ctx, s, read)
	if errors.Is(err, ErrVersionConflict) {
		if fresh, ferr := h.repo.FindByID(ctx, s.ID); ferr == nil {
			conflict(w, fresh)
			return
		}
	}
	if err != nil {
		log.Printf("scoring: replace: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	_ = json.NewEncoder(w).Encode(buildGetResp(s))
}

// conflict answers a patch made against an old reading, with the pad as it
// now stands so the client can redraw it and re-apply.
func conflict(w http.ResponseWriter, s ScoringSession) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusConflict)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"code":    "SCOREPAD_CHANGED",
		"message": "this score table changed since you opened it",
		"current": buildGetResp(s),
	})
}

func (h *Handlers) export(w http.ResponseWriter, req *http.Request) {
	// v1 stub: return JSON that a client can share/print.
	s, ok := h.load(w, req)
	if !ok {
		return
	}

	resp := buildGetResp(s)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"type": "export_stub",
		"data": resp,
	})
}

func buildGetResp(s ScoringSession) getResp {
	playerTotals := make([]playerOut, 0, len(s.Players))
	for _, p := range s.Players {
		total := 0
		for _, v := range p.Scores {
			total += v
		}
		playerTotals = append(playerTotals, playerOut{
			Name:      p.Name,
			Scores:    append([]int(nil), p.Scores...),
			Total:     total,
			RoundsWon: 0,
		})
	}

	// Compute roundsWon with strict minima.
	for roundIdx := 0; roundIdx < 7; roundIdx++ {
		minScore := int(^uint(0) >> 1) // max int
		for _, po := range playerTotals {
			if po.Scores[roundIdx] < minScore {
				minScore = po.Scores[roundIdx]
			}
		}
		for i := range playerTotals {
			if playerTotals[i].Scores[roundIdx] == minScore {
				// if multiple players tie, nobody "wins" the round for tiebreak purposes
				tie := false
				for j := range playerTotals {
					if j != i && playerTotals[j].Scores[roundIdx] == minScore {
						tie = true
						break
					}
				}
				if !tie {
					playerTotals[i].RoundsWon++
				}
			}
		}
	}

	// Winner: lowest TotalScores.
	minTotal := int(^uint(0) >> 1)
	for _, po := range playerTotals {
		if po.Total < minTotal {
			minTotal = po.Total
		}
	}

	var candidates []playerOut
	for _, po := range playerTotals {
		if po.Total == minTotal {
			candidates = append(candidates, po)
		}
	}

	var winner *string
	if len(candidates) == 1 {
		w := candidates[0].Name
		winner = &w
	} else {
		// tiebreak: fewest rounds won
		sort.Slice(candidates, func(i, j int) bool {
			if candidates[i].RoundsWon != candidates[j].RoundsWon {
				return candidates[i].RoundsWon < candidates[j].RoundsWon
			}
			return candidates[i].Name < candidates[j].Name
		})
		// if still tied on roundsWon, draw (nil winner)
		if candidates[0].RoundsWon != candidates[1].RoundsWon {
			w := candidates[0].Name
			winner = &w
		}
	}

	return getResp{
		ID:        s.ID,
		Version:   s.Version,
		Players:   playerTotals,
		Winner:    winner,
		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
	}
}
