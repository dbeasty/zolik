package notify

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"

	"zolik/server/internal/admission"
	"zolik/server/internal/auth"
	"zolik/server/internal/module"
	"zolik/server/internal/ws"
)

// Same idle-connection handling as the waiting room and the match socket.
const (
	pingInterval = 25 * time.Second
	pongWait     = 60 * time.Second
)

// Handlers serve the notification routes and the personal socket.
type Handlers struct {
	svc       *Service
	hub       *ws.Hub
	upgrader  websocket.Upgrader
	admission *admission.Controller
}

func NewHandlers(svc *Service, hub *ws.Hub) *Handlers {
	return &Handlers{
		svc: svc,
		hub: hub,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin:     func(*http.Request) bool { return true },
		},
	}
}

// SetAdmission wires in the capacity gate, as the other socket handlers do.
func (h *Handlers) SetAdmission(c *admission.Controller) { h.admission = c }

func (h *Handlers) RegisterRoutes(r chi.Router) {
	// Public: a client asks before it asks the player, and a friend link is
	// opened by people who may not have signed in yet.
	r.Get("/notify/config", h.config)
	r.Get("/notify/friend/{code}", h.friendPreview)

	r.Group(func(r chi.Router) {
		r.Use(auth.AuthMiddleware)
		r.Get("/notify/me", h.me)
		r.Patch("/notify/me", h.patchMe)
		r.Get("/notify/circle", h.circle)
		r.Get("/notify/circle/suggestions", h.suggestions)
		r.Post("/notify/circle", h.add)
		r.Delete("/notify/circle/{key}", h.remove)
		r.Post("/notify/circle/{key}/mute", h.mute)
		r.Post("/notify/circle/requests/{key}/accept", h.accept)
		r.Post("/notify/circle/requests/{key}/decline", h.decline)
		r.Post("/notify/friend/{code}", h.addFriend)
		r.Post("/notify/announce", h.announce)
		r.Post("/notify/devices", h.registerDevice)
		r.Delete("/notify/devices/{id}", h.deleteDevice)
	})
	r.Get("/ws/me", h.handleWS)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// writeError maps a refusal onto a status, carrying the code in the body the
// same way the match runtime does, so a client words it from err.<CODE>.
func writeError(w http.ResponseWriter, err error) {
	if errors.Is(err, errBadRequest) {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	var me module.Error
	if !errors.As(err, &me) {
		log.Printf("notify: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	status := http.StatusBadRequest
	switch me.Code {
	case "UNKNOWN_PLAYER", "UNKNOWN_FRIEND_CODE", "MATCH_NOT_FOUND":
		status = http.StatusNotFound
	case "NOT_PLAYED_TOGETHER", "NOT_THE_HOST":
		status = http.StatusForbidden
	case "MATCH_ALREADY_STARTED":
		status = http.StatusConflict
	case "RATE_LIMITED":
		status = http.StatusTooManyRequests
	case "PUSH_UNAVAILABLE":
		status = http.StatusServiceUnavailable
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"code": me.Code, "message": me.Error()})
}

// keyParam is the {key} segment, unescaped. A subject key carries a colon,
// which clients rightly percent-encode, and chi hands the segment over still
// encoded when the path had any escapes in it.
func keyParam(req *http.Request) string {
	raw := chi.URLParam(req, "key")
	if k, err := url.PathUnescape(raw); err == nil {
		return k
	}
	return raw
}

func user(w http.ResponseWriter, req *http.Request) (auth.UserContext, bool) {
	uc, ok := auth.GetUserContext(req)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}
	return uc, ok
}

func (h *Handlers) config(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, h.svc.PublicConfig())
}

func (h *Handlers) me(w http.ResponseWriter, req *http.Request) {
	uc, ok := user(w, req)
	if !ok {
		return
	}
	p, err := h.svc.Profile(req.Context(), uc)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, p)
}

func (h *Handlers) patchMe(w http.ResponseWriter, req *http.Request) {
	uc, ok := user(w, req)
	if !ok {
		return
	}
	var body struct {
		Invites *Invites `json:"invites"`
		Nearby  *bool    `json:"nearby"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeError(w, errBadRequest)
		return
	}
	p, err := h.svc.UpdateProfile(req.Context(), uc, body.Invites, body.Nearby)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, p)
}

func (h *Handlers) circle(w http.ResponseWriter, req *http.Request) {
	uc, ok := user(w, req)
	if !ok {
		return
	}
	c, err := h.svc.Circle(req.Context(), uc)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, c)
}

func (h *Handlers) suggestions(w http.ResponseWriter, req *http.Request) {
	uc, ok := user(w, req)
	if !ok {
		return
	}
	s, err := h.svc.Suggestions(req.Context(), uc)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, map[string]any{"players": s})
}

func (h *Handlers) add(w http.ResponseWriter, req *http.Request) {
	uc, ok := user(w, req)
	if !ok {
		return
	}
	var body struct {
		Key      string `json:"key"`
		Username string `json:"username"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeError(w, errBadRequest)
		return
	}
	e, err := h.svc.Add(req.Context(), uc, body.Key, body.Username)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, map[string]any{"entry": e})
}

