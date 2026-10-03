package zolikmod

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
	"zolik/server/internal/rules"
)

// The tables the adapter has to serve: every ruleset it names, at two, three
// and four seats.
var learnTables = []struct {
	variation string
	seats     int
}{
	{varClassic, 2}, {varClassic, 3}, {varClassic, 4},
	{varFloor35, 2}, {varFloor35, 4},
	{varContinental, 2}, {varContinental, 3}, {varContinental, 4},
}

// learnWalker plays matches for the adapter's tests: each seat is either the
// random policy over Candidates or the hand-written bot, chosen per match, so
// the positions reached are both the ones a learner makes and the ones it
// meets.
type learnWalker struct {
	t       testing.TB
	m       *Module
	rnd     *rand.Rand
	players []module.PlayerRef
	cfg     module.MatchConfig
	seed    int64
	state   module.State
	random  map[string]bool
	steps   int
}

func newLearnWalker(t testing.TB, variation string, seats int, seed int64) *learnWalker {
	w := &learnWalker{t: t, m: New(), rnd: rand.New(rand.NewSource(seed)), players: learn.Players(seats),
		cfg: learnGame{}.Config(seats, variation), seed: seed}
	w.deal()
	return w
}

// deal starts the next match. Half of Continental's start at a later deal, so
// every contract of the seven — two sets, three runs, the final deal that is
// melded out — is reached without playing the six before it.
func (w *learnWalker) deal() {
	w.seed++
	raw, err := w.m.NewMatch(w.cfg, w.players, w.seed)
	if err != nil {
		w.t.Fatal(err)
	}
	if s, _ := decode(raw); s.Rules.Rules.FixedDealCount > 0 && w.rnd.Intn(2) == 0 {
		n := 2 + w.rnd.Intn(s.Rules.Rules.FixedDealCount-1)
		s.Rules.GameNumber = n
		s.Ledger.Deal = n
		raw, _ = encode(s)
	}
	w.state, w.steps = raw, 0
	w.random = map[string]bool{}
	for _, p := range w.players {
		w.random[p.ID] = w.rnd.Intn(2) == 0
	}
}

func (w *learnWalker) actor() string {
	if done, _, _ := w.m.Finished(w.state); done {
		return ""
	}
	return module.ActiveSeat(w.m, w.state, w.players[0].ID, w.players)
}

// move plays one decision for the seat on turn. Reports false when the match
// should be abandoned: the heuristic has nothing the engine accepts, which is
// its wedge and not this adapter's.
func (w *learnWalker) move(seat string, cands []learn.Candidate) bool {
	w.t.Helper()
	if w.random[seat] && len(cands) > 0 {
		c := cands[w.rnd.Intn(len(cands))]
		for i, a := range c.Steps() {
			next, _, err := w.m.Apply(w.state, seat, a)
			if err != nil {
				w.t.Fatalf("seed %d: step %d of %+v refused: %v", w.seed, i, c.Steps(), err)
			}
			w.state = next
		}
		return true
	}
	offers, _ := w.m.LegalActions(w.state, seat)
	if a, ok := (heuristicBot{}).Act(w.state, module.BotSeat{PlayerID: seat, Skill: module.SkillHard, Seed: w.seed}, offers); ok {
		if next, _, err := w.m.Apply(w.state, seat, a); err == nil {
			w.state = next
			return true
		}
	}
	for _, a := range module.ChooseActions(withoutUndos(offers), nil) {
		if next, _, err := w.m.Apply(w.state, seat, a); err == nil {
			w.state = next
			return true
		}
	}
	return false
}

func learnFinite(v []float32) bool {
	for _, x := range v {
		if math.IsNaN(float64(x)) || math.IsInf(float64(x), 0) {
			return false
		}
	}
	return true
}

// onlyUndos reports a position whose every enabled offer is a take-back: the
// dead end no candidate may lead into.
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

func learnOffers(t testing.TB, m *Module, raw module.State, seat string) []module.ActionOffer {
	t.Helper()
	offers, err := m.LegalActions(raw, seat)
	if err != nil {
		t.Fatal(err)
	}
	return offers
}

