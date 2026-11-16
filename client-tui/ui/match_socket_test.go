package ui

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gorilla/websocket"
)

// fakeMatchServer answers the two things the match screen talks to: the
// HTTP read of a match and its live socket. Every socket it accepts streams
// `states` boards, then waits for the client to hang up, and closed counts
// the sockets the client has hung up.
type fakeMatchServer struct {
	*httptest.Server
	states  int
	opened  atomic.Int32
	closed  atomic.Int32
	gotAuth atomic.Value // the Authorization header the HTTP read carried
}

func newFakeMatchServer(t *testing.T, states int) *fakeMatchServer {
	t.Helper()
	f := &fakeMatchServer{states: states}
	f.gotAuth.Store("")
	up := websocket.Upgrader{}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/ws/matches/"):
			c, err := up.Upgrade(w, r, nil)
			if err != nil {
				return
			}
			f.opened.Add(1)
			go func() {
				for i := 1; i <= f.states; i++ {
					msg := fmt.Sprintf(`{"type":"match_state","matchId":"m1","status":"playing","joinCode":"S%d"}`, i)
					if c.WriteMessage(websocket.TextMessage, []byte(msg)) != nil {
						break
					}
					time.Sleep(2 * time.Millisecond)
				}
			}()
			// Block until the client goes away.
			for {
				if _, _, err := c.ReadMessage(); err != nil {
					f.closed.Add(1)
					_ = c.Close()
					return
				}
			}
		case strings.HasPrefix(r.URL.Path, "/matches/"):
			f.gotAuth.Store(r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"type":"match_state","matchId":"m1","status":"playing","joinCode":"HTTP"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(f.Close)
	return f
}

// driver plays the part of tea.Program: every command runs on its own
// goroutine and its message comes back to Update on this one, which is the
// concurrency the real runtime has and the one the old reader got wrong.
type driver struct {
	t    *testing.T
	root *Root
	msgs chan tea.Msg
	wg   sync.WaitGroup
}

func newDriver(t *testing.T, root *Root) *driver {
	return &driver{t: t, root: root, msgs: make(chan tea.Msg, 64)}
}

func (d *driver) run(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		if msg := cmd(); msg != nil {
			d.msgs <- msg
		}
	}()
}

func (d *driver) send(msg tea.Msg) {
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			d.run(c)
		}
		return
	}
	_, cmd := d.root.Update(msg)
	d.run(cmd)
}

// until pumps messages until cond holds or the deadline passes.
func (d *driver) until(what string, cond func() bool) {
	d.t.Helper()
	deadline := time.After(3 * time.Second)
	tick := time.NewTicker(5 * time.Millisecond)
	defer tick.Stop()
	for !cond() {
		select {
		case msg := <-d.msgs:
			d.send(msg)
		case <-tick.C:
		case <-deadline:
			d.t.Fatalf("timed out waiting for %s", what)
		}
	}
}

// enterMatch takes a Root from the lobby into a started match, the way a
// player gets there.
func enterMatch(t *testing.T, srv *fakeMatchServer) (*Root, *driver) {
	t.Helper()
	root := NewRoot(nil, srv.URL, PlayerSession{AccessToken: "tok", UserID: "u1"}, Build{})
	root.screen = ScreenLobby
	root.lobby.matchID = "m1"
	d := newDriver(t, root)
	d.send(lobbyStateMsg{})
	if root.screen != ScreenMatch {
		t.Fatalf("screen = %v after the match started, want the match", root.screen)
	}
	return root, d
}

// The socket has exactly one reader. The old screen re-armed its socket
// reader after *every* state, including the one the plain HTTP read
// delivered, so from then on two goroutines sat in ReadMessage on one
// gorilla connection — which gorilla does not allow, and which -race reports.
func TestMatchSocketHasOneReader(t *testing.T) {
	srv := newFakeMatchServer(t, 40)
	root, d := enterMatch(t, srv)
	d.until("the last board", func() bool { return root.match.state.JoinCode == "S40" })
	if got := srv.gotAuth.Load().(string); got != "Bearer tok" {
		t.Errorf("the HTTP read carried Authorization %q: without the token it is a spectator's view, with every hand empty", got)
	}
	d.send(tea.KeyMsg{Type: tea.KeyEsc})
	d.until("the socket to close", func() bool { return srv.closed.Load() == 1 })
}

// A board from the socket is never replaced by the HTTP read, which may
// have been answered before the move the socket reported.
func TestHTTPStateDoesNotOverwriteTheSocket(t *testing.T) {
	srv := newFakeMatchServer(t, 1)
	root, d := enterMatch(t, srv)
	d.until("the socket's board", func() bool { return root.match.state.JoinCode == "S1" })
	// Let the HTTP read land, whenever it does.
	time.Sleep(50 * time.Millisecond)
	for len(d.msgs) > 0 {
		d.send(<-d.msgs)
	}
	if got := root.match.state.JoinCode; got != "S1" {
		t.Errorf("board = %q, want the socket's S1", got)
	}
	root.Close()
}

// Quitting from inside a match hangs up the socket. ctrl+c used to return
// tea.Quit with the socket still open, and over SSH the process outlives the
// session — so every player who quit mid-game left a socket and a reader
// goroutine behind on the server until it restarted.
func TestQuitClosesTheMatchSocket(t *testing.T) {
	srv := newFakeMatchServer(t, 1)
	root, d := enterMatch(t, srv)
	d.until("the socket's board", func() bool { return root.match.state.JoinCode == "S1" })
	_, cmd := root.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("ctrl+c did not quit")
	}
	deadline := time.Now().Add(2 * time.Second)
	for srv.closed.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if srv.closed.Load() == 0 {
		t.Fatal("quitting left the match socket open")
	}
}

// An SSH session that simply drops (the terminal closed, the network went)
// never sends ctrl+c; the server calls Close when the session ends.
func TestCloseHangsUpTheMatchSocket(t *testing.T) {
	srv := newFakeMatchServer(t, 1)
	root, d := enterMatch(t, srv)
	d.until("the socket's board", func() bool { return root.match.state.JoinCode == "S1" })
	root.Close()
	root.Close() // twice is harmless
	deadline := time.Now().Add(2 * time.Second)
	for srv.closed.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if srv.closed.Load() == 0 {
		t.Fatal("Close left the match socket open")
	}
}

// Leaving one match and starting the next never lets the first socket's
// late messages land on the second match's screen.
func TestALeftMatchSocketIsIgnored(t *testing.T) {
	srv := newFakeMatchServer(t, 1)
	root, d := enterMatch(t, srv)
	d.until("the socket's board", func() bool { return root.match.state.JoinCode == "S1" })
	old := root.match.conn
	d.send(tea.KeyMsg{Type: tea.KeyEsc})
	if root.screen != ScreenMainMenu {
		t.Fatalf("screen = %v after leaving, want the menu", root.screen)
	}
	root.match.state.JoinCode = "MENU"
	d.send(wsStateMsg{conn: old, state: root.match.state})
	d.send(matchClosedMsg{conn: old})
	if root.match.status != "" {
		t.Errorf("a stale socket's message reached the screen: status %q", root.match.status)
	}
}
