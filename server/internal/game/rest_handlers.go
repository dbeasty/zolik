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

	"zolik/server/internal/ai"
	"zolik/server/internal/auth"
	"zolik/server/internal/models"
	"zolik/server/internal/rules"
	"zolik/server/internal/stats"
)

type GameRestHandlers struct {
	repo    *Repository
	hub     *Hub
	manager *Manager
	// stats is optional; when nil the scoreboard simply omits the lifetime
	// records it would otherwise attach.
	stats *stats.Repository
	// testEndpoints gates debugState — see its doc comment and
	// app.Config.TestEndpointsEnabled.
	testEndpoints bool
}

func NewGameRestHandlers(repo *Repository, hub *Hub, manager *Manager, statsRepo *stats.Repository, testEndpoints bool) *GameRestHandlers {
	return &GameRestHandlers{repo: repo, hub: hub, manager: manager, stats: statsRepo, testEndpoints: testEndpoints}
}

type CreateGameReq struct {
	// RulesProfile selects the base ruleset ("continental" | "zolik_classic").
	// Empty defaults to "zolik_classic". The two fields below, when present,
	// override that profile's defaults — the "custom house rules" path.
	RulesProfile        *string `json:"rulesProfile,omitempty"`
	InitialMeldMinimum  *int    `json:"initialMeldMinimum,omitempty"`
	DiscardDrawMinRound *int    `json:"discardDrawMinRound,omitempty"`
}

type AddAIReq struct {
	Difficulty string `json:"difficulty"`
}

func (h *GameRestHandlers) RegisterRoutes(r chi.Router) {
	r.Get("/rules", h.getRules)
	r.Get("/games/{id}", h.getGame)
	r.Get("/games/{id}/scoreboard", h.scoreboard)
	r.With(auth.AuthMiddleware).Post("/games", h.createGame)
	r.With(auth.AuthMiddleware).Post("/games/{id}/join", h.joinGame)
	r.With(auth.AuthMiddleware).Patch("/games/{id}/settings", h.updateSettings)
	r.With(auth.AuthMiddleware).Post("/games/{id}/start", h.startGame)
	r.With(auth.AuthMiddleware).Post("/games/{id}/add-ai", h.addAI)
	r.With(auth.AuthMiddleware).Get("/games/{id}/replay", h.replayGame)
	if h.testEndpoints {
		r.With(auth.AuthMiddleware).Post("/games/{id}/debug-state", h.debugState)
	}
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
	// Start from the game's current ruleset, or from the newly-chosen
	// profile's defaults when the host switches variation — switching resets
	// every knob, then this same request's explicit overrides go back on top.
	cfg := GameRules(g)
	if body.RulesProfile != nil {
		g.RulesProfile = *body.RulesProfile
		cfg = rules.ResolveProfile(g.RulesProfile)
	}
	cfg = applyRuleOverrides(cfg, body.InitialMeldMinimum, body.DiscardDrawMinRound)
	setGameRules(&g, cfg)

	if err := h.repo.UpdateWithVersion(ctx, g.ID, g.Version, g); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"rulesProfile":        g.RulesProfile,
		"initialMeldMinimum":  cfg.InitialMeldMinimum,
		"discardDrawMinRound": cfg.DiscardDrawMinRound,
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

	profile := "zolik_classic"
	if body.RulesProfile != nil {
		profile = *body.RulesProfile
	}
	// Resolve the ruleset once, here, and freeze it onto the document (see
	// setGameRules) — it is never re-derived from the profile name again.
	cfg := applyRuleOverrides(rules.ResolveProfile(profile), body.InitialMeldMinimum, body.DiscardDrawMinRound)

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
		ID:             gameID,
		Status:         "lobby",
		RulesProfile:   profile,
		GameNumber:     0,
		Phase:          "",
		JoinCode:       joinCode,
		HostID:         uc.UserID,
		CurrentTurn:    "",
		TurnOrder:      []string{p.ID},
		ReshuffleCount: 0,
		Hands:          map[string][]string{},
		Melds:          map[string][][]string{},
		MeldMeta:       map[string][]models.MeldInfo{},
		RoundReqMet:    map[string]bool{p.ID: false},
		Players:        []models.Player{p},
		ActionLog:      []models.Action{},
		DeckSeed:       rules.NewShuffleSeed(),
		Version:        1,
		CreatedAt:      time.Now().UTC(),
		TotalScores:    map[string]int{p.ID: 0},
		GameScores:     map[string][]int{p.ID: {}},
		NextMeldSeq:    0,
	}

	setGameRules(&g, cfg)

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

