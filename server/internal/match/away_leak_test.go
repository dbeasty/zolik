package match

import (
	"context"
	"strings"
	"testing"
	"time"

	"zolik/server/internal/module"
)

func (m *Manager) awayEntries(matchID string) int {
	m.awayMu.Lock()
	defer m.awayMu.Unlock()
	n := 0
	for k := range m.awaySince {
		if strings.HasPrefix(k, matchID+"|") {
			n++
		}
	}
	return n
}

// An away clock is cleared by its seat coming back, and nothing else: a match
// that finished with somebody away kept their entry for as long as the process
// lived, and so did every seat whose socket closed on the result screen.
func TestAFinishedMatchKeepsNoAwayClocks(t *testing.T) {
	g := withRounds(t, "prsi")
	h := newLiveHarness(t)
	m := h.startGame(t, g, 1)
	id := m.ID.Hex()
	ctx := context.Background()

	h.m.markAway(id, g.players[1].ID, time.Now())
	// A stop that is not an end keeps it: a suspended table's clock is what
	// brings its stand-in.
	h.m.matchStopped(id, "suspended")
	if h.m.awayEntries(id) != 1 {
		t.Fatal("a suspended table lost its away clock")
	}

	h.play(t, g, id, 5000, func(int, module.State) {})
	cur, err := h.m.current(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if cur.Status != "completed" {
		t.Fatalf("the game did not finish: %s", cur.Status)
	}
	if n := h.m.awayEntries(id); n != 0 {
		t.Fatalf("%d away clocks kept after the match finished", n)
	}

	// A socket closing on the result screen, at a table that offers stand-ins.
	e, err := h.m.lockMatch(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if e.match.Options == nil {
		e.match.Options = module.Options{}
	}
	e.match.Options[module.OptStandInAfter] = 120
	e.mu.Unlock()
	h.m.SuspendOnDisconnect(ctx, id, g.players[0].ID, "socket closed")
	if n := h.m.awayEntries(id); n != 0 {
		t.Fatalf("a socket closing on a finished table started %d away clocks", n)
	}
}
