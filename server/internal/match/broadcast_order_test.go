package match_test

import (
	"encoding/json"
	"sync"
	"testing"

	"zolik/server/internal/ws"
)

// stateRecorder is a seat's socket that keeps every match_state it is sent.
type stateRecorder struct {
	mu     sync.Mutex
	states []map[string]any
}

func (r *stateRecorder) WriteJSON(v interface{}) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	var msg map[string]any
	if err := json.Unmarshal(raw, &msg); err != nil {
		return err
	}
	if msg["type"] != "match_state" {
		return nil
	}
	r.mu.Lock()
	r.states = append(r.states, msg)
	r.mu.Unlock()
	return nil
}
func (r *stateRecorder) Close() error                    { return nil }
func (r *stateRecorder) Ping() error                     { return nil }
func (r *stateRecorder) CloseWithCode(int, string) error { return nil }

func (r *stateRecorder) last(t *testing.T) map[string]any {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.states) == 0 {
		t.Fatal("no state was sent")
	}
	return r.states[len(r.states)-1]
}

var _ ws.WSConn = (*stateRecorder)(nil)

// A board is copied under the match lock and sent after it is released, so a
// copy taken before a write can reach the wire after the write's own
// broadcast. It used to be sent anyway, and was the last thing the player saw:
// here a table that had been suspended went on reading as active on the screen
// of the player still at it.
func TestAnOlderBoardIsNotSentAfterANewerOne(t *testing.T) {
	ctx := t.Context()
	m, id, gone, stays := standInTable(t, nil)
	rec := &stateRecorder{}
	m.Hub().Registry().Add(id, stays, rec)

	before, err := m.Current(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	// The write, and its own broadcast.
	m.SuspendOnDisconnect(ctx, id, gone, "test")
	if got := rec.last(t)["status"]; got != "suspended" {
		t.Fatalf("after the suspend the seat saw %v, want suspended", got)
	}
	// The copy from before it, arriving late.
	m.Broadcast(before)
	if got := rec.last(t)["status"]; got != "suspended" {
		t.Fatalf("a board from before the suspend was sent after it: the seat now sees %v", got)
	}

	// And a broadcast that writes nothing — somebody arriving — still goes out.
	now, err := m.Current(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	n := len(rec.states)
	m.Broadcast(now)
	if len(rec.states) != n+1 {
		t.Fatal("a broadcast of the current board was dropped")
	}
}
