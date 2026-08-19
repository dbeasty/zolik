package game

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/auth"
	"zolik/server/internal/ai"
	"zolik/server/internal/rules"
	"zolik/server/internal/models"
)

type GameRestHandlers struct {
	repo     *Repository
	hub      *Hub
	manager  *Manager
}

func NewGameRestHandlers(repo *Repository, hub *Hub, manager *Manager) *GameRestHandlers {
	return &GameRestHandlers{repo: repo, hub: hub, manager: manager}
}

type CreateGameReq struct {
	InitialMeldMinimum  *int `json:"initialMeldMinimum,omitempty"`
	DiscardDrawMinRound *int `json:"discardDrawMinRound,omitempty"`
}

type AddAIReq struct {
	Difficulty string `json:"difficulty"`
}

func (h *GameRestHandlers) RegisterRoutes(r chi.Router) {
	r.Get("/games/{id}", h.getGame)
	r.With(auth.AuthMiddleware).Post("/games", h.createGame)
	r.With(auth.AuthMiddleware).Post("/games/{id}/join", h.joinGame)
	r.With(auth.AuthMiddleware).Patch("/games/{id}/settings", h.updateSettings)
	r.With(auth.AuthMiddleware).Post("/games/{id}/start", h.startGame)
	r.With(auth.AuthMiddleware).Post("/games/{id}/add-ai", h.addAI)
	r.With(auth.AuthMiddleware).Get("/games/{id}/replay", h.replayGame)
}

// updateSettings lets the host adjust pre-start options (initial meld
// minimum, discard-pickup round gate) while the game is still in the lobby.
func (h *GameRestHandlers) updateSettings(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	uc, ok := auth.GetUserContext(req)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	idOrJoin := chi.URLParam(req, "id")

	g, _, err := h.repo.ParseGameIDOrJoin(ctx, idOrJoin)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if g.Status != "lobby" {
		http.Error(w, "game not in lobby", http.StatusBadRequest)
		return
	}
	if g.HostID != uc.UserID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var body CreateGameReq
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if body.InitialMeldMinimum != nil {
		g.InitialMeldMinimum = *body.InitialMeldMinimum
	}
	if body.DiscardDrawMinRound != nil {
		g.DiscardDrawMinRound = *body.DiscardDrawMinRound
	}

	if err := h.repo.UpdateWithVersion(ctx, g.ID, g.Version, g); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"initialMeldMinimum":  g.InitialMeldMinimum,
		"discardDrawMinRound": g.DiscardDrawMinRound,
	})
}

func (h *GameRestHandlers) createGame(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	uc, ok := auth.GetUserContext(req)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var body CreateGameReq
	_ = json.NewDecoder(req.Body).Decode(&body)

	initial := 35
	if body.InitialMeldMinimum != nil {
		initial = *body.InitialMeldMinimum
	}
	// Continental Rummy: discard-pile pickup only opens up from round 3.
	discardMinRound := 3
	if body.DiscardDrawMinRound != nil {
		discardMinRound = *body.DiscardDrawMinRound
	}

	gameID := bson.NewObjectID()
	joinCode := randomJoinCode(6)

	// Create lobby with host as first player.
	p := models.Player{
		ID:   uc.UserID,
		Name: uc.Username,
		IsAI: false,
		UserID: func() string {
			if uc.IsGuest {
				return ""
			}
			return uc.UserID
		}(),
	}

	g := models.Game{
		ID:                gameID,
		Status:           "lobby",
		Round:            0,
		Phase:            "",
		JoinCode:         joinCode,
		HostID:           uc.UserID,
		CurrentTurn:      "",
		TurnOrder:        []string{p.ID},
		ReshuffleCount:   0,
		Hands:            map[string][]string{},
		Melds:            map[string][][]string{},
		MeldMeta:         map[string][]models.MeldInfo{},
		RoundReqMet:      map[string]bool{p.ID: false},
		InitialMeldMinimum: initial,
		DiscardDrawMinRound: discardMinRound,
		Players:          []models.Player{p},
		ActionLog:        []models.Action{},
		DeckSeed:         rules.NewShuffleSeed(),
		Version:          1,
		CreatedAt:        time.Now().UTC(),
		TotalScores:      map[string]int{p.ID: 0},
		RoundScores:     map[string][]int{p.ID: {}},
		NextMeldSeq:       0,
	}

	// Insert lobby into Mongo.
	created, err := h.repo.Insert(ctx, g)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = created

	_ = json.NewEncoder(w).Encode(map[string]any{
		"gameId":   gameID.Hex(),
		"joinCode": joinCode,
	})
}

