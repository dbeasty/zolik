package zolikcore

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// The internet door: remote guests at this phone's table, through the cloud.
//
// The phone stays the server. It dials the cloud's relay once
// (server/internal/relay), and every remote guest who opens the table's link
// arrives down that one connection as a numbered stream. Each stream is the
// same Bluetooth tunnel a guest in the room gets (tunnel.go) — handshake,
// sealed messages and all — so the cloud carries bytes it cannot read, and
// the table cannot tell a guest across town from one across the room.
//
// Only an enrolled phone may do this: the relay admits the node credential the
// cloud issued at sign-in, and nothing else. The link is kept up by itself —
// redialled with backoff, asking for the same code so links already shared
// keep working — until CloseRelay or Stop.

// Relay states, as RelayStatus reports them.
const (
	RelayOff        = "off"
	RelayConnecting = "connecting"
	RelayOnline     = "online"
)

type relayLink struct {
	h    *Host
	name string

	mu      sync.Mutex
	status  string
	code    string
	ws      *websocket.Conn
	tunnels map[uint32]*Tunnel
	wasUp   bool
	stop    chan struct{}
	wmu     sync.Mutex
}

// OpenRelay lets people anywhere join this table, and returns its code. name
// is what a guest's screen calls this table ("Ada's phone"). Calling it again
// while the door is open returns the same code.
func (h *Host) OpenRelay(name string) (string, error) {
	if h.nodeCredential == "" || h.cloudBaseURL == "" {
		return "", errors.New("sign in to let people join over the internet")
	}
	h.relayMu.Lock()
	r := h.relay
	if r == nil {
		r = &relayLink{h: h, name: name, status: RelayConnecting, tunnels: map[uint32]*Tunnel{}, stop: make(chan struct{})}
		h.relay = r
		go r.run()
	}
	h.relayMu.Unlock()

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if code := r.currentCode(); code != "" {
			return code, nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return "", errors.New("could not reach the server to open the table to the internet")
}

// CloseRelay shuts the internet door: remote guests are let go, and the code
// stops working.
func (h *Host) CloseRelay() {
	h.relayMu.Lock()
	r := h.relay
	h.relay = nil
	h.relayMu.Unlock()
	if r == nil {
		return
	}
	close(r.stop)
	r.mu.Lock()
	ws := r.ws
	r.status = RelayOff
	r.mu.Unlock()
	if ws != nil {
		r.wmu.Lock()
		_ = ws.WriteJSON(map[string]string{"t": "end"})
		r.wmu.Unlock()
		_ = ws.Close()
	}
	r.closeTunnels()
	h.app.HoldStandIns(false)
}

// RelayStatus is "off", "connecting" or "online".
func (h *Host) RelayStatus() string {
	h.relayMu.Lock()
	r := h.relay
	h.relayMu.Unlock()
	if r == nil {
		return RelayOff
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.status
}

// RelayCode is the table's code while the door is open, empty otherwise.
func (h *Host) RelayCode() string {
	h.relayMu.Lock()
	r := h.relay
	h.relayMu.Unlock()
	if r == nil {
		return ""
	}
	return r.currentCode()
}

// RelayURL is the link a remote guest opens: the cloud's page for this table.
func (h *Host) RelayURL() string {
	code := h.RelayCode()
	if code == "" {
		return ""
	}
	return h.cloudBaseURL + "/r/" + code
}

// RelayGuests is how many remote guests are connected now.
func (h *Host) RelayGuests() int {
	h.relayMu.Lock()
	r := h.relay
	h.relayMu.Unlock()
	if r == nil {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.tunnels)
}

// Resumed is the app coming back to the foreground. The table was stopped
// while it was away; nobody else is charged for that (match/hostpause.go).
func (h *Host) Resumed() {
	h.app.HostResumed()
}

func (r *relayLink) currentCode() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.status != RelayOnline {
		return ""
	}
	return r.code
}

// run keeps the link up until stop.
func (r *relayLink) run() {
	backoff := time.Second
	for {
		select {
		case <-r.stop:
			return
		default:
		}
		err := r.session()
		select {
		case <-r.stop:
			return
		default:
		}
		r.mu.Lock()
		r.status = RelayConnecting
		up := r.wasUp
		r.mu.Unlock()
		r.closeTunnels()
		// Remote guests cannot be here while the link is down, so no bot
		// takes their seats on that account.
		if up {
			r.h.app.HoldStandIns(true)
		}
		log.Printf("zolikcore: relay: %v; again in %s", err, backoff)
		select {
		case <-r.stop:
			return
		case <-time.After(backoff):
		}
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}

// session is one connection to the relay, from dial to drop.
func (r *relayLink) session() error {
	r.mu.Lock()
	code := r.code
	r.mu.Unlock()
	q := url.Values{}
	q.Set("instance", r.h.instanceID)
	q.Set("name", r.name)
	q.Set("protocol", strconv.Itoa(ProtocolVersion))
	q.Set("code", code)
	target := socketURL(r.h.cloudBaseURL) + "/relay/host?" + q.Encode()
	dialer := websocket.Dialer{HandshakeTimeout: 15 * time.Second}
	ws, res, err := dialer.Dial(target, http.Header{"Authorization": {"Bearer " + r.h.nodeCredential}})
	if err != nil {
		if res != nil {
			return fmt.Errorf("dial: %s", res.Status)
		}
		return fmt.Errorf("dial: %w", err)
	}
	defer ws.Close()
	ws.SetPingHandler(func(data string) error {
		r.wmu.Lock()
		defer r.wmu.Unlock()
		return ws.WriteControl(websocket.PongMessage, []byte(data), time.Now().Add(5*time.Second))
	})

	var first struct {
		T    string `json:"t"`
		Code string `json:"code"`
	}
	_ = ws.SetReadDeadline(time.Now().Add(15 * time.Second))
	if err := ws.ReadJSON(&first); err != nil || first.T != "code" || first.Code == "" {
		return fmt.Errorf("no code from the relay: %v", err)
	}
	_ = ws.SetReadDeadline(time.Time{})
	r.mu.Lock()
	r.ws, r.code, r.status = ws, first.Code, RelayOnline
	r.wasUp = true
	r.mu.Unlock()
	r.h.app.HoldStandIns(false)
	log.Printf("zolikcore: relay: table %s open to the internet", first.Code)

	for {
		kind, data, err := ws.ReadMessage()
		if err != nil {
			r.mu.Lock()
			r.ws = nil
			r.mu.Unlock()
			return err
		}
		if kind == websocket.BinaryMessage {
			if len(data) < 4 {
				continue
			}
			id := binary.BigEndian.Uint32(data)
			r.mu.Lock()
			t := r.tunnels[id]
			r.mu.Unlock()
			if t != nil {
				t.Receive(data[4:])
			}
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
		case "open":
			sink := &relaySink{r: r, id: m.C}
			t := r.h.NewTunnel(sink, "relay-"+strconv.FormatUint(uint64(m.C), 10))
			r.mu.Lock()
			r.tunnels[m.C] = t
			r.mu.Unlock()
		case "close":
			r.mu.Lock()
			t := r.tunnels[m.C]
			delete(r.tunnels, m.C)
			r.mu.Unlock()
			if t != nil {
				t.Close()
			}
		}
	}
}

func (r *relayLink) closeTunnels() {
	r.mu.Lock()
	ts := r.tunnels
	r.tunnels = map[uint32]*Tunnel{}
	r.mu.Unlock()
	for _, t := range ts {
		t.Close()
	}
}

// relaySink is one remote guest's way back: its stream on the relay link.
type relaySink struct {
	r  *relayLink
	id uint32
}

func (s *relaySink) Send(msg []byte) {
	s.r.mu.Lock()
	ws := s.r.ws
	s.r.mu.Unlock()
	if ws == nil {
		return
	}
	frame := make([]byte, 4+len(msg))
	binary.BigEndian.PutUint32(frame, s.id)
	copy(frame[4:], msg)
	s.r.wmu.Lock()
	defer s.r.wmu.Unlock()
	_ = ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
	_ = ws.WriteMessage(websocket.BinaryMessage, frame)
}

// TunnelClosed hangs up a guest whose tunnel ended on this side.
func (s *relaySink) TunnelClosed() {
	s.r.mu.Lock()
	ws := s.r.ws
	_, here := s.r.tunnels[s.id]
	delete(s.r.tunnels, s.id)
	s.r.mu.Unlock()
	if ws == nil || !here {
		return
	}
	s.r.wmu.Lock()
	defer s.r.wmu.Unlock()
	_ = ws.WriteJSON(map[string]any{"t": "close", "c": s.id})
}

// socketURL is the ws(s) origin for an http(s) base.
func socketURL(base string) string {
	switch {
	case strings.HasPrefix(base, "https://"):
		return "wss://" + strings.TrimPrefix(base, "https://")
	case strings.HasPrefix(base, "http://"):
		return "ws://" + strings.TrimPrefix(base, "http://")
	}
	return base
}
