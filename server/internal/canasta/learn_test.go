package canasta

import (
	"fmt"
	"math"
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"time"

	"zolik/server/internal/learn"
	"zolik/server/internal/module"
)

// The tables the adapter has to serve: both variations, heads-up and in
// partnerships, and Samba's six seats — the only table with three sides.
var learnTables = []struct {
	variation string
	seats     int
}{
	{"classic", 2}, {"classic", 4}, {"samba", 4}, {"samba", 6},
}

// walker plays matches for the adapter's tests: every seat is either the
// random policy over Candidates or the hand-written bot, chosen per match, so
// the positions reached are both the ones a learner makes and the ones it
// meets.
type walker struct {
	t       testing.TB
	g       learnGame
	m       *Module
	rnd     *rand.Rand
	players []module.PlayerRef
	cfg     module.MatchConfig
	seed    int64
	state   module.State
	random  map[string]bool
	steps   int
}

func newWalker(t testing.TB, variation string, seats int, seed int64) *walker {
	w := &walker{t: t, m: New(), rnd: rand.New(rand.NewSource(seed)), players: learn.Players(seats),
		cfg: learnGame{}.Config(seats, variation), seed: seed}
	w.deal()
	return w
}

// deal starts the next match. Half of them start with the sides already on
// scores that put the opening minimum at 90, 120 or (Samba) 150, which is
// where openings take two and three melds and where the heuristic's own wedge
// lives; a match from zero would reach them only after hours of play.
func (w *walker) deal() {
	w.seed++
	raw, err := w.m.NewMatch(w.cfg, w.players, w.seed)
	if err != nil {
		w.t.Fatal(err)
	}
	if w.rnd.Intn(2) == 0 {
		s, _ := decode(raw)
		bands := []int{0, 1500, 3000, 3100}
		if s.rules().Sequences {
			bands = append(bands, 7000)
		}
		for i := range s.Teams {
			s.Teams[i].Score = bands[w.rnd.Intn(len(bands))]
		}
		raw, _ = encode(s)
	}
	w.state, w.steps = raw, 0
	w.random = map[string]bool{}
	for _, p := range w.players {
		w.random[p.ID] = w.rnd.Intn(2) == 0
	}
}

// actor is whoever the match waits on, or "" when it is over.
func (w *walker) actor() string {
	if done, _, _ := w.m.Finished(w.state); done {
		return ""
	}
	return module.ActiveSeat(w.m, w.state, w.players[0].ID, w.players)
}

// apply makes a whole candidate, failing the test on any refused step.
func (w *walker) apply(seat string, c learn.Candidate) {
	w.t.Helper()
	for i, a := range c.Steps() {
		next, _, err := w.m.Apply(w.state, seat, a)
		if err != nil {
			w.t.Fatalf("seed %d: step %d of %+v refused: %v", w.seed, i, c.Steps(), err)
		}
		w.state = next
	}
}

// move plays one decision for the seat on turn: the random policy, or the
// heuristic. Reports false when the match should be abandoned — the
// heuristic's own known wedge (a partial opening it cannot finish), which is
// not this adapter's to fix.
func (w *walker) move(seat string, cands []learn.Candidate) bool {
	if w.random[seat] && len(cands) > 0 {
		w.apply(seat, cands[w.rnd.Intn(len(cands))])
		return true
	}
	offers, _ := w.m.LegalActions(w.state, seat)
	if a, ok := (bot{}).Act(w.state, module.BotSeat{PlayerID: seat, Skill: module.SkillHard}, offers); ok {
		if next, _, err := w.m.Apply(w.state, seat, a); err == nil {
			w.state = next
			return true
		}
	}
	if len(cands) > 0 {
		w.apply(seat, cands[0])
		return true
	}
	return false
}

func finite(v []float32) bool {
	for _, x := range v {
		if math.IsNaN(float64(x)) || math.IsInf(float64(x), 0) {
			return false
		}
	}
	return true
}

// onlyUndos reports a position whose every enabled offer is a take-back: the
// dead end this adapter must never walk into.
func onlyUndos(offers []module.ActionOffer) bool {
	any := false
	for _, o := range offers {
		if !o.Enabled {
			continue
		}
		if !o.Undo {
			return false
		}
		any = true
	}
	return any
}

