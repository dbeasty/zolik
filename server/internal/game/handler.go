package game

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/auth"
	"zolik/server/internal/db"
)

type WebSocketServer struct {
	manager  *Manager
	upgrader websocket.Upgrader
}

func NewWebSocketServer(m *Manager) *WebSocketServer {
	return &WebSocketServer{
		manager: m,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
}

func (s *WebSocketServer) RegisterRoutes(r chi.Router, mongo *db.Mongo) {
	_ = mongo // reserved for future use (replay projections, etc.)

	r.Get("/ws/games/{id}", func(w http.ResponseWriter, req *http.Request) {
		s.handleWS(w, req)
	})
}

func (s *WebSocketServer) handleWS(w http.ResponseWriter, req *http.Request) {
	gameID := chi.URLParam(req, "id")
	token := req.URL.Query().Get("token")

	playerID, err := auth.SubjectFromToken(token)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := s.upgrader.Upgrade(w, req, nil)
	if err != nil {
		http.Error(w, "upgrade failed", http.StatusBadRequest)
		return
	}

	// Register connection.
	prev := s.manager.hub.Registry().Add(gameID, playerID, conn)
	if prev != nil {
		_ = prev.Close()
	}

	// Send initial personalised state.
	ctx := context.Background()
	oid, err := bson.ObjectIDFromHex(gameID)
	if err != nil {
		http.Error(w, "invalid game id", http.StatusBadRequest)
		_ = conn.Close()
		return
	}
	s.manager.ResumeIfReturning(ctx, gameID, playerID)

	game, err := s.manager.repo.FindByID(ctx, oid)
	if err == nil {
		s.manager.hub.WriteDirect(gameID, playerID, BuildGameStateMsg(game, playerID))
	}

	// Read loop.
	defer func() {
		// Only treat this as a real disconnect (and suspend the game) if this
		// connection is still the one registered for the player — if they've
		// already reconnected on a newer conn, RemoveIfCurrent is a no-op and
		// we must not tear down or suspend the newer, live connection.
		if s.manager.hub.Registry().RemoveIfCurrent(gameID, playerID, conn) {
			s.manager.SuspendOnDisconnect(context.Background(), gameID, playerID, "disconnected")
		}
	}()
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			break
		}
		var in WSIncoming
		if err := json.Unmarshal(data, &in); err != nil {
			_ = conn.WriteJSON(map[string]any{
				"type":    "error",
				"code":    "BAD_JSON",
				"message": err.Error(),
			})
			continue
		}
		if err := s.manager.HandleAction(ctx, gameID, playerID, in); err != nil {
			_ = conn.WriteJSON(map[string]any{
				"type":    "error",
				"code":    "RULES_ERROR",
				"message": err.Error(),
			})
		}
	}
}
