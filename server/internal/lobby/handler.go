package lobby

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"

	"zolik/server/internal/admission"
	"zolik/server/internal/auth"
	"zolik/server/internal/ws"
)

// Idle-connection handling mirrors the match runtime's WebSocket server
// exactly (same intervals, same PingableConn adapter, same
// registry-wrapped-write discipline) — this is the same kind of connection,
// just registered under a different room, so it earns the same care.
const (
	pingInterval = 25 * time.Second
	pongWait     = 60 * time.Second
)

// Handlers serves the waiting room's WebSocket and its REST snapshot.
type Handlers struct {
	hub      *ws.Hub
	store    Store
	upgrader websocket.Upgrader
	// admission gates new arrivals when the server is short of memory. The
	// waiting room is the first thing shed: a player idling here has no game
	// to lose. Nil means no gating — see SetAdmission.
	admission *admission.Controller
}

// SetAdmission wires in the capacity gate. Injected rather than taken in
// NewHandlers so existing call sites and tests are untouched.
func (h *Handlers) SetAdmission(c *admission.Controller) { h.admission = c }

func NewHandlers(hub *ws.Hub, store Store) *Handlers {
	return &Handlers{
		hub:   hub,
		store: store,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin:     func(r *http.Request) bool { return true },
		},
	}
}

func (h *Handlers) RegisterRoutes(r chi.Router) {
	// A snapshot for a host browsing who to invite — a poll, not a socket,
	// since the host is likely already spending their one WebSocket
	// connection on their own match's room, not this one.
	r.With(auth.AuthMiddleware).Get("/lobby/waiting", h.waiting)
	r.Get("/ws/lobby", h.handleWS)
}

func (h *Handlers) waiting(w http.ResponseWriter, req *http.Request) {
	entries := h.store.List(req.Context())
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"players": entries})
}

// handleWS registers a connection as "waiting to be picked up" for as long
// as it stays open. There is nothing to negotiate on connect beyond the
// token — being here at all *is* the request to wait — and nothing to read
// from the client afterwards beyond keepalive pongs, so the read loop below
// exists only to detect a dead connection, exactly as the game handler's
// does for the same reason.
func (h *Handlers) handleWS(w http.ResponseWriter, req *http.Request) {
	token := req.URL.Query().Get("token")
	claims, err := auth.ParseAccessClaims(token)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	playerID := claims.Subject
	if playerID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Checked before the upgrade so a refused player reads an ordinary HTTP
	// 503. A player already waiting is admitted without asking — their new
	// socket displaces their old one below — but still takes a slot, so the
	// ledger keeps matching the sockets actually open while the displaced
	// handler unwinds and releases its own.
	var slot *admission.Release
	if h.hub.Registry().Has(RoomID, playerID) {
		slot = h.admission.AdmitReconnect()
	} else {
		var err error
		if slot, err = h.admission.Admit(admission.ClassWaiting); err != nil {
			admission.WriteBusy(w, err)
			return
		}
	}
	defer slot.Release()

	conn, err := h.upgrader.Upgrade(w, req, nil)
	if err != nil {
		http.Error(w, "upgrade failed", http.StatusBadRequest)
		return
	}

	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	wsConn, prev := h.hub.Registry().Add(RoomID, playerID, ws.PingableConn{Conn: conn})
	if prev != nil {
		log.Printf("lobby player=%s ws connect: replacing existing connection", playerID)
		_ = prev.Close()
	}

	ctx := context.Background()
	h.store.Join(ctx, Entry{PlayerID: playerID, Username: claims.Username, IsGuest: claims.IsGuest})
	h.broadcastWaitingList(ctx)

	pingDone := make(chan struct{})
	defer close(pingDone)
	go func() {
		ticker := time.NewTicker(pingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				// The same tick that keeps the socket alive also refreshes
				// this player's presence — one clock for both concerns
				// rather than a second ticker duplicating it.
				h.store.Heartbeat(context.Background(), playerID)
				if err := wsConn.Ping(); err != nil {
					return
				}
			case <-pingDone:
				return
			}
		}
	}()

	defer func() {
		// Only a real disconnect (not superseded by a newer connection from
		// the same player, e.g. an app foreground/background reconnect)
		// actually leaves the pool — see ConnRegistry.RemoveIfCurrent's own
		// reasoning, which applies here unchanged.
		if h.hub.Registry().RemoveIfCurrent(RoomID, playerID, wsConn) {
			cleanupCtx := context.Background()
			h.store.Leave(cleanupCtx, playerID)
			h.broadcastWaitingList(cleanupCtx)
		}
	}()

	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
		// No inbound message carries meaning yet — presence is entirely a
		// function of the connection being open. Anything read here only
		// needed to happen so a dead peer is detected via a failed read.
	}
}

// broadcastWaitingList pushes the current pool to everyone connected to the
// room, on every instance. Publish's recipients have to be named
// explicitly (see game.Hub.Publish), so the full cross-instance list from
// Store.List is exactly the recipient set: each instance's local write only
// lands for the subset it actually holds, and the rest are harmless no-ops
// that Redis has already replayed to whichever instance does hold them.
func (h *Handlers) broadcastWaitingList(ctx context.Context) {
	entries := h.store.List(ctx)
	if len(entries) == 0 {
		return
	}
	msgs := make([]ws.PlayerMessage, 0, len(entries))
	payload := map[string]any{"type": "lobby_waiting", "players": entries}
	for _, e := range entries {
		msgs = append(msgs, ws.PlayerMessage{PlayerID: e.PlayerID, Payload: payload})
	}
	h.hub.Publish(RoomID, msgs)
}
