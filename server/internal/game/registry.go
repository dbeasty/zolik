package game

import (
	"sync"

	"github.com/gorilla/websocket"
)

type ConnRegistry struct {
	mu    sync.RWMutex
	conns map[string]map[string]*websocket.Conn // gameId -> playerId -> conn
}

func NewConnRegistry() *ConnRegistry {
	return &ConnRegistry{conns: map[string]map[string]*websocket.Conn{}}
}

func (r *ConnRegistry) Add(gameID, playerID string, conn *websocket.Conn) (prev *websocket.Conn) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.conns[gameID] == nil {
		r.conns[gameID] = map[string]*websocket.Conn{}
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

func (r *ConnRegistry) ForGame(gameID string) map[string]*websocket.Conn {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := map[string]*websocket.Conn{}
	for pid, c := range r.conns[gameID] {
		out[pid] = c
	}
	return out
}

