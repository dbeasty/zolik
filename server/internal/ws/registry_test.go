package ws

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// stuckConn is a socket whose peer has stopped reading: every write blocks
// until the test releases it, as a write to a full TCP buffer would.
type stuckConn struct {
	entered chan struct{}
	release chan struct{}
}

func (c *stuckConn) WriteJSON(interface{}) error {
	select {
	case c.entered <- struct{}{}:
	default:
	}
	<-c.release
	return nil
}
func (c *stuckConn) Close() error                    { return nil }
func (c *stuckConn) Ping() error                     { return nil }
func (c *stuckConn) CloseWithCode(int, string) error { return nil }

// One dead peer must not freeze the instance. The write to it used to happen
// under the registry's read lock, so the next Add queued for the write lock
// behind it and every later reader queued behind that Add: broadcasts to
// every other table stopped until the stuck write gave up.
func TestStuckWriteDoesNotBlockOtherGamesOrAdd(t *testing.T) {
	reg := NewConnRegistry()
	stuck := &stuckConn{entered: make(chan struct{}, 1), release: make(chan struct{})}
	defer close(stuck.release)
	reg.Add("dead-game", "p1", stuck)

	got := make(chan struct{}, 1)
	reg.Add("live-game", "p2", &fakeConn{onWrite: func(interface{}) { got <- struct{}{} }})

	go reg.WriteJSON("dead-game", "p1", map[string]any{"type": "game_state"})
	select {
	case <-stuck.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("the write to the stuck connection never started")
	}

	// A connect queued behind the stuck write…
	added := make(chan struct{})
	go func() {
		w, _ := reg.Add("new-game", "p3", &fakeConn{})
		reg.RemoveIfCurrent("new-game", "p3", w)
		close(added)
	}()
	// …must not hold up a broadcast to an unrelated table, nor itself wait.
	time.Sleep(20 * time.Millisecond)
	wrote := make(chan bool, 1)
	go func() { wrote <- reg.WriteJSON("live-game", "p2", map[string]any{"type": "game_state"}) }()

	deadline := time.After(2 * time.Second)
	for _, step := range []struct {
		name string
		ch   <-chan struct{}
	}{{"Add/RemoveIfCurrent", added}, {"delivery to the live game", got}} {
		select {
		case <-step.ch:
		case <-deadline:
			t.Fatalf("%s blocked behind a stuck write to another game", step.name)
		}
	}
	select {
	case ok := <-wrote:
		if !ok {
			t.Fatal("WriteJSON to the live game reported no connection")
		}
	case <-deadline:
		t.Fatal("WriteJSON to the live game never returned")
	}
}

// A write to a peer that has stopped reading gives up after writeWait
// rather than blocking for as long as the kernel retries, and the connection
// is closed so its read loop ends and its seat is cleaned up.
func TestPingableConnWriteHasADeadline(t *testing.T) {
	serverConn := make(chan *websocket.Conn, 1)
	up := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		serverConn <- c
	}))
	defer srv.Close()

	client, _, err := websocket.DefaultDialer.Dial(strings.Replace(srv.URL, "http://", "ws://", 1), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	c := <-serverConn
	// The client never reads: the server's writes fill the socket buffers
	// and then block until PingableConn's own deadline cuts them off.
	defer func(old time.Duration) { writeWait = old }(writeWait)
	writeWait = 200 * time.Millisecond
	pc := PingableConn{Conn: c}
	big := strings.Repeat("x", 1<<20)
	done := make(chan error, 1)
	go func() {
		for {
			if err := pc.WriteJSON(big); err != nil {
				done <- err
				return
			}
		}
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("a write to a peer that never reads did not time out")
	}
	// And the socket is closed, so the handler's read loop ends.
	if _, _, err := c.ReadMessage(); err == nil {
		t.Fatal("connection still readable after a failed write; want it closed")
	}
}
