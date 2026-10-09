package zolikcore

// The guest's end of the tunnel, and a gateway that makes a table reached
// through it look like any other table.
//
// A guest at a table over Bluetooth or the internet has no HTTP route to the
// host. tunnel.go is the host's end of what crosses instead: a handshake and
// sealed messages that carry requests and sockets. The phones run the guest's
// end in the page (src/net/ble/transport.ts); the Mac app has its web view
// kept off the network, and any window may show any game, so the guest's end
// lives here instead, where it belongs to the app rather than to a page.
//
// A GuestTable is one such tunnel. It serves the table on a loopback
// address ("http://127.0.0.1:port"): every request and socket a page makes
// there is carried through the tunnel. To a page, a table across the
// internet is then an address like one on Wi-Fi, and every window can use it.
//
// The tunnel is opened lazily and reopened after a drop, by the next request
// or socket, exactly as the page's was; the gateway's address stays the same,
// so windows already open carry on.

import (
	"bytes"
	"compress/flate"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Close codes the relay ends a guest's link with (server/internal/relay).
const (
	relayHostGone  = 4002
	relayHostEnded = 4003
)

// ErrHostKeyChanged is what joining a table says when it answers with a
// different key from the one this guest pinned for it.
var ErrHostKeyChanged = errors.New("BLE_HOST_KEY_CHANGED")

// guestLink is one connection to a host carrying whole messages. Messages it
// receives go to onMessage; when it ends, onClose is called once.
type guestLink interface {
	send(msg []byte) error
	close()
}

// guestDialer opens a fresh link: the first one, and after a drop.
type guestDialer func(onMessage func([]byte), onClose func(code int)) (guestLink, error)

type guestSession struct {
	link      guestLink
	g2h, h2g  cipher.AEAD
	sendCount uint64
	recvCount uint64
	check     string

	welcome chan []byte // the first message, until the handshake is done
	keys    bool
}

// guestTunnel is the guest's end: handshake, sealing, and requests and
// sockets multiplexed over whatever link the dialer opens.
type guestTunnel struct {
	dial   guestDialer
	pinned []byte

	ensureMu sync.Mutex // one handshake at a time

	mu      sync.Mutex
	live    *guestSession
	hostKey []byte
	check   string
	closed  bool
	away    string // "", "away" or "ended": why the host is not there
	nextID  int64
	pending map[int64]chan tunnelMsg
	sockets map[int64]*guestSocket
}

func newGuestTunnel(dial guestDialer, pinned []byte) *guestTunnel {
	return &guestTunnel{
		dial: dial, pinned: pinned,
		pending: map[int64]chan tunnelMsg{}, sockets: map[int64]*guestSocket{},
	}
}

// guestSocket is one socket through the tunnel.
type guestSocket struct {
	id     int64
	opened chan struct{}
	in     chan string
	done   chan struct{}
	once   sync.Once
}

func (s *guestSocket) end() { s.once.Do(func() { close(s.done) }) }

func (t *guestTunnel) hello() ([]byte, *ecdh.PrivateKey, error) {
	eph, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	return append([]byte{msgHello, tunnelVersion}, eph.PublicKey().Bytes()...), eph, nil
}

// ensure returns the live session, shaking hands first if there is none.
func (t *guestTunnel) ensure() (*guestSession, error) {
	t.ensureMu.Lock()
	defer t.ensureMu.Unlock()
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return nil, errors.New("the table was left")
	}
	if t.live != nil {
		s := t.live
		t.mu.Unlock()
		return s, nil
	}
	t.mu.Unlock()

	sess := &guestSession{welcome: make(chan []byte, 1)}
	helloMsg, eph, err := t.hello()
	if err != nil {
		return nil, err
	}
	link, err := t.dial(
		func(msg []byte) { t.receive(sess, msg) },
		func(code int) { t.dropped(sess, code) },
	)
	if err != nil {
		return nil, err
	}
	sess.link = link
	if err := link.send(helloMsg); err != nil {
		link.close()
		return nil, err
	}
	var welcome []byte
	select {
	case welcome = <-sess.welcome:
	case <-time.After(15 * time.Second):
		link.close()
		return nil, errors.New("the table did not answer")
	}
	if len(welcome) != 2+64 || welcome[0] != msgWelcome || welcome[1] != tunnelVersion {
		link.close()
		return nil, errors.New("not a welcome from a table this app can talk to")
	}
	hostStatic, err := ecdh.X25519().NewPublicKey(welcome[2:34])
	if err != nil {
		link.close()
		return nil, err
	}
	hostEph, err := ecdh.X25519().NewPublicKey(welcome[34:66])
	if err != nil {
		link.close()
		return nil, err
	}
	if len(t.pinned) > 0 && !bytes.Equal(t.pinned, hostStatic.Bytes()) {
		link.close()
		return nil, ErrHostKeyChanged
	}
	ee, err := eph.ECDH(hostEph)
	if err != nil {
		link.close()
		return nil, err
	}
	es, err := eph.ECDH(hostStatic)
	if err != nil {
		link.close()
		return nil, err
	}
	keys, check, err := schedule(ee, es, eph.PublicKey().Bytes(), hostStatic.Bytes(), hostEph.Bytes())
	if err != nil {
		link.close()
		return nil, err
	}
	t.mu.Lock()
	sess.g2h, sess.h2g, sess.check = keys.g2h, keys.h2g, check
	sess.keys = true
	t.live = sess
	t.hostKey = hostStatic.Bytes()
	t.check = check
	t.away = ""
	t.mu.Unlock()
	return sess, nil
}

