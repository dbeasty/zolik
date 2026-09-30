package holdem

import (
	"testing"

	"zolik/server/internal/learn"
	"zolik/server/internal/learn/learntest"
	"zolik/server/internal/module"
)

// Positional answers exactly what the raw-state methods do, at every seat of
// every decision the adapter sweep reaches, from one decode shared by all of
// them. Consecutive decisions are the reward pairs: a hand ends between many
// of them.
func TestPositionalMatchesRawPath(t *testing.T) {
	matches := 12
	if testing.Short() {
		matches = 4
	}
	checked := 0
	for seed := int64(1); seed <= int64(matches); seed++ {
		seats := []int{2, 3, 6}[seed%3]
		var states []module.State
		playDecisions(t, seats, seed, func(d decision, _ []learn.Candidate) {
			if len(states) < 150 {
				states = append(states, d.state)
			}
		})
		var ids []string
		for _, p := range learn.Players(seats) {
			ids = append(ids, p.ID)
		}
		checked += learntest.CheckPositional(t, learnGame{}, ids, states)
	}
	if checked < 500 {
		t.Fatalf("only %d seat-positions checked", checked)
	}
}