func (h *GameRestHandlers) getGame(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	idOrJoin := chi.URLParam(req, "id")

	g, _, err := h.repo.ParseGameIDOrJoin(ctx, idOrJoin)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	type playerPublic struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		IsAI  bool   `json:"isAI"`
	}
	players := make([]playerPublic, 0, len(g.Players))
	for _, p := range g.Players {
		players = append(players, playerPublic{ID: p.ID, Name: p.Name, IsAI: p.IsAI})
	}

	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":                  g.ID.Hex(),
		"status":              g.Status,
		"round":               g.Round,
		"phase":               g.Phase,
		"currentTurn":         g.CurrentTurn,
		"players":             players,
		"hostId":              g.HostID,
		"initialMeldMinimum":  g.InitialMeldMinimum,
		"discardDrawMinRound": g.DiscardDrawMinRound,
		"discardPileTop": func() any {
			if len(g.DiscardPile) == 0 {
				return nil
			}
			return g.DiscardPile[len(g.DiscardPile)-1]
		}(),
	})
}

func (h *GameRestHandlers) joinGame(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	uc, ok := auth.GetUserContext(req)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	idOrJoin := chi.URLParam(req, "id")

	g, gameIDHex, err := h.repo.ParseGameIDOrJoin(ctx, idOrJoin)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if g.Status != "lobby" {
		http.Error(w, "game not in lobby", http.StatusBadRequest)
		return
	}

	for _, p := range g.Players {
		if p.ID == uc.UserID {
			_ = json.NewEncoder(w).Encode(map[string]any{"gameId": gameIDHex, "alreadyJoined": true})
			return
		}
	}
	if len(g.Players) >= 8 {
		http.Error(w, "lobby full", http.StatusBadRequest)
		return
	}

	p := models.Player{
		ID:    uc.UserID,
		Name:  uc.Username,
		IsAI:  false,
		UserID: func() string {
			if uc.IsGuest {
				return ""
			}
			return uc.UserID
		}(),
	}

	g.Players = append(g.Players, p)
	g.TurnOrder = append(g.TurnOrder, p.ID)
	if g.RoundReqMet == nil {
		g.RoundReqMet = map[string]bool{}
	}
	g.RoundReqMet[p.ID] = false
	if g.Hands == nil {
		g.Hands = map[string][]string{}
	}
	if g.Melds == nil {
		g.Melds = map[string][][]string{}
	}

	if err := h.repo.UpdateWithVersion(ctx, g.ID, g.Version, g); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]any{"gameId": gameIDHex, "joined": true, "playerCount": len(g.Players)})
}

func (h *GameRestHandlers) startGame(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	uc, ok := auth.GetUserContext(req)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	idOrJoin := chi.URLParam(req, "id")

	g, gameIDHex, err := h.repo.ParseGameIDOrJoin(ctx, idOrJoin)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	if g.Status != "lobby" {
		http.Error(w, "game not in lobby", http.StatusBadRequest)
		return
	}
	if g.HostID != uc.UserID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if len(g.Players) < 2 {
		http.Error(w, "need at least 2 players", http.StatusBadRequest)
		return
	}

	turnOrder := make([]string, 0, len(g.Players))
	for _, p := range g.Players {
		turnOrder = append(turnOrder, p.ID)
	}

	seed := g.DeckSeed
	if seed == 0 {
		seed = rules.NewShuffleSeed()
		g.DeckSeed = seed
	}

	// Create initial round state.
	rState := rules.GameState{
		Status:              rules.StatusActive,
		Round:               1,
		Phase:               rules.PhaseDraw,
		Created:             time.Now().UTC(),
		CurrentTurn:         turnOrder[0],
		TurnOrder:           turnOrder,
		DrawPile:            nil,
		DiscardPile:         nil,
		ReshuffleCount:      0,
		Hands:               map[string][]string{},
		Melds:               map[string][][]string{},
		MeldMeta:            map[string][]rules.MeldInfo{},
		RoundReqMet:         map[string]bool{},
		InitialMeldMinimum: g.InitialMeldMinimum,
		DiscardDrawMinRound: g.DiscardDrawMinRound,
		DeckSeed:            seed,
		RoundScores:         map[string][]int{},
		TotalScores:         map[string]int{},
		NextMeldSeq:         0,
	}

	deck := rules.BuildDeck(len(turnOrder))
	rState.DrawPile = rules.Shuffle(deck, seed+int64(rState.Round)*9973)
	var err2 error
	rState, err2 = rules.Deal12(rState)
	if err2 != nil {
		http.Error(w, err2.Error(), http.StatusInternalServerError)
		return
	}
	if len(rState.DrawPile) == 0 {
		http.Error(w, "no cards for discard init", http.StatusInternalServerError)
		return
	}
	top := rState.DrawPile[len(rState.DrawPile)-1]
	rState.DrawPile = rState.DrawPile[:len(rState.DrawPile)-1]
	rState.DiscardPile = []string{top}
	// Initialize per-player meld/req maps.
	for _, pid := range turnOrder {
		rState.RoundReqMet[pid] = false
		rState.Melds[pid] = nil
		rState.RoundScores[pid] = []int{}
		rState.TotalScores[pid] = 0
	}

	// Persist into lobby game document.
	nextGame := g
	nextGame.Status = string(rState.Status)
	nextGame.Round = rState.Round
	nextGame.Phase = string(rState.Phase)
	nextGame.CurrentTurn = rState.CurrentTurn
	nextGame.TurnOrder = rState.TurnOrder
	nextGame.DrawPile = rState.DrawPile
	nextGame.DiscardPile = rState.DiscardPile
	nextGame.ReshuffleCount = rState.ReshuffleCount
	nextGame.Hands = rState.Hands
	nextGame.Melds = rState.Melds
	// Convert meld meta
	nextGame.MeldMeta = map[string][]models.MeldInfo{}
	for owner, metas := range rState.MeldMeta {
		for _, mi := range metas {
			nextGame.MeldMeta[owner] = append(nextGame.MeldMeta[owner], models.MeldInfo{
				MeldID:  mi.MeldID,
				Type:    string(mi.Type),
				OwnerID: mi.OwnerID,
			})
		}
	}
	nextGame.RoundReqMet = rState.RoundReqMet
	nextGame.InitialMeldMinimum = rState.InitialMeldMinimum
	nextGame.DiscardDrawMinRound = rState.DiscardDrawMinRound
	nextGame.MeldsLaidThisTurn = rState.MeldsLaidThisTurn
	nextGame.DiscardDrawnCardPendingMeld = rState.DiscardDrawnCardPendingMeld
	nextGame.RoundScores = rState.RoundScores
	nextGame.TotalScores = rState.TotalScores
	nextGame.DeckSeed = rState.DeckSeed
	nextGame.NextMeldSeq = rState.NextMeldSeq
	nextGame.Version = g.Version

	if err := h.repo.UpdateWithVersion(ctx, g.ID, g.Version, nextGame); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	h.hub.BroadcastGameState(gameIDHex, BroadcastRecipients(nextGame), func(pid string) interface{} {
		return BuildGameStateMsg(nextGame, pid)
	})

	// Trigger AI immediately if the first actor is an AI.
	if h.manager != nil {
		h.manager.RunAIIfNeeded(ctx, gameIDHex)
	}

	_ = json.NewEncoder(w).Encode(map[string]any{"gameId": gameIDHex, "started": true})
}