// Two positions that differ only in what this seat cannot see encode alike.
// Every other hand and the whole stock are pooled, shuffled and dealt back out
// at the same sizes — so every count on screen is unchanged and every card
// behind one is different.
func TestEncodeDoesNotPeek(t *testing.T) {
	g := learnGame{}
	checked := 0
	for _, tb := range learnTables {
		w := newWalker(t, tb.variation, tb.seats, 500+int64(tb.seats))
		for n := 0; n < 400; n++ {
			seat := w.actor()
			if seat == "" {
				w.deal()
				continue
			}
			if n%7 == 0 {
				for _, p := range w.players {
					before, err := g.Encode(w.state, p.ID)
					if err != nil {
						t.Fatal(err)
					}
					after, err := g.Encode(scrambleHidden(t, w.state, p.ID, w.rnd), p.ID)
					if err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(before, after) {
						for i := range before {
							if before[i] != after[i] {
								t.Fatalf("%s/%d seat %s: feature %d changed (%v -> %v) with only hidden cards moved",
									tb.variation, tb.seats, p.ID, i, before[i], after[i])
							}
						}
					}
					checked++
				}
			}
			offers, _ := w.m.LegalActions(w.state, seat)
			cands, _ := g.Candidates(w.state, seat, offers)
			if !w.move(seat, cands) {
				w.deal()
			}
		}
	}
	if checked < 100 {
		t.Fatalf("only %d positions checked", checked)
	}
}

