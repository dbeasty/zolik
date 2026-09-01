package rummytiles

import (
	"testing"

	"zolik/server/internal/module"
)

// TestConformance_TakesRealTurnsAndNeverSticks is not expected to finish a
// match — place/add/take/split are Composite, so a driver with no taste for
// combinations can never actually lay a tile, the same honest limit
// extensibility-plan.md §1.6 records for the Continental case. What this
// proves instead: draw, reset_turn and commit alone are enough to keep a
// match moving indefinitely without ever hitting a state with nothing legal
// to do, at 2, 3 and 4 seats.
func TestConformance_TakesRealTurnsAndNeverSticks(t *testing.T) {
	m := New()
	// reset_turn is deliberately always enabled (see deadend_test.go), so it
	// must not out-rank draw here — a driver preferring it first would just
	// spin on a no-op forever, which is "never sticks" in only a technical
	// sense.
	prefer := []string{"commit", "draw", "swap_joker", "reset_turn"}
	for _, seats := range []int{2, 3, 4} {
		t.Run(seatsLabel(seats), func(t *testing.T) {
			refs := players(seats)
			for seed := int64(1); seed <= 5; seed++ {
				state, err := m.NewMatch(module.MatchConfig{}, refs, seed)
				if err != nil {
					t.Fatalf("seed %d: NewMatch: %v", seed, err)
				}
				_, res, err := module.PlayWithOffers(m, state, refs, module.DriverOptions{
					MaxActions: 600, Prefer: prefer,
				})
				if err != nil {
					t.Fatalf("seed %d: %v", seed, err)
				}
				if res.Actions == 0 {
					t.Fatalf("seed %d: no progress at all", seed)
				}
				if res.Verbs["draw"] == 0 {
					t.Errorf("seed %d: a driver with no combinations to offer should still be drawing every turn", seed)
				}
			}
		})
	}
}

// TestConformance_BotPlaysAWholeMatchToAWinner is the version driven by the
// module's own bot rather than the naive offer driver — the one that
// actually lays tiles, since the bot composes moves directly instead of
// following SubmissionFor.
//
// A round limit, not a target score: the greedy bot (§3.6 — lay what is
// formable, extend what exists, draw otherwise) is "enough for a first
// playable opponent", not a strong one, and it goes out rarely enough that a
// pool-exhausted round — which scores every hand negative, the winner
// included — can trend a two-bot match's scores downward indefinitely
// against a fixed positive target. A round limit always terminates
// regardless of how the score trends, which is exactly what it is for.
func TestConformance_BotPlaysAWholeMatchToAWinner(t *testing.T) {
	m := New()
	b := bot{}
	for _, seats := range []int{2, 3, 4} {
		t.Run(seatsLabel(seats), func(t *testing.T) {
			refs := players(seats)
			for seed := int64(1); seed <= 3; seed++ {
				cfg := module.MatchConfig{Options: module.Options{OptTargetScore: 0, OptRoundLimit: 8}}
				state, err := m.NewMatch(cfg, refs, seed)
				if err != nil {
					t.Fatalf("seed %d: NewMatch: %v", seed, err)
				}
				finished := false
				var winners []string
				for step := 0; step < 2500; step++ {
					done, w, err := m.Finished(state)
					if err != nil {
						t.Fatalf("seed %d step %d: Finished: %v", seed, step, err)
					}
					if done {
						finished, winners = true, w
						break
					}
					actor, ok := activePlayer(t, m, state, refs)
					if !ok {
						t.Fatalf("seed %d step %d: nobody has an offer, but the match is not finished", seed, step)
					}
					offers, err := m.LegalActions(state, actor)
					if err != nil {
						t.Fatalf("seed %d step %d: LegalActions: %v", seed, step, err)
					}
					a, ok := b.Act(state, module.BotSeat{PlayerID: actor}, offers)
					if !ok {
						t.Fatalf("seed %d step %d: bot had no move", seed, step)
					}
					next, _, err := m.Apply(state, actor, a)
					if err != nil {
						t.Fatalf("seed %d step %d: bot chose %+v but Apply refused: %v", seed, step, a, err)
					}
					state = next
				}
				if !finished {
					t.Fatalf("seed %d: did not finish", seed)
				}
				if len(winners) != 1 {
					t.Fatalf("seed %d: expected exactly one winner, got %v", seed, winners)
				}
			}
		})
	}
}
