package zolikmod

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"math"
	"os"
	"testing"

	"zolik/server/internal/learn"
	"zolik/server/internal/module"
)

// This file records and replays a sweep of real Žolíky decisions, so that a
// network trained for one encoder can be shown to choose exactly as it did on
// that encoder's own build after the encoder has grown. It compiles unchanged
// on the build that recorded testdata/zolik-v2-choices.json.gz (the
// claude/zolik-adapter-v2 branch, encoder 900/62), which is the point: the
// recording and the check are the same code on either side of the change.

// v2Fixture is a sweep of matches, every step of every one recorded with what
// the network made of the position.
type v2Fixture struct {
	// Model is the sha256 of the network the choices were recorded with.
	Model   string    `json:"model"`
	Encoder [2]int    `json:"encoder"`
	Matches []v2Match `json:"matches"`
}

type v2Match struct {
	Variation string   `json:"variation"`
	Seats     int      `json:"seats"`
	Seed      int64    `json:"seed"`
	Steps     []v2Step `json:"steps"`
}

// v2Step is one move of a match: who made it and what it was, which the replay
// applies whatever it decides, and what the network would have played there.
type v2Step struct {
	Player string          `json:"p"`
	Played json.RawMessage `json:"a"`
	// Cands is how many candidates the adapter offered the network.
	Cands int `json:"n"`
	// Obs and Feats fingerprint the network's input: the state vector, and
	// every candidate's action and feature vector in order (only with two or
	// more candidates, when the network is actually asked).
	Obs   string `json:"o,omitempty"`
	Feats string `json:"f,omitempty"`
	// Net is the network's choice at temperature 0.
	Net json.RawMessage `json:"c"`
}

// v2Spec is the sweep: every variation the adapter trains on, at the table
// sizes it is played at, the network in a different seat each match beside
// the heuristic's Hard and Medium.
var v2Spec = []struct {
	variation string
	seats     int
	seeds     []int64
}{
	{varClassic, 2, []int64{11, 12, 13}},
	{varFloor35, 4, []int64{21, 22, 23}},
	{varContinental, 3, []int64{31, 32}},
}

// v2MaxSteps bounds a match, so the fixture stays small.
const v2MaxSteps = 600

// declines is a fallback that never moves, so a step the network itself did
// not choose shows up as a null choice rather than as the heuristic's.
type declines struct{}

func (declines) Act(module.State, module.BotSeat, []module.ActionOffer) (module.Action, bool) {
	return module.Action{}, false
}

func floatsHash(h interface{ Write([]byte) (int, error) }, v []float32) {
	var b [4]byte
	for _, x := range v {
		binary.LittleEndian.PutUint32(b[:], math.Float32bits(x))
		_, _ = h.Write(b[:])
	}
}

// v2Decide is what the network (on g, the encoder it sees) makes of one
// decision.
func v2Decide(t *testing.T, g learn.Game, p *learn.Policy, raw module.State, actor string, offers []module.ActionOffer) v2Step {
	t.Helper()
	view := learn.Positions(g)
	pos, err := view.Position(raw)
	if err != nil {
		t.Fatal(err)
	}
	cands, err := view.CandidatesFor(pos, actor, offers)
	if err != nil {
		t.Fatal(err)
	}
	st := v2Step{Player: actor, Cands: len(cands)}
	if len(cands) >= 2 {
		obs, err := view.EncodeFor(pos, actor)
		if err != nil {
			t.Fatal(err)
		}
		h := sha256.New()
		floatsHash(h, obs)
		st.Obs = hex.EncodeToString(h.Sum(nil))[:16]
		h.Reset()
		for _, c := range cands {
			for _, a := range c.Steps() {
				b, _ := json.Marshal(a)
				_, _ = h.Write(b)
			}
			_, _ = h.Write([]byte{0})
			floatsHash(h, c.Features)
		}
		st.Feats = hex.EncodeToString(h.Sum(nil))[:16]
	}
	seat := module.BotSeat{PlayerID: actor, Skill: module.SkillHard}
	if a, ok := (learn.NetBot{Game: g, Policy: p, Fallback: declines{}}).Act(raw, seat, offers); ok {
		st.Net, _ = json.Marshal(a)
	} else {
		st.Net = json.RawMessage("null")
	}
	return st
}

