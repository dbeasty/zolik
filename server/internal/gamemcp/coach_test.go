package gamemcp

import (
	"math"
	"math/rand"
	"strings"
	"testing"
)

// coach gives every listed move one probability, the probabilities sum to
// one, and the Hard bot's pick is named — at every learnable game, for the
// decisions a claude seat actually meets.
func TestCoachScoresEveryMove(t *testing.T) {
	for _, tc := range tableCases {
		t.Run(tc.name, func(t *testing.T) {
			model := randomModel(t, tc.in.Game, 1)
			svc := NewService()
			out := mustCall[NewTableOut](t, svc, "new_table", tc.in)
			rnd := rand.New(rand.NewSource(4))
			coached := 0
			for plays := 0; plays < 60 && coached < 15; plays++ {
				tb := svc.tables[out.Table]
				if done, _ := tb.finished(); done {
					break
				}
				seat := -1
				for _, s := range tb.Seats {
					if s.Claude && tb.isAwaited(s.Player) {
						seat = s.Index
						break
					}
				}
				obs := mustCall[Observation](t, svc, "observe", ObserveIn{Table: out.Table, Seat: seat})
				c := mustCall[CoachOut](t, svc, "coach", CoachIn{Table: out.Table, Seat: seat, Model: model})
				if len(c.Moves) != len(obs.Moves) {
					t.Fatalf("%d scores for %d moves", len(c.Moves), len(obs.Moves))
				}
				sum := 0.0
				for i, m := range c.Moves {
					if m.Prob == nil {
						t.Fatalf("move %d has no probability: %s", i, c.Note)
					}
					if m.Text != obs.Moves[i].Text {
						t.Fatalf("move %d is %q in coach and %q in observe", i, m.Text, obs.Moves[i].Text)
					}
					sum += *m.Prob
				}
				if math.Abs(sum-1) > 1e-9 {
					t.Fatalf("probabilities sum to %v", sum)
				}
				if c.Hard.Text == "" || c.Value == nil {
					t.Fatalf("no heuristic pick or value: %+v", c)
				}
				if !strings.Contains(c.Text, "Hard heuristic would play") {
					t.Fatalf("coach text:\n%s", c.Text)
				}
				coached++
				i := rnd.Intn(len(obs.Moves))
				mustCall[PlayOut](t, svc, "play", PlayIn{Table: out.Table, Seat: seat, Move: &i})
			}
			if coached < 3 {
				t.Fatalf("only %d decisions coached", coached)
			}
		})
	}
}

func TestCoachWithoutModel(t *testing.T) {
	t.Setenv(modelEnv("zolik"), "")
	svc := NewService()
	out := mustCall[NewTableOut](t, svc, "new_table", tableCases[0].in)
	c := mustCall[CoachOut](t, svc, "coach", CoachIn{Table: out.Table, Seat: 0})
	if c.Note == "" || c.Hard.Text == "" || c.Moves[0].Prob != nil {
		t.Fatalf("without a model: %+v", c)
	}
	// A model for another game is refused, not played with.
	if _, err := svc.Call("coach", mustJSON(t, CoachIn{Table: out.Table, Seat: 0, Model: randomModel(t, "holdem", 1)})); err == nil {
		t.Fatal("a Hold'em model scored a Žolíky position")
	}
}

// A trained model can sit at the table, and replay_log's reveal shows what
// it thought of its own decisions once the match is over.
func TestNetOpponentAndReveal(t *testing.T) {
	model := randomModel(t, "holdem", 2)
	svc := NewService()
	out := mustCall[NewTableOut](t, svc, "new_table", NewTableIn{Game: "holdem", Seats: 2, ClaudeSeats: []int{0},
		Opponents: []string{"net:" + model}, Seed: seed(12)})
	if _, err := svc.Call("replay_log", mustJSON(t, ReplayIn{Table: out.Table, Reveal: true})); err == nil {
		t.Fatal("reveal was allowed mid-match")
	}
	playRandomly(t, svc, out.Table, rand.New(rand.NewSource(5)), 3000, func(*Table) bool { return false })
	r := mustCall[ReplayOut](t, svc, "replay_log", ReplayIn{Table: out.Table, Reveal: true})
	audits := 0
	for _, e := range r.Entries {
		if e.Audit != nil {
			audits++
			if e.Audit.ChosenProb <= 0 || e.Audit.ChosenProb > 1+1e-9 || len(e.Audit.Top) == 0 {
				t.Fatalf("audit %+v", e.Audit)
			}
		}
	}
	if audits == 0 {
		t.Fatal("no net decision was audited")
	}
	if !strings.Contains(r.Text, "model: played") || !strings.Contains(r.Text, "Final hands") {
		t.Fatalf("reveal text:\n%s", r.Text)
	}
}
