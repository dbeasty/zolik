package ws

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// WSConn is the subset of gorilla/websocket used by the game hub.
type WSConn interface {
	WriteJSON(v interface{}) error
	Close() error
	Ping() error
	// CloseWithCode closes the connection after writing a WS close control
	// frame carrying code and reason, so the peer's onclose handler can tell
	// why the socket ended instead of seeing an ordinary TCP close, which
	// reads identically to a dropped network.
	CloseWithCode(code int, reason string) error
}

// CloseCodeDisplaced is the close code sent to a player's older connection
// when a newer one (another tab, most often) has taken their seat — as
// opposed to an ordinary network drop. It is in the 4000-4999 private-use
// range reserved for applications by RFC 6455.
const CloseCodeDisplaced = 4001

// syncConn serializes all writes to one underlying connection. gorilla's
// websocket.Conn allows only one concurrent writer — without this, a
// broadcast (hub/registry path, e.g. from another player's action or the AI
// loop) and this connection's own read-loop writing a direct error response
// (handler.go, e.g. a rejected lay_meld) can race on the same socket. The
// loser's write is not queued or retried, so a symptom looks like a
// game_state or error message that silently never arrives — e.g. a batch of
// several lay_meld calls where one is quietly dropped with no error shown.
type syncConn struct {
	mu   sync.Mutex
	conn WSConn
}

func (c *syncConn) WriteJSON(v interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.WriteJSON(v)
}

func (c *syncConn) Close() error {
	return c.conn.Close()
}

func (c *syncConn) CloseWithCode(code int, reason string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.CloseWithCode(code, reason)
}

func (c *syncConn) Ping() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.Ping()
}

type ConnRegistry struct {
	mu    sync.RWMutex
	conns map[string]map[string]*syncConn // gameId -> playerId -> conn
}

func NewConnRegistry() *ConnRegistry {
	return &ConnRegistry{conns: map[string]map[string]*syncConn{}}
}

// Add registers conn for playerID and returns a write-serializing wrapper
// around it — callers must use the returned WSConn (not the raw conn they
// passed in) for any further direct writes and for RemoveIfCurrent, so those
// writes share the same lock as broadcasts routed through this registry.
func (r *ConnRegistry) Add(gameID, playerID string, conn WSConn) (wrapped WSConn, prev WSConn) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.conns[gameID] == nil {
		r.conns[gameID] = map[string]*syncConn{}
	}
	if p := r.conns[gameID][playerID]; p != nil {
		prev = p
	}
	sc := &syncConn{conn: conn}
	r.conns[gameID][playerID] = sc
	return sc, prev
}

// RemoveIfCurrent removes the registered connection for playerID only if it
// is still exactly conn (the wrapped WSConn returned by Add). If the player
// has since reconnected (a newer conn replaced this one via Add), this is a
// no-op and returns false — the caller should not treat that as a real
// disconnect (e.g. should not suspend the game), since the player is in fact
// still connected via the newer conn.
func (r *ConnRegistry) RemoveIfCurrent(gameID, playerID string, conn WSConn) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.conns[gameID] == nil {
		return false
	}
	if r.conns[gameID][playerID] != conn {
		return false
	}
	delete(r.conns[gameID], playerID)
	if len(r.conns[gameID]) == 0 {
		delete(r.conns, gameID)
	}
	return true
}

// Has reports whether playerID already holds a connection in this room.
//
// Admission control leans on this to tell a reconnect from a new arrival. A
// reconnecting player's socket displaces their own dead one via Add, so it
// costs no net capacity, and refusing it would strand someone mid-hand — so it
// is admitted even when the server has stopped taking anyone else.
func (r *ConnRegistry) Has(gameID, playerID string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.conns[gameID] == nil {
		return false
	}
	return r.conns[gameID][playerID] != nil
}

