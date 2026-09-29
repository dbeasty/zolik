package learn

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"testing"

	"zolik/server/internal/module"
)

// nim is the smallest game that exercises everything here: a pile of stones,
// each turn takes one to three, whoever takes the last wins. It has a known
// perfect strategy (leave a multiple of four), which makes it a bench with a
// right answer, and it is a real GameModule, so the bench and the environment
// drive it exactly as they drive a card game.
type nim struct{}

type nimState struct {
	Pile    int      `json:"pile"`
	Players []string `json:"players"`
	Turn    int      `json:"turn"`
	Winner  string   `json:"winner,omitempty"`
}

func decodeNim(s module.State) nimState {
	var n nimState
	_ = json.Unmarshal(s, &n)
	return n
}

func (nim) Descriptor() module.ModuleDescriptor { return module.ModuleDescriptor{} }

func (nim) NewMatch(_ module.MatchConfig, players []module.PlayerRef, seed int64) (module.State, error) {
	ids := make([]string, len(players))
	for i, p := range players {
		ids[i] = p.ID
	}
	return json.Marshal(nimState{Pile: 10 + int(seed%7), Players: ids})
}

func (nim) Apply(s module.State, playerID string, a module.Action) (module.State, []module.Event, error) {
	n := decodeNim(s)
	if n.Winner != "" || n.Players[n.Turn] != playerID {
		return nil, nil, fmt.Errorf("not your turn")
	}
	var take int
	if _, err := fmt.Sscanf(a.OfferID, "take%d", &take); err != nil || take < 1 || take > 3 || take > n.Pile {
		return nil, nil, fmt.Errorf("illegal take %q", a.OfferID)
	}
	n.Pile -= take
	if n.Pile == 0 {
		n.Winner = playerID
	} else {
		n.Turn = (n.Turn + 1) % len(n.Players)
	}
	out, err := json.Marshal(n)
	return out, nil, err
}

func (nim) View(s module.State, _ string) (module.ViewModel, error) {
	n := decodeNim(s)
	var vm module.ViewModel
	for i, id := range n.Players {
		vm.Seats = append(vm.Seats, module.Seat{PlayerID: id, Active: n.Winner == "" && i == n.Turn})
	}
	return vm, nil
}

func (nim) LegalActions(s module.State, playerID string) ([]module.ActionOffer, error) {
	n := decodeNim(s)
	var out []module.ActionOffer
	for k := 1; k <= 3; k++ {
		out = append(out, module.ActionOffer{ID: fmt.Sprintf("take%d", k), Verb: "take",
			Enabled: n.Winner == "" && n.Players[n.Turn] == playerID && k <= n.Pile})
	}
	return out, nil
}

func (nim) Finished(s module.State) (bool, []string, error) {
	n := decodeNim(s)
	if n.Winner == "" {
		return false, nil, nil
	}
	return true, []string{n.Winner}, nil
}

// perfect leaves a multiple of four whenever it can.
type perfect struct{}

func (perfect) Act(s module.State, _ module.BotSeat, offers []module.ActionOffer) (module.Action, bool) {
	if k := decodeNim(s).Pile % 4; k != 0 {
		for _, o := range offers {
			if o.ID == fmt.Sprintf("take%d", k) && o.Enabled {
				return module.Action{OfferID: o.ID, Verb: o.Verb}, true
			}
		}
	}
	return module.ChooseAction(offers, nil)
}

// liar always proposes a move the engine will refuse.
type liar struct{}

func (liar) Act(module.State, module.BotSeat, []module.ActionOffer) (module.Action, bool) {
	return module.Action{OfferID: "take9", Verb: "take"}, true
}

type nimGame struct{}

func (nimGame) Name() string                          { return "nim" }
func (nimGame) Module() module.GameModule             { return nim{} }
func (nimGame) Config(int, string) module.MatchConfig { return module.MatchConfig{} }
func (nimGame) Heuristic() module.Bot                 { return perfect{} }
func (nimGame) Styles() map[string]module.Bot         { return map[string]module.Bot{"greedy": greedy{}} }
func (nimGame) StateDim() int                         { return 5 }
func (nimGame) CandDim() int                          { return 3 }