func scrambleHidden(t *testing.T, raw module.State, seat string, rnd *rand.Rand) module.State {
	s, err := decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	var pool []string
	var sizes []int
	var ids []string
	for _, id := range s.TurnOrder {
		if id == seat {
			continue
		}
		ids = append(ids, id)
		sizes = append(sizes, len(s.Hands[id]))
		pool = append(pool, s.Hands[id]...)
	}
	stock := len(s.DrawPile)
	pool = append(pool, s.DrawPile...)
	rnd.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
	for i, id := range ids {
		s.Hands[id] = append([]string(nil), pool[:sizes[i]]...)
		pool = pool[sizes[i]:]
	}
	if len(pool) != stock {
		t.Fatalf("scramble lost cards: %d left for a stock of %d", len(pool), stock)
	}
	s.DrawPile = pool
	out, err := encode(s)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// The sweep: thousands of real decisions, and every candidate at every one of
// them — every step of every composite — is put to the engine.
//
// Also checked at each: the encoding is the declared width and finite; so is
// every candidate's; a seat with a move never gets an empty candidate list; a
// position part-way through a composite has candidates that finish it (the
// NetBot contract, learn.Candidate.Then); and no candidate leaves the seat on
// turn with nothing but undos.
func TestEveryCandidateApplies(t *testing.T) {
	g := learnGame{}
	want := 10000
	if testing.Short() {
		want = 1200
	}
	per := want / len(learnTables)
	var total, decisions, most, composites int
	var encodeTime time.Duration
	var encodes int
	for ti, tb := range learnTables {
		w := newWalker(t, tb.variation, tb.seats, int64(1000*(ti+1)))
		for seen := 0; seen < per; {
			seat := w.actor()
			if seat == "" || w.steps > 3000 {
				w.deal()
				continue
			}
			w.steps++
			offers, err := w.m.LegalActions(w.state, seat)
			if err != nil {
				t.Fatal(err)
			}
			start := time.Now()
			obs, err := g.Encode(w.state, seat)
			encodeTime += time.Since(start)
			encodes++
			if err != nil || len(obs) != g.StateDim() || !finite(obs) {
				t.Fatalf("seed %d: Encode %d wide (want %d), err %v, finite %v", w.seed, len(obs), g.StateDim(), err, finite(obs))
			}
			cands, err := g.Candidates(w.state, seat, offers)
			if err != nil {
				t.Fatal(err)
			}
			if len(cands) == 0 && len(module.ChooseActions(withoutUndos(offers), nil)) > 0 {
				s, _ := decode(w.state)
				if s.team(seat).HasMelded || s.LaidThisTurn == 0 {
					t.Fatalf("seed %d: %s has moves and no candidates (phase %s, opened %v)",
						w.seed, seat, s.Phase, s.team(seat).HasMelded)
				}
				// Part-way through an opening the *heuristic* started and
				// cannot finish: its known wedge, reached here because the
				// walker lets it play. Not a position a candidate leads to.
				w.deal()
				continue
			}
			seen++
			decisions++
			total += len(cands)
			if len(cands) > most {
				most = len(cands)
			}
			for _, c := range cands {
				if len(c.Features) != g.CandDim() || !finite(c.Features) {
					t.Fatalf("seed %d: candidate %+v has %d features (want %d)", w.seed, c.Action, len(c.Features), g.CandDim())
				}
				state := w.state
				for i, a := range c.Steps() {
					if i > 0 {
						composites++
						mid, _ := g.Candidates(state, seat, mustOffers(t, w.m, state, seat))
						if len(mid) == 0 {
							t.Fatalf("seed %d: nothing finishes %+v after step %d", w.seed, c.Steps(), i)
						}
					}
					next, _, err := w.m.Apply(state, seat, a)
					if err != nil {
						t.Fatalf("seed %d: %s step %d of %+v refused: %v", w.seed, seat, i, c.Steps(), err)
					}
					state = next
				}
				if done, _, _ := w.m.Finished(state); !done {
					if s, _ := decode(state); s.Current == seat && !s.Break.Open {
						if onlyUndos(mustOffers(t, w.m, state, seat)) {
							t.Fatalf("seed %d: %+v leaves %s with nothing but undos", w.seed, c.Steps(), seat)
						}
					}
				}
			}
			if !w.move(seat, cands) {
				w.deal()
			}
			if done, _, _ := w.m.Finished(w.state); !done {
				if onlyUndos(mustOffers(t, w.m, w.state, seat)) && w.random[seat] {
					t.Fatalf("seed %d: the random policy left %s with only undos", w.seed, seat)
				}
			}
		}
	}
	t.Logf("%d decisions: %.1f candidates on average, %d at most; %d composite steps re-derived; Encode %v each",
		decisions, float64(total)/float64(decisions), most, composites, encodeTime/time.Duration(encodes))
}

func mustOffers(t testing.TB, m *Module, raw module.State, seat string) []module.ActionOffer {
	t.Helper()
	offers, err := m.LegalActions(raw, seat)
	if err != nil {
		t.Fatal(err)
	}
	return offers
}

// Reward: once a deal, the same for partners, and the whole deal's result.
func TestRewardIsOncePerDealAndShared(t *testing.T) {
	g := learnGame{}
	moves := 4000
	if testing.Short() {
		moves = 1200
	}
	for _, tb := range learnTables {
		w := newWalker(t, tb.variation, tb.seats, 77)
		dones := map[string]int{}
		deals := 0
		for n := 0; n < moves; n++ {
			seat := w.actor()
			if seat == "" {
				break
			}
			before := w.state
			offers, _ := w.m.LegalActions(w.state, seat)
			cands, _ := g.Candidates(w.state, seat, offers)
			if !w.move(seat, cands) {
				break
			}
			sb, _ := decode(before)
			sa, _ := decode(w.state)
			ended := len(sa.Deals) - len(sb.Deals)
			deals += ended
			rewards := map[int]float32{}
			for _, p := range w.players {
				r, done, err := g.Reward(before, w.state, p.ID)
				if err != nil {
					t.Fatal(err)
				}
				if done != (ended == 1) {
					t.Fatalf("%s/%d: done %v for a move that ended %d deals", tb.variation, tb.seats, done, ended)
				}
				if !done && r != 0 {
					t.Fatalf("%s/%d: %v paid mid-deal", tb.variation, tb.seats, r)
				}
				if done {
					dones[p.ID]++
				}
				team := sa.TeamOf[p.ID]
				if prev, ok := rewards[team]; ok && prev != r {
					t.Fatalf("%s/%d: partners paid %v and %v", tb.variation, tb.seats, prev, r)
				}
				rewards[team] = r
			}
		}
		if deals == 0 {
			t.Fatalf("%s/%d: no deal finished", tb.variation, tb.seats)
		}
		for _, p := range w.players {
			if dones[p.ID] != deals {
				t.Errorf("%s/%d: %s closed %d episodes over %d deals", tb.variation, tb.seats, p.ID, dones[p.ID], deals)
			}
		}
	}
}

// A random learner through the real environment: alone against the heuristic,
// and as a partnership of two against a partnership of heuristics. Not one
// move refused, and any stall is on a heuristic seat's turn.
func TestRandomLearnerThroughTheEnvironment(t *testing.T) {
	// A shorter budget under -short: the heuristic's own wedge spins until the
	// budget runs out, and at the full 20000 one of them is most of the run.
	want, budget := 300, 20000
	if testing.Short() {
		want, budget = 30, 3000
	}
	runs := []struct {
		variation string
		plan      []string
	}{
		{"classic", []string{learn.Learner, "hard"}},
		{"classic", []string{learn.Learner, "hard", learn.Learner, "hard"}},
		{"samba", []string{learn.Learner, "hard", learn.Learner, "hard"}},
		{"samba", []string{learn.Learner, "hard", "medium", learn.Learner, "hard", "medium"}},
	}
	for _, run := range runs {
		specs := make([]learn.TableSpec, 8)
		for i := range specs {
			specs[i] = learn.TableSpec{Plan: run.plan}
		}
		env, err := learn.NewEnv(learnGame{}, run.variation, specs, 9000, budget)
		if err != nil {
			t.Fatal(err)
		}
		obs, err := env.Observe()
		if err != nil {
			t.Fatal(err)
		}
		rnd := rand.New(rand.NewSource(3))
		deals, steps := 0, 0
		stalls := make([]int, len(specs))
		why := map[string]int{}
		for deals < want && steps < 200000 {
			choices := make([]int, len(obs))
			for i, o := range obs {
				if len(o.Cands) < 2 || len(o.Obs) != learnStateDim {
					t.Fatalf("table %d: %d candidates, obs %d", i, len(o.Cands), len(o.Obs))
				}
				choices[i] = rnd.Intn(len(o.Cands))
			}
			if obs, err = env.Step(choices); err != nil {
				t.Fatal(err)
			}
			steps++
			for i, o := range obs {
				if o.Illegal != 0 {
					t.Fatalf("%s %v table %d: %d illegal moves", run.variation, run.plan, i, o.Illegal)
				}
				if o.Stalls != stalls[i] {
					stalls[i] = o.Stalls
					why[o.LastStall]++
					if !stuckOnHeuristic(o.LastStall) && !merelyLong(o.LastStall) {
						t.Fatalf("%s %v table %d: a stall not on a heuristic seat: %q", run.variation, run.plan, i, o.LastStall)
					}
				}
				for _, ev := range o.Events {
					// p0 is a learner at every table: its episodes are deals.
					if ev.Done && ev.Seat == "p0" {
						deals++
					}
				}
			}
		}
		t.Logf("%s %v: %d deals in %d steps; stalls %v", run.variation, run.plan, deals, steps, why)
		if deals < want {
			t.Errorf("%s %v: only %d deals", run.variation, run.plan, deals)
		}
	}
}

// stuckOnHeuristic is a stall on a hand-written seat's turn: its own wedge, not
// this adapter's.
func stuckOnHeuristic(why string) bool {
	return strings.Contains(why, "(hard)") || strings.Contains(why, "(medium)") || strings.Contains(why, "(easy)")
}

// merelyLong is a match that ran out of action budget with nobody stuck: the
// seat on turn had been acting for an ordinary turn's length, not going round
// in circles. A random partnership can lose a thousand points a deal and never
// let a match reach its target; that is a long match, not a stall to fix.
func merelyLong(why string) bool {
	var run int
	i := strings.Index(why, " after ")
	if !strings.HasPrefix(why, "action budget") || i < 0 {
		return false
	}
	if _, err := fmt.Sscanf(why[i:], " after %d actions in a row", &run); err != nil {
		return false
	}
	return run < 100
}

func BenchmarkEncode(b *testing.B) {
	w := newWalker(b, "samba", 4, 1)
	for i := 0; i < 200; i++ {
		seat := w.actor()
		cands, _ := learnGame{}.Candidates(w.state, seat, mustOffers(b, w.m, w.state, seat))
		w.move(seat, cands)
	}
	seat := w.actor()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = learnGame{}.Encode(w.state, seat)
	}
}

