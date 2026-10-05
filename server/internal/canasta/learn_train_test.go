package canasta

import (
	"math"
	"reflect"
	"testing"

	"zolik/server/internal/learn"
	"zolik/server/internal/module"
)

// The privileged vector is exactly what the seat cannot see: moving only the
// hidden cards changes it (when there is anything hidden to move), and it
// holds every other seat's hand and the stock, card for card.
func TestPrivilegedIsTheHiddenCards(t *testing.T) {
	g := learnGame{}
	changed, checked := 0, 0
	for _, tb := range learnTables {
		w := newWalker(t, tb.variation, tb.seats, 700+int64(tb.seats))
		for n := 0; n < 200; n++ {
			seat := w.actor()
			if seat == "" {
				w.deal()
				continue
			}
			if n%9 == 0 {
				p, _ := g.Position(w.state)
				v, err := g.PrivilegedFor(p, seat)
				if err != nil {
					t.Fatal(err)
				}
				if len(v) != g.PrivDim() {
					t.Fatalf("privileged vector %d wide, PrivDim %d", len(v), g.PrivDim())
				}
				s, _ := decode(w.state)
				if s.Status == "active" && !s.Break.Open {
					var held float32
					for i := 0; i < maxOthers*numCats; i++ {
						held += v[privOthers+i] * 4
					}
					want := 0
					for _, id := range othersFrom(s, seat) {
						want += len(s.Hands[id])
					}
					if int(math.Round(float64(held))) != want {
						t.Fatalf("privileged hands hold %v cards, the other seats %d", held, want)
					}
					if got := int(math.Round(float64(v[privStock] * 100))); got != len(s.DrawPile) {
						t.Fatalf("stock %d, privileged says %d", len(s.DrawPile), got)
					}
				}
				q, _ := g.Position(scrambleHidden(t, w.state, seat, w.rnd))
				u, _ := g.PrivilegedFor(q, seat)
				if !reflect.DeepEqual(u, v) {
					changed++
				}
				checked++
			}
			offers, _ := w.m.LegalActions(w.state, seat)
			cands, _ := g.Candidates(w.state, seat, offers)
			if !w.move(seat, cands) {
				w.deal()
			}
		}
	}
	if checked < 50 || changed < checked/2 {
		t.Fatalf("hidden cards moved the privileged vector at %d of %d positions", changed, checked)
	}
}

// The environment sends the privileged vector only to a trainer that asked,
// and the observation itself is the same either way.
func TestEnvPrivilegedOnlyWhenAsked(t *testing.T) {
	specs := []learn.TableSpec{{Plan: []string{learn.Learner, "hard"}}, {Plan: []string{learn.Learner, "hard", learn.Learner, "closer"}}}
	plain, err := learn.NewEnv(learnGame{}, "classic", specs, 4200, 20000)
	if err != nil {
		t.Fatal(err)
	}
	priv, err := learn.NewEnvWith(learnGame{}, "classic", specs, 4200, 20000, learn.EnvOptions{Privileged: true})
	if err != nil {
		t.Fatal(err)
	}
	a, err := plain.Observe()
	if err != nil {
		t.Fatal(err)
	}
	b, err := priv.Observe()
	if err != nil {
		t.Fatal(err)
	}
	for step := 0; step < 30; step++ {
		choices := make([]int, len(a))
		for i := range a {
			if a[i].Priv != nil {
				t.Fatal("privileged vector sent unasked")
			}
			if len(b[i].Priv) != privDim {
				t.Fatalf("asked, got a privileged vector %d wide", len(b[i].Priv))
			}
			if !reflect.DeepEqual(a[i].Obs, b[i].Obs) || !reflect.DeepEqual(a[i].Cands, b[i].Cands) {
				t.Fatal("asking for the privileged vector changed the observation")
			}
			choices[i] = step % len(a[i].Cands)
		}
		if a, err = plain.Step(choices); err != nil {
			t.Fatal(err)
		}
		if b, err = priv.Step(choices); err != nil {
			t.Fatal(err)
		}
	}
}

// linearModel is a WinModel whose P is sigmoid(w · features).
func linearModel(w []float64) *learn.WinModel {
	return &learn.WinModel{Features: len(w), Layers: []learn.WinModelLay{{W: [][]float64{w}, B: []float64{0}}}}
}

// The match reward is the deal reward at alpha 1, k times the change in P at
// alpha 0, and over a whole match that change sums to the result less P at
// 0-0 — a potential difference, so it prices deals only by the match.
func TestMatchRewardMixesDealAndWinChance(t *testing.T) {
	model := linearModel([]float64{0, 0, 3, 0, 0, 0, 0}) // P = sigmoid(3 * lead)
	run := func(alpha float64) (rewards []float32) {
		mr := &learn.MatchReward{Alpha: alpha, K: 2, Model: model}
		env, err := learn.NewEnvWith(learnGame{}, "classic", []learn.TableSpec{{Plan: []string{learn.Learner, "hard"}}}, 31, 50000, learn.EnvOptions{MatchReward: mr})
		if err != nil {
			t.Fatal(err)
		}
		obs, err := env.Observe()
		if err != nil {
			t.Fatal(err)
		}
		for steps := 0; obs[0].Matches < 1 && steps < 20000; steps++ {
			for _, ev := range obs[0].Events {
				if ev.Seat == "p0" && ev.Done {
					rewards = append(rewards, ev.Reward)
				}
			}
			if obs, err = env.Step([]int{0}); err != nil {
				t.Fatal(err)
			}
		}
		for _, ev := range obs[0].Events {
			if ev.Seat == "p0" && ev.Done {
				rewards = append(rewards, ev.Reward)
			}
		}
		return rewards
	}
	deal := run(1)
	match := run(0)
	if len(deal) < 2 || len(deal) != len(match) {
		t.Fatalf("%d deals at alpha 1, %d at alpha 0", len(deal), len(match))
	}
	var sum float64
	for _, r := range match {
		sum += float64(r)
	}
	// The learner's first-choice play loses or wins; either way the sum is
	// k * (result - 0.5).
	if math.Abs(sum/2-0.5) > 1e-4 && math.Abs(sum/2+0.5) > 1e-4 {
		t.Fatalf("ΔP over the match sums to %v, not ±k/2", sum)
	}
	half := run(0.5)
	for i := range deal {
		want := 0.5*float64(deal[i]) + 0.5*float64(match[i])
		if math.Abs(float64(half[i])-want) > 1e-4 {
			t.Fatalf("deal %d: mixed %v, want %v", i, half[i], want)
		}
	}
}

