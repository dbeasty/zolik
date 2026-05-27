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

func (r *ConnRegistry) Remove(gameID, playerID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.conns[gameID] == nil {
		return
	}
	if c := r.conns[gameID][playerID]; c != nil {
		_ = c.Close()
	}
	delete(r.conns[gameID], playerID)
	if len(r.conns[gameID]) == 0 {
		delete(r.conns, gameID)
	}
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

