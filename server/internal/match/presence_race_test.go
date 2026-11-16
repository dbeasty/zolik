package match_test

import (
	"testing"
	"time"
)

// The old socket's cleanup can run after the player's new socket has
// registered and resumed: nothing orders a socket that died on its own against
// the reconnect that follows it. Suspending then paused the table on a player
// who was sitting at it, every move refused as MATCH_NOT_ACTIVE, and nothing
// left to resume it, since the resume had already happened.
func TestALateSuspendDoesNotPauseAPlayerWhoIsBack(t *testing.T) {
	ctx := t.Context()
	m, id, gone, stays := standInTable(t, nil)
	sitDown(t, m, id, stays)

	// The old socket is gone; the new one is registered and has resumed.
	leaveOld := sitDown(t, m, id, gone)
	leaveOld()
	sitDown(t, m, id, gone)
	m.ResumeIfReturning(ctx, id, gone)

	// Only now does the old socket's deferred cleanup get the lock.
	m.SuspendOnDisconnect(ctx, id, gone, "socket closed")

	cur, err := m.Current(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if cur.Status != "active" {
		t.Fatalf("status %q with the player at the table, want active", cur.Status)
	}
	// Nor is a stand-in clock started for somebody who never left.
	time.Sleep(300 * time.Millisecond)
	if seat(t, m, id, gone).StandIn != nil {
		t.Fatal("a bot took the seat of a player who is at the table")
	}
}