func (nimGame) Outcome(s module.State, seat string) (float64, error) {
	if decodeNim(s).Winner == seat {
		return 1, nil
	}
	return 0, nil
}

func (nimGame) Encode(s module.State, _ string) ([]float32, error) {
	n := decodeNim(s)
	obs := make([]float32, 5)
	obs[0] = float32(n.Pile) / 16
	obs[1+n.Pile%4] = 1
	return obs, nil
}

func (nimGame) Candidates(_ module.State, _ string, offers []module.ActionOffer) ([]Candidate, error) {
	var out []Candidate
	for i, o := range offers {
		if !o.Enabled {
			continue
		}
		f := make([]float32, 3)
		f[i] = 1
		out = append(out, Candidate{Action: module.Action{OfferID: o.ID, Verb: o.Verb}, Features: f})
	}
	return out, nil
}

func (nimGame) Reward(before, after module.State, seat string) (float32, bool, error) {
	w := decodeNim(after).Winner
	if w == "" || decodeNim(before).Winner != "" {
		return 0, false, nil
	}
	if w == seat {
		return 1, true, nil
	}
	return -1, true, nil
}

// greedy always takes three.
type greedy struct{}

func (greedy) Act(_ module.State, _ module.BotSeat, offers []module.ActionOffer) (module.Action, bool) {
	for i := len(offers) - 1; i >= 0; i-- {
		if offers[i].Enabled {
			return module.Action{OfferID: offers[i].ID, Verb: offers[i].Verb}, true
		}
	}
	return module.Action{}, false
}

func init() { Register(nimGame{}) }

// --- the network --------------------------------------------------------------

func dense(in, out int, w, b []float32) Dense { return Dense{In: in, Out: out, W: w, B: b} }

// preferThird is a hand-built net whose logit is the candidate's third feature:
// it always takes three.
func preferThird() *Net {
	// trunk: 5 -> 1, always zero after ReLU (all weights zero)
	return &Net{
		Game: "nim", StateDim: 5, CandDim: 3,
		Trunk:  []Dense{dense(5, 1, make([]float32, 5), []float32{0})},
		Scorer: []Dense{dense(4, 1, []float32{0, 0, 0, 1}, []float32{0})},
		Value:  []Dense{dense(1, 1, []float32{1}, []float32{0.5})},
	}
}

func TestNetRoundTrips(t *testing.T) {
	n := preferThird()
	b, err := n.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	m, err := LoadNet(b)
	if err != nil {
		t.Fatal(err)
	}
	emb := m.Embed([]float32{1, 2, 3, 4, 5})
	got := m.Logits(emb, [][]float32{{1, 0, 0}, {0, 0, 1}})
	if got[0] != 0 || got[1] != 1 {
		t.Errorf("logits %v, want [0 1]", got)
	}
	if v := m.ValueOf(emb); v != 0.5 {
		t.Errorf("value %v, want 0.5", v)
	}
}

func TestNetForwardMatchesHandArithmetic(t *testing.T) {
	// trunk 2->2 with a ReLU that must clip, scorer 3->2->1.
	n := &Net{StateDim: 2, CandDim: 1,
		Trunk:  []Dense{dense(2, 2, []float32{1, 2, -1, -1}, []float32{0.5, 0})},
		Scorer: []Dense{dense(3, 2, []float32{1, 0, 1, 0, 1, -1}, []float32{0, 0}), dense(2, 1, []float32{2, 3}, []float32{-1})},
		Value:  []Dense{dense(2, 1, []float32{1, 1}, []float32{0})},
	}
	if err := n.check(); err != nil {
		t.Fatal(err)
	}
	emb := n.Embed([]float32{1, 1}) // [1+2+0.5, max(0,-2)] = [3.5, 0]
	if emb[0] != 3.5 || emb[1] != 0 {
		t.Fatalf("emb %v", emb)
	}
	// x=[3.5,0,2]: h=[3.5+2, 0-2 -> 0] = [5.5,0]; out = 11 - 1 = 10
	if got := n.Logits(emb, [][]float32{{2}})[0]; got != 10 {
		t.Errorf("logit %v, want 10", got)
	}
}