// v2Record plays the sweep: the network in one seat (it moves as it chose),
// the heuristic in the others.
func v2Record(t *testing.T, g learn.Game, p *learn.Policy, modelSHA string) v2Fixture {
	t.Helper()
	m := New()
	fx := v2Fixture{Model: modelSHA, Encoder: [2]int{g.StateDim(), g.CandDim()}}
	for _, sp := range v2Spec {
		for _, seed := range sp.seeds {
			players := learn.Players(sp.seats)
			net := players[int(seed)%sp.seats].ID
			skills := []module.Skill{module.SkillHard, module.SkillMedium}
			raw, err := m.NewMatch(learnGame{}.Config(sp.seats, sp.variation), players, seed)
			if err != nil {
				t.Fatal(err)
			}
			rec := v2Match{Variation: sp.variation, Seats: sp.seats, Seed: seed}
			for len(rec.Steps) < v2MaxSteps {
				if done, _, err := m.Finished(raw); err != nil {
					t.Fatal(err)
				} else if done {
					break
				}
				actor := module.ActiveSeat(m, raw, players[0].ID, players)
				if actor == "" {
					break
				}
				offers, err := m.LegalActions(raw, actor)
				if err != nil {
					t.Fatal(err)
				}
				st := v2Decide(t, g, p, raw, actor, offers)
				seat := module.BotSeat{PlayerID: actor, Skill: skills[len(rec.Steps)%2], Seed: module.SeatSeed(seed, actor, "bot")}
				var a module.Action
				ok := false
				if actor == net && string(st.Net) != "null" {
					ok = json.Unmarshal(st.Net, &a) == nil
				} else {
					a, ok = heuristicBot{}.Act(raw, seat, offers)
				}
				next, _, err := m.Apply(raw, actor, a)
				if !ok || err != nil {
					a, ok = module.ChooseAction(offers, nil)
					if !ok {
						break
					}
					if next, _, err = m.Apply(raw, actor, a); err != nil {
						break
					}
				}
				st.Played, _ = json.Marshal(a)
				rec.Steps = append(rec.Steps, st)
				raw = next
			}
			fx.Matches = append(fx.Matches, rec)
		}
	}
	return fx
}

// v2Check replays a recorded sweep on this build, the network seeing g, and
// fails on the first decision it makes differently or sees differently.
// It returns how many decisions the network was asked (two or more
// candidates) and how many steps were replayed in all.
func v2Check(t *testing.T, g learn.Game, p *learn.Policy, fx v2Fixture) (asked, steps int) {
	t.Helper()
	m := New()
	for _, rec := range fx.Matches {
		players := learn.Players(rec.Seats)
		raw, err := m.NewMatch(learnGame{}.Config(rec.Seats, rec.Variation), players, rec.Seed)
		if err != nil {
			t.Fatal(err)
		}
		for i, want := range rec.Steps {
			where := func() string {
				return rec.Variation + " seed " + itoa(rec.Seed) + " step " + itoa(int64(i))
			}
			offers, err := m.LegalActions(raw, want.Player)
			if err != nil {
				t.Fatalf("%s: %v", where(), err)
			}
			got := v2Decide(t, g, p, raw, want.Player, offers)
			if got.Cands != want.Cands {
				t.Fatalf("%s: %d candidates, recorded %d", where(), got.Cands, want.Cands)
			}
			if got.Obs != want.Obs || got.Feats != want.Feats {
				t.Fatalf("%s: input differs from the recording (state %s/%s, candidates %s/%s)",
					where(), got.Obs, want.Obs, got.Feats, want.Feats)
			}
			if string(got.Net) != string(want.Net) {
				t.Fatalf("%s: the network plays %s, recorded %s", where(), got.Net, want.Net)
			}
			if want.Cands >= 2 {
				asked++
			}
			var a module.Action
			if err := json.Unmarshal(want.Played, &a); err != nil {
				t.Fatal(err)
			}
			if raw, _, err = m.Apply(raw, want.Player, a); err != nil {
				t.Fatalf("%s: replaying %s: %v", where(), want.Played, err)
			}
			steps++
		}
	}
	return asked, steps
}

func itoa(n int64) string { b, _ := json.Marshal(n); return string(b) }

func writeV2Fixture(t *testing.T, path string, fx v2Fixture) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zw := gzip.NewWriter(f)
	enc := json.NewEncoder(zw)
	if err := enc.Encode(fx); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
}

func readV2Fixture(t *testing.T, path string) v2Fixture {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	var fx v2Fixture
	if err := json.NewDecoder(zr).Decode(&fx); err != nil {
		t.Fatal(err)
	}
	return fx
}
