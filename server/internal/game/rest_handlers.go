package game

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
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

// WaitingLookup answers whether a player is currently in the waiting-room
// pool, and lets them be picked up out of it. Satisfied by *lobby.Store —
// named here as a narrow, primitive-typed interface rather than importing
// internal/lobby directly, because lobby already imports this package for
// its WebSocket transport (Hub, ConnRegistry); importing it back here would
// cycle. See app.go, which wires the concrete *lobby.Store in.
type WaitingLookup interface {
	// IsWaiting reports the display details of a waiting player, so an
	// invite can build their seat without a second round trip to whatever
	// store lobby uses.
	IsWaiting(ctx context.Context, playerID string) (name string, isGuest bool, ok bool)
	// Pickup removes a player from the pool, reporting whether they were
	// actually present. Called only after they've been seated — a failed
	// seat attempt must leave them waiting, not silently drop them.
	Pickup(ctx context.Context, playerID string) bool
}

type GameRestHandlers struct {
	repo    *Repository
	hub     *Hub
	manager *Manager
	// stats is optional; when nil the scoreboard simply omits the lifetime
	// records it would otherwise attach.
	stats *stats.Repository
	// waiting is optional; when nil, inviting a waiting player is simply
	// unavailable (the route still exists but 503s) — a deployment can run
	// the game server without the lobby package wired in.
	waiting WaitingLookup
	// waitingRoom is the Hub room a picked-up player's own connection is
	// registered under, so they can be notified there — see lobby.RoomID,
	// injected as a plain string for the same import-cycle reason as
	// WaitingLookup above.
	waitingRoom string
	// testEndpoints gates debugState — see its doc comment and
	// app.Config.TestEndpointsEnabled.
	testEndpoints bool
}

func NewGameRestHandlers(
	repo *Repository, hub *Hub, manager *Manager, statsRepo *stats.Repository,
	waiting WaitingLookup, waitingRoom string, testEndpoints bool,
) *GameRestHandlers {
	return &GameRestHandlers{
		repo: repo, hub: hub, manager: manager, stats: statsRepo,
		waiting: waiting, waitingRoom: waitingRoom, testEndpoints: testEndpoints,
	}
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
	// The module's self-description: what a lobby may configure. Public and
	// static, so no auth and no game id — a client fetches it once to render
	// its new-game form (see docs/extensibility-plan.md Phase 2.1).
	r.Get("/module", h.moduleDescriptor)
	// Deprecated: a strict subset of /module, kept for callers that predate
	// the descriptor. Projected from it rather than computed separately.
	r.Get("/rules", h.getRules)
	r.Get("/games/{id}", h.getGame)
	r.Get("/games/{id}/scoreboard", h.scoreboard)
	r.With(auth.AuthMiddleware).Post("/games", h.createGame)
	r.With(auth.AuthMiddleware).Post("/games/{id}/join", h.joinGame)
	r.With(auth.AuthMiddleware).Post("/games/{id}/invite", h.invitePlayer)
	r.With(auth.AuthMiddleware).Patch("/games/{id}/settings", h.updateSettings)
	r.With(auth.AuthMiddleware).Post("/games/{id}/start", h.startGame)
	r.With(auth.AuthMiddleware).Post("/games/{id}/add-ai", h.addAI)
	r.With(auth.AuthMiddleware).Get("/games/{id}/replay", h.replayGame)
	if h.testEndpoints {
		r.With(auth.AuthMiddleware).Post("/games/{id}/debug-state", h.debugState)
	}
}

// moduleDescriptor serves the module's self-description: player range, the
// variations it ships (each with its fully-resolved ruleset) and the options a
// lobby may set. Static and public — a client fetches it once and renders its
// whole new-game form from it, so a new knob or variation needs no client
// change at all.
func (h *GameRestHandlers) moduleDescriptor(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(BuildModuleDescriptorMsg())
}