// WriteJSON sends payload to playerID's connection in gameID, if there is one.
//
// The connection is looked up under the registry lock and written to after
// releasing it. The write is a socket write: to a dead peer whose TCP buffer
// is full it blocks until the write deadline. Done under r.mu, that one stuck
// socket held a read lock, the next Add or RemoveIfCurrent queued for the
// write lock behind it, and every later RLock queued behind *that* — so one
// dead phone froze every broadcast to every table on the instance. Writes to
// one connection stay serialized by its syncConn, which is all gorilla needs.
func (r *ConnRegistry) WriteJSON(gameID, playerID string, payload interface{}) bool {
	r.mu.RLock()
	conn := r.conns[gameID][playerID]
	r.mu.RUnlock()
	if conn == nil {
		return false
	}
	_ = conn.WriteJSON(payload)
	return true
}

func (r *ConnRegistry) ForGame(gameID string) map[string]WSConn {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := map[string]WSConn{}
	for pid, c := range r.conns[gameID] {
		out[pid] = c
	}
	return out
}

// PingableConn adapts gorilla's *websocket.Conn — which has no plain
// Ping() error method, only WriteControl — to the WSConn interface, so a
// keepalive ticker goes through the same registry wrapper (and its write
// mutex) as every other write to this socket.
//
// It lives here, beside the registry it is registered with, rather than in
// whichever handler happened to need it first — every game's socket handler
// needs the same adapter, and a second copy of it would be a second place to
// get the control-frame deadline wrong.
type PingableConn struct {
	*websocket.Conn
}

// Limits shared by every socket a client opens to the game server.
const (
	// MaxActionMessageBytes bounds one inbound frame on a socket that carries
	// player actions (the match socket). The largest legitimate message is a
	// module.Action: a verb, an offer id, a few params and at most a whole
	// hand of card ids — well under 4 KiB even for a two-deck rummy hand.
	// Without a limit gorilla reads a frame of any size into memory, so one
	// guest could exhaust the process with a single multi-gigabyte frame.
	MaxActionMessageBytes = 64 << 10

	// MaxIdleMessageBytes bounds one inbound frame on a socket the client
	// only listens on (the waiting room, notifications). Nothing a client
	// sends there means anything; the read loop exists to notice a dead peer.
	MaxIdleMessageBytes = 4 << 10

	// WriteWait bounds one write to a client. A dead peer's full TCP buffer
	// otherwise blocks a write for as long as the kernel keeps retrying —
	// many minutes — holding that connection's write lock all the while.
	WriteWait = 10 * time.Second
)

// writeWait is WriteWait, as a variable only so a test can shorten it.
var writeWait = WriteWait

// WriteJSON writes v with a deadline. A write that times out leaves a gorilla
// connection unusable for writing, so it is closed as well: the read loop
// then fails and the handler's own cleanup removes it from the registry,
// rather than leaving a seat that looks connected but can hear nothing.
func (c PingableConn) WriteJSON(v interface{}) error {
	_ = c.SetWriteDeadline(time.Now().Add(writeWait))
	err := c.Conn.WriteJSON(v)
	if err != nil {
		_ = c.Conn.Close()
	}
	return err
}

func (c PingableConn) Ping() error {
	return c.WriteControl(websocket.PingMessage, nil, time.Now().Add(10*time.Second))
}

func (c PingableConn) CloseWithCode(code int, reason string) error {
	_ = c.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(code, reason), time.Now().Add(2*time.Second))
	return c.Close()
}

// Totals reports how many rooms currently hold at least one connection, and
// how many connections there are across all of them.
//
// Rooms, not games: the lobby's waiting room rides this registry under a
// reserved id of its own (see lobby.RoomID), so a caller that wants a count of
// *matches* has to subtract it — CountRoom is what that is for.
//
// This is per-process: it counts sockets held by *this* instance, not by the
// fleet. With more than one instance behind a load balancer the figure is a
// share of the whole, which is why the console labels it as this instance's
// rather than as the total.
func (r *ConnRegistry) Totals() (rooms, conns int) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, players := range r.conns {
		if len(players) == 0 {
			continue
		}
		rooms++
		conns += len(players)
	}
	return rooms, conns
}

// CountRoom reports how many connections one room holds.
func (r *ConnRegistry) CountRoom(roomID string) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.conns[roomID])
}
