package match

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/auth"
	"zolik/server/internal/game"
	"zolik/server/internal/models"
	"zolik/server/internal/module"
)

// Handlers expose the generic runtime over HTTP and WebSocket.
//
// The route set is small because there is almost nothing game-specific to
// expose: list the games, open one, join, start, and then a socket that
// carries actions in and views out.
type Handlers struct {
	manager  *Manager
	upgrader websocket.Upgrader
}

func NewHandlers(m *Manager) *Handlers {
	return &Handlers{
		manager: m,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin:     func(*http.Request) bool { return true },
		},
	}
}

func (h *Handlers) RegisterRoutes(r chi.Router) {
	// Every game this server can host, and what each one lets a lobby set. A
	// client renders its whole game-picker and new-match form from this.
	r.Get("/modules", h.listModules)
	r.Get("/matches/{id}", h.getMatch)
	r.With(auth.AuthMiddleware).Post("/matches", h.createMatch)
	r.With(auth.AuthMiddleware).Post("/matches/{id}/join", h.joinMatch)
	r.With(auth.AuthMiddleware).Post("/matches/{id}/start", h.startMatch)
	r.With(auth.AuthMiddleware).Post("/matches/{id}/add-bot", h.addBot)
	r.Get("/ws/matches/{id}", h.handleWS)
}

func (h *Handlers) listModules(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"modules": h.manager.Registry().Descriptors(),
	})
}

type createMatchReq struct {
	ModuleID  string         `json:"moduleId"`
	Variation string         `json:"variation,omitempty"`
	Options   map[string]int `json:"options,omitempty"`
}

func (h *Handlers) createMatch(w http.ResponseWriter, req *http.Request) {
	uc, ok := auth.GetUserContext(req)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var body createMatchReq
	_ = json.NewDecoder(req.Body).Decode(&body)
	if body.ModuleID == "" {
		http.Error(w, "moduleId is required", http.StatusBadRequest)
		return
	}

	host := models.Player{ID: uc.UserID, Name: uc.Username}
	if !uc.IsGuest {
		host.UserID = uc.UserID
	}
	m, err := h.manager.Create(req.Context(), body.ModuleID,
		module.MatchConfig{Variation: body.Variation, Options: body.Options}, host)
	if err != nil {
		writeModuleError(w, err)
		return
	}
	writeJSON(w, map[string]any{"matchId": m.ID.Hex(), "joinCode": m.JoinCode})
}

func (h *Handlers) joinMatch(w http.ResponseWriter, req *http.Request) {
	uc, ok := auth.GetUserContext(req)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	p := models.Player{ID: uc.UserID, Name: uc.Username}
	m, err := h.manager.Join(req.Context(), chi.URLParam(req, "id"), p)
	if err != nil {
		writeModuleError(w, err)
		return
	}
	writeJSON(w, map[string]any{"matchId": m.ID.Hex()})
}

// addBot seats a non-human player.
//
// It is deliberately not an *agent*: driving one is module-scoped work the
// plan puts alongside the second module's own AI, and seating a body is enough
// for a two-player game to start while that lands.
func (h *Handlers) addBot(w http.ResponseWriter, req *http.Request) {
	if _, ok := auth.GetUserContext(req); !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	ctx := req.Context()
	m, err := h.manager.Repo().Resolve(ctx, chi.URLParam(req, "id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	bot := models.Player{
		ID:   "bot:" + randomJoinCode(8),
		Name: "Bot " + randomJoinCode(2),
		IsAI: true,
	}
	if _, err := h.manager.Join(ctx, m.ID.Hex(), bot); err != nil {
		writeModuleError(w, err)
		return
	}
	writeJSON(w, map[string]any{"playerId": bot.ID})
}

func (h *Handlers) startMatch(w http.ResponseWriter, req *http.Request) {
	if _, ok := auth.GetUserContext(req); !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	m, err := h.manager.Start(req.Context(), chi.URLParam(req, "id"))
	if err != nil {
		writeModuleError(w, err)
		return
	}
	writeJSON(w, map[string]any{"matchId": m.ID.Hex(), "status": m.Status})
}

// getMatch returns a viewer's state over plain HTTP.
//
// The socket is the live path, but a plain GET makes the runtime testable and
// debuggable without opening one — and, unauthenticated, it deliberately
// returns the *spectator* view, which is the same projection with nobody's
// hand in it.
func (h *Handlers) getMatch(w http.ResponseWriter, req *http.Request) {
	m, err := h.manager.Repo().Resolve(req.Context(), chi.URLParam(req, "id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	viewer := req.URL.Query().Get("as")
	writeJSON(w, h.manager.BuildStateMsg(m, viewer))
}

// handleWS carries actions in and per-viewer state out.
//
// Compare game.handleWS: no phases, no undo verbs, no rules error taxonomy —
// it decodes a module.Action and hands it over. Everything that made the rummy
// socket long was rummy.
func (h *Handlers) handleWS(w http.ResponseWriter, req *http.Request) {
	matchID := chi.URLParam(req, "id")
	playerID, err := auth.SubjectFromToken(req.URL.Query().Get("token"))
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	oid, err := bson.ObjectIDFromHex(matchID)
	if err != nil {
		http.Error(w, "invalid match id", http.StatusBadRequest)
		return
	}

	conn, err := h.upgrader.Upgrade(w, req, nil)
	if err != nil {
		http.Error(w, "upgrade failed", http.StatusBadRequest)
		return
	}
	wsConn, prev := h.manager.Hub().Registry().Add(matchID, playerID, game.PingableConn{Conn: conn})
	if prev != nil {
		_ = prev.Close()
	}
	defer h.manager.Hub().Registry().RemoveIfCurrent(matchID, playerID, wsConn)

	ctx := context.Background()
	if m, err := h.manager.Repo().FindByID(ctx, oid); err == nil {
		h.manager.Hub().WriteDirect(matchID, playerID, h.manager.BuildStateMsg(m, playerID))
	}

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		var a module.Action
		if err := json.Unmarshal(data, &a); err != nil {
			_ = wsConn.WriteJSON(map[string]any{
				"type": "error", "code": "BAD_JSON", "message": err.Error(),
			})
			continue
		}
		if err := h.manager.HandleAction(ctx, matchID, playerID, a); err != nil {
			log.Printf("match=%s player=%s verb=%s refused: %v", matchID, playerID, a.Verb, err)
			_ = wsConn.WriteJSON(map[string]any{
				"type": "error", "code": module.CodeOf(err), "message": err.Error(),
			})
		}
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// writeModuleError maps a module refusal onto a status code.
//
// The code travels in the body as well: it is the same stable vocabulary the
// offers use, so a client renders it from the same locale bundle rather than
// parsing prose out of an HTTP error.
func writeModuleError(w http.ResponseWriter, err error) {
	code := module.CodeOf(err)
	status := http.StatusBadRequest
	switch code {
	case "UNKNOWN_MODULE", "UNKNOWN_VARIATION":
		status = http.StatusNotFound
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"code": code, "message": err.Error()})
}
