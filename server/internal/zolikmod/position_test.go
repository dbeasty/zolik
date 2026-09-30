package zolikmod

import (
	"testing"

	"zolik/server/internal/learn/learntest"
	"zolik/server/internal/module"
)

// Positional answers exactly what the raw-state methods do, at every seat of
// every position the adapter's walker reaches — every ruleset and table, late
// Continental deals included — from one decode shared by all of them. The
// candidate search branches through the engine from that decode, which is
// what this pins most of all: nothing it tries may reach back into the shared
// state. Consecutive states are one move apart, and are the reward pairs.
func TestPositionalMatchesRawPath(t *testing.T) {
	moves := 300
	if testing.Short() {
		moves = 100
	}
	checked := 0
	for i, tb := range learnTables {
		w := newLearnWalker(t, tb.variation, tb.seats, 900+int64(i))
		var ids []string
		for _, p := range w.players {
			ids = append(ids, p.ID)
		}
		states := []module.State{w.state}
		flush := func() {
			checked += learntest.CheckPositional(t, learnGame{}, ids, states)
		}
		for n := 0; n < moves; n++ {
			seat := w.actor()
			if seat == "" {
				flush()
				w.deal()
				states = []module.State{w.state}
				continue
			}
			cands, _ := learnGame{}.Candidates(w.state, seat, learnOffers(t, w.m, w.state, seat))
			if !w.move(seat, cands) {
				flush()
				w.deal()
				states = []module.State{w.state}
				continue
			}
			states = append(states, w.state)
		}
		flush()
	}
	if checked < 500 {
		t.Fatalf("only %d seat-positions checked", checked)
	}
}
