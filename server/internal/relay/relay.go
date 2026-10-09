// Package relay lets people anywhere sit at a table a phone is hosting.
//
// A hybrid table is served by the phone, as every offline table is: the
// phone holds the game, the people in the room reach it over Wi-Fi or
// Bluetooth, and nothing about that changes. What the relay adds is a way in
// for everybody else. The phone opens one connection out to this server, and
// a remote guest opens one in; the relay pairs them and copies bytes.
//
// It never reads them. What travels is the Bluetooth tunnel's protocol
// (server/mobile/zolikcore/tunnel.go): an X25519 handshake and then
// ChaCha20-Poly1305 sealed messages, end to end between the phone and each
// guest. Hands, tokens and join codes are as hidden from this server as they
// are from a radio listener in the room. The relay knows which phone, how
// many guests and how many bytes — which is what it needs to limit — and
// nothing else.
//
// The wire, both ends WebSocket:
//
//	host  ← {"t":"code","code":"ABC123"}       text: the table's relay code, once
//	host  ← {"t":"open","c":7}                 text: guest 7 connected
//	host  ← {"t":"close","c":7}                text: guest 7 went
//	host  ↔ c(4 bytes, big-endian) ‖ payload   binary: a tunnel message for/from guest c
//	host  → {"t":"close","c":7}                text: the phone hangs up on guest 7
//	guest ↔ payload                            binary: its tunnel messages
//
// When the phone's connection goes, every guest's is closed with
// CloseHostGone, and the code stays reserved for HostGrace so the same phone
// can take it back: links already shared keep working once it returns.
package relay

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// CloseHostGone is the close code a guest's socket gets when the phone
	// serving the table has gone. A guest's screen says "the server is
	// offline" on it, not "you are offline".
	CloseHostGone = 4002
	// CloseHostEnded is the phone having stopped hosting for good.
	CloseHostEnded = 4003

	// HostGrace is how long a code is kept for a phone that has dropped:
	// long enough for an app sent to the background and back, a lift, a
	// tunnel.
	HostGrace = 10 * time.Minute

	// MaxGuests is the most remote guests one table takes. Every game here
	// seats at most nine, and the room takes some of them.
	MaxGuests = 8
	// MaxFrame bounds one tunnel message. A full state for a large table is
	// well under a tenth of it.
	MaxFrame = 256 << 10
	// frameBurst and frameRefill bound how fast one guest may send: a person
	// playing cards sends a handful of frames a second at most.
	frameBurst  = 100
	frameRefill = 20 // per second

	codeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	codeLen      = 6
)

// Errors a handler turns into an answer.
var (
	ErrNotFound = errors.New("relay: no such table")
	ErrHostAway = errors.New("relay: the table's server is offline")
	ErrFull     = errors.New("relay: the table is full")
	ErrNotYours = errors.New("relay: that code belongs to another table")
)

// Info is what anybody may know about a relayed table before joining it.
type Info struct {
	Code       string `json:"code"`
	Online     bool   `json:"online"`
	InstanceID string `json:"instanceId"`
	Name       string `json:"name"`
	Guests     int    `json:"guests"`
	// Protocol is the tunnel protocol version the phone speaks, so a guest
	// on a different build is told to update rather than failing obscurely.
	Protocol int `json:"protocol"`
}

// Hub holds every relayed table this server is carrying. In memory: a relay
// is a pair of live connections and nothing else, and a restart drops them
// exactly as it drops every other socket.
type Hub struct {
	mu     sync.Mutex
	byCode map[string]*table
	byNode map[string]*table
	now    func() time.Time
	grace  time.Duration
}

// NewHub returns an empty hub.
func NewHub() *Hub {
	return &Hub{byCode: map[string]*table{}, byNode: map[string]*table{}, now: time.Now, grace: HostGrace}
}

type table struct {
	code       string
	nodeID     string
	instanceID string
	name       string
	protocol   int

	mu     sync.Mutex
	host   *conn
	away   time.Time // when the host went; zero while it is here
	guests map[uint32]*guest
	nextID uint32
}

type conn struct {
	ws *websocket.Conn
	wm sync.Mutex
}

func (c *conn) write(kind int, data []byte) error {
	c.wm.Lock()
	defer c.wm.Unlock()
	_ = c.ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return c.ws.WriteMessage(kind, data)
}

func (c *conn) writeJSON(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return c.write(websocket.TextMessage, b)
}

func (c *conn) closeWith(code int, reason string) {
	c.wm.Lock()
	_ = c.ws.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(code, reason), time.Now().Add(time.Second))
	c.wm.Unlock()
	_ = c.ws.Close()
}

type guest struct {
	id uint32
	*conn
	tokens float64
	last   time.Time
}

// allow is a token bucket: a guest sending faster than any game needs is cut
// off rather than relayed.
func (g *guest) allow(now time.Time) bool {
	if g.last.IsZero() {
		g.tokens, g.last = frameBurst, now
	}
	g.tokens += now.Sub(g.last).Seconds() * frameRefill
	if g.tokens > frameBurst {
		g.tokens = frameBurst
	}
	g.last = now
	if g.tokens < 1 {
		return false
	}
	g.tokens--
	return true
}