// receive takes one message from the host's link.
func (t *guestTunnel) receive(sess *guestSession, msg []byte) {
	t.mu.Lock()
	if !sess.keys {
		t.mu.Unlock()
		select {
		case sess.welcome <- append([]byte(nil), msg...):
		default:
		}
		return
	}
	if len(msg) == 0 || msg[0] != msgSealed {
		t.mu.Unlock()
		sess.link.close()
		return
	}
	plain, err := sess.h2g.Open(nil, nonce(sess.recvCount), msg[1:], msg[:1])
	if err != nil || len(plain) == 0 {
		t.mu.Unlock()
		// A message that does not open is an attack or a broken link; the
		// counters no longer agree either way.
		sess.link.close()
		return
	}
	sess.recvCount++
	t.mu.Unlock()

	body := plain[1:]
	if plain[0]&flagDeflate != 0 {
		r := flate.NewReader(bytes.NewReader(body))
		out, err := io.ReadAll(io.LimitReader(r, 4<<20))
		_ = r.Close()
		if err != nil {
			sess.link.close()
			return
		}
		body = out
	}
	var m tunnelMsg
	if json.Unmarshal(body, &m) != nil {
		return
	}
	t.deliver(m)
}

func (t *guestTunnel) deliver(m tunnelMsg) {
	switch m.T {
	case "res":
		t.mu.Lock()
		ch := t.pending[m.ID]
		delete(t.pending, m.ID)
		t.mu.Unlock()
		if ch != nil {
			ch <- m
		}
	case "wsopen":
		t.mu.Lock()
		s := t.sockets[m.ID]
		t.mu.Unlock()
		if s != nil {
			select {
			case <-s.opened:
			default:
				close(s.opened)
			}
		}
	case "wsm":
		t.mu.Lock()
		s := t.sockets[m.ID]
		t.mu.Unlock()
		if s != nil {
			select {
			case s.in <- m.D:
			case <-s.done:
			}
		}
	case "wsc":
		t.mu.Lock()
		s := t.sockets[m.ID]
		delete(t.sockets, m.ID)
		t.mu.Unlock()
		if s != nil {
			s.end()
		}
	}
}