// validateOptionReq holds the descriptor to its word: a value the schema does
// not declare is rejected here rather than quietly accepted. Without this the
// option space would be advertised by the server but enforced only by whichever
// client happened to render the right controls.
func validateOptionReq(body CreateGameReq) error {
	return rules.ValidateOptions(map[string]*int{
		rules.OptInitialMeldMinimum:  body.InitialMeldMinimum,
		rules.OptDiscardDrawMinRound: body.DiscardDrawMinRound,
	})
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
	if err := validateOptionReq(body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
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
	if err := validateOptionReq(body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
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
		// Exactly one of these is set — a seat is held by an account or by a
		// device's guest identity. The guest id is what lets this match be
		// re-attributed if the player signs up later.
		UserID:  uc.PlayerUserID(),
		GuestID: uc.PlayerGuestID(),
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
		internalError(w, "createGame", err)
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
// selectable initial-meld-minimum and discard-lock-round values, and a
// profile's defaults for those two).
// getRules is the pre-descriptor shape of the same information, projected
// from rules.Descriptor() so the two can never disagree about what a lobby may
// set. Deprecated: new clients read /module, which also carries each
// variation's resolved ruleset and every label.
//
// The response shape carries one pair of defaults, not one per variation, so
// which variation it describes has to be asked for: ?profile=<id> names it,
// and an absent or unknown value keeps answering for Continental, which is
// what this endpoint has always returned. Reading the values off the named
// profile rather than a hardcoded reference to one is what stops a lobby
// prefilled from here handing Žolík Classic a 35-point floor it does not
// default to.
func (h *GameRestHandlers) getRules(w http.ResponseWriter, req *http.Request) {
	d := rules.Descriptor()
	values := func(name string) []int {
		spec := d.Option(name)
		if spec == nil {
			return nil
		}
		out := make([]int, 0, len(spec.Choices))
		for _, c := range spec.Choices {
			out = append(out, c.Value)
		}
		return out
	}
	defaults := rules.ProfileContinental
	if p := d.Profile(req.URL.Query().Get("profile")); p != nil {
		defaults = p.Rules
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"minPlayers":                 d.MinPlayers,
		"maxPlayers":                 d.MaxPlayers,
		"initialMeldMinOptions":      values(rules.OptInitialMeldMinimum),
		"discardDrawMinRoundOptions": values(rules.OptDiscardDrawMinRound),
		"defaultInitialMeldMinimum":  defaults.InitialMeldMinimum,
		"defaultDiscardDrawMinRound": defaults.DiscardDrawMinRound,
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
	if err := seatPlayer(&g, uc.UserID, uc.Username, uc.PlayerUserID(), uc.PlayerGuestID()); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.repo.UpdateWithVersion(ctx, g.ID, g.Version, g); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]any{"gameId": gameIDHex, "joined": true, "playerCount": len(g.Players)})
}

// seatPlayer appends one human seat to a lobby game, mutating g in place.
// Shared by joinGame (a player seating themself) and invitePlayer (a host
// seating someone they picked up from the waiting room) so the capacity
// check and the per-map initialisation a fresh seat needs exist in exactly
// one place.
// internalError logs the real error server-side and responds with a generic
// 500 — see the identical helper in internal/auth for why: http.Error alone
// leaves no trace in the server log, and a raw internal error string in the
// response body is exactly the kind of implementation detail a caller should
// never see.
func internalError(w http.ResponseWriter, route string, err error) {
	log.Printf("game: %s: %v", route, err)
	http.Error(w, "internal server error", http.StatusInternalServerError)
}

func seatPlayer(g *models.Game, id, name, userID, guestID string) error {
	if len(g.Players) >= rules.MaxPlayers {
		return fmt.Errorf("lobby full")
	}
	p := models.Player{
		ID:   id,
		Name: name,
		IsAI: false,
		// Exactly one of these is set — a seat is held by an account or by a
		// device's guest identity. The guest id is what lets this match be
		// re-attributed if the player signs up later.
		UserID:  userID,
		GuestID: guestID,
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
	return nil
}

type inviteReq struct {
	PlayerID string `json:"playerId"`
}

// invitePlayer lets the host seat someone straight out of the waiting room,
// without either side needing a join code. It is the "pick them up" half of
// the waiting room: the room says who's available, this is what acts on it.
//
// Gated to the host, same as startGame — it's the host's lobby being built,
// and letting any seated player invite on their behalf would make the
// waiting room's membership something two people could fight over.
func (h *GameRestHandlers) invitePlayer(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	uc, ok := auth.GetUserContext(req)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if h.waiting == nil {
		http.Error(w, "the waiting room is not available on this deployment", http.StatusServiceUnavailable)
		return
	}

	var body inviteReq
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil || body.PlayerID == "" {
		http.Error(w, "playerId required", http.StatusBadRequest)
		return
	}

	idOrJoin := chi.URLParam(req, "id")
	g, gameIDHex, err := h.repo.ParseGameIDOrJoin(ctx, idOrJoin)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if g.HostID != uc.UserID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if g.Status != "lobby" {
		http.Error(w, "game not in lobby", http.StatusBadRequest)
		return
	}
	for _, p := range g.Players {
		if p.ID == body.PlayerID {
			_ = json.NewEncoder(w).Encode(map[string]any{"gameId": gameIDHex, "alreadyJoined": true})
			return
		}
	}

	// Re-checked here rather than trusted from whatever snapshot the host's
	// client last polled: the target may have left, been picked up
	// elsewhere, or disconnected in the meantime, and this is the only
	// point that gets to decide whether they're still actually available.
	name, isGuest, stillWaiting := h.waiting.IsWaiting(ctx, body.PlayerID)
	if !stillWaiting {
		http.Error(w, "that player is no longer waiting", http.StatusConflict)
		return
	}

	userID, guestID := body.PlayerID, ""
	if isGuest {
		userID, guestID = "", body.PlayerID
	}
	if err := seatPlayer(&g, body.PlayerID, name, userID, guestID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.repo.UpdateWithVersion(ctx, g.ID, g.Version, g); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	// The seat is now committed; only now does the player actually leave the
	// pool. Picking up before the seat was confirmed would risk losing them
	// from the waiting room over a write that then failed.
	h.waiting.Pickup(ctx, body.PlayerID)
	if h.hub != nil && h.waitingRoom != "" {
		h.hub.WriteDirect(h.waitingRoom, body.PlayerID, map[string]any{
			"type":     "lobby_invited",
			"gameId":   gameIDHex,
			"joinCode": g.JoinCode,
		})
	}

	_ = json.NewEncoder(w).Encode(map[string]any{"gameId": gameIDHex, "invited": true, "playerCount": len(g.Players)})
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
		internalError(w, "startGame", err)
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