// MatchHistory starts at 0-0 and ends with the result.
func TestMatchHistory(t *testing.T) {
	g := learnGame{}
	players := learn.Players(4)
	final, st, err := learn.PlayOut(New(), g.Config(4, "classic"), players, 77, 50000, func(string) (module.Bot, module.Skill) {
		return bot{}, module.SkillHard
	})
	if err != nil || st.Stalled {
		t.Fatalf("%v %v", err, st.Why)
	}
	for _, p := range players[:2] {
		h, err := g.MatchHistory(final, p.ID)
		if err != nil {
			t.Fatal(err)
		}
		if h[0].Own != 0 || h[0].Best != 0 || h[0].Deal != 0 || h[0].Over {
			t.Fatalf("history starts at %+v", h[0])
		}
		last := h[len(h)-1]
		if !last.Over || last.Sides != 2 || last.Seats != 4 || last.Target != 5000 {
			t.Fatalf("history ends at %+v", last)
		}
		if (last.Won == 1) != (last.Own > last.Best) {
			t.Fatalf("won %v at %d to %d", last.Won, last.Own, last.Best)
		}
	}
}

// A model for the encoder before the card inference (383/40) is handed that
// encoder's vectors: today's, cut to its widths. Any other width is refused.
func TestNarrowedToTheEarlierEncoder(t *testing.T) {
	g := learnGame{}
	ng, err := learn.Narrowed(g, offInfer, fCapture)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := learn.Narrowed(g, offInfer+1, fCapture); err == nil {
		t.Fatal("a width no encoder had was accepted")
	}
	w := newWalker(t, "classic", 4, 99)
	for n := 0; n < 60; n++ {
		seat := w.actor()
		if seat == "" {
			w.deal()
			continue
		}
		full, _ := g.Encode(w.state, seat)
		cut, err := ng.Encode(w.state, seat)
		if err != nil || !reflect.DeepEqual(cut, full[:offInfer]) {
			t.Fatalf("narrowed encoding is not the prefix (%v)", err)
		}
		offers, _ := w.m.LegalActions(w.state, seat)
		cands, _ := g.Candidates(w.state, seat, offers)
		narrow, _ := ng.Candidates(w.state, seat, offers)
		for i := range cands {
			if !reflect.DeepEqual(narrow[i].Features, cands[i].Features[:fCapture]) || len(cands[i].Features) != learnCandDim {
				t.Fatal("narrowed candidate is not the prefix, or cut today's")
			}
		}
		if !w.move(seat, cands) {
			w.deal()
		}
	}
}

// Go-out shaping moves each deal's reward off the game's own (Base) by the
// bonus for going out or the capped penalty for being caught, and by nothing
// else; a deal the stock ends moves by nothing.
func TestGoOutShapingOffsetsTheDealReward(t *testing.T) {
	sh := &learn.GoOutShaping{Out: 0.1, Held: 1, Cap: 0.3}
	specs := []learn.TableSpec{{Plan: []string{learn.Learner, "hard", learn.Learner, "hard"}}}
	env, err := learn.NewEnvWith(learnGame{}, "classic", specs, 41, 50000, learn.EnvOptions{GoOut: sh})
	if err != nil {
		t.Fatal(err)
	}
	obs, err := env.Observe()
	if err != nil {
		t.Fatal(err)
	}
	var outs, caught int
	for steps := 0; obs[0].Matches < 1 && steps < 40000; steps++ {
		for _, ev := range obs[0].Events {
			if !ev.Done {
				continue
			}
			if ev.Base == nil {
				t.Fatal("a shaped episode without its base reward")
			}
			d := float64(ev.Reward - *ev.Base)
			switch {
			case math.Abs(d-sh.Out) < 1e-5:
				outs++
			case d < 0 && d >= -sh.Cap-1e-5:
				caught++
			case math.Abs(d) < 1e-6:
			default:
				t.Fatalf("shaping moved a deal by %v", d)
			}
		}
		if obs, err = env.Step([]int{0}); err != nil {
			t.Fatal(err)
		}
	}
	if outs+caught == 0 {
		t.Fatal("no deal was shaped")
	}
	if _, err := learn.NewEnvWith(learnGame{}, "classic", specs, 41, 50000, learn.EnvOptions{GoOut: &learn.GoOutShaping{Out: -1}}); err == nil {
		t.Fatal("a negative bonus was taken")
	}
}