// Two positions that differ only in what this seat cannot see encode alike.
// Every other hand and the whole stock are pooled, shuffled and dealt back out
// at the same sizes — so every count on screen is unchanged and every card
// behind one is different.
func TestLearnEncodeDoesNotPeek(t *testing.T) {
	g := learnGame{}
	checked := 0
	for i, tb := range learnTables {
		w := newLearnWalker(t, tb.variation, tb.seats, 500+int64(i))
		for n := 0; n < 300; n++ {
			seat := w.actor()
			if seat == "" {
				w.deal()
				continue
			}
			if n%5 == 0 {
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
						for k := range before {
							if before[k] != after[k] {
								t.Fatalf("%s/%d seat %s: feature %d changed (%v -> %v) with only hidden cards moved",
									tb.variation, tb.seats, p.ID, k, before[k], after[k])
							}
						}
					}
					checked++
				}
			}
			cands, _ := g.Candidates(w.state, seat, learnOffers(t, w.m, w.state, seat))
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
	gs := &s.Rules
	var pool []string
	var ids []string
	var sizes []int
	for _, id := range gs.TurnOrder {
		if id == seat {
			continue
		}
		ids = append(ids, id)
		sizes = append(sizes, len(gs.Hands[id]))
		pool = append(pool, gs.Hands[id]...)
	}
	stock := len(gs.DrawPile)
	pool = append(pool, gs.DrawPile...)
	rnd.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
	for i, id := range ids {
		gs.Hands[id] = append([]string(nil), pool[:sizes[i]]...)
		pool = pool[sizes[i]:]
	}
	if len(pool) != stock {
		t.Fatalf("scramble lost cards: %d left for a stock of %d", len(pool), stock)
	}
	gs.DrawPile = pool
	gs.DeckSeed = rnd.Int63()
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
// every candidate's; a seat on turn with a move gets candidates; a position
// part-way through a composite has candidates that finish it (the NetBot
// contract, learn.Candidate.Then); and no candidate leaves the seat with
// nothing but undos.
func TestLearnEveryCandidateApplies(t *testing.T) {
	g := learnGame{}
	want := 10000
	if testing.Short() {
		want = 1600
	}
	per := want / len(learnTables)
	var total, decisions, most, composites, wedges int
	var encodeTime, candTime time.Duration
	kinds := make([]int, numKinds)
	for ti, tb := range learnTables {
		w := newLearnWalker(t, tb.variation, tb.seats, int64(1000*(ti+1)))
		for seen := 0; seen < per; {
			seat := w.actor()
			if seat == "" || w.steps > 4000 {
				w.deal()
				continue
			}
			w.steps++
			offers := learnOffers(t, w.m, w.state, seat)
			start := time.Now()
			obs, err := g.Encode(w.state, seat)
			encodeTime += time.Since(start)
			if err != nil || len(obs) != g.StateDim() || !learnFinite(obs) {
				t.Fatalf("seed %d: Encode %d wide (want %d), err %v, finite %v", w.seed, len(obs), g.StateDim(), err, learnFinite(obs))
			}
			start = time.Now()
			cands, err := g.Candidates(w.state, seat, offers)
			candTime += time.Since(start)
			if err != nil {
				t.Fatal(err)
			}
			if len(cands) == 0 {
				s, _ := decode(w.state)
				gs := s.Rules
				if len(module.ChooseActions(withoutUndos(offers), nil)) > 0 || !onlyUndos(offers) && gs.CurrentTurn == seat {
					// A turn the heuristic walked into and cannot finish: a pile
					// pickup or a partial lay-down it has no completion for. Not a
					// position any candidate leads to (checked below at every
					// candidate), and the heuristic's own to answer.
					if gs.MeldsLaidThisTurn > 0 || gs.DiscardDrawnCardPendingMeld != "" || len(gs.JokersReclaimedPendingMeld) > 0 {
						wedges++
						w.deal()
						continue
					}
					t.Fatalf("seed %d %s/%d: %s has moves and no candidates (phase %s, down %v, hand %v)\n%s",
						w.seed, tb.variation, tb.seats, seat, gs.Phase, gs.RoundReqMet[seat], gs.Hands[seat], module.DescribeOffers(offers))
				}
			}
			seen++
			decisions++
			total += len(cands)
			if len(cands) > most {
				most = len(cands)
			}
			for _, c := range cands {
				if len(c.Features) != g.CandDim() || !learnFinite(c.Features) {
					t.Fatalf("seed %d: candidate %+v has %d features (want %d)", w.seed, c.Action, len(c.Features), g.CandDim())
				}
				for k := 0; k < numKinds; k++ {
					if c.Features[k] == 1 {
						kinds[k]++
					}
				}
				state := w.state
				for i, a := range c.Steps() {
					if i > 0 {
						composites++
						mid, _ := g.Candidates(state, seat, learnOffers(t, w.m, state, seat))
						if len(mid) == 0 {
							t.Fatalf("seed %d %s/%d: nothing finishes %+v after step %d", w.seed, tb.variation, tb.seats, c.Steps(), i)
						}
					}
					next, _, err := w.m.Apply(state, seat, a)
					if err != nil {
						t.Fatalf("seed %d %s/%d: %s step %d of %+v refused: %v", w.seed, tb.variation, tb.seats, seat, i, c.Steps(), err)
					}
					state = next
				}
				if done, _, _ := w.m.Finished(state); !done {
					if s, _ := decode(state); s.Rules.CurrentTurn == seat && !s.Break.Open {
						if onlyUndos(learnOffers(t, w.m, state, seat)) {
							t.Fatalf("seed %d: %+v leaves %s with nothing but undos", w.seed, c.Steps(), seat)
						}
					}
				}
			}
			if !w.move(seat, cands) {
				w.deal()
			}
		}
	}
	t.Logf("%d decisions: %.1f candidates on average, %d at most; %d composite steps re-derived; heuristic wedges %d",
		decisions, float64(total)/float64(decisions), most, composites, wedges)
	t.Logf("Encode %v, Candidates %v per decision; candidates by kind %v",
		encodeTime/time.Duration(decisions), candTime/time.Duration(decisions), kinds)
	for _, k := range []int{kindDrawDeck, kindTakePile, kindOpening, kindLayMeld, kindLayOff, kindDiscard} {
		if kinds[k] == 0 {
			t.Errorf("no candidate of kind %d in the whole sweep", k)
		}
	}
}

// Reward: once a deal for every seat, nothing mid-deal, and the right sign —
// the seat that went out is paid at least nothing, everyone else at most
// nothing.
func TestLearnRewardIsOncePerDeal(t *testing.T) {
	g := learnGame{}
	moves := 6000
	if testing.Short() {
		moves = 2000
	}
	for i, tb := range learnTables {
		w := newLearnWalker(t, tb.variation, tb.seats, 77+int64(i))
		dones := map[string]int{}
		deals := 0
		for n := 0; n < moves; n++ {
			seat := w.actor()
			if seat == "" {
				w.deal()
				continue
			}
			before := w.state
			cands, _ := g.Candidates(w.state, seat, learnOffers(t, w.m, w.state, seat))
			if !w.move(seat, cands) {
				w.deal()
				continue
			}
			sb, _ := decode(before)
			sa, _ := decode(w.state)
			ended := len(sa.Rules.DealWinners) - len(sb.Rules.DealWinners)
			deals += ended
			winner := ""
			if ended == 1 {
				winner = sa.Rules.DealWinners[len(sa.Rules.DealWinners)-1]
			}
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
					if p.ID == winner && r < 0 {
						t.Errorf("%s/%d: %s went out and was paid %v", tb.variation, tb.seats, p.ID, r)
					}
					if winner != "" && p.ID != winner && r > 0 {
						t.Errorf("%s/%d: %s lost the deal and was paid %v", tb.variation, tb.seats, p.ID, r)
					}
				}
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

// A random learner through the real environment: against the heuristic, and
// against itself. Not one move refused, and any stall is on a heuristic seat's
// turn or a match that merely ran long.
//
// The action budget is a match's, and five thousand is several times what a
// finished one takes (a Continental match of seven deals is about a thousand
// actions). It is not generous on purpose: the heuristic cannot finish most
// Continental seventh deals at all — three runs are wanted, and every seat's
// hand has converged on twos, threes and jokers, the cheapest cards to hold —
// and a match it wedges grows its state (the deal's public ledger) with every
// action, so a budget of twenty thousand spends most of the run decoding one.
// Such a match ends as an "action budget" stall with an ordinary turn length,
// which merelyLong accepts.
func TestLearnRandomLearnerThroughTheEnvironment(t *testing.T) {
	want, budget := 300, 5000
	if testing.Short() {
		want, budget = 40, 3000
	}
	runs := []struct {
		variation string
		plan      []string
	}{
		{varClassic, []string{learn.Learner, "hard"}},
		{varClassic, []string{learn.Learner, learn.Learner}},
		{varClassic, []string{learn.Learner, "hard", learn.Learner, "hard"}},
		{varFloor35, []string{learn.Learner, "hard", "medium"}},
		{varContinental, []string{learn.Learner, "hard"}},
		{varContinental, []string{learn.Learner, learn.Learner, learn.Learner}},
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
		start := time.Now()
		for deals < want && steps < 100000 {
			choices := make([]int, len(obs))
			for i, o := range obs {
				if len(o.Cands) < 2 || len(o.Obs) != learnState {
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
					if ev.Done && ev.Seat == "p0" {
						deals++
					}
				}
			}
		}
		t.Logf("%s %v: %d deals in %d steps (%.0f decisions/s); stalls %v", run.variation, run.plan, deals, steps,
			float64(steps*len(specs))/time.Since(start).Seconds(), why)
		if deals < want {
			t.Errorf("%s %v: only %d deals", run.variation, run.plan, deals)
		}
	}
}

func stuckOnHeuristic(why string) bool {
	return strings.Contains(why, "(hard)") || strings.Contains(why, "(medium)") || strings.Contains(why, "(easy)")
}

// merelyLong is a match that ran out of action budget with nobody stuck: the
// seat on turn had been acting for an ordinary turn's length.
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

// The bench agrees with the ladder: hard against itself is level, and ahead of
// easy. Short seeds under -short; the numbers the adapter was accepted on are
// from cmd/gamebench.
func TestLearnBenchSeesTheLadder(t *testing.T) {
	if testing.Short() {
		t.Skip("bench")
	}
	g := learnGame{}
	hard := learn.Contender{Name: "hard", Bot: g.Heuristic(), Skill: module.SkillHard}
	easy := learn.Contender{Name: "easy", Bot: g.Heuristic(), Skill: module.SkillEasy}
	r, err := learn.Bench(g, 2, "", hard, easy, 1, 60, 50000)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("hard vs easy: %v", r)
	if r.Illegal+r.Stalls > 0 || r.Mean <= 0 {
		t.Errorf("hard vs easy: %v", r)
	}
}

func TestLearnRegistered(t *testing.T) {
	g, err := learn.LookupGame("zolik")
	if err != nil {
		t.Fatal(err)
	}
	if g.StateDim() != 1161 || g.CandDim() != 64 {
		t.Fatalf("dims %d/%d: a changed layout is a new encoder — update ml/ and this pin together", g.StateDim(), g.CandDim())
	}
	for _, v := range learnVariations {
		cfg := resolveConfig(g.Config(2, v))
		if variationIndex(cfg) != map[string]int{varClassic: 0, varFloor35: 1, varContinental: 2}[v] {
			t.Errorf("%s resolves to variation %d", v, variationIndex(cfg))
		}
	}
	if resolveConfig(g.Config(2, varFloor35)).InitialMeldMinimum != 35 {
		t.Error("floor35 has no floor")
	}
	_ = rules.ProfileZolikClassic
}

func BenchmarkLearnEncode(b *testing.B) {
	w := newLearnWalker(b, varClassic, 4, 1)
	for i := 0; i < 200; i++ {
		seat := w.actor()
		cands, _ := learnGame{}.Candidates(w.state, seat, learnOffers(b, w.m, w.state, seat))
		w.move(seat, cands)
	}
	seat := w.actor()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = learnGame{}.Encode(w.state, seat)
	}
}

// BenchmarkLearnEnvRandom32 is the environment's pace with a random learner at
// 32 tables, in learner decisions per second.
func BenchmarkLearnEnvRandom32(b *testing.B) {
	for _, run := range []struct {
		name, variation string
		plan            []string
	}{
		{"classic2", varClassic, []string{learn.Learner, "hard"}},
		{"classic4", varClassic, []string{learn.Learner, "hard", learn.Learner, "hard"}},
		{"floor35_2", varFloor35, []string{learn.Learner, "hard"}},
		{"continental2", varContinental, []string{learn.Learner, "hard"}},
	} {
		b.Run(run.name, func(b *testing.B) {
			specs := make([]learn.TableSpec, 32)
			for i := range specs {
				specs[i] = learn.TableSpec{Plan: run.plan}
			}
			env, err := learn.NewEnv(learnGame{}, run.variation, specs, 1, 5000)
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

// TestLearnFloor35IsTheLobbyTable pins zolik_classic+floor35 to a lobby table
// someone actually plays: Classic with a 35-point first meld, the pile locked
// until the third lap, and every other option at the value the lobby sends.
// Trained on the adapter's name, a model has to be playing that lobby's rules
// exactly, not a neighbour of them.
func TestLearnFloor35IsTheLobbyTable(t *testing.T) {
	lobby := module.MatchConfig{Variation: "zolik_classic", Options: module.Options{
		rules.OptInitialMeldMinimum:   35,
		rules.OptRequireCleanRun:      rules.OptOn,
		rules.OptDiscardDrawMinRound:  3,
		module.OptOpenDiscardPile:     module.OptOn,
		rules.OptJokerReclaimMustPlay: rules.OptOn,
		module.OptPauseBetweenRounds:  module.OptOn,
		rules.OptDealStarter:          rules.DealStarterOpt(rules.DealStarterRotate),
	}}
	want := resolveConfig(lobby)
	for _, seats := range []int{2, 3, 4} {
		got := resolveConfig(learnGame{}.Config(seats, varFloor35))
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%d seats: floor35 resolves to\n%+v\nthe lobby table to\n%+v", seats, got, want)
		}
	}
	if !want.ContractFor(1).RequireCleanRun || want.DiscardDrawMinRound != 3 || want.InitialMeldMinimum != 35 ||
		want.DiscardPileTopOnly || !want.JokerReclaimMustPlay || !want.PauseBetweenDeals || want.DealStarter != rules.DealStarterRotate {
		t.Errorf("the lobby table is not the one described: %+v", want)
	}
	for round, locked := range map[int]bool{1: true, 2: true, 3: false, 4: false} {
		if rules.IsDiscardLocked(round, want.DiscardDrawMinRound) != locked {
			t.Errorf("round %d: locked %v, want %v", round, !locked, locked)
		}
	}
}
