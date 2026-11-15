package match

import (
	"context"
	"sync/atomic"
	"testing"

	"zolik/server/internal/module"
)

type countingConn struct{ n atomic.Int32 }

func (c *countingConn) WriteJSON(interface{}) error     { c.n.Add(1); return nil }
func (c *countingConn) Close() error                    { return nil }
func (c *countingConn) Ping() error                     { return nil }
func (c *countingConn) CloseWithCode(int, string) error { return nil }

// Moves do not write the envelope, so two boards a move apart share a
// Version: it is the move count that tells them apart. A board from before
// two moves, broadcast after them, is the stale-turn case — the player sees it
// still their turn when it has passed.
func TestABoardFromBeforeAMoveIsNotSentAfterIt(t *testing.T) {
	g := withRounds(t, "prsi")
	h := newLiveHarness(t)
	m := h.startGame(t, g, 1)
	id := m.ID.Hex()
	h.play(t, g, id, 1, func(int, module.State) {})
	before, err := h.m.current(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	h.play(t, g, id, 2, func(int, module.State) {})
	after, err := h.m.current(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if before.Version != after.Version || after.LogSeq != before.LogSeq+2 {
		t.Fatalf("want two moves under one version: before v%d/%d, after v%d/%d",
			before.Version, before.LogSeq, after.Version, after.LogSeq)
	}

	conn := &countingConn{}
	h.m.hub.Registry().Add(id, g.players[0].ID, conn)
	h.m.Broadcast(after)
	if conn.n.Load() != 1 {
		t.Fatalf("the current board was sent %d times, want 1", conn.n.Load())
	}
	h.m.Broadcast(before)
	if conn.n.Load() != 1 {
		t.Fatal("a board from before two moves was sent after them")
	}
	// A reload starts the count from the store, not from nothing.
	h.m.live.forget(id)
	reloaded, err := h.m.current(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.LogSeq != after.LogSeq {
		t.Fatalf("reloaded at move %d, want %d", reloaded.LogSeq, after.LogSeq)
	}
}
