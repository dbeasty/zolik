package canasta

import (
	"testing"

	"zolik/server/internal/learn/learntest"
	"zolik/server/internal/module"
)

// Positional answers exactly what the raw-state methods do, at every seat of
// every position the adapter's walker reaches — random learners and the
// heuristic, openings under raised minimums, every table — from one decode
// shared by all of them. Consecutive states are one move apart, and are the
// reward pairs.
func TestPositionalMatchesRawPath(t *testing.T) {
	moves := 400
	if testing.Short() {
		moves = 150
	}
	checked := 0
	for _, tb := range learnTables {
		w := newWalker(t, tb.variation, tb.seats, 900+int64(tb.seats))
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
			offers, _ := w.m.LegalActions(w.state, seat)
			cands, _ := learnGame{}.Candidates(w.state, seat, offers)
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
