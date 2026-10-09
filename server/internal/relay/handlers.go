package relay

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

// HostVerifier turns the credential a phone presents into which enrolled node
// it is. Only an enrolled phone may host: a relay is a door onto the
// internet, and it is opened for somebody this server knows.
type HostVerifier func(token string) (nodeID string, err error)

// Handlers serves the relay's three routes.
type Handlers struct {
	hub      *Hub
	verify   HostVerifier
	upgrader websocket.Upgrader
}

// NewHandlers returns the relay's routes over hub.
func NewHandlers(hub *Hub, verify HostVerifier) *Handlers {
	return &Handlers{
		hub:    hub,
		verify: verify,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
			// A guest's page is served by this server or by the app; the
			// socket carries nothing but sealed bytes either way.
			CheckOrigin: func(*http.Request) bool { return true },
		},
	}
}

// RegisterRoutes mounts the relay.
func (h *Handlers) RegisterRoutes(r chi.Router) {
	r.Get("/relay/host", h.host)
	r.Get("/relay/join/{code}", h.join)
	r.Get("/relay/info/{code}", h.info)
}

const (
	pingEvery = 25 * time.Second
	readWait  = 70 * time.Second
)

func (h *Handlers) info(w http.ResponseWriter, r *http.Request) {
	info, err := h.hub.Info(normalise(chi.URLParam(r, "code")))
	if err != nil {
		refuse(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(info)
}

// host is a phone opening its table to the internet.
func (h *Handlers) host(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	nodeID, err := h.verify(strings.TrimSpace(token))
	if err != nil || nodeID == "" {
		http.Error(w, `{"code":"RELAY_NOT_ENROLLED"}`, http.StatusUnauthorized)
		return
	}
	q := r.URL.Query()
	protocol, _ := strconv.Atoi(q.Get("protocol"))
	ws, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	t, err := h.hub.attachHost(nodeID, q.Get("instance"), q.Get("name"), protocol, normalise(q.Get("code")), ws)
	if err != nil {
		_ = ws.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(4009, err.Error()), time.Now().Add(time.Second))
		_ = ws.Close()
		return
	}
	t.mu.Lock()
	c := t.host
	t.mu.Unlock()
	log.Printf("relay: table %s open for node %s", t.code, nodeID)
	_ = c.writeJSON(map[string]any{"t": "code", "code": t.code})

	stop := keepAlive(c)
	defer close(stop)
	ended := false
	ws.SetReadLimit(MaxFrame + 4)
	for {
		kind, data, err := ws.ReadMessage()
		if err != nil {
			break
		}
		_ = ws.SetReadDeadline(time.Now().Add(readWait))
		if kind == websocket.BinaryMessage {
			t.fromHost(data)
			continue
		}
		var m struct {
			T string `json:"t"`
			C uint32 `json:"c"`
		}
		if json.Unmarshal(data, &m) != nil {
			continue
		}
		switch m.T {
		case "close":
			t.hangUp(m.C)
		case "end":
			ended = true
		}
		if ended {
			break
		}
	}
	h.hub.hostGone(t, c, ended)
	log.Printf("relay: table %s host gone (ended=%v)", t.code, ended)
}

// join is a remote guest sitting down.
func (h *Handlers) join(w http.ResponseWriter, r *http.Request) {
	code := normalise(chi.URLParam(r, "code"))
	// Answered before the upgrade, so a browser gets a status it can show
	// rather than a socket that opens and immediately closes.
	if info, err := h.hub.Info(code); err != nil {
		refuse(w, err)
		return
	} else if !info.Online {
		refuse(w, ErrHostAway)
		return
	}
	ws, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	t, g, err := h.hub.addGuest(code, ws)
	if err != nil {
		closeCode := CloseHostGone
		if errors.Is(err, ErrFull) {
			closeCode = 4004
		}
		_ = ws.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(closeCode, err.Error()), time.Now().Add(time.Second))
		_ = ws.Close()
		return
	}
	stop := keepAlive(g.conn)
	defer close(stop)
	defer t.dropGuest(g)
	ws.SetReadLimit(MaxFrame)
	for {
		kind, data, err := ws.ReadMessage()
		if err != nil {
			return
		}
		_ = ws.SetReadDeadline(time.Now().Add(readWait))
		if kind != websocket.BinaryMessage {
			continue
		}
		if !g.allow(time.Now()) {
			g.closeWith(4008, "too fast")
			return
		}
		if t.fromGuest(g, data) != nil {
			return
		}
	}
}

// keepAlive pings a connection so a dead one is noticed, and so the
// proxies between here and a phone do not close an idle one.
func keepAlive(c *conn) chan struct{} {
	stop := make(chan struct{})
	_ = c.ws.SetReadDeadline(time.Now().Add(readWait))
	c.ws.SetPongHandler(func(string) error {
		return c.ws.SetReadDeadline(time.Now().Add(readWait))
	})
	go func() {
		t := time.NewTicker(pingEvery)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				c.wm.Lock()
				err := c.ws.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second))
				c.wm.Unlock()
				if err != nil {
					return
				}
			}
		}
	}()
	return stop
}

func refuse(w http.ResponseWriter, err error) {
	status, code := http.StatusNotFound, "RELAY_NOT_FOUND"
	switch {
	case errors.Is(err, ErrHostAway):
		status, code = http.StatusServiceUnavailable, "RELAY_HOST_AWAY"
	case errors.Is(err, ErrFull):
		status, code = http.StatusConflict, "RELAY_FULL"
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"code": code})
}

// normalise reads a code the way a person types one: any case, with the
// spaces and dashes they put in to read it aloud.
func normalise(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	return strings.NewReplacer(" ", "", "-", "").Replace(code)
}
