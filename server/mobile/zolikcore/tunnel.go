package zolikcore

import (
	"bytes"
	"compress/flate"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/crypto/chacha20poly1305"
)

// The Bluetooth tunnel.
//
// A guest at a table over Bluetooth has no IP route to the host. Its phone
// connects to the host's BLE GATT service, and everything the client would
// have sent over HTTP and WebSocket travels as messages on that link. This
// file is the host's end: it opens each message, serves it against the same
// router the loopback and LAN listeners use, and seals the answer.
//
// Native code (Swift, Kotlin) only moves bytes. It splits each message into
// MTU-sized chunks and puts them back together, and hands Go whole messages
// through Tunnel.Receive and TunnelSink.Send. The two phones never pair, so
// the BLE link itself is not encrypted and this layer brings its own:
//
//	guest → host  HELLO    0x01 | v | guest ephemeral X25519 public (32)
//	host → guest  WELCOME  0x02 | v | host static public (32) | host ephemeral public (32)
//	either way    SEALED   0x03 | ChaCha20-Poly1305(key, nonce = 0⁴ ‖ counter⁸ BE, aad = 0x03)
//
// Keys come from HKDF-SHA256 over DH(eg, eh) ‖ DH(eg, sh), salted with
// bleSalt and bound to all three public keys. That gives two directional keys
// and a short check code both screens can show. The host's static key is
// what a guest pins per instance id, so a phone pretending to be a table the
// guest has sat at before is refused. Each direction counts its own nonces
// from zero. A message that fails to open, arrives out of order or is
// replayed ends the tunnel.
//
// A sealed plaintext is a flags byte (bit 0: raw DEFLATE) and then a JSON
// tunnel message:
//
//	{"t":"req","id":1,"m":"POST","p":"/auth/guest","h":{…},"b":"…"}  → {"t":"res","id":1,"s":200,"h":{…},"b":"…"}
//	{"t":"wso","id":2,"p":"/ws/matches/…?token=…"}                    → {"t":"wsopen","id":2} or {"t":"wsc","id":2}
//	{"t":"wsm","id":2,"d":"…"}                                         both ways
//	{"t":"wsc","id":2}                                                 both ways
//	{"t":"ping"}                                                       → {"t":"pong"}

const (
	msgHello   = 0x01
	msgWelcome = 0x02
	msgSealed  = 0x03

	tunnelVersion = 1

	flagDeflate = 0x01
	// Anything shorter goes out as it is: DEFLATE's own framing would eat
	// most of the saving on a small answer.
	deflateOver = 256
)

var bleSalt = []byte("zolik-ble-v1")

// TunnelSink is where a tunnel's outgoing messages go: the native side,
// which chunks each one onto the link. Implemented in Swift and Kotlin.
type TunnelSink interface {
	Send(msg []byte)
}

// Tunnel is one guest's BLE link to this host.
type Tunnel struct {
	host *Host
	sink TunnelSink
	peer string

	mu        sync.Mutex // guards everything below, and orders sends
	closed    bool
	keys      *sessionKeys
	check     string
	recvCount uint64
	sendCount uint64
	sockets   map[int64]*websocket.Conn
}

type sessionKeys struct {
	g2h, h2g cipher.AEAD
}

// NewTunnel starts a tunnel for a guest that has just connected. `peer` is
// anything that tells guests apart for rate limiting: the native side passes
// the central's identifier.
func (h *Host) NewTunnel(sink TunnelSink, peer string) *Tunnel {
	return &Tunnel{host: h, sink: sink, peer: peer, sockets: map[int64]*websocket.Conn{}}
}

// CheckCode is the short code both screens can show once the handshake is
// done, so two players can see that nobody sits between their phones. It is
// empty before that.
func (t *Tunnel) CheckCode() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.check
}

// Receive takes one whole message from the guest. It never blocks on the
// server: requests are served on their own goroutines.
func (t *Tunnel) Receive(msg []byte) {
	if len(msg) == 0 {
		return
	}
	switch msg[0] {
	case msgHello:
		if err := t.handshake(msg); err != nil {
			log.Printf("zolikcore: tunnel %s: handshake: %v", t.peer, err)
			t.Close()
		}
	case msgSealed:
		plain, err := t.open(msg)
		if err != nil {
			log.Printf("zolikcore: tunnel %s: %v", t.peer, err)
			t.Close()
			return
		}
		t.dispatch(plain)
	default:
		t.Close()
	}
}

// Close ends the tunnel and hangs up every socket it opened, which is what
// tells the match runtime this guest has gone.
func (t *Tunnel) Close() {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return
	}
	t.closed = true
	socks := t.sockets
	t.sockets = map[int64]*websocket.Conn{}
	t.mu.Unlock()
	for _, c := range socks {
		_ = c.Close()
	}
}

