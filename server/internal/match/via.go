package match

import "context"

// Which way each seat reached a table hosted on a device (internal/netvia).
//
// In memory, and only while the seat's socket is open: a route is a fact about
// a connection, not about a player, and a player who comes back over Wi-Fi
// after joining across the internet is on Wi-Fi now.

// noteVia records a seat's route, or forgets it with "", and reports whether
// that changed what the table shows.
func (m *Manager) noteVia(matchID, playerID, via string) bool {
	key := matchID + "|" + playerID
	m.awayMu.Lock()
	defer m.awayMu.Unlock()
	if m.via[key] == via {
		return false
	}
	if via == "" {
		delete(m.via, key)
		return true
	}
	if m.via == nil {
		m.via = map[string]string{}
	}
	m.via[key] = via
	return true
}

// announceVia tells the table a seat's route changed: nothing about the match
// moved, so nothing else would.
func (m *Manager) announceVia(ctx context.Context, matchID string) {
	if cur, err := m.current(ctx, matchID); err == nil && (cur.Status == "active" || cur.Status == "suspended" || cur.Status == "lobby") {
		m.Broadcast(cur)
	}
}

// viaOf is the route a seat is connected by, or "" when it is not connected
// or the table is not on a device.
func (m *Manager) viaOf(matchID, playerID string) string {
	m.awayMu.Lock()
	defer m.awayMu.Unlock()
	return m.via[matchID+"|"+playerID]
}
