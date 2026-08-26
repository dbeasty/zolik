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
}

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

func (r *ConnRegistry) WriteJSON(gameID, playerID string, payload interface{}) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.conns[gameID] == nil {
		return false
	}
	conn := r.conns[gameID][playerID]
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

// Totals reports how many rooms currently hold at least one connection, and
// how many connections there are across all of them.
//
// Rooms, not games: the lobby's waiting room rides this registry under a
// reserved id of its own (see lobby.RoomID), so a caller that wants a count of
// *matches* has to subtract it — CountRoom is what that is for.
//
// This is per-process: it counts sockets held by *this* instance, not by the
// fleet. With more than one instance behind a load balancer the figure is a
// share of the whole, which is why the admin overview labels it as this
// instance's rather than as the total.
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

func (c PingableConn) Ping() error {
	return c.WriteControl(websocket.PingMessage, nil, time.Now().Add(10*time.Second))
}