func TestLoadNetRefusesBrokenModels(t *testing.T) {
	good, _ := preferThird().Marshal()
	for name, b := range map[string][]byte{
		"empty":     nil,
		"magic":     append([]byte("NOTNET\n"), good[7:]...),
		"truncated": good[:len(good)-4],
		"trailing":  append(append([]byte(nil), good...), 0, 0, 0, 0),
	} {
		if _, err := LoadNet(b); err == nil {
			t.Errorf("%s: loaded", name)
		}
	}
	bad := preferThird()
	bad.Scorer = []Dense{dense(3, 1, []float32{0, 0, 1}, []float32{0})} // ignores the embedding width
	b, _ := bad.Marshal()
	if _, err := LoadNet(b); err == nil || !strings.Contains(err.Error(), "scorer") {
		t.Errorf("mis-chained scorer: %v", err)
	}
}

// --- the bot ------------------------------------------------------------------

func TestNetBotPlaysItsFavourite(t *testing.T) {
	s, _ := nim{}.NewMatch(module.MatchConfig{}, Players(2), 3) // pile 13
	offers, _ := nim{}.LegalActions(s, "p0")
	bot := NetBot{Game: nimGame{}, Net: preferThird(), Fallback: perfect{}}
	a, ok := bot.Act(s, module.BotSeat{PlayerID: "p0"}, offers)
	if !ok || a.OfferID != "take3" {
		t.Errorf("got %+v, want take3", a)
	}
}

func TestNetBotFallsBackWithoutAUsableModel(t *testing.T) {
	s, _ := nim{}.NewMatch(module.MatchConfig{}, Players(2), 3) // pile 13: perfect takes 1
	offers, _ := nim{}.LegalActions(s, "p0")
	wrong := preferThird()
	wrong.CandDim = 4
	for name, n := range map[string]*Net{"nil": nil, "other encoder": wrong} {
		a, _ := NetBot{Game: nimGame{}, Net: n, Fallback: perfect{}}.Act(s, module.BotSeat{PlayerID: "p0"}, offers)
		if a.OfferID != "take1" {
			t.Errorf("%s: got %s, want the heuristic's take1", name, a.OfferID)
		}
	}
}

func TestNetBotSamplingIsReproducible(t *testing.T) {
	s, _ := nim{}.NewMatch(module.MatchConfig{}, Players(2), 3)
	offers, _ := nim{}.LegalActions(s, "p0")
	bot := NetBot{Game: nimGame{}, Net: preferThird(), Temperature: 5}
	seen := map[string]bool{}
	for seed := int64(0); seed < 60; seed++ {
		seat := module.BotSeat{PlayerID: "p0", Seed: seed}
		a, _ := bot.Act(s, seat, offers)
		b, _ := bot.Act(s, seat, offers)
		if a.OfferID != b.OfferID {
			t.Fatalf("seed %d: %s then %s", seed, a.OfferID, b.OfferID)
		}
		seen[a.OfferID] = true
	}
	if len(seen) < 2 {
		t.Errorf("a hot temperature only ever played %v", seen)
	}
}

// --- the bench ----------------------------------------------------------------

func TestBenchFindsThePerfectPlayer(t *testing.T) {
	g := nimGame{}
	r, err := Bench(g, 2, "", Contender{Name: "perfect", Bot: perfect{}}, Contender{Name: "greedy", Bot: greedy{}}, 1, 40, 200)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Significant() || r.Illegal != 0 || r.Stalls != 0 {
		t.Errorf("perfect vs greedy: %v", r)
	}
}

func TestBenchOfABotAgainstItselfIsZero(t *testing.T) {
	r, err := Bench(nimGame{}, 2, "", Contender{Bot: greedy{}}, Contender{Bot: greedy{}}, 1, 30, 200)
	if err != nil {
		t.Fatal(err)
	}
	if r.Mean != 0 {
		t.Errorf("greedy vs greedy: %v — the two seatings did not cancel", r)
	}
}

func TestBenchCountsIllegalMovesAndPlaysOn(t *testing.T) {
	r, err := Bench(nimGame{}, 2, "", Contender{Bot: liar{}}, Contender{Bot: greedy{}}, 1, 5, 200)
	if err != nil {
		t.Fatal(err)
	}
	if r.Illegal == 0 || r.Stalls != 0 || r.Seeds != 5 {
		t.Errorf("liar: %v", r)
	}
}

