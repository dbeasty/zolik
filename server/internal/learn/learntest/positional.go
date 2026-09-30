// Package learntest holds the checks every learn.Game adapter runs in its own
// tests, over positions its own sweep reaches.
package learntest

import (
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"testing"

	"zolik/server/internal/learn"
	"zolik/server/internal/module"
)

// CheckPositional pins an adapter's learn.Positional to its raw-state methods
// over a run of states at a table of these seats: consecutive states are one reward's before and after,
// and at every state every seat is asked for its encoding and its candidates.
//
// Three things are compared, and all must agree to the bit:
//
//   - the raw path, which decodes afresh for every call — what every caller
//     did before Positional existed;
//   - one shared Position per state, asked by every seat at once from its own
//     goroutine (so `go test -race` sees any write to what a Position holds);
//   - the same shared Position asked again afterwards, which differs from the
//     first answers if an earlier call changed what it decoded.
//
// A Position is also reused as the next pair's "before", as the environment
// reuses it. Returns the number of seat-positions compared.
func CheckPositional(tb testing.TB, g learn.Game, seats []string, states []module.State) int {
	tb.Helper()
	view, ok := g.(learn.Positional)
	if !ok {
		tb.Fatalf("%s does not implement learn.Positional", g.Name())
	}
	m := g.Module()
	checked := 0
	var prevPos learn.Position
	for i, s := range states {
		pos, err := view.Position(s)
		if err != nil {
			tb.Fatalf("%s: state %d: Position: %v", g.Name(), i, err)
		}
		offers := make(map[string][]module.ActionOffer, len(seats))
		for _, seat := range seats {
			o, err := m.LegalActions(s, seat)
			if err != nil {
				o = nil // asked of a seat off turn; both paths get the same nothing
			}
			offers[seat] = o
		}

		want := make(map[string]string, len(seats))
		for _, seat := range seats {
			want[seat] = answer(g.Encode(s, seat)) + "\n" + answer(g.Candidates(s, seat, offers[seat]))
		}
		got := make(map[string]string, len(seats))
		var mu sync.Mutex
		var wg sync.WaitGroup
		for _, seat := range seats {
			wg.Add(1)
			go func(seat string) {
				defer wg.Done()
				a := answer(view.EncodeFor(pos, seat)) + "\n" + answer(view.CandidatesFor(pos, seat, offers[seat]))
				mu.Lock()
				got[seat] = a
				mu.Unlock()
			}(seat)
		}
		wg.Wait()
		for _, seat := range seats {
			if got[seat] != want[seat] {
				tb.Fatalf("%s: state %d seat %s: shared Position answers differently from the raw path\nshared: %.400s\nraw:    %.400s",
					g.Name(), i, seat, got[seat], want[seat])
			}
			again := answer(view.EncodeFor(pos, seat)) + "\n" + answer(view.CandidatesFor(pos, seat, offers[seat]))
			if again != want[seat] {
				tb.Fatalf("%s: state %d seat %s: a Position answered differently the second time — something wrote to it",
					g.Name(), i, seat)
			}
			checked++
		}

		if i > 0 {
			rewards := make(map[string]string, len(seats))
			for _, seat := range seats {
				wg.Add(1)
				go func(seat string) {
					defer wg.Done()
					a := answer3(view.RewardFor(prevPos, pos, seat))
					mu.Lock()
					rewards[seat] = a
					mu.Unlock()
				}(seat)
			}
			wg.Wait()
			for _, seat := range seats {
				if w := answer3(g.Reward(states[i-1], s, seat)); rewards[seat] != w {
					tb.Fatalf("%s: state %d seat %s: RewardFor %s, Reward %s", g.Name(), i, seat, rewards[seat], w)
				}
			}
		}
		prevPos = pos
	}
	return checked
}

// answer renders a result exactly: every float by its bits, and the error by
// its text.
func answer[T any](v T, err error) string {
	if err != nil {
		return "error: " + err.Error()
	}
	b, jerr := json.Marshal(bitsOf(v))
	if jerr != nil {
		return "unmarshalable: " + jerr.Error()
	}
	return string(b)
}

func answer3(r float32, done bool, err error) string {
	if err != nil {
		return "error: " + err.Error()
	}
	return fmt.Sprintf("%08x %v", math.Float32bits(r), done)
}

// bitsOf replaces float32s with their bit patterns, so -0 and NaN compare
// as what they are.
func bitsOf(v any) any {
	switch x := v.(type) {
	case []float32:
		out := make([]uint32, len(x))
		for i, f := range x {
			out[i] = math.Float32bits(f)
		}
		return out
	case []learn.Candidate:
		type cand struct {
			Action   module.Action
			Then     []module.Action
			Features []uint32
		}
		out := make([]cand, len(x))
		for i, c := range x {
			out[i] = cand{c.Action, c.Then, bitsOf(c.Features).([]uint32)}
		}
		return out
	}
	return v
}
