package zolikcore

import (
	"bytes"
	"compress/flate"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"

	"zolik/server/internal/relay"
)

// A cloud with nothing but the relay on it, admitting any "node-…" token.
func relayCloud(t *testing.T) *httptest.Server {
	t.Helper()
	r := chi.NewRouter()
	relay.NewHandlers(relay.NewHub(), func(token string) (string, error) {
		if id, ok := strings.CutPrefix(token, "node-"); ok {
			return id, nil
		}
		return "", errors.New("not a node")
	}).RegisterRoutes(r)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv
}

// remoteGuest is a guest across the internet: the tunnel protocol, over a
// WebSocket to the relay instead of a radio.
type remoteGuest struct {
	t      *testing.T
	ws     *websocket.Conn
	keys   *sessionKeys
	sent   uint64
	got    uint64
	nextID int64
}

func joinRemotely(t *testing.T, srv *httptest.Server, code string, h *Host) *remoteGuest {
	t.Helper()
	ws, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+"/relay/join/"+code, nil)
	if err != nil {
		t.Fatalf("join: %v", err)
	}
	t.Cleanup(func() { _ = ws.Close() })
	g := &remoteGuest{t: t, ws: ws}
	eph, _ := ecdh.X25519().GenerateKey(rand.Reader)
	_ = ws.WriteMessage(websocket.BinaryMessage, append([]byte{msgHello, tunnelVersion}, eph.PublicKey().Bytes()...))
	welcome := g.read()
	if len(welcome) != 2+64 || welcome[0] != msgWelcome {
		t.Fatalf("welcome = %x", welcome)
	}
	hostStatic, _ := ecdh.X25519().NewPublicKey(welcome[2:34])
	hostEph, _ := ecdh.X25519().NewPublicKey(welcome[34:66])
	if hostStatic.Equal(h.bleKey.PublicKey()) == false {
		t.Fatal("the welcome through the relay carries another table's key")
	}
	keys, _, err := deriveGuest(eph, hostStatic, hostEph)
	if err != nil {
		t.Fatal(err)
	}
	g.keys = keys
	return g
}

func (g *remoteGuest) read() []byte {
	g.t.Helper()
	_ = g.ws.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, data, err := g.ws.ReadMessage()
	if err != nil {
		g.t.Fatalf("read: %v", err)
	}
	return data
}

func (g *remoteGuest) do(method, path string, body any) tunnelMsg {
	g.t.Helper()
	g.nextID++
	raw, _ := json.Marshal(body)
	m := tunnelMsg{T: "req", ID: g.nextID, M: method, P: path, H: map[string]string{"Content-Type": "application/json"}, B: string(raw)}
	plain, _ := json.Marshal(m)
	sealed := g.keys.g2h.Seal([]byte{msgSealed}, nonce(g.sent), append([]byte{0}, plain...), []byte{msgSealed})
	g.sent++
	_ = g.ws.WriteMessage(websocket.BinaryMessage, sealed)
	for i := 0; i < 20; i++ {
		msg := g.read()
		open, err := g.keys.h2g.Open(nil, nonce(g.got), msg[1:], msg[:1])
		if err != nil {
			g.t.Fatalf("a message from the phone failed to open: %v", err)
		}
		g.got++
		b := open[1:]
		if open[0]&flagDeflate != 0 {
			b, _ = io.ReadAll(flate.NewReader(bytes.NewReader(b)))
		}
		var res tunnelMsg
		_ = json.Unmarshal(b, &res)
		if res.T == "res" && res.ID == m.ID {
			return res
		}
	}
	g.t.Fatal("no answer")
	return tunnelMsg{}
}

// The whole door: the phone opens its table to the internet, a guest across
// it sits down through the cloud, and closing the door lets them go.
func TestARemoteGuestSitsDownThroughTheRelay(t *testing.T) {
	srv := relayCloud(t)
	h := startHost(t, t.TempDir())
	if _, err := h.OpenRelay("Ada's phone"); err == nil {
		t.Fatal("a phone nobody enrolled opened a relay")
	}
	h.nodeCredential, h.cloudBaseURL = "node-n1", srv.URL

	code, err := h.OpenRelay("Ada's phone")
	if err != nil {
		t.Fatalf("OpenRelay: %v", err)
	}
	if h.RelayStatus() != RelayOnline || h.RelayURL() != srv.URL+"/r/"+code {
		t.Fatalf("status %s url %s", h.RelayStatus(), h.RelayURL())
	}
	if again, _ := h.OpenRelay("Ada's phone"); again != code {
		t.Errorf("opening twice gave %s then %s", code, again)
	}

	g := joinRemotely(t, srv, code, h)
	res := g.do("POST", "/auth/guest", map[string]string{"guestName": "Remote Rita"})
	if res.S != 200 || !strings.Contains(res.B, "accessToken") {
		t.Fatalf("guest sign-in through the relay: %d %s", res.S, res.B)
	}
	deadline := time.Now().Add(2 * time.Second)
	for h.RelayGuests() != 1 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if h.RelayGuests() != 1 {
		t.Errorf("relay guests = %d, want 1", h.RelayGuests())
	}

	h.CloseRelay()
	_ = g.ws.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, _, err = g.ws.ReadMessage()
	var ce *websocket.CloseError
	if !errors.As(err, &ce) || ce.Code != relay.CloseHostEnded {
		t.Fatalf("after closing, guest read %v; want close %d", err, relay.CloseHostEnded)
	}
	if h.RelayStatus() != RelayOff || h.RelayCode() != "" {
		t.Errorf("after closing: status %s code %q", h.RelayStatus(), h.RelayCode())
	}
}

// A phone that comes back to the foreground with its listener reclaimed is
// revived, and a table that was open to the room and to the internet is
// still open to both afterwards, at the same port and the same code. The
// revive used to put both doors down with the old host and bring the new one
// up shut, while the phone's screen still showed the table as open.
func TestARevivedHostKeepsItsDoorsOpen(t *testing.T) {
	srv := relayCloud(t)
	h := startHost(t, t.TempDir())
	h.nodeCredential, h.cloudBaseURL = "node-n1", srv.URL
	lanPort, err := h.OpenLAN()
	if err != nil {
		t.Fatalf("OpenLAN: %v", err)
	}
	code, err := h.OpenRelay("Ada's phone")
	if err != nil {
		t.Fatalf("OpenRelay: %v", err)
	}

	reclaim(t, h)
	if err := Revive(); err != nil {
		t.Fatalf("Revive: %v", err)
	}
	revived := Current()
	if revived == nil || revived == h {
		t.Fatalf("Revive left %v", revived)
	}
	t.Cleanup(revived.Stop)

	if got := revived.LANPort(); got != lanPort {
		t.Errorf("after the revive the room is on port %d, want %d", got, lanPort)
	}
	deadline := time.Now().Add(5 * time.Second)
	for revived.RelayCode() == "" && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if got := revived.RelayCode(); got != code {
		t.Fatalf("after the revive the relay code is %q (status %s), want %q", got, revived.RelayStatus(), code)
	}

	// And a guest can reach the revived table both ways.
	g := joinRemotely(t, srv, code, revived)
	if res := g.do("POST", "/auth/guest", map[string]string{"guestName": "Remote Rita"}); res.S != 200 {
		t.Fatalf("guest sign-in through the relay after the revive: %d %s", res.S, res.B)
	}
	res, err := http.Get("http://127.0.0.1:" + strconv.Itoa(lanPort) + "/nearby/info")
	if err != nil {
		t.Fatalf("the room's port after the revive: %v", err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Errorf("/nearby/info on the room's port: %s", res.Status)
	}
}