func (h *GameRestHandlers) replayGame(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	uc, ok := auth.GetUserContext(req)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	idOrJoin := chi.URLParam(req, "id")

	g, _, err := h.repo.ParseGameIDOrJoin(ctx, idOrJoin)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	allowed := false
	for _, p := range g.Players {
		if p.ID == uc.UserID {
			allowed = true
			break
		}
	}
	if !allowed {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	resp := BuildReplayResponse(g, uc.UserID)
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *GameRestHandlers) addAI(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	uc, ok := auth.GetUserContext(req)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	idOrJoin := chi.URLParam(req, "id")

	g, gameIDHex, err := h.repo.ParseGameIDOrJoin(ctx, idOrJoin)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if g.Status != "lobby" {
		http.Error(w, "game not in lobby", http.StatusBadRequest)
		return
	}
	if g.HostID != uc.UserID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if len(g.Players) >= 8 {
		http.Error(w, "lobby full", http.StatusBadRequest)
		return
	}

	var body AddAIReq
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	diff := body.Difficulty
	if diff != "easy" && diff != "medium" && diff != "hard" {
		diff = "medium"
	}

	aiCount := 0
	for _, p := range g.Players {
		if p.IsAI && p.AIDifficulty == diff {
			aiCount++
		}
	}
	names := ai.AINames[diff]
	name := names[aiCount%len(names)]

	aid := fmt.Sprintf("ai:%s:%d:%d", diff, time.Now().UnixNano(), aiCount)
	p := models.Player{
		ID:           aid,
		Name:         name,
		IsAI:         true,
		AIDifficulty: diff,
		UserID:       "",
	}

	g.Players = append(g.Players, p)
	g.TurnOrder = append(g.TurnOrder, p.ID)

	if g.RoundReqMet == nil {
		g.RoundReqMet = map[string]bool{}
	}
	g.RoundReqMet[p.ID] = false
	if g.Hands == nil {
		g.Hands = map[string][]string{}
	}

	if err := h.repo.UpdateWithVersion(ctx, g.ID, g.Version, g); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]any{
		"gameId":   gameIDHex,
		"addedAI":  true,
		"playerId": p.ID,
	})
}

func randomJoinCode(n int) string {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	max := big.NewInt(int64(len(alphabet)))
	out := make([]byte, 0, n)
	for i := 0; i < n; i++ {
		v, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "JOINERR"
		}
		out = append(out, alphabet[v.Int64()])
	}
	return string(out)
}

