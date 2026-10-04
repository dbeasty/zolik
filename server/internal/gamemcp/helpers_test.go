package gamemcp

import (
	"encoding/json"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"zolik/server/internal/learn"
)

func mustJSON(t testing.TB, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func mustCall[Out any](t testing.TB, svc *Service, tool string, args any) Out {
	t.Helper()
	raw, err := json.Marshal(args)
	if err != nil {
		t.Fatal(err)
	}
	out, err := svc.Call(tool, raw)
	if err != nil {
		t.Fatalf("%s %s: %v", tool, raw, err)
	}
	return out.(Out)
}

// randomModel writes a network of the game's widths with random weights, as
// the learn tests build one: it scores candidates differently, which is all
// a test of the plumbing needs.
func randomModel(t testing.TB, gameID string, seed int64) string {
	t.Helper()
	g, err := learn.LookupGame(gameID)
	if err != nil {
		t.Fatal(err)
	}
	rnd := rand.New(rand.NewSource(seed))
	stack := func(in int, widths []int) []learn.Dense {
		var ls []learn.Dense
		for _, w := range widths {
			d := learn.Dense{In: in, Out: w, W: make([]float32, in*w), B: make([]float32, w)}
			for i := range d.W {
				d.W[i] = float32(rnd.NormFloat64() / 8)
			}
			for i := range d.B {
				d.B[i] = float32(rnd.NormFloat64() / 8)
			}
			ls = append(ls, d)
			in = w
		}
		return ls
	}
	n := &learn.Net{Game: g.Name(), StateDim: g.StateDim(), CandDim: g.CandDim(),
		Trunk:  stack(g.StateDim(), []int{32}),
		Scorer: stack(32+g.CandDim(), []int{16, 1}),
		Value:  stack(32, []int{8, 1}),
	}
	b, err := n.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), gameID+".bin")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// playRandomly plays the claude seats of a table with uniformly random
// legal moves until stop says so, the match ends, or maxPlays runs out. It
// returns how many moves it made.
func playRandomly(t testing.TB, svc *Service, table string, rnd *rand.Rand, maxPlays int, stop func(*Table) bool) int {
	t.Helper()
	for plays := 0; plays < maxPlays; plays++ {
		tb := svc.tables[table]
		if done, _ := tb.finished(); done || stop(tb) {
			return plays
		}
		if tb.Stalled != "" {
			t.Fatalf("table stalled: %s", tb.Stalled)
		}
		acted := false
		for _, s := range tb.Seats {
			if !s.Claude || !tb.isAwaited(s.Player) {
				continue
			}
			obs := mustCall[Observation](t, svc, "observe", ObserveIn{Table: table, Seat: s.Index})
			if !obs.YourTurn || len(obs.Moves) == 0 {
				t.Fatalf("seat %d is awaited but has no moves:\n%s", s.Index, obs.Text)
			}
			i := rnd.Intn(len(obs.Moves))
			mustCall[PlayOut](t, svc, "play", PlayIn{Table: table, Seat: s.Index, Move: &i})
			acted = true
			break
		}
		if !acted {
			t.Fatalf("nobody to move and the match is not over; awaited %v", tb.awaited())
		}
	}
	t.Fatalf("still going after %d moves", maxPlays)
	return maxPlays
}