func (h *Handlers) remove(w http.ResponseWriter, req *http.Request) {
	uc, ok := user(w, req)
	if !ok {
		return
	}
	if err := h.svc.Remove(req.Context(), uc, keyParam(req)); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) mute(w http.ResponseWriter, req *http.Request) {
	uc, ok := user(w, req)
	if !ok {
		return
	}
	body := struct {
		Muted *bool `json:"muted"`
	}{}
	_ = json.NewDecoder(req.Body).Decode(&body)
	muted := body.Muted == nil || *body.Muted
	key := keyParam(req)
	if !validKey(key) {
		writeError(w, module.Error{Code: "UNKNOWN_PLAYER", Message: key})
		return
	}
	if err := h.svc.Mute(req.Context(), uc, key, muted); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) accept(w http.ResponseWriter, req *http.Request) {
	uc, ok := user(w, req)
	if !ok {
		return
	}
	e, err := h.svc.Accept(req.Context(), uc, keyParam(req))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, map[string]any{"entry": e})
}

func (h *Handlers) decline(w http.ResponseWriter, req *http.Request) {
	uc, ok := user(w, req)
	if !ok {
		return
	}
	if err := h.svc.Decline(req.Context(), uc, keyParam(req)); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) friendPreview(w http.ResponseWriter, req *http.Request) {
	name, avatar, err := h.svc.FriendPreview(req.Context(), chi.URLParam(req, "code"))
	if err != nil {
		writeError(w, err)
		return
	}
	out := map[string]any{"name": name}
	if avatar != "" {
		out["avatar"] = avatar
	}
	writeJSON(w, out)
}

func (h *Handlers) addFriend(w http.ResponseWriter, req *http.Request) {
	uc, ok := user(w, req)
	if !ok {
		return
	}
	e, err := h.svc.AddByFriendCode(req.Context(), uc, chi.URLParam(req, "code"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, map[string]any{"entry": e})
}

func (h *Handlers) announce(w http.ResponseWriter, req *http.Request) {
	uc, ok := user(w, req)
	if !ok {
		return
	}
	var body struct {
		MatchID string   `json:"matchId"`
		Keys    []string `json:"keys"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil || body.MatchID == "" {
		writeError(w, errBadRequest)
		return
	}
	n, already, err := h.svc.Announce(req.Context(), uc, body.MatchID, body.Keys)
	if err != nil {
		writeError(w, err)
		return
	}
	out := map[string]any{"notified": n}
	if already {
		out["already"] = true
	}
	writeJSON(w, out)
}

func (h *Handlers) registerDevice(w http.ResponseWriter, req *http.Request) {
	uc, ok := user(w, req)
	if !ok {
		return
	}
	var reg DeviceRegistration
	if err := json.NewDecoder(req.Body).Decode(&reg); err != nil {
		writeError(w, errBadRequest)
		return
	}
	id, err := h.svc.RegisterDevice(req.Context(), uc, reg)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, map[string]any{"id": id})
}

func (h *Handlers) deleteDevice(w http.ResponseWriter, req *http.Request) {
	uc, ok := user(w, req)
	if !ok {
		return
	}
	if err := h.svc.DeleteDevice(req.Context(), uc, chi.URLParam(req, "id")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleWS holds a client's personal connection: the one socket an app keeps
// open on every screen, so an invite reaches somebody wherever they are in
// it. Like the waiting room's, it carries nothing inbound; reading only
// detects a dead peer.
func (h *Handlers) handleWS(w http.ResponseWriter, req *http.Request) {
	claims, err := auth.ParseAccessClaims(req.URL.Query().Get("token"))
	if err != nil || claims.Subject == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	key := KeyFor(auth.UserContext{UserID: claims.Subject, IsGuest: claims.IsGuest})

	// Reconnects are always let back in; a first connection is gated with
	// the waiting room's class, the first thing shed under pressure — an
	// invite that arrives by push instead is a small loss.
	var slot *admission.Release
	if h.hub.Registry().Has(RoomID, key) {
		slot = h.admission.AdmitReconnect()
	} else if slot, err = h.admission.Admit(admission.ClassWaiting); err != nil {
		admission.WriteBusy(w, err)
		return
	}
	defer slot.Release()

	conn, err := h.upgrader.Upgrade(w, req, nil)
	if err != nil {
		return
	}
	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	wsConn, prev := h.hub.Registry().Add(RoomID, key, ws.PingableConn{Conn: conn})
	if prev != nil {
		_ = prev.Close()
	}
	defer h.hub.Registry().RemoveIfCurrent(RoomID, key, wsConn)

	pingDone := make(chan struct{})
	defer close(pingDone)
	go func() {
		ticker := time.NewTicker(pingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := wsConn.Ping(); err != nil {
					return
				}
			case <-pingDone:
				return
			}
		}
	}()

	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}
