package gamemcp

import (
	"math/rand"
	"testing"
)

// The games with no learn adapter are played from their offer lists: a
// table deals, a claude seat is shown moves, and random ones are accepted.
func TestGamesWithoutAdapter(t *testing.T) {
	for _, g := range listGames() {
		if g.Learnable {
			continue
		}
		t.Run(g.ID, func(t *testing.T) {
			svc := NewService()
			seats := g.MinSeats
			if v := g.Variations; len(v) > 0 && v[0].MinSeats > seats {
				seats = v[0].MinSeats
			}
			out := mustCall[NewTableOut](t, svc, "new_table", NewTableIn{Game: g.ID, Seats: seats, ClaudeSeats: []int{0}, Seed: seed(1)})
			rnd := rand.New(rand.NewSource(1))
			played := 0
			for i := 0; i < 60; i++ {
				tb := svc.tables[out.Table]
				if done, _ := tb.finished(); done || tb.Stalled != "" {
					break
				}
				obs := mustCall[Observation](t, svc, "observe", ObserveIn{Table: out.Table, Seat: 0})
				if !obs.YourTurn {
					t.Fatalf("advance stopped with claude not on turn:\n%s", obs.Text)
				}
				if len(obs.Moves) == 0 {
					t.Logf("no enumerable move (composite offers only):\n%s", obs.Text)
					break
				}
				k := rnd.Intn(len(obs.Moves))
				if _, err := svc.Call("play", mustJSON(t, PlayIn{Table: out.Table, Seat: 0, Move: &k})); err != nil {
					t.Fatalf("offered move %q refused: %v", obs.Moves[k].Text, err)
				}
				played++
			}
			t.Logf("%s: %d moves played", g.ID, played)
		})
	}
}
