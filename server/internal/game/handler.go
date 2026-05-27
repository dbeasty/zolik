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
	manager *Manager
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
	prev := s.manager.registry.Add(gameID, playerID, conn)
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
	game, err := s.manager.repo.FindByID(ctx, oid)
	if err == nil {
		_ = conn.WriteJSON(BuildGameStateMsg(game, playerID))
	}

	// Read loop.
	defer func() {
		s.manager.registry.Remove(gameID, playerID)
		_ = func() error {
			s.manager.SuspendOnDisconnect(context.Background(), gameID, playerID, "disconnected")
			return nil
		}()
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

