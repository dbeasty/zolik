// Command valuecal measures how well a trained network's value head predicts
// what a seat goes on to earn: the network plays every seat (temperature 0),
// and at each real decision (two or more candidates) the value head's
// estimate is set beside the reward the seat actually collected from there
// to the end of the episode (the deal, the hand), in the value head's unit.
//
//	go run ./cmd/valuecal -game zolik -model final.bin -seeds 50
//
// It prints the correlation, the mean error, the slope of a fit of the actual
// return on the prediction, and a table of returns by predicted decile. The
// seeds default to the held-out range the benches use.
//
// The head measured is the exported one. A model trained with a
// perfect-information critic exports its plain head, and the critic itself
// is measured by ml/valuecal.py. A model for an earlier encoder the game only
// appended to plays on its prefix (learn.Narrowed).
//
// Ported from claude/search-on-policy, without the search's per-game value
// scale: the unit here is the reward's own.
package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"runtime"
	"sort"
	"sync"

	"zolik/server/internal/learn"
	"zolik/server/internal/module"

	_ "zolik/server/internal/canasta"
	_ "zolik/server/internal/holdem"
	_ "zolik/server/internal/zolikmod"
)

type sample struct{ pred, actual float64 }

func main() {
	game := flag.String("game", "zolik", "game")
	seats := flag.Int("seats", 2, "seats")
	variation := flag.String("variation", "", "variation")
	model := flag.String("model", "", "model file")
	first := flag.Int64("first", 1_000_000, "first seed")
	seeds := flag.Int("seeds", 50, "matches to play")
	flag.Parse()

	g, err := learn.LookupGame(*game)
	if err != nil {
		fail(err)
	}
	c, err := learn.ParseContender(g, "net:"+*model)
	if err != nil {
		fail(err)
	}
	net := c.Bot.(learn.NetBot)
	scale := 1.0
	out := make([][]sample, *seeds)
	jobs := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < runtime.GOMAXPROCS(0); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				out[i] = play(g, net, *seats, *variation, *first+int64(i), scale)
			}
		}()
	}
	for i := 0; i < *seeds; i++ {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	var all []sample
	for _, s := range out {
		all = append(all, s...)
	}
	report(all)
}

// play runs one match and returns every decision's prediction and outcome.
func play(g learn.Game, net learn.NetBot, seats int, variation string, seed int64, scale float64) []sample {
	m := g.Module()
	view := learn.Positions(net.Game)
	players := learn.Players(seats)
	state, err := m.NewMatch(g.Config(seats, variation), players, seed)
	if err != nil {
		fail(err)
	}
	pos, _ := view.Position(state)
	// open is each seat's predictions awaiting their episode's end, and the
	// reward collected since each was made.
	type pending struct {
		pred, acc float64
	}
	open := map[string][]*pending{}
	var done []sample
	for step := 0; step < 50_000; step++ {
		if fin, _, _ := m.Finished(state); fin {
			break
		}
		actor := module.ActiveSeat(m, state, players[0].ID, players)
		if actor == "" {
			break
		}
		offers, err := m.LegalActions(state, actor)
		if err != nil {
			fail(err)
		}
		if cands, err := view.CandidatesFor(pos, actor, offers); err == nil && len(cands) > 1 {
			if obs, err := view.EncodeFor(pos, actor); err == nil {
				n := net.Policy.Net()
				open[actor] = append(open[actor], &pending{pred: float64(n.ValueOf(n.Embed(obs)))})
			}
		}
		seat := module.BotSeat{PlayerID: actor, Skill: module.SkillHard, Seed: module.SeatSeed(seed, actor, "bot")}
		a, ok := net.Act(state, seat, offers)
		if !ok {
			break
		}
		next, _, err := m.Apply(state, actor, a)
		if err != nil {
			a, _ = module.ChooseAction(offers, nil)
			if next, _, err = m.Apply(state, actor, a); err != nil {
				break
			}
		}
		np, _ := view.Position(next)
		for _, p := range players {
			r, fin, err := view.RewardFor(pos, np, p.ID)
			if err != nil {
				continue
			}
			for _, o := range open[p.ID] {
				o.acc += float64(r) * scale
			}
			if fin {
				for _, o := range open[p.ID] {
					done = append(done, sample{o.pred, o.acc})
				}
				open[p.ID] = nil
			}
		}
		state, pos = next, np
	}
	return done
}

func report(xs []sample) {
	n := float64(len(xs))
	if n < 2 {
		fail(fmt.Errorf("only %d samples", len(xs)))
	}
	var mp, ma float64
	for _, x := range xs {
		mp += x.pred
		ma += x.actual
	}
	mp /= n
	ma /= n
	var cov, vp, va, mae float64
	for _, x := range xs {
		cov += (x.pred - mp) * (x.actual - ma)
		vp += (x.pred - mp) * (x.pred - mp)
		va += (x.actual - ma) * (x.actual - ma)
		mae += math.Abs(x.pred - x.actual)
	}
	r := cov / math.Sqrt(vp*va)
	fmt.Printf("%d decisions: mean predicted %.3f, mean actual %.3f, correlation %.3f (R² %.3f), slope %.3f, MAE %.3f, sd(actual) %.3f\n",
		len(xs), mp, ma, r, r*r, cov/vp, mae/n, math.Sqrt(va/n))
	sort.Slice(xs, func(i, j int) bool { return xs[i].pred < xs[j].pred })
	fmt.Println("decile  predicted  actual")
	for d := 0; d < 10; d++ {
		lo, hi := len(xs)*d/10, len(xs)*(d+1)/10
		var p, a float64
		for _, x := range xs[lo:hi] {
			p += x.pred
			a += x.actual
		}
		k := float64(hi - lo)
		fmt.Printf("%6d  %9.3f  %6.3f\n", d+1, p/k, a/k)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "valuecal:", err)
	os.Exit(2)
}
