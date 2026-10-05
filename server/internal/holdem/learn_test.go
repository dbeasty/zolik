package holdem

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"math"
	"math/rand"
	"strings"
	"testing"
	"time"

	"zolik/server/internal/learn"
	"zolik/server/internal/module"
)

// The learning adapter (learn.go) and the public record it reads (history.go).
//
// What is pinned here is the contract internal/learn states for every game,
// asserted over thousands of real positions rather than a handful of built
// ones: the encoder sees only what the seat may, every candidate is a move the
// engine accepts, the widths never move, and the chips a hand pays out sum to
// nothing.

// decision is one position a seat had to act in, captured on the way through a
// match.
type decision struct {
	state  module.State
	actor  string
	offers []module.ActionOffer
}

// playDecisions plays one match at `seats` seats and hands every betting
// decision to visit before it is made. A seat is played at random from the
// adapter's candidates or by the heuristic, chosen per seat from the seed, so
// a sweep sees both the positions good play reaches and the ones only bad
// play does.
//
// visit may return an action to play instead; nil plays the seat's own.
// Rewards are checked here, on every action, for every seat: each hand pays
// once, to exactly the seats dealt into it, and the payments cancel.
func playDecisions(t testing.TB, seats int, seed int64, visit func(d decision, cands []learn.Candidate)) (hands int) {
	t.Helper()
	g := learnGame{}
	m := New()
	players := learn.Players(seats)
	state, err := m.NewMatch(g.Config(seats, ""), players, seed)
	if err != nil {
		t.Fatal(err)
	}
	rnd := rand.New(rand.NewSource(seed))
	heuristic := map[string]bool{}
	for _, p := range players {
		heuristic[p.ID] = rnd.Intn(2) == 0
	}
	for step := 0; step < 20000; step++ {
		if done, _, _ := m.Finished(state); done {
			return hands
		}
		actor := module.ActiveSeat(m, state, players[0].ID, players)
		if actor == "" {
			t.Fatalf("seed %d: nobody on turn", seed)
		}
		offers, err := m.LegalActions(state, actor)
		if err != nil {
			t.Fatal(err)
		}
		cands, err := g.Candidates(state, actor, offers)
		if err != nil {
			t.Fatal(err)
		}
		if len(cands) == 0 {
			t.Fatalf("seed %d: no candidates for %s", seed, actor)
		}
		var a module.Action
		if len(cands) > 1 {
			visit(decision{state, actor, offers}, cands)
			if heuristic[actor] {
				var ok bool
				if a, ok = (bot{}).Act(state, module.BotSeat{PlayerID: actor, Skill: module.SkillHard}, offers); !ok {
					t.Fatalf("heuristic had no move")
				}
			} else {
				a = cands[rnd.Intn(len(cands))].Action
			}
		} else {
			a = cands[0].Action
		}
		next, _, err := m.Apply(state, actor, a)
		if err != nil {
			t.Fatalf("seed %d: %s played %+v: %v", seed, actor, a, err)
		}

		var sum float32
		dones := 0
		dealt := 0
		was, err := decode(state)
		if err != nil {
			t.Fatal(err)
		}
		for _, st := range was.Seats {
			if !st.Out {
				dealt++
			}
		}
		for _, p := range players {
			r, done, err := g.Reward(state, next, p.ID)
			if err != nil {
				t.Fatal(err)
			}
			if !done && r != 0 {
				t.Fatalf("seed %d: %s paid %v with no hand ending", seed, p.ID, r)
			}
			if done {
				dones++
			}
			sum += r
		}
		if dones > 0 {
			hands++
			if dones != dealt {
				t.Fatalf("seed %d: a hand closed for %d seats, %d were dealt in", seed, dones, dealt)
			}
		}
		if math.Abs(float64(sum)) > 1e-3 {
			t.Fatalf("seed %d: a hand paid out %v big blinds in total, want 0", seed, sum)
		}
		state = next
	}
	t.Fatalf("seed %d: match did not finish", seed)
	return hands
}

func finite(v []float32) bool {
	for _, x := range v {
		if math.IsNaN(float64(x)) || math.IsInf(float64(x), 0) {
			return false
		}
	}
	return true
}

func rawBits(v []float32) []byte {
	var b bytes.Buffer
	_ = binary.Write(&b, binary.LittleEndian, v)
	return b.Bytes()
}