// dropped is the link going away: everything waiting on it fails, and the
// next request or socket reconnects.
func (t *guestTunnel) dropped(sess *guestSession, code int) {
	t.mu.Lock()
	if t.live == sess {
		t.live = nil
	}
	switch code {
	case relayHostGone:
		t.away = "away"
	case relayHostEnded:
		t.away = "ended"
	}
	pending := t.pending
	socks := t.sockets
	t.pending = map[int64]chan tunnelMsg{}
	t.sockets = map[int64]*guestSocket{}
	t.mu.Unlock()
	for _, ch := range pending {
		close(ch)
	}
	for _, s := range socks {
		s.end()
	}
}

// post seals and sends one message.
func (t *guestTunnel) post(v tunnelMsg) error {
	sess, err := t.ensure()
	if err != nil {
		return err
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return err
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
	// Sealing and sending share one lock so the counter order is the order
	// on the link.
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.live != sess {
		return errors.New("the link to the table dropped")
	}
	sealed := sess.g2h.Seal([]byte{msgSealed}, nonce(sess.sendCount), plain, []byte{msgSealed})
	sess.sendCount++
	return sess.link.send(sealed)
}

func (t *guestTunnel) newID() int64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.nextID++
	return t.nextID
}

// request carries one HTTP request to the host and waits for its answer.
func (t *guestTunnel) request(method, path string, headers map[string]string, body string, wait time.Duration) (tunnelMsg, error) {
	id := t.newID()
	ch := make(chan tunnelMsg, 1)
	t.mu.Lock()
	t.pending[id] = ch
	t.mu.Unlock()
	if err := t.post(tunnelMsg{T: "req", ID: id, M: method, P: path, H: headers, B: body}); err != nil {
		t.mu.Lock()
		delete(t.pending, id)
		t.mu.Unlock()
		return tunnelMsg{}, err
	}
	select {
	case res, ok := <-ch:
		if !ok {
			return tunnelMsg{}, errors.New("the link to the table dropped")
		}
		return res, nil
	case <-time.After(wait):
		t.mu.Lock()
		delete(t.pending, id)
		t.mu.Unlock()
		return tunnelMsg{}, errors.New("the table did not answer")
	}
}

// openSocket asks the host to open a socket at path and waits for it.
func (t *guestTunnel) openSocket(path string) (*guestSocket, error) {
	id := t.newID()
	s := &guestSocket{id: id, opened: make(chan struct{}), in: make(chan string, 64), done: make(chan struct{})}
	t.mu.Lock()
	t.sockets[id] = s
	t.mu.Unlock()
	forget := func() {
		t.mu.Lock()
		delete(t.sockets, id)
		t.mu.Unlock()
	}
	if err := t.post(tunnelMsg{T: "wso", ID: id, P: path}); err != nil {
		forget()
		return nil, err
	}
	select {
	case <-s.opened:
		return s, nil
	case <-s.done:
		forget()
		return nil, errors.New("the table refused the socket")
	case <-time.After(15 * time.Second):
		forget()
		_ = t.post(tunnelMsg{T: "wsc", ID: id})
		return nil, errors.New("the table did not open the socket")
	}
}

func (t *guestTunnel) closeSocket(s *guestSocket) {
	t.mu.Lock()
	delete(t.sockets, s.id)
	t.mu.Unlock()
	s.end()
	_ = t.post(tunnelMsg{T: "wsc", ID: s.id})
}

func (t *guestTunnel) close() {
	t.mu.Lock()
	t.closed = true
	sess := t.live
	t.mu.Unlock()
	if sess != nil {
		sess.link.close()
	}
}

// ---- the relay as a link ----------------------------------------------------

