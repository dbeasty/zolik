package gamemcp

import (
	"math/rand"
	"strings"
	"testing"
)

type tableCase struct {
	name      string
	in        NewTableIn
	maxPlays  int
	wantMoves bool
}

func seed(n int64) *int64 { return &n }

var tableCases = []tableCase{
	{name: "zolik 2 seats classic", in: NewTableIn{Game: "zolik", Seats: 2, ClaudeSeats: []int{0}, Opponents: []string{"hard"}, Seed: seed(3)}},
	{name: "zolik 4 seats 35 floor clean run", in: NewTableIn{Game: "zolik", Seats: 4, ClaudeSeats: []int{0, 2},
		Options: map[string]int{"initialMeldMinimum": 35, "requireCleanRun": 1}, Opponents: []string{"hard", "medium"}, Seed: seed(4)}},
	{name: "zolik 2 seats continental", in: NewTableIn{Game: "zolik", Variation: "continental", Seats: 2, ClaudeSeats: []int{1}, Seed: seed(5)}},
	{name: "canasta 4 seats", in: NewTableIn{Game: "canasta", Seats: 4, ClaudeSeats: []int{0}, Opponents: []string{"hard"}, Seed: seed(6)}},
	{name: "canasta 2 seats", in: NewTableIn{Game: "canasta", Seats: 2, ClaudeSeats: []int{1}, Opponents: []string{"medium"}, Seed: seed(7)}},
	{name: "holdem 3 seats", in: NewTableIn{Game: "holdem", Seats: 3, ClaudeSeats: []int{0}, Opponents: []string{"hard"}, Seed: seed(8)}},
}

// A claude seat playing random legal moves reaches the end of a deal at
// every table shape, with no refusal and no stall, and the match moves on.
func TestPlayToTheEndOfADeal(t *testing.T) {
	for _, tc := range tableCases {
		t.Run(tc.name, func(t *testing.T) {
			svc := NewService()
			out := mustCall[NewTableOut](t, svc, "new_table", tc.in)
			if !strings.Contains(out.Observation.Text, "table "+out.Table) {
				t.Fatalf("observation text does not name the table:\n%s", out.Observation.Text)
			}
			rnd := rand.New(rand.NewSource(1))
			plays := playRandomly(t, svc, out.Table, rnd, 5000, func(tb *Table) bool { return tb.RoundsLen > 0 })
			tb := svc.tables[out.Table]
			if tb.RoundsLen == 0 {
				if done, _ := tb.finished(); !done {
					t.Fatalf("no deal finished after %d plays", plays)
				}
			}
			if tb.Illegal != 0 {
				t.Fatalf("%d bot moves refused", tb.Illegal)
			}
			if plays == 0 {
				t.Fatal("the claude seat never moved")
			}
			var ended bool
			for _, e := range tb.Log {
				for _, l := range e.Public {
					if strings.HasPrefix(l, "Round ") || strings.HasPrefix(l, "Match over") {
						ended = true
					}
				}
			}
			if !ended {
				t.Fatalf("the log never says the deal ended")
			}
			t.Logf("%d claude moves, %d actions", plays, tb.Actions)
		})
	}
}

func TestPlayRefusals(t *testing.T) {
	svc := NewService()
	out := mustCall[NewTableOut](t, svc, "new_table", tableCases[0].in)
	bad := len(out.Observation.Moves) + 3
	if _, err := svc.Call("play", mustJSON(t, PlayIn{Table: out.Table, Seat: 0, Move: &bad})); err == nil || !strings.Contains(err.Error(), "no move") {
		t.Fatalf("out-of-range move: %v", err)
	}
	zero := 0
	if _, err := svc.Call("play", mustJSON(t, PlayIn{Table: out.Table, Seat: 1, Move: &zero})); err == nil || !strings.Contains(err.Error(), "bot") {
		t.Fatalf("a bot's seat: %v", err)
	}
	// An explicit action the engine refuses comes back in words: here, a
	// card the seat does not hold.
	held := map[string]bool{}
	for _, z := range out.Observation.View.Zones {
		if z.OwnerID == "p0" {
			for _, c := range z.Cards {
				held[c.Card] = true
			}
		}
	}
	missing := ""
	for _, r := range "A23456789TJQK" {
		for _, su := range "SHDC" {
			if c := string(r) + string(su); !held[c] && missing == "" {
				missing = c
			}
		}
	}
	_, err := svc.Call("play", mustJSON(t, map[string]any{"table": out.Table, "seat": 0,
		"action": map[string]any{"verb": "discard", "cards": []string{missing}}}))
	if err == nil {
		t.Fatalf("discarding %s, which the seat does not hold, was accepted", missing)
	}
	t.Logf("refusal: %v", err)
	if _, err := svc.Call("play", []byte(`{"table":"`+out.Table+`","seat":0,"mvoe":1}`)); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("misspelt argument: %v", err)
	}
}

// The same seed and seating deal and play the same match.
func TestDeterministicFromSeed(t *testing.T) {
	a, b := firstMoves(t, 40), firstMoves(t, 40)
	if a != b {
		t.Fatalf("two runs differ:\n%s\n---\n%s", a, b)
	}
}

// firstMoves plays n random claude moves at the 4-seat Žolíky table and
// returns the public history.
func firstMoves(t *testing.T, n int) string {
	t.Helper()
	svc := NewService()
	o := mustCall[NewTableOut](t, svc, "new_table", tableCases[1].in)
	rnd := rand.New(rand.NewSource(9))
	for i := 0; i < n; i++ {
		tb := svc.tables[o.Table]
		if done, _ := tb.finished(); done {
			break
		}
		for _, s := range tb.Seats {
			if s.Claude && tb.isAwaited(s.Player) {
				obs := mustCall[Observation](t, svc, "observe", ObserveIn{Table: o.Table, Seat: s.Index})
				k := rnd.Intn(len(obs.Moves))
				mustCall[PlayOut](t, svc, "play", PlayIn{Table: o.Table, Seat: s.Index, Move: &k})
				break
			}
		}
	}
	return mustCall[ReplayOut](t, svc, "replay_log", ReplayIn{Table: o.Table}).Text
}