// TestAdapterHoldsOverManyDecisions is the sweep: every decision of real
// matches at two, three and six seats, and at each one every candidate is
// applied to see the engine accept it.
func TestAdapterHoldsOverManyDecisions(t *testing.T) {
	g := learnGame{}
	m := New()
	target := 10000
	if testing.Short() {
		target = 2000
	}
	decisions, hands := 0, 0
	kinds := make([]int, candKinds)
	var encodeTime time.Duration
	for seed := int64(1); decisions < target; seed++ {
		seats := []int{2, 3, 6}[seed%3]
		hands += playDecisions(t, seats, seed, func(d decision, cands []learn.Candidate) {
			decisions++
			start := time.Now()
			obs, err := g.Encode(d.state, d.actor)
			encodeTime += time.Since(start)
			if err != nil {
				t.Fatal(err)
			}
			if len(obs) != g.StateDim() || !finite(obs) {
				t.Fatalf("obs: %d wide (want %d), finite %v", len(obs), g.StateDim(), finite(obs))
			}
			canCheck := menuOf(d.offers).can(VerbCheck)
			seen := map[string]bool{}
			for _, c := range cands {
				if len(c.Features) != g.CandDim() || !finite(c.Features) {
					t.Fatalf("candidate %+v: %d features (want %d)", c.Action, len(c.Features), g.CandDim())
				}
				if _, _, err := m.Apply(d.state, d.actor, c.Action); err != nil {
					t.Fatalf("candidate %+v refused: %v", c.Action, err)
				}
				if canCheck && c.Action.Verb == VerbFold {
					t.Fatalf("fold offered with a free check")
				}
				key := c.Action.Verb + "/" + c.Action.Params[ParamAmount]
				if seen[key] {
					t.Fatalf("candidate %s offered twice", key)
				}
				seen[key] = true
				for k := 0; k < candKinds; k++ {
					if c.Features[candKind+k] == 1 {
						kinds[k]++
					}
				}
			}
		})
	}
	t.Logf("%d decisions over %d hands; candidate kinds %v; Encode %v per call",
		decisions, hands, kinds, encodeTime/time.Duration(decisions))
	for k, n := range kinds {
		if n == 0 {
			t.Errorf("candidate kind %d never offered", k)
		}
	}
}

// TestEncodeDoesNotPeek is TestBotDoesNotPeek for the encoder, and stricter,
// since a network's reasons cannot be read afterwards.
//
// At real decisions from real matches, every other seat's hole cards and the
// whole undealt deck are dealt again from the cards this seat cannot see — and
// the encoding must not move by a bit. That covers the rollout too: it must
// deal its trials from its own deck, with its own coin, and not from the
// state's, whose order is the rest of the hand.
func TestEncodeDoesNotPeek(t *testing.T) {
	g := learnGame{}
	checked := 0
	for seed := int64(100); checked < 400; seed++ {
		playDecisions(t, []int{2, 3, 6}[seed%3], seed, func(d decision, _ []learn.Candidate) {
			if checked >= 400 {
				return
			}
			s := mustDecode(t, d.state)
			for _, st := range s.Seats {
				if len(st.Hole) < 2 {
					continue
				}
				want, err := g.Encode(d.state, st.PlayerID)
				if err != nil {
					t.Fatal(err)
				}
				rigged := scrambleFor(t, d.state, st.PlayerID, seed+int64(checked))
				if bytes.Equal(rigged, d.state) {
					continue
				}
				got, err := g.Encode(rigged, st.PlayerID)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(rawBits(want), rawBits(got)) {
					for i := range want {
						if want[i] != got[i] {
							t.Fatalf("%s's encoding moved at %d (%v -> %v) when only cards it cannot see changed",
								st.PlayerID, i, want[i], got[i])
						}
					}
				}
				checked++
			}
		})
	}
}