// relayDialer dials the cloud's relay for a table's code: one WebSocket
// carrying the tunnel's binary messages.
func relayDialer(cloudBase, code string) guestDialer {
	return func(onMessage func([]byte), onClose func(code int)) (guestLink, error) {
		target := socketURL(cloudBase) + "/relay/join/" + url.PathEscape(code)
		d := websocket.Dialer{HandshakeTimeout: 15 * time.Second}
		ws, res, err := d.Dial(target, nil)
		if err != nil {
			if res != nil {
				return nil, fmt.Errorf("could not reach the table: %s", res.Status)
			}
			return nil, fmt.Errorf("could not reach the table: %w", err)
		}
		l := &wsLink{ws: ws}
		go func() {
			for {
				kind, data, err := ws.ReadMessage()
				if err != nil {
					code := 1006
					var ce *websocket.CloseError
					if errors.As(err, &ce) {
						code = ce.Code
					}
					onClose(code)
					return
				}
				if kind == websocket.BinaryMessage {
					onMessage(data)
				}
			}
		}()
		return l, nil
	}
}

type wsLink struct {
	ws *websocket.Conn
	mu sync.Mutex
}

func (l *wsLink) send(msg []byte) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	_ = l.ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return l.ws.WriteMessage(websocket.BinaryMessage, msg)
}

func (l *wsLink) close() { _ = l.ws.Close() }

// ---- the gateway ------------------------------------------------------------

// GuestTable is a table this app reaches through a tunnel, served on
// loopback. Windows use BaseURL like any other table's address.
type GuestTable struct {
	tun        *guestTunnel
	instanceID string
	ln         net.Listener
	srv        *http.Server
}

var (
	guestMu     sync.Mutex
	guestTables = map[string]*GuestTable{}
)

// JoinRelay sits down at the table with this relay code, through the cloud,
// and serves it on loopback. instanceID names the table (the cloud's info
// for the code said which); pinnedKey is the table's key as this guest last
// saw it, base64, or empty for a table it has never sat at: a different key
// refuses the join. The tunnel is opened now, so a table that cannot be
// reached fails here. One table per instance: joining again replaces the
// first.
func JoinRelay(code, instanceID, pinnedKey string) (*GuestTable, error) {
	netMu.Lock()
	base := cloudBase
	netMu.Unlock()
	if base == "" {
		return nil, errors.New("no server address")
	}
	var pinned []byte
	if pinnedKey != "" {
		b, err := base64.StdEncoding.DecodeString(pinnedKey)
		if err != nil {
			return nil, fmt.Errorf("bad pinned key: %w", err)
		}
		pinned = b
	}
	t := newGuestTunnel(relayDialer(base, strings.ToUpper(code)), pinned)
	return serveGuest(t, instanceID)
}

func serveGuest(t *guestTunnel, instanceID string) (*GuestTable, error) {
	if _, err := t.ensure(); err != nil {
		return nil, err
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.close()
		return nil, err
	}
	g := &GuestTable{tun: t, instanceID: instanceID, ln: ln}
	g.srv = &http.Server{Handler: http.HandlerFunc(g.handle), ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = g.srv.Serve(ln) }()

	guestMu.Lock()
	old := guestTables[instanceID]
	guestTables[instanceID] = g
	guestMu.Unlock()
	if old != nil {
		old.Close()
	}
	return g, nil
}

// BaseURL is where the table answers on this machine.
func (g *GuestTable) BaseURL() string { return "http://" + g.ln.Addr().String() }

// InstanceID is the table's.
func (g *GuestTable) InstanceID() string { return g.instanceID }

// HostKey is the table's static key, base64, for the guest to pin.
func (g *GuestTable) HostKey() string {
	g.tun.mu.Lock()
	defer g.tun.mu.Unlock()
	return base64.StdEncoding.EncodeToString(g.tun.hostKey)
}

// CheckCode is the four characters the host's screen shows for this link,
// changing at every reconnect.
func (g *GuestTable) CheckCode() string {
	g.tun.mu.Lock()
	defer g.tun.mu.Unlock()
	return g.tun.check
}

// HostAway is why the table is not there, when it is not: "away" (the
// host's device went), "ended" (the host closed it) or empty.
func (g *GuestTable) HostAway() string {
	g.tun.mu.Lock()
	defer g.tun.mu.Unlock()
	return g.tun.away
}

