package learn_test

import (
	"math/rand"
	"testing"

	_ "zolik/server/internal/canasta"
	_ "zolik/server/internal/holdem"
	"zolik/server/internal/learn"
	"zolik/server/internal/module"
	_ "zolik/server/internal/zolikmod"
)

// The three real games, at the tables the shared-agent tests and benchmarks
// play: every seat of a partnership table, and Hold'em's widest ring.
var sharedTables = []struct {
	name, game, variation string
	seats                 int
}{
	{"holdem6", "holdem", "", 6},
	{"canasta4", "canasta", "classic", 4},
	{"zolik4", "zolik", "", 4},
}

func mustGame(tb testing.TB, name string) learn.Game {
	tb.Helper()
	g, err := learn.LookupGame(name)
	if err != nil {
		tb.Fatal(err)
	}
	return g
}

// decisionAt is one position with a real choice in it: a seat on turn with
// more than one candidate.
type decisionAt struct {
	state  module.State
	seat   string
	offers []module.ActionOffer
}

// collectDecisions plays heuristic matches from seed 1 up and keeps the first
// n positions where a seat has a real choice.
func collectDecisions(tb testing.TB, g learn.Game, variation string, seats, n int) []decisionAt {
	tb.Helper()
	m := g.Module()
	var out []decisionAt
	for seed := int64(1); len(out) < n && seed < 200; seed++ {
		players := learn.Players(seats)
		state, err := m.NewMatch(g.Config(seats, variation), players, seed)
		if err != nil {
			tb.Fatal(err)
		}
		for step := 0; step < 3000 && len(out) < n; step++ {
			if done, _, _ := m.Finished(state); done {
				break
			}
			actor := module.ActiveSeat(m, state, players[0].ID, players)
			if actor == "" {
				break
			}
			offers, err := m.LegalActions(state, actor)
			if err != nil {
				tb.Fatal(err)
			}
			if cands, err := g.Candidates(state, actor, offers); err == nil && len(cands) > 1 {
				out = append(out, decisionAt{state, actor, offers})
			}
			seat := module.BotSeat{PlayerID: actor, Skill: module.SkillHard, Seed: seed}
			a, ok := g.Heuristic().Act(state, seat, offers)
			next, _, err := m.Apply(state, actor, a)
			if !ok || err != nil {
				if a, ok = module.ChooseAction(offers, nil); !ok {
					break
				}
				if next, _, err = m.Apply(state, actor, a); err != nil {
					break
				}
			}
			state = next
		}
	}
	if len(out) == 0 {
		tb.Fatalf("%s: no decisions", g.Name())
	}
	return out
}

// randomNet is a network of the game's widths with random weights, built in
// Go: the tests need one that distinguishes candidates, not a good one.
func randomNet(g learn.Game, trunk, scorer []int, seed int64) *learn.Net {
	rnd := rand.New(rand.NewSource(seed))
	stack := func(in int, widths []int, out int) []learn.Dense {
		var ls []learn.Dense
		for _, w := range append(append([]int(nil), widths...), out) {
			if w == 0 {
				continue
			}
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
	emb := trunk[len(trunk)-1]
	return &learn.Net{
		Game: g.Name(), StateDim: g.StateDim(), CandDim: g.CandDim(),
		Trunk:  stack(g.StateDim(), trunk, 0),
		Scorer: stack(emb+g.CandDim(), scorer, 1),
		Value:  stack(emb, []int{32}, 1),
	}
}

func benchBot(g learn.Game, n *learn.Net) module.Bot {
	return learn.NetBot{Game: g, Policy: learn.NewPolicy(n), Fallback: g.Heuristic()}
}