func (t *Tunnel) handshake(msg []byte) error {
	if len(msg) != 2+32 || msg[1] != tunnelVersion {
		return fmt.Errorf("bad hello (%d bytes)", len(msg))
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.keys != nil || t.closed {
		return errors.New("hello after the handshake")
	}
	guestEph, err := ecdh.X25519().NewPublicKey(msg[2:])
	if err != nil {
		return err
	}
	hostEph, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	keys, check, err := deriveHost(t.host.bleKey, hostEph, guestEph)
	if err != nil {
		return err
	}
	welcome := make([]byte, 0, 2+64)
	welcome = append(welcome, msgWelcome, tunnelVersion)
	welcome = append(welcome, t.host.bleKey.PublicKey().Bytes()...)
	welcome = append(welcome, hostEph.PublicKey().Bytes()...)
	t.keys, t.check = keys, check
	t.sink.Send(welcome)
	return nil
}

// deriveHost is the host's half of the key schedule. The guest runs the same
// schedule from its side (deriveGuest in the tests, and src/net/ble/crypto.ts
// in the app), and all three must agree byte for byte.
func deriveHost(hostStatic, hostEph *ecdh.PrivateKey, guestEph *ecdh.PublicKey) (*sessionKeys, string, error) {
	ee, err := hostEph.ECDH(guestEph)
	if err != nil {
		return nil, "", err
	}
	es, err := hostStatic.ECDH(guestEph)
	if err != nil {
		return nil, "", err
	}
	return schedule(ee, es, guestEph.Bytes(), hostStatic.PublicKey().Bytes(), hostEph.PublicKey().Bytes())
}

func schedule(ee, es, guestEph, hostStatic, hostEph []byte) (*sessionKeys, string, error) {
	ikm := append(append([]byte{}, ee...), es...)
	transcript := append(append(append([]byte{}, guestEph...), hostStatic...), hostEph...)
	okm, err := hkdf.Key(sha256.New, ikm, bleSalt, "keys"+string(transcript), 64)
	if err != nil {
		return nil, "", err
	}
	g2h, err := chacha20poly1305.New(okm[:32])
	if err != nil {
		return nil, "", err
	}
	h2g, err := chacha20poly1305.New(okm[32:])
	if err != nil {
		return nil, "", err
	}
	raw, err := hkdf.Key(sha256.New, ikm, bleSalt, "check"+string(transcript), 4)
	if err != nil {
		return nil, "", err
	}
	return &sessionKeys{g2h: g2h, h2g: h2g}, checkCode(raw), nil
}

// checkCode renders four bytes as four characters nobody will misread aloud:
// no 0/O, no 1/I/L.
func checkCode(b []byte) string {
	const alphabet = "23456789ABCDEFGHJKMNPQRSTUVWXYZ"
	out := make([]byte, 4)
	for i := range out {
		out[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(out)
}

func nonce(counter uint64) []byte {
	n := make([]byte, chacha20poly1305.NonceSize)
	binary.BigEndian.PutUint64(n[4:], counter)
	return n
}

func (t *Tunnel) open(msg []byte) ([]byte, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.keys == nil {
		return nil, errors.New("sealed message before the handshake")
	}
	plain, err := t.keys.g2h.Open(nil, nonce(t.recvCount), msg[1:], msg[:1])
	if err != nil {
		return nil, errors.New("a message failed to open")
	}
	t.recvCount++
	if len(plain) == 0 {
		return nil, errors.New("empty message")
	}
	body := plain[1:]
	if plain[0]&flagDeflate != 0 {
		r := flate.NewReader(bytes.NewReader(body))
		defer r.Close()
		if body, err = io.ReadAll(io.LimitReader(r, 4<<20)); err != nil {
			return nil, fmt.Errorf("inflate: %w", err)
		}
	}
	return body, nil
}

// send seals one tunnel message and hands it to the sink. Sealing and sending
// share one lock so the counter order is the order on the link.
func (t *Tunnel) send(v any) {
	raw, err := json.Marshal(v)
	if err != nil {
		return
	}
	flags := byte(0)
	if len(raw) > deflateOver {
		var buf bytes.Buffer
		w, _ := flate.NewWriter(&buf, flate.BestSpeed)
		_, _ = w.Write(raw)
		_ = w.Close()
		if buf.Len() < len(raw) {
			raw, flags = buf.Bytes(), flagDeflate
		}
	}
	plain := append([]byte{flags}, raw...)

	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed || t.keys == nil {
		return
	}
	sealed := t.keys.h2g.Seal([]byte{msgSealed}, nonce(t.sendCount), plain, []byte{msgSealed})
	t.sendCount++
	t.sink.Send(sealed)
}

type tunnelMsg struct {
	T  string            `json:"t"`
	ID int64             `json:"id,omitempty"`
	M  string            `json:"m,omitempty"`
	P  string            `json:"p,omitempty"`
	H  map[string]string `json:"h,omitempty"`
	B  string            `json:"b,omitempty"`
	S  int               `json:"s,omitempty"`
	D  string            `json:"d,omitempty"`
}

func (t *Tunnel) dispatch(raw []byte) {
	var m tunnelMsg
	if err := json.Unmarshal(raw, &m); err != nil {
		return
	}
	switch m.T {
	case "ping":
		go t.send(tunnelMsg{T: "pong"})
	case "req":
		go t.guard(func() { t.serve(m) })
	case "wso":
		go t.guard(func() { t.openSocket(m) })
	case "wsm":
		t.mu.Lock()
		c := t.sockets[m.ID]
		t.mu.Unlock()
		if c != nil {
			_ = c.WriteMessage(websocket.TextMessage, []byte(m.D))
		}
	case "wsc":
		t.mu.Lock()
		c := t.sockets[m.ID]
		delete(t.sockets, m.ID)
		t.mu.Unlock()
		if c != nil {
			_ = c.Close()
		}
	}
}

// guard runs one guest message's work. A panic on a gomobile goroutine ends
// the whole process, which is the host phone's app and everybody's table, so
// whatever a guest sends that trips one costs that guest its tunnel and
// nothing more.
func (t *Tunnel) guard(work func()) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("zolikcore: tunnel %s: recovered: %v", t.peer, r)
			t.Close()
		}
	}()
	work()
}