// Close leaves the table: the tunnel ends, and its sockets with it.
func (g *GuestTable) Close() {
	guestMu.Lock()
	if guestTables[g.instanceID] == g {
		delete(guestTables, g.instanceID)
	}
	guestMu.Unlock()
	g.tun.close()
	_ = g.srv.Close()
	_ = g.ln.Close()
}

// LeaveGuestTables leaves every table reached through a tunnel: signing in
// elsewhere, back to online play, or quitting.
func LeaveGuestTables() {
	guestMu.Lock()
	all := make([]*GuestTable, 0, len(guestTables))
	for _, g := range guestTables {
		all = append(all, g)
	}
	guestMu.Unlock()
	for _, g := range all {
		g.Close()
	}
}

// LeaveGuestTable leaves the one table with this instance id.
func LeaveGuestTable(instanceID string) {
	guestMu.Lock()
	g := guestTables[instanceID]
	guestMu.Unlock()
	if g != nil {
		g.Close()
	}
}

var gatewayUpgrader = websocket.Upgrader{
	// Only this machine reaches the gateway, and the app's pages are not
	// served from it, so there is no origin to check.
	CheckOrigin: func(*http.Request) bool { return true },
}

func (g *GuestTable) handle(w http.ResponseWriter, r *http.Request) {
	if websocket.IsWebSocketUpgrade(r) {
		g.handleSocket(w, r)
		return
	}
	var body string
	if r.Body != nil {
		b, err := io.ReadAll(io.LimitReader(r.Body, 8<<20))
		if err != nil {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}
		body = string(b)
	}
	headers := map[string]string{}
	for _, k := range []string{"Authorization", "Content-Type", "Accept", "Accept-Language", "If-None-Match"} {
		if v := r.Header.Get(k); v != "" {
			headers[k] = v
		}
	}
	res, err := g.tun.request(r.Method, r.URL.RequestURI(), headers, body, 20*time.Second)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	for k, v := range res.H {
		w.Header().Set(k, v)
	}
	if res.B != "" && w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "application/json")
	}
	status := res.S
	if status == 0 {
		status = http.StatusBadGateway
	}
	w.WriteHeader(status)
	_, _ = io.WriteString(w, res.B)
}

func (g *GuestTable) handleSocket(w http.ResponseWriter, r *http.Request) {
	s, err := g.tun.openSocket(r.URL.RequestURI())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	c, err := gatewayUpgrader.Upgrade(w, r, nil)
	if err != nil {
		g.tun.closeSocket(s)
		return
	}
	defer c.Close()
	// Tunnel → page.
	go func() {
		for {
			select {
			case data := <-s.in:
				if c.WriteMessage(websocket.TextMessage, []byte(data)) != nil {
					return
				}
			case <-s.done:
				// Drain what arrived before the end, then hang up.
				for {
					select {
					case data := <-s.in:
						_ = c.WriteMessage(websocket.TextMessage, []byte(data))
					default:
						_ = c.Close()
						return
					}
				}
			}
		}
	}()
	// Page → tunnel.
	for {
		_, data, err := c.ReadMessage()
		if err != nil {
			break
		}
		if g.tun.post(tunnelMsg{T: "wsm", ID: s.id, D: string(data)}) != nil {
			break
		}
	}
	g.tun.closeSocket(s)
}

// GuestStatus is what a page asks about a table held here: JSON with the
// check code of the current link and why the table is away, if it is.
func GuestStatus(instanceID string) string {
	guestMu.Lock()
	g := guestTables[instanceID]
	guestMu.Unlock()
	out := map[string]string{"checkCode": "", "away": ""}
	if g != nil {
		out["checkCode"], out["away"] = g.CheckCode(), g.HostAway()
	}
	raw, _ := json.Marshal(out)
	return string(raw)
}