// Info describes the table behind a code.
func (h *Hub) Info(code string) (Info, error) {
	h.mu.Lock()
	t := h.byCode[code]
	h.mu.Unlock()
	if t == nil {
		return Info{}, ErrNotFound
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return Info{
		Code: t.code, Online: t.host != nil, InstanceID: t.instanceID, Name: t.name,
		Guests: len(t.guests), Protocol: t.protocol,
	}, nil
}

// attachHost seats a phone's connection behind a code: the one it asks for,
// if it is that phone's, or its own existing one, or a new one.
func (h *Hub) attachHost(nodeID, instanceID, name string, protocol int, want string, ws *websocket.Conn) (*table, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.sweepLocked()
	t := h.byNode[nodeID]
	if want != "" {
		if other := h.byCode[want]; other != nil && other.nodeID != nodeID {
			return nil, ErrNotYours
		}
	}
	if t == nil {
		code := want
		if code == "" || h.byCode[code] != nil {
			code = h.freshCodeLocked()
		}
		t = &table{code: code, nodeID: nodeID, guests: map[uint32]*guest{}}
		h.byCode[code] = t
		h.byNode[nodeID] = t
	}
	t.mu.Lock()
	old := t.host
	t.host = &conn{ws: ws}
	t.away = time.Time{}
	t.instanceID, t.name, t.protocol = instanceID, name, protocol
	t.mu.Unlock()
	if old != nil {
		// The same phone again, on a fresh connection: the old one is a
		// socket the network has not noticed is dead yet.
		_ = old.ws.Close()
	}
	return t, nil
}

// hostGone is the phone's connection ending. Its guests are let go — their
// tunnels end with it — and the code waits for the phone to come back.
func (h *Hub) hostGone(t *table, c *conn, ended bool) {
	t.mu.Lock()
	if t.host != c {
		t.mu.Unlock()
		return // already replaced by a newer connection from the same phone
	}
	t.host = nil
	t.away = h.now()
	guests := t.guests
	t.guests = map[uint32]*guest{}
	t.mu.Unlock()
	code := CloseHostGone
	if ended {
		code = CloseHostEnded
	}
	for _, g := range guests {
		g.closeWith(code, "host gone")
	}
	if ended {
		h.mu.Lock()
		if h.byCode[t.code] == t {
			delete(h.byCode, t.code)
			delete(h.byNode, t.nodeID)
		}
		h.mu.Unlock()
	}
}

// addGuest admits a remote guest to a table whose phone is here.
func (h *Hub) addGuest(code string, ws *websocket.Conn) (*table, *guest, error) {
	h.mu.Lock()
	t := h.byCode[code]
	h.mu.Unlock()
	if t == nil {
		return nil, nil, ErrNotFound
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.host == nil {
		return nil, nil, ErrHostAway
	}
	if len(t.guests) >= MaxGuests {
		return nil, nil, ErrFull
	}
	t.nextID++
	g := &guest{id: t.nextID, conn: &conn{ws: ws}}
	t.guests[g.id] = g
	_ = t.host.writeJSON(map[string]any{"t": "open", "c": g.id})
	return t, g, nil
}

// dropGuest is a guest's connection ending; the phone is told so it can let
// that tunnel go.
func (t *table) dropGuest(g *guest) {
	t.mu.Lock()
	_, here := t.guests[g.id]
	delete(t.guests, g.id)
	host := t.host
	t.mu.Unlock()
	if here && host != nil {
		_ = host.writeJSON(map[string]any{"t": "close", "c": g.id})
	}
}

// fromGuest carries a guest's message to the phone.
func (t *table) fromGuest(g *guest, payload []byte) error {
	t.mu.Lock()
	host := t.host
	t.mu.Unlock()
	if host == nil {
		return ErrHostAway
	}
	frame := make([]byte, 4+len(payload))
	binary.BigEndian.PutUint32(frame, g.id)
	copy(frame[4:], payload)
	return host.write(websocket.BinaryMessage, frame)
}

// fromHost carries the phone's message to one guest.
func (t *table) fromHost(frame []byte) {
	if len(frame) < 4 {
		return
	}
	id := binary.BigEndian.Uint32(frame)
	t.mu.Lock()
	g := t.guests[id]
	t.mu.Unlock()
	if g != nil {
		_ = g.write(websocket.BinaryMessage, frame[4:])
	}
}

// hangUp is the phone ending one guest's connection.
func (t *table) hangUp(id uint32) {
	t.mu.Lock()
	g := t.guests[id]
	delete(t.guests, id)
	t.mu.Unlock()
	if g != nil {
		g.closeWith(websocket.CloseNormalClosure, "")
	}
}

// sweepLocked forgets codes whose phone has been gone past the grace.
func (h *Hub) sweepLocked() {
	now := h.now()
	for code, t := range h.byCode {
		t.mu.Lock()
		stale := t.host == nil && !t.away.IsZero() && now.Sub(t.away) > h.grace
		t.mu.Unlock()
		if stale {
			delete(h.byCode, code)
			delete(h.byNode, t.nodeID)
		}
	}
}

func (h *Hub) freshCodeLocked() string {
	for {
		var b [codeLen]byte
		_, _ = rand.Read(b[:])
		for i := range b {
			b[i] = codeAlphabet[int(b[i])%len(codeAlphabet)]
		}
		if h.byCode[string(b[:])] == nil {
			return string(b[:])
		}
	}
}
