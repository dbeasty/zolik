package match

import (
	"fmt"
	"testing"

	"zolik/server/internal/canasta"
	"zolik/server/internal/module"
)

// The log keeps far more than a broadcast carries, numbers every line, and
// hands out either its tail or the lot.
func TestLiveLogKeepsHistoryAndNumbersIt(t *testing.T) {
	e := &liveMatch{}
	mod := canasta.New()
	players := []string{"a", "b"}
	const n = movesKept + 30
	for i := 0; i < n; i++ {
		e.record(mod, nil, players, []module.Event{{
			Type: "card_discarded",
			Data: map[string]any{"playerId": players[i%2], "card": fmt.Sprintf("c%d", i)},
		}})
	}

	all := e.recentFor("a", 0)
	if len(all) != movesKept {
		t.Fatalf("kept %d moves, want %d", len(all), movesKept)
	}
	if all[0].Seq != n-movesKept+1 || all[len(all)-1].Seq != n {
		t.Fatalf("seq runs %d..%d, want %d..%d", all[0].Seq, all[len(all)-1].Seq, n-movesKept+1, n)
	}
	for i := 1; i < len(all); i++ {
		if all[i].Seq != all[i-1].Seq+1 {
			t.Fatalf("seq gap at %d: %d after %d", i, all[i].Seq, all[i-1].Seq)
		}
	}

	tail := e.recentFor("a", recentMoves)
	if len(tail) != recentMoves || tail[len(tail)-1].Seq != n {
		t.Fatalf("broadcast window: %d moves ending at %d, want %d ending at %d", len(tail), tail[len(tail)-1].Seq, recentMoves, n)
	}
}