// BenchmarkEnvRandom32 is the environment's pace with a random learner at 32
// tables, in learner decisions per second.
func BenchmarkEnvRandom32(b *testing.B) {
	for _, run := range []struct {
		name, variation string
		plan            []string
	}{
		{"classic2", "classic", []string{learn.Learner, "hard"}},
		{"classic4", "classic", []string{learn.Learner, "hard", learn.Learner, "hard"}},
		{"samba4", "samba", []string{learn.Learner, "hard", learn.Learner, "hard"}},
	} {
		b.Run(run.name, func(b *testing.B) {
			specs := make([]learn.TableSpec, 32)
			for i := range specs {
				specs[i] = learn.TableSpec{Plan: run.plan}
			}
			env, err := learn.NewEnv(learnGame{}, run.variation, specs, 1, 20000)
			if err != nil {
				b.Fatal(err)
			}
			obs, err := env.Observe()
			if err != nil {
				b.Fatal(err)
			}
			rnd := rand.New(rand.NewSource(1))
			b.ResetTimer()
			start := time.Now()
			for i := 0; i < b.N; i++ {
				choices := make([]int, len(obs))
				for j, o := range obs {
					choices[j] = rnd.Intn(len(o.Cands))
				}
				if obs, err = env.Step(choices); err != nil {
					b.Fatal(err)
				}
			}
			b.ReportMetric(float64(b.N*len(specs))/time.Since(start).Seconds(), "decisions/s")
		})
	}
}
