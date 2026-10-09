package relay

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

func newServer(t *testing.T) (*httptest.Server, *Hub) {
	t.Helper()
	hub := NewHub()
	r := chi.NewRouter()
	NewHandlers(hub, func(token string) (string, error) {
		if strings.HasPrefix(token, "node-") {
			return strings.TrimPrefix(token, "node-"), nil
		}
		return "", errors.New("not a node")
	}).RegisterRoutes(r)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv, hub
}

func wsURL(srv *httptest.Server, path string) string {
	return "ws" + strings.TrimPrefix(srv.URL, "http") + path
}

// dialHost opens a phone's connection and returns it with the code it got.
func dialHost(t *testing.T, srv *httptest.Server, node, code string) (*websocket.Conn, string) {
	t.Helper()
	h := http.Header{"Authorization": {"Bearer node-" + node}}
	c, _, err := websocket.DefaultDialer.Dial(wsURL(srv, "/relay/host?instance=inst-"+node+"&name=Ada&protocol=1&code="+code), h)
	if err != nil {
		t.Fatalf("host dial: %v", err)
	}
	var m map[string]any
	_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
	if err := c.ReadJSON(&m); err != nil || m["t"] != "code" {
		t.Fatalf("first host message = %v, %v; want the code", m, err)
	}
	return c, m["code"].(string)
}

func readText(t *testing.T, c *websocket.Conn) map[string]any {
	t.Helper()
	_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
	kind, data, err := c.ReadMessage()
	if err != nil || kind != websocket.TextMessage {
		t.Fatalf("read text: kind %d err %v", kind, err)
	}
	var m map[string]any
	_ = json.Unmarshal(data, &m)
	return m
}

func readBinary(t *testing.T, c *websocket.Conn) []byte {
	t.Helper()
	_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
	kind, data, err := c.ReadMessage()
	if err != nil || kind != websocket.BinaryMessage {
		t.Fatalf("read binary: kind %d err %v", kind, err)
	}
	return data
}

// The whole point: bytes a guest sends reach the phone tagged with who sent
// them, and the phone's answer reaches that guest alone.
func TestARelayCarriesBytesBothWays(t *testing.T) {
	srv, _ := newServer(t)
	host, code := dialHost(t, srv, "n1", "")
	defer host.Close()

	g, _, err := websocket.DefaultDialer.Dial(wsURL(srv, "/relay/join/"+strings.ToLower(code)), nil)
	if err != nil {
		t.Fatalf("guest dial: %v", err)
	}
	defer g.Close()
	open := readText(t, host)
	if open["t"] != "open" {
		t.Fatalf("host heard %v, want open", open)
	}
	id := uint32(open["c"].(float64))

	_ = g.WriteMessage(websocket.BinaryMessage, []byte("hello"))
	frame := readBinary(t, host)
	if binary.BigEndian.Uint32(frame) != id || string(frame[4:]) != "hello" {
		t.Fatalf("host got %q, want guest %d's hello", frame, id)
	}

	reply := make([]byte, 4+5)
	binary.BigEndian.PutUint32(reply, id)
	copy(reply[4:], "world")
	_ = host.WriteMessage(websocket.BinaryMessage, reply)
	if got := readBinary(t, g); string(got) != "world" {
		t.Fatalf("guest got %q, want world", got)
	}

	// A guest leaving is said.
	_ = g.Close()
	if m := readText(t, host); m["t"] != "close" {
		t.Fatalf("host heard %v, want close", m)
	}
}

// The phone going is "the server is offline" to every guest, and the code
// waits for the same phone — and only it — to take it back.
func TestTheHostGoingClosesGuestsAndKeepsItsCode(t *testing.T) {
	srv, hub := newServer(t)
	host, code := dialHost(t, srv, "n1", "")
	g, _, err := websocket.DefaultDialer.Dial(wsURL(srv, "/relay/join/"+code), nil)
	if err != nil {
		t.Fatalf("guest dial: %v", err)
	}
	readText(t, host) // open

	_ = host.Close()
	_ = g.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, err = g.ReadMessage()
	var ce *websocket.CloseError
	if !errors.As(err, &ce) || ce.Code != CloseHostGone {
		t.Fatalf("guest read %v, want close %d", err, CloseHostGone)
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		info, err := hub.Info(code)
		if err != nil {
			t.Fatalf("info: %v", err)
		}
		if !info.Online {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("still online after the host went")
		}
		time.Sleep(10 * time.Millisecond)
	}

	// A guest arriving meanwhile is told so before any socket opens.
	res, _ := http.Get(srv.URL + "/relay/join/" + code)
	if res.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("join while away: status %d, want 503", res.StatusCode)
	}

	// Another phone cannot take the code.
	h := http.Header{"Authorization": {"Bearer node-n2"}}
	other, _, err := websocket.DefaultDialer.Dial(wsURL(srv, "/relay/host?code="+code), h)
	if err == nil {
		_ = other.SetReadDeadline(time.Now().Add(2 * time.Second))
		var m map[string]any
		if other.ReadJSON(&m) == nil && m["code"] == code {
			t.Fatal("another phone took the code")
		}
		other.Close()
	}

	// The same phone does.
	back, again := dialHost(t, srv, "n1", code)
	defer back.Close()
	if again != code {
		t.Fatalf("code after return = %s, want %s", again, code)
	}
	if info, _ := hub.Info(code); !info.Online || info.Name != "Ada" {
		t.Errorf("info after return = %+v", info)
	}
}

// Only an enrolled phone opens a door onto the internet.
func TestOnlyANodeMayHost(t *testing.T) {
	srv, _ := newServer(t)
	_, res, err := websocket.DefaultDialer.Dial(wsURL(srv, "/relay/host"), http.Header{"Authorization": {"Bearer nope"}})
	if err == nil || res == nil || res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("dial without a node credential: err %v status %v", err, res)
	}
}

// A table is not a stadium.
func TestATableTakesSoManyGuests(t *testing.T) {
	srv, _ := newServer(t)
	host, code := dialHost(t, srv, "n1", "")
	defer host.Close()
	var guests []*websocket.Conn
	for i := 0; i < MaxGuests; i++ {
		g, _, err := websocket.DefaultDialer.Dial(wsURL(srv, "/relay/join/"+code), nil)
		if err != nil {
			t.Fatalf("guest %d: %v", i, err)
		}
		guests = append(guests, g)
		readText(t, host)
	}
	defer func() {
		for _, g := range guests {
			g.Close()
		}
	}()
	extra, _, err := websocket.DefaultDialer.Dial(wsURL(srv, "/relay/join/"+code), nil)
	if err != nil {
		return // refused outright: fine
	}
	defer extra.Close()
	_ = extra.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, err = extra.ReadMessage()
	var ce *websocket.CloseError
	if !errors.As(err, &ce) || ce.Code != 4004 {
		t.Fatalf("one guest too many: %v, want close 4004", err)
	}
}
