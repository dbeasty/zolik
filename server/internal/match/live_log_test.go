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
		}}, logMark{})
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

// A new round starts the log again: the history is the game in play, not the
// games before it in the match. Seq runs on across the cut.
func TestLiveLogStartsAgainEachRound(t *testing.T) {
	e := &liveMatch{}
	mod := canasta.New()
	players := []string{"a", "b"}
	discard := func(i, round int) {
		e.record(mod, nil, players, []module.Event{{
			Type: "card_discarded",
			Data: map[string]any{"playerId": players[i%2], "card": fmt.Sprintf("c%d", i)},
		}}, logMark{round: round})
	}
	for i := 0; i < 5; i++ {
		discard(i, 1)
	}
	if got := e.recentFor("a", 0); len(got) != 5 || got[4].Round != 1 {
		t.Fatalf("round 1: %d moves, last round %d", len(got), got[len(got)-1].Round)
	}
	for i := 5; i < 8; i++ {
		discard(i, 2)
	}
	got := e.recentFor("b", 0)
	if len(got) != 3 {
		t.Fatalf("round 2 log holds %d moves, want 3 (round 1 dropped)", len(got))
	}
	for _, m := range got {
		if m.Round != 2 {
			t.Fatalf("round 1 move %d left in round 2's log", m.Seq)
		}
	}
	if got[0].Seq != 6 || got[2].Seq != 8 {
		t.Fatalf("seq runs %d..%d, want 6..8", got[0].Seq, got[2].Seq)
	}
}

// A round's log opens with a note that it began and closes with one that it
// ended, in the game's word for a round; a game without rounds says "game".
func TestLiveLogNotesRoundBeginAndEnd(t *testing.T) {
	e := &liveMatch{}
	mod := canasta.New()
	players := []string{"a", "b"}
	ev := []module.Event{{Type: "card_discarded", Data: map[string]any{"playerId": "a", "card": "c1"}}}
	e.record(mod, nil, players, ev, logMark{round: 1, roundLabel: "canasta.round.deal", begins: true})
	e.record(mod, nil, players, ev, logMark{round: 1, roundLabel: "canasta.round.deal", ends: true})
	got := e.recentFor("a", 0)
	keys := []string{}
	for _, m := range got {
		keys = append(keys, m.Fact.LabelKey)
	}
	if len(got) != 4 || keys[0] != "moves.roundBegins" || keys[3] != "moves.roundEnds" {
		t.Fatalf("log keys %v, want a begins note, two moves, an ends note", keys)
	}
	if got[0].Fact.Params["round"] != "canasta.round.deal" || got[0].Fact.Params["n"] != 1 || got[0].PlayerID != "" {
		t.Fatalf("begins note %+v", got[0])
	}
	// The next round's note starts the log again.
	e.record(mod, nil, players, nil, logMark{round: 2, roundLabel: "canasta.round.deal", begins: true})
	if got := e.recentFor("b", 0); len(got) != 1 || got[0].Fact.LabelKey != "moves.roundBegins" || got[0].Seq != 5 {
		t.Fatalf("round 2 log %+v", got)
	}

	plain := &liveMatch{}
	plain.record(mod, nil, players, ev, logMark{begins: true, ends: true})
	if got := plain.recentFor("a", 0); len(got) != 3 || got[0].Fact.LabelKey != "moves.gameBegins" || got[2].Fact.LabelKey != "moves.gameEnds" {
		t.Fatalf("game without rounds: %+v", got)
	}
}

// The table between rounds still shows the round that ended: a seat saying to
// go on is not the next round, and only the move that resumes play begins it.
func TestLogMarkKeepsTheEndedRoundThroughThePause(t *testing.T) {
	open := &module.RoundLog{LabelKey: "x.round"}
	paused := &module.RoundLog{LabelKey: "x.round", Paused: true}
	steps := []struct {
		name          string
		seq           int
		before, after int
		rounds        *module.RoundLog
		want          logMark
	}{
		{"first move", 1, 0, 0, open, logMark{round: 1, roundLabel: "x.round", begins: true}},
		{"a move", 2, 0, 0, open, logMark{round: 1, roundLabel: "x.round"}},
		{"closes round 1", 3, 0, 1, paused, logMark{round: 1, roundLabel: "x.round", ends: true}},
		{"a seat continues", 4, 1, 1, paused, logMark{round: 1, roundLabel: "x.round"}},
		{"the last seat continues", 5, 1, 1, open, logMark{round: 2, roundLabel: "x.round", begins: true}},
		{"a move in round 2", 6, 1, 1, open, logMark{round: 2, roundLabel: "x.round"}},
		{"closes round 2, dealing on", 7, 1, 2, open, logMark{round: 2, roundLabel: "x.round", ends: true}},
		{"round 3 under way", 8, 2, 2, open, logMark{round: 3, roundLabel: "x.round", begins: true}},
	}
	pending := false
	for _, s := range steps {
		var got logMark
		got, pending = logMarkFor(s.seq, s.before, s.after, s.rounds, pending)
		if got != s.want {
			t.Fatalf("%s: mark %+v, want %+v", s.name, got, s.want)
		}
	}
	if got, _ := logMarkFor(1, 0, 0, nil, false); got != (logMark{begins: true}) {
		t.Fatalf("game without rounds, first move: %+v", got)
	}
}