// getRules exposes the table/lobby constants and options the client would
// otherwise have to hardcode a second copy of (min/max players, the
// selectable initial-meld-minimum and discard-lock-round values, and each
// profile's defaults for those two).
func (h *GameRestHandlers) getRules(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"minPlayers":                 rules.MinPlayers,
		"maxPlayers":                 rules.MaxPlayers,
		"initialMeldMinOptions":      rules.InitialMeldMinOptions,
		"discardDrawMinRoundOptions": rules.DiscardDrawMinRoundOptions,
		"defaultInitialMeldMinimum":  rules.ProfileContinental.InitialMeldMinimum,
		"defaultDiscardDrawMinRound": rules.ProfileContinental.DiscardDrawMinRound,
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
		ID   string `json:"id"`
		Name string `json:"name"`
		IsAI bool   `json:"isAI"`
	}
	players := make([]playerPublic, 0, len(g.Players))
	for _, p := range g.Players {
		players = append(players, playerPublic{ID: p.ID, Name: p.Name, IsAI: p.IsAI})
	}

	// Rule values come from the game's resolved ruleset, not the legacy
	// scalar columns, so the lobby always shows what will actually be enforced.
	cfg := GameRules(g)

	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":                  g.ID.Hex(),
		"status":              g.Status,
		"game":                g.GameNumber,
		"round":               g.Round,
		"phase":               g.Phase,
		"currentTurn":         g.CurrentTurn,
		"players":             players,
		"hostId":              g.HostID,
		"rulesProfile":        g.RulesProfile,
		"initialMeldMinimum":  cfg.InitialMeldMinimum,
		"discardDrawMinRound": cfg.DiscardDrawMinRound,
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
	if len(g.Players) >= rules.MaxPlayers {
		http.Error(w, "lobby full", http.StatusBadRequest)
		return
	}

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
	if len(g.Players) < rules.MinPlayers {
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

	cfg := GameRules(g)

	// One implementation of "shuffle, deal, turn the first discard" for both
	// the opening deal and every later one — this used to be hand-rolled here
	// with a slightly different field set (MeldMeta built without WildCount)
	// and had to be kept in sync with rules.StartNextGame by hand.
	rState, err := rules.StartMatch(cfg, turnOrder, seed)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Persist into lobby game document.
	nextGame := g
	fromRulesState(&nextGame, rState)
	nextGame.Version = g.Version
	nextGame.DealInitialState = captureDealSnapshot(nextGame, 0)

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
	if len(g.Players) >= rules.MaxPlayers {
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

// DebugStateReq is a partial patch applied straight to a game document,
// bypassing rules validation entirely — see debugState.
type DebugStateReq struct {
	Phase       *string             `json:"phase,omitempty"`
	CurrentTurn *string             `json:"currentTurn,omitempty"`
	Hands       map[string][]string `json:"hands,omitempty"`
	// Melds maps playerID -> list of card groups. Each group is validated
	// with the game's own rules profile (same ValidateMeld the real
	// lay_meld action uses) so a seeded table can't silently be nonsense —
	// its MeldInfo (type, meldId) is derived from that validation rather
	// than accepted from the request, so it's always consistent with what
	// ValidateLayOff/ValidateSwapJoker expect to find.
	Melds                       map[string][][]string `json:"melds,omitempty"`
	DiscardPile                 []string              `json:"discardPile,omitempty"`
	RoundReqMet                 map[string]bool       `json:"roundReqMet,omitempty"`
	DiscardDrawnCardPendingMeld *string               `json:"discardDrawnCardPendingMeld,omitempty"`
}

// debugState lets an e2e test put a game into an arbitrary mid-round state
// in one call — specific hands, melds already on the table, whose turn it
// is — instead of driving a full deal turn-by-turn over WebSocket just to
// reach the UI state under test. Only registered at all when
// app.Config.TestEndpointsEnabled is set (see RegisterRoutes); never
// reachable in a deployment that doesn't opt in.
func (h *GameRestHandlers) debugState(w http.ResponseWriter, req *http.Request) {
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
	isPlayer := false
	for _, p := range g.Players {
		if p.ID == uc.UserID {
			isPlayer = true
			break
		}
	}
	if !isPlayer {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var body DebugStateReq
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	nextGame := g
	if body.Phase != nil {
		nextGame.Phase = *body.Phase
	}
	if body.CurrentTurn != nil {
		nextGame.CurrentTurn = *body.CurrentTurn
	}
	if body.Hands != nil {
		nextGame.Hands = body.Hands
	}
	if body.DiscardPile != nil {
		nextGame.DiscardPile = body.DiscardPile
	}
	if body.RoundReqMet != nil {
		nextGame.RoundReqMet = body.RoundReqMet
	}
	if body.DiscardDrawnCardPendingMeld != nil {
		nextGame.DiscardDrawnCardPendingMeld = *body.DiscardDrawnCardPendingMeld
	}
	if body.Melds != nil {
		cfg := rules.ResolveProfile(g.RulesProfile)
		melds := map[string][][]string{}
		meldMeta := map[string][]models.MeldInfo{}
		for owner, groups := range body.Melds {
			for idx, cards := range groups {
				mv, err := rules.ValidateMeld(cards, cfg)
				if err != nil {
					http.Error(w, fmt.Sprintf("melds[%s][%d]: %v", owner, idx, err), http.StatusBadRequest)
					return
				}
				melds[owner] = append(melds[owner], cards)
				meldMeta[owner] = append(meldMeta[owner], models.MeldInfo{
					MeldID:    fmt.Sprintf("%s-t%d", owner, idx),
					Type:      string(mv.Type),
					OwnerID:   owner,
					WildCount: mv.WildCount,
				})
			}
		}
		nextGame.Melds = melds
		nextGame.MeldMeta = meldMeta
	}

	if err := h.repo.UpdateWithVersion(ctx, g.ID, g.Version, nextGame); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	h.hub.BroadcastGameState(gameIDHex, BroadcastRecipients(nextGame), func(pid string) interface{} {
		return BuildGameStateMsg(nextGame, pid)
	})

	_ = json.NewEncoder(w).Encode(map[string]any{
		"gameId":   gameIDHex,
		"meldMeta": nextGame.MeldMeta,
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