// --- the environment ----------------------------------------------------------

func TestEnvStopsOnlyAtLearnerDecisions(t *testing.T) {
	specs := []TableSpec{{Plan: []string{Learner, "hard"}}, {Plan: []string{"greedy", Learner}}, {Plan: []string{Learner, Learner}}}
	e, err := NewEnv(nimGame{}, "", specs, 1, 1000)
	if err != nil {
		t.Fatal(err)
	}
	obs, err := e.Observe()
	if err != nil {
		t.Fatal(err)
	}
	rnd := rand.New(rand.NewSource(1))
	rewards := map[int]float32{}
	dones := map[int]int{}
	for step := 0; step < 500; step++ {
		choices := make([]int, len(obs))
		for i, o := range obs {
			if e.tables[i].plan[o.Seat] != Learner {
				t.Fatalf("table %d asked about %s, a %s seat", i, o.Seat, e.tables[i].plan[o.Seat])
			}
			if len(o.Cands) < 2 || len(o.Obs) != 5 {
				t.Fatalf("table %d: %d candidates, obs %d", i, len(o.Cands), len(o.Obs))
			}
			choices[i] = rnd.Intn(len(o.Cands))
		}
		if obs, err = e.Step(choices); err != nil {
			t.Fatal(err)
		}
		for i, o := range obs {
			for _, ev := range o.Events {
				rewards[i] += ev.Reward
				if ev.Done {
					dones[i]++
				}
			}
			if o.Illegal != 0 || o.Stalls != 0 {
				t.Fatalf("table %d: illegal %d stalls %d", i, o.Illegal, o.Stalls)
			}
		}
	}
	for i := range specs {
		if dones[i] == 0 {
			t.Errorf("table %d never finished an episode", i)
		}
	}
	// A random learner loses to perfect play; two learners at one table are
	// zero-sum between them.
	if rewards[0] >= 0 {
		t.Errorf("random vs perfect earned %v", rewards[0])
	}
	if rewards[2] != 0 {
		t.Errorf("learner vs learner earned %v in total, want 0", rewards[2])
	}
}

func TestEnvSeatsACheckpoint(t *testing.T) {
	dir := t.TempDir()
	b, _ := preferThird().Marshal()
	path := dir + "/third.bin"
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	e, err := NewEnv(nimGame{}, "", []TableSpec{{Plan: []string{Learner, "net:" + path + "@0"}}}, 1, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Observe(); err != nil {
		t.Fatal(err)
	}
	if _, err := NewEnv(nimGame{}, "", []TableSpec{{Plan: []string{Learner, "nobody"}}}, 1, 1000); err == nil {
		t.Error("an unknown seat was accepted")
	}
}

func TestServeSpeaksJSONLines(t *testing.T) {
	in := strings.Join([]string{
		`{"op":"info","game":"nim"}`,
		`{"op":"step","choices":[0]}`,
		`{"op":"reset","game":"nim","seed":1,"tables":[{"plan":["learner","greedy"]}]}`,
		`{"op":"step","choices":[1]}`,
		`{"op":"reset","game":"chess"}`,
	}, "\n")
	var out bytes.Buffer
	if err := Serve(strings.NewReader(in), &out); err != nil {
		t.Fatal(err)
	}
	var reps []reply
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		var r reply
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatalf("%q: %v", line, err)
		}
		reps = append(reps, r)
	}
	if len(reps) != 5 {
		t.Fatalf("%d replies", len(reps))
	}
	if reps[0].StateDim != 5 || reps[0].CandDim != 3 {
		t.Errorf("info: %+v", reps[0])
	}
	if reps[1].Error == "" || reps[4].Error == "" {
		t.Errorf("errors not reported: %q %q", reps[1].Error, reps[4].Error)
	}
	if len(reps[2].Tables) != 1 || len(reps[3].Tables) != 1 || reps[3].Error != "" {
		t.Errorf("reset/step: %+v %+v", reps[2], reps[3])
	}
}
