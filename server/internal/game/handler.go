package game

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/auth"
	"zolik/server/internal/db"
	"zolik/server/internal/rules"
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

	// Register connection. wsConn wraps conn with a write mutex shared with
	// the broadcast path (hub/registry) — every write to this connection,
	// whether a broadcast or this read loop's own direct error response,
	// must go through wsConn, since gorilla's websocket.Conn does not allow
	// concurrent writers and an unsynchronized write can silently vanish.
	wsConn, prev := s.manager.hub.Registry().Add(gameID, playerID, conn)
	if prev != nil {
		log.Printf("game=%s player=%s ws connect: replacing existing connection", gameID, playerID)
		_ = prev.Close()
	} else {
		log.Printf("game=%s player=%s ws connect: new connection", gameID, playerID)
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
		if s.manager.hub.Registry().RemoveIfCurrent(gameID, playerID, wsConn) {
			log.Printf("game=%s player=%s ws disconnect: suspending (this was the live connection)", gameID, playerID)
			s.manager.SuspendOnDisconnect(context.Background(), gameID, playerID, "disconnected")
		} else {
			log.Printf("game=%s player=%s ws disconnect: superseded by a newer connection, not suspending", gameID, playerID)
		}
	}()
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			log.Printf("game=%s player=%s ws read loop ending: %v", gameID, playerID, err)
			break
		}
		var in WSIncoming
		if err := json.Unmarshal(data, &in); err != nil {
			log.Printf("game=%s player=%s ws bad json: %v raw=%s", gameID, playerID, err, data)
			_ = wsConn.WriteJSON(map[string]any{
				"type":    "error",
				"code":    "BAD_JSON",
				"message": err.Error(),
			})
			continue
		}
		// A preview changes nothing: it answers "what would this be worth?"
		// for a candidate the player is still assembling. It goes straight
		// back to the asking connection rather than through HandleAction, so
		// it never persists, never broadcasts and never touches the action
		// log. See rules.PreviewMeld.
		if in.Type == "preview_meld" {
			if err := s.writeMeldPreview(ctx, wsConn, oid, playerID, in.Cards); err != nil {
				log.Printf("game=%s player=%s preview failed: %v", gameID, playerID, err)
			}
			continue
		}
		if err := s.manager.HandleAction(ctx, gameID, playerID, in); err != nil {
			_ = wsConn.WriteJSON(map[string]any{
				"type":    "error",
				"code":    "RULES_ERROR",
				"message": err.Error(),
			})
		}
	}
}

// MeldPreviewMsg is the reply to a preview_meld frame: what the candidate
// cards would be if played. Read-only — nothing is persisted or broadcast.
type MeldPreviewMsg struct {
	Type string `json:"type"`
	rules.MeldPreview
}

// writeMeldPreview answers one preview request on the asking connection.
//
// It reloads the game rather than trusting anything cached, because a preview
// is only useful if it reflects the state the submission would actually hit —
// an opponent's lay-off can change what the player's own candidate is worth.
func (s *WebSocketServer) writeMeldPreview(
	ctx context.Context, wsConn WSConn, oid bson.ObjectID, playerID string, cards []string,
) error {
	game, err := s.manager.repo.FindByID(ctx, oid)
	if err != nil {
		return err
	}
	preview := rules.PreviewMeld(toRulesState(game), playerID, cards)
	return wsConn.WriteJSON(MeldPreviewMsg{Type: "meld_preview", MeldPreview: preview})
}