// scrambleFor re-deals everything `playerID` cannot see: the other seats'
// hole cards and the deck, from the cards not in its hand or on the board.
func scrambleFor(t *testing.T, raw module.State, playerID string, seed int64) module.State {
	t.Helper()
	s := mustDecode(t, raw)
	visible := map[string]bool{}
	for _, c := range s.seat(playerID).Hole {
		visible[c] = true
	}
	for _, c := range s.Board {
		visible[c] = true
	}
	var hidden []string
	for _, c := range buildDeck() {
		if !visible[c] {
			hidden = append(hidden, c)
		}
	}
	hidden = shuffle(hidden, seed)
	for i := range s.Seats {
		if s.Seats[i].PlayerID == playerID || len(s.Seats[i].Hole) == 0 {
			continue
		}
		s.Seats[i].Hole = []string{hidden[0], hidden[1]}
		hidden = hidden[2:]
	}
	s.Deck = hidden[:len(s.Deck)]
	out, err := encode(s)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// TestCandidatesAtTheShowdownAreOne: nothing to decide at a showdown stop, so
// one candidate — go on, not show — and the environment plays it unasked.
func TestCandidatesAtTheShowdownAreOne(t *testing.T) {
	m := New()
	state, _ := m.NewMatch(learnGame{}.Config(2, ""), learn.Players(2), 3)
	for i := 0; i < 200; i++ {
		s := mustDecode(t, state)
		actor := module.ActiveSeat(m, state, "p0", learn.Players(2))
		offers, _ := m.LegalActions(state, actor)
		if s.Break.Open {
			cands, err := learnGame{}.Candidates(state, actor, offers)
			if err != nil || len(cands) != 1 || cands[0].Action.Verb != module.VerbContinue {
				t.Fatalf("at a showdown: %+v, %v", cands, err)
			}
			return
		}
		a, _ := module.ChooseAction(offers, []string{VerbCall, VerbCheck})
		state, _, _ = m.Apply(state, actor, a)
	}
	t.Fatal("never reached a showdown")
}

// TestOldStatesPlayOn: a state stored before the public record existed —
// no hand log, no aggressor, no reads — decodes, encodes, and plays to the
// end, picking the counting up from wherever it finds itself.
func TestOldStatesPlayOn(t *testing.T) {
	m := New()
	g := learnGame{}
	players := learn.Players(3)
	state, _ := m.NewMatch(g.Config(3, ""), players, 11)
	// A few actions in, so the stored hand is mid-flight.
	for i := 0; i < 4; i++ {
		actor := module.ActiveSeat(m, state, "p0", players)
		offers, _ := m.LegalActions(state, actor)
		a, _ := module.ChooseAction(offers, []string{VerbCall, VerbCheck})
		state, _, _ = m.Apply(state, actor, a)
	}
	var doc map[string]any
	if err := json.Unmarshal(state, &doc); err != nil {
		t.Fatal(err)
	}
	delete(doc, "handLog")
	delete(doc, "aggressor")
	for _, st := range doc["seats"].([]any) {
		delete(st.(map[string]any), "reads")
	}
	old, _ := json.Marshal(doc)
	if strings.Contains(string(old), "handLog") || strings.Contains(string(old), "reads") {
		t.Fatal("the old-format state still carries the new fields")
	}

	actor := module.ActiveSeat(m, old, "p0", players)
	if obs, err := g.Encode(old, actor); err != nil || len(obs) != stateDim {
		t.Fatalf("encoding an old state: %v", err)
	}
	final := playBots(t, old, players, botSeats(players, bot{}), 6000)
	if final.Status != "completed" {
		t.Fatalf("status %s", final.Status)
	}
	for _, st := range final.Seats {
		if st.Reads == nil || st.Reads.Hands == 0 {
			t.Errorf("%s: no reads after playing on: %+v", st.PlayerID, st.Reads)
		}
	}
}

// TestViewDoesNotSendTheRecord: the hand log and the reads are for the
// encoder. Nothing on a client reads them, so nothing sends them.
func TestViewDoesNotSendTheRecord(t *testing.T) {
	m := New()
	players := learn.Players(3)
	state, _ := m.NewMatch(learnGame{}.Config(3, ""), players, 5)
	for i := 0; i < 5; i++ {
		actor := module.ActiveSeat(m, state, "p0", players)
		offers, _ := m.LegalActions(state, actor)
		a, _ := module.ChooseAction(offers, []string{VerbRaise, VerbCall})
		state, _, _ = m.Apply(state, actor, a)
	}
	if s := mustDecode(t, state); len(s.HandLog) == 0 {
		t.Fatal("nothing was recorded")
	}
	vm, err := m.View(state, "p0")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(vm)
	for _, field := range []string{"handLog", "reads", "vpip", "aggressor"} {
		if strings.Contains(string(raw), `"`+field+`"`) {
			t.Errorf("the view sends %q", field)
		}
	}
}

// TestRecordIsReplayable: the record is a function of the actions alone, so
// the same seed and the same moves rebuild it byte for byte — which is what
// match/replay.go relies on.
func TestRecordIsReplayable(t *testing.T) {
	players := learn.Players(4)
	play := func() *GameState {
		state, _ := New().NewMatch(learnGame{}.Config(4, ""), players, 21)
		return playBots(t, state, players, botSeats(players, bot{}), 6000)
	}
	a, _ := json.Marshal(play())
	b, _ := json.Marshal(play())
	if !bytes.Equal(a, b) {
		t.Fatal("two plays of one seed recorded different histories")
	}
}

// TestReadsCountWhatTheyName pins each counter against a hand played by hand.
func TestReadsCountWhatTheyName(t *testing.T) {
	m := New()
	players := refs("a", "b", "c")
	state, _ := m.NewMatch(module.MatchConfig{Variation: "timed",
		Options: module.Options{OptStartingStack: 1000, OptBigBlind: 20, OptHandLimit: 5}}, players, 1)
	act := func(verb string, to int) {
		t.Helper()
		s := mustDecode(t, state)
		actor := s.Seats[s.Current].PlayerID
		a := module.Action{Verb: verb}
		if verb == VerbRaise {
			a = raiseTo(to)
		}
		next, _, err := m.Apply(state, actor, a)
		if err != nil {
			t.Fatalf("%s %s: %v", actor, verb, err)
		}
		state = next
	}
	s := mustDecode(t, state)
	first := s.Current
	// Preflop: first to act raises, the next calls, the big blind folds.
	act(VerbRaise, 60)
	act(VerbCall, 0)
	act(VerbFold, 0)
	s = mustDecode(t, state)
	if s.Street != streetFlop || s.Aggressor != -1 {
		t.Fatalf("street %s aggressor %d, want a fresh flop", s.Street, s.Aggressor)
	}
	raiser, caller := first, s.nextSeat(first, func(*Seat) bool { return true })
	r, c := s.Seats[raiser].Reads, s.Seats[caller].Reads
	if r.VPIP != 1 || r.PFR != 1 || c.VPIP != 1 || c.PFR != 0 {
		t.Fatalf("preflop reads: raiser %+v caller %+v", r, c)
	}
	// Flop: the first to act bets the pot, the other folds to it.
	bettor := s.Current
	act(VerbRaise, potNow(s))
	s = mustDecode(t, state)
	if s.Aggressor != bettor {
		t.Errorf("aggressor %d, want %d", s.Aggressor, bettor)
	}
	folder := s.Current
	act(VerbFold, 0)
	s = mustDecode(t, state)
	if got := s.Seats[bettor].Reads; got.PostBets != 1 {
		t.Errorf("bettor: %+v", got)
	}
	if got := s.Seats[folder].Reads; got.FacedBet != 1 || got.FoldedToBet != 1 {
		t.Errorf("folder: %+v", got)
	}
	for _, st := range s.Seats {
		if st.Reads.Hands != 1 {
			t.Errorf("%s dealt %d hands, want 1", st.PlayerID, st.Reads.Hands)
		}
		// Nobody showed: a pot-sized bet that was never turned over is not
		// evidence of anything.
		if st.Reads.BigBetsShown != 0 {
			t.Errorf("%s: a hand nobody saw was counted: %+v", st.PlayerID, st.Reads)
		}
	}
	// And the winner turns it over: now it counts, judged against the flop
	// the bet was made on.
	shown, _, err := m.Apply(state, s.Seats[bettor].PlayerID, module.Action{Verb: VerbShow})
	if err != nil {
		t.Fatal(err)
	}
	s = mustDecode(t, shown)
	got := s.Seats[bettor]
	wantBluff := 0
	if weakAt(got.Hole, s.Board[:3], streetFlop) {
		wantBluff = 1
	}
	if got.Reads.BigBetsShown != 1 || got.Reads.BluffsShown != wantBluff {
		t.Errorf("shown bettor: %+v, want one big bet and %d bluffs", got.Reads, wantBluff)
	}
}

// TestWeakAtNamesABluff pins the bluff test on hands whose answer is not in
// doubt.
func TestWeakAtNamesABluff(t *testing.T) {
	cases := []struct {
		hole, board []string
		street      string
		weak        bool
	}{
		{[]string{"2C", "7D"}, []string{"AH", "KS", "9C", "4D", "JH"}, streetRiver, true},  // air
		{[]string{"2C", "7D"}, []string{"AH", "AS", "9C", "4D", "JH"}, streetRiver, true},  // the board's pair
		{[]string{"9D", "7D"}, []string{"AH", "KS", "9C", "4D", "JH"}, streetRiver, false}, // a pair of its own
		{[]string{"5C", "5D"}, []string{"AH", "KS", "9C", "4D", "JH"}, streetRiver, false}, // a pocket pair
		{[]string{"QH", "TH"}, []string{"AH", "5H", "2C"}, streetFlop, false},              // a flush draw
		{[]string{"QH", "TH"}, []string{"AH", "5H", "2C", "3D", "8S"}, streetRiver, true},  // the draw, missed
		{[]string{"8C", "7D"}, []string{"6H", "5S", "KC"}, streetFlop, false},              // open-ended
		{[]string{"8C", "2D"}, []string{"6H", "5S", "KC"}, streetFlop, true},               // nothing to draw to
	}
	for _, tc := range cases {
		if got := weakAt(tc.hole, tc.board, tc.street); got != tc.weak {
			t.Errorf("weakAt(%v, %v) = %v, want %v", tc.hole, tc.board, got, tc.weak)
		}
	}
}

// TestReadsCatchTheRiverBluffer: over a match of thirty-odd hands against a
// player who overbets every river with anything, the showdowns it is called
// at are enough for its bluff rate to stand out from the prior, and from the
// player calling it.
func TestReadsCatchTheRiverBluffer(t *testing.T) {
	m := New()
	players := refs("hard", "bluffer")
	seats := map[string]module.Bot{"hard": skilled{m.Bot(), module.SkillHard}, "bluffer": riverBluffer{}}
	var bluffer, hard SeatReads
	for seed := int64(1); seed <= 3; seed++ {
		state, _ := m.NewMatch(module.MatchConfig{Variation: "timed",
			Options: module.Options{OptStartingStack: 10000, OptBigBlind: 20, OptHandLimit: 30}}, players, seed)
		final := playBots(t, state, players, seats, 20000)
		add := func(into *SeatReads, r *SeatReads) {
			into.Hands += r.Hands
			into.BigBetsShown += r.BigBetsShown
			into.BluffsShown += r.BluffsShown
		}
		add(&bluffer, final.seat("bluffer").Reads)
		add(&hard, final.seat("hard").Reads)
	}
	rate := readsOf(&bluffer)[4]
	t.Logf("bluffer %+v (read %.2f), hard %+v (read %.2f)", bluffer, rate, hard, readsOf(&hard)[4])
	if bluffer.BigBetsShown < 4 || rate < 0.45 || rate <= readsOf(&hard)[4] {
		t.Errorf("the river bluffer reads as bluffing %.2f of the time from %d big bets shown", rate, bluffer.BigBetsShown)
	}
}

// TestStylesPlayLegally: every style, whole matches, every move accepted.
func TestStylesPlayLegally(t *testing.T) {
	styles := learnGame{}.Styles()
	for name, b := range styles {
		for _, n := range []int{2, 6} {
			players := learn.Players(n)
			seats := botSeats(players, bot{})
			seats[players[0].ID] = b
			seats[players[n-1].ID] = b
			state, _ := New().NewMatch(learnGame{}.Config(n, ""), players, int64(n))
			if final := playBots(t, state, players, seats, 20000); final.Status != "completed" {
				t.Errorf("%s at %d seats: %s", name, n, final.Status)
			}
		}
	}
}

// TestStylesAreWhatTheyAreCalled checks each style's defining habit in the
// reads it leaves behind.
func TestStylesAreWhatTheyAreCalled(t *testing.T) {
	styles := learnGame{}.Styles()
	players := refs("x", "y", "z")
	reads := func(style string) SeatReads {
		var total SeatReads
		for seed := int64(1); seed <= 4; seed++ {
			seats := map[string]module.Bot{"x": styles[style], "y": bot{}, "z": bot{}}
			state, _ := New().NewMatch(learnGame{}.Config(3, ""), players, seed)
			r := playBots(t, state, players, seats, 20000).seat("x").Reads
			total.Hands += r.Hands
			total.VPIP += r.VPIP
			total.PFR += r.PFR
			total.PostBets += r.PostBets
			total.PostCalls += r.PostCalls
			total.FacedBet += r.FacedBet
			total.FoldedToBet += r.FoldedToBet
		}
		return total
	}
	rate := func(a, b int) float64 { return float64(a) / math.Max(1, float64(b)) }
	if r := reads("maniac"); rate(r.PFR, r.Hands) < 0.6 || r.FoldedToBet != 0 {
		t.Errorf("maniac: %+v", r)
	}
	if r := reads("rock"); rate(r.VPIP, r.Hands) > 0.2 {
		t.Errorf("rock: %+v", r)
	}
	if r := reads("station"); r.PFR != 0 || r.PostBets != 0 || r.FoldedToBet != 0 {
		t.Errorf("station: %+v", r)
	}
}

// TestEnvRunsHoldem is a smoke run of the training environment: a random
// policy at mixed tables, for well over a thousand hands, with nothing illegal
// and nothing stalled.
func TestEnvRunsHoldem(t *testing.T) {
	specs := []learn.TableSpec{
		{Plan: []string{learn.Learner, "hard"}},
		{Plan: []string{"maniac", learn.Learner}},
		{Plan: []string{learn.Learner, "rock", "station"}},
		{Plan: []string{learn.Learner, learn.Learner}},
		{Plan: []string{learn.Learner, "hard", "maniac", "rock", "riverbluffer", learn.Learner}},
	}
	// Half the deals short-stacked, so the push/fold table is in the run.
	env, err := learn.NewEnvWith(learnGame{}, "", specs, 1, 20000, learn.EnvOptions{ShortStacks: 0.5})
	if err != nil {
		t.Fatal(err)
	}
	obs, err := env.Observe()
	if err != nil {
		t.Fatal(err)
	}
	rnd := rand.New(rand.NewSource(1))
	episodes, steps := 0, 0
	var total float64
	selfPlay := map[string]float64{}
	for episodes < 1200 {
		choices := make([]int, len(obs))
		for i, o := range obs {
			if len(o.Obs) != stateDim || len(o.Cands) < 2 {
				t.Fatalf("table %d: obs %d, %d candidates", i, len(o.Obs), len(o.Cands))
			}
			for _, c := range o.Cands {
				if len(c) != candDim {
					t.Fatalf("table %d: candidate of %d", i, len(c))
				}
			}
			choices[i] = rnd.Intn(len(o.Cands))
		}
		if obs, err = env.Step(choices); err != nil {
			t.Fatal(err)
		}
		steps++
		for i, o := range obs {
			if o.Illegal != 0 || o.Stalls != 0 {
				t.Fatalf("table %d: illegal %d, stalls %d", i, o.Illegal, o.Stalls)
			}
			for _, ev := range o.Events {
				total += float64(ev.Reward)
				if ev.Done {
					episodes++
				}
				if i == 3 {
					selfPlay[ev.Seat] += float64(ev.Reward)
				}
			}
		}
	}
	t.Logf("%d steps, %d episodes, random policy earned %+.1f bb in all", steps, episodes, total)
	if sum := selfPlay["p0"] + selfPlay["p1"]; math.Abs(sum) > 1e-2 {
		t.Errorf("learner against learner: %+.3f bb in total, want 0", sum)
	}
}

// BenchmarkEncode is the encoder's cost per call, over decisions from real
// matches at two to six seats.
//
//	go test ./internal/holdem/ -run XXX -bench Encode
func BenchmarkEncode(b *testing.B) {
	var ds []decision
	for seed := int64(1); len(ds) < 300; seed++ {
		playDecisions(b, 2+int(seed%5), seed, func(d decision, _ []learn.Candidate) { ds = append(ds, d) })
	}
	g := learnGame{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d := ds[i%len(ds)]
		if _, err := g.Encode(d.state, d.actor); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkEnv is decisions per second through the environment: a random
// policy at 32 tables of two to six seats, against the heuristic.
func BenchmarkEnv(b *testing.B) {
	var specs []learn.TableSpec
	for i := 0; i < 32; i++ {
		plan := []string{learn.Learner}
		for j := 1; j < 2+i%5; j++ {
			plan = append(plan, []string{"hard", "maniac", "rock", learn.Learner}[(i+j)%4])
		}
		specs = append(specs, learn.TableSpec{Plan: plan})
	}
	env, err := learn.NewEnv(learnGame{}, "", specs, 1, 20000)
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
	decisions := 0
	for i := 0; i < b.N; i++ {
		choices := make([]int, len(obs))
		for j, o := range obs {
			choices[j] = rnd.Intn(len(o.Cands))
		}
		if obs, err = env.Step(choices); err != nil {
			b.Fatal(err)
		}
		decisions += len(choices)
	}
	b.ReportMetric(float64(decisions)/time.Since(start).Seconds(), "decisions/s")
}
