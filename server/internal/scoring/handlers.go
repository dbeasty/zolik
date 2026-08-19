package scoring

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"time"

	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"zolik/server/internal/db"
)

type Handlers struct {
	dbm *db.Mongo
}

// NewHandlers creates scoring session REST handlers.
func NewHandlers(m *db.Mongo) *Handlers {
	return &Handlers{dbm: m}
}

type createReq struct {
	Players []string `json:"players"`
}

type patchReq struct {
	Round int            `json:"round"`  // 1..7
	Scores map[string]int `json:"scores,omitempty"`
	ScoresArr []int       `json:"scoresArr,omitempty"`
}

type playerOut struct {
	Name       string `json:"name"`
	Scores     []int  `json:"scores"`
	Total      int     `json:"total"`
	RoundsWon  int     `json:"roundsWon"`
}

type getResp struct {
	ID        bson.ObjectID `json:"id"`
	Players   []playerOut        `json:"players"`
	Winner    *string            `json:"winner,omitempty"`
	CreatedAt time.Time          `json:"createdAt"`
	UpdatedAt time.Time          `json:"updatedAt"`
}

func (h *Handlers) RegisterRoutes(r chi.Router) {
	r.Post("/scoring-sessions", h.create)
	r.Get("/scoring-sessions/{id}", h.get)
	r.Patch("/scoring-sessions/{id}", h.patch)
	r.Get("/scoring-sessions/{id}/export", h.export)
}

func (h *Handlers) create(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	var body createReq
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if len(body.Players) < 2 || len(body.Players) > 8 {
		http.Error(w, "players must be 2..8", http.StatusBadRequest)
		return
	}
	now := time.Now().UTC()

	players := make([]PlayerScore, 0, len(body.Players))
	for _, name := range body.Players {
		players = append(players, PlayerScore{
			Name:   name,
			Scores: make([]int, 7),
		})
	}

	s := ScoringSession{
		ID:        bson.NewObjectID(),
		OwnerUser: "",
		Players:  players,
		Rounds:    7,
		CreatedAt: now,
		UpdatedAt: now,
	}

	coll := h.dbm.Collections().Scoring
	if _, err := coll.InsertOne(ctx, s); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]any{"sessionId": s.ID.Hex()})
}

func (h *Handlers) get(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	idStr := chi.URLParam(req, "id")
	oid, err := bson.ObjectIDFromHex(idStr)
	if err != nil {
		http.Error(w, "invalid session id", http.StatusBadRequest)
		return
	}

	var s ScoringSession
	coll := h.dbm.Collections().Scoring
	if err := coll.FindOne(ctx, bson.M{"_id": oid}).Decode(&s); err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	_ = json.NewEncoder(w).Encode(buildGetResp(s))
}

func (h *Handlers) patch(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	idStr := chi.URLParam(req, "id")
	oid, err := bson.ObjectIDFromHex(idStr)
	if err != nil {
		http.Error(w, "invalid session id", http.StatusBadRequest)
		return
	}

	var body patchReq
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if body.Round < 1 || body.Round > 7 {
		http.Error(w, "round must be 1..7", http.StatusBadRequest)
		return
	}
	idx := body.Round - 1

	coll := h.dbm.Collections().Scoring
	var s ScoringSession
	if err := coll.FindOne(ctx, bson.M{"_id": oid}).Decode(&s); err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	if len(body.ScoresArr) > 0 {
		// scoresArr aligns with players order.
		for i := 0; i < len(s.Players) && i < len(body.ScoresArr); i++ {
			s.Players[i].Scores[idx] = body.ScoresArr[i]
		}
	} else {
		for i := range s.Players {
			if v, ok := body.Scores[s.Players[i].Name]; ok {
				s.Players[i].Scores[idx] = v
			}
		}
	}

	s.UpdatedAt = time.Now().UTC()
	_, err = coll.ReplaceOne(ctx, bson.M{"_id": oid}, s, options.Replace().SetUpsert(false))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_ = json.NewEncoder(w).Encode(buildGetResp(s))
}

func (h *Handlers) export(w http.ResponseWriter, req *http.Request) {
	// v1 stub: return JSON that a client can share/print.
	ctx := req.Context()
	idStr := chi.URLParam(req, "id")
	oid, err := bson.ObjectIDFromHex(idStr)
	if err != nil {
		http.Error(w, "invalid session id", http.StatusBadRequest)
		return
	}
	var s ScoringSession
	coll := h.dbm.Collections().Scoring
	if err := coll.FindOne(ctx, bson.M{"_id": oid}).Decode(&s); err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	resp := buildGetResp(s)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"type": "export_stub",
		"data": resp,
	})
	_ = context.Background()
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
		Players:   playerTotals,
		Winner:    winner,
		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
	}
}