// who names this guest to the server: to the rate limiters, and in logs.
func (t *Tunnel) who() string { return "ble-" + t.peer }

// serve answers one request in-process, against the same router the
// listeners use. No socket, no loopback round trip.
func (t *Tunnel) serve(m tunnelMsg) {
	if !strings.HasPrefix(m.P, "/") {
		t.send(tunnelMsg{T: "res", ID: m.ID, S: http.StatusBadRequest})
		return
	}
	// http.NewRequest, not httptest.NewRequest: the path is the guest's, and
	// httptest panics on one it cannot parse, which would take the host's
	// whole app down with it.
	req, err := http.NewRequest(cmpOr(m.M, http.MethodGet), "http://ble"+m.P, strings.NewReader(m.B))
	if err != nil {
		t.send(tunnelMsg{T: "res", ID: m.ID, S: http.StatusBadRequest})
		return
	}
	req.RequestURI = m.P
	for k, v := range m.H {
		req.Header.Set(k, v)
	}
	// Set, not passed through: a guest must not be able to name itself as
	// somebody else to the rate limiters.
	req.Header.Set("X-Forwarded-For", t.who())
	req.RemoteAddr = t.who() + ":0"
	rec := httptest.NewRecorder()
	t.host.handler.ServeHTTP(rec, req)

	res := tunnelMsg{T: "res", ID: m.ID, S: rec.Code, B: rec.Body.String()}
	if ra := rec.Header().Get("Retry-After"); ra != "" {
		res.H = map[string]string{"Retry-After": ra}
	}
	t.send(res)
}

// openSocket dials the host's own loopback listener on the guest's behalf
// and pumps frames both ways. Going through a real socket rather than a fake
// one keeps the match runtime's view of a connection exactly what it is for
// every other player: the guest leaving closes it, and presence follows.
func (t *Tunnel) openSocket(m tunnelMsg) {
	if !strings.HasPrefix(m.P, "/ws/") {
		t.send(tunnelMsg{T: "wsc", ID: m.ID})
		return
	}
	hdr := http.Header{}
	hdr.Set("X-Forwarded-For", t.who())
	d := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
	c, _, err := d.Dial("ws://127.0.0.1:"+strconv.Itoa(t.host.Port())+m.P, hdr)
	if err != nil {
		t.send(tunnelMsg{T: "wsc", ID: m.ID})
		return
	}
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		_ = c.Close()
		return
	}
	t.sockets[m.ID] = c
	t.mu.Unlock()
	t.send(tunnelMsg{T: "wsopen", ID: m.ID})

	for {
		_, data, err := c.ReadMessage()
		if err != nil {
			break
		}
		t.send(tunnelMsg{T: "wsm", ID: m.ID, D: string(data)})
	}
	t.mu.Lock()
	_, still := t.sockets[m.ID]
	delete(t.sockets, m.ID)
	t.mu.Unlock()
	if still {
		t.send(tunnelMsg{T: "wsc", ID: m.ID})
	}
}

func cmpOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
