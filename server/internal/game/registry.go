package game

import (
	"sync"
)

// WSConn is the subset of gorilla/websocket used by the game hub.
type WSConn interface {
	WriteJSON(v interface{}) error
	Close() error
}

type ConnRegistry struct {
	mu    sync.RWMutex
	conns map[string]map[string]WSConn // gameId -> playerId -> conn
}

func NewConnRegistry() *ConnRegistry {
	return &ConnRegistry{conns: map[string]map[string]WSConn{}}
}

func (r *ConnRegistry) Add(gameID, playerID string, conn WSConn) (prev WSConn) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.conns[gameID] == nil {
		r.conns[gameID] = map[string]WSConn{}
	}
	prev = r.conns[gameID][playerID]
	r.conns[gameID][playerID] = conn
	return prev
}

// RemoveIfCurrent removes the registered connection for playerID only if it
// is still exactly conn. If the player has since reconnected (a newer conn
// replaced this one via Add), this is a no-op and returns false — the caller
// should not treat that as a real disconnect (e.g. should not suspend the
// game), since the player is in fact still connected via the newer conn.
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

