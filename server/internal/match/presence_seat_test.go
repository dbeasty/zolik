package match_test

import (
	"testing"

	"zolik/server/internal/match"
	"zolik/server/internal/ws"
)

// A seat that is *here*, for tests about who may bring a table back.
//
// Presence is the whole of the new resume rule, and the registry is where the
// runtime keeps it: a player is at the table when they hold a socket in its
// room. Nothing in these tests needs the socket to carry anything, so this
// swallows every write — what is being asserted is that the connection exists
// at all.
type seatedConn struct{}

func (seatedConn) WriteJSON(interface{}) error     { return nil }
func (seatedConn) Close() error                    { return nil }
func (seatedConn) Ping() error                     { return nil }
func (seatedConn) CloseWithCode(int, string) error { return nil }

// sitDown puts playerID at matchID's table, and returns the function that
// stands them back up — because half of these tests are about what happens
// the moment somebody leaves.
func sitDown(t *testing.T, m *match.Manager, matchID, playerID string) (leave func()) {
	t.Helper()
	conn, _ := m.Hub().Registry().Add(matchID, playerID, seatedConn{})
	return func() { m.Hub().Registry().RemoveIfCurrent(matchID, playerID, conn) }
}

var _ ws.WSConn = seatedConn{}
