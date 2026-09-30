package learn

import (
	"encoding/json"
	"fmt"
	"testing"

	"zolik/server/internal/module"
)

// trap is the shape of Canasta's wedge with nothing else in it. On its turn a
// seat may "lay", which leaves it in a position where the only thing enabled
// is taking the lay back, or "pass", which ends the turn. The match ends after
// six passes.
type trap struct{}

type trapState struct {
	Players []string `json:"players"`
	Turn    int      `json:"turn"`
	Laid    bool     `json:"laid"`
	Passes  int      `json:"passes"`
}

func decodeTrap(s module.State) trapState {
	var t trapState
	_ = json.Unmarshal(s, &t)
	return t
}

func (trap) Descriptor() module.ModuleDescriptor { return module.ModuleDescriptor{} }

func (trap) NewMatch(_ module.MatchConfig, players []module.PlayerRef, _ int64) (module.State, error) {
	ids := make([]string, len(players))
	for i, p := range players {
		ids[i] = p.ID
	}
	return json.Marshal(trapState{Players: ids})
}

func (trap) Apply(s module.State, playerID string, a module.Action) (module.State, []module.Event, error) {
	t := decodeTrap(s)
	if t.Passes >= 6 || t.Players[t.Turn] != playerID {
		return nil, nil, fmt.Errorf("not your turn")
	}
	switch {
	case a.Verb == "lay" && !t.Laid:
		t.Laid = true
	case a.Verb == "undo" && t.Laid:
		t.Laid = false
	case a.Verb == "pass" && !t.Laid:
		t.Passes++
		t.Turn = (t.Turn + 1) % len(t.Players)
	default:
		return nil, nil, fmt.Errorf("illegal %q", a.Verb)
	}
	out, err := json.Marshal(t)
	return out, nil, err
}

func (trap) View(s module.State, _ string) (module.ViewModel, error) {
	t := decodeTrap(s)
	var vm module.ViewModel
	for i, id := range t.Players {
		vm.Seats = append(vm.Seats, module.Seat{PlayerID: id, Active: t.Passes < 6 && i == t.Turn})
	}
	return vm, nil
}

func (trap) LegalActions(s module.State, playerID string) ([]module.ActionOffer, error) {
	t := decodeTrap(s)
	mine := t.Passes < 6 && t.Players[t.Turn] == playerID
	// The undo comes first, as Canasta's does, so a driver that falls back to
	// the first enabled offer takes the lay back.
	return []module.ActionOffer{
		{ID: "undo", Verb: "undo", Undo: true, Enabled: mine && t.Laid},
		{ID: "lay", Verb: "lay", Enabled: mine && !t.Laid},
		{ID: "pass", Verb: "pass", Enabled: mine && !t.Laid},
	}, nil
}

func (trap) Finished(s module.State) (bool, []string, error) {
	return decodeTrap(s).Passes >= 6, nil, nil
}

// layer lays whenever it can and has nothing to say once it has: the bot that
// walks into the trap and then declines to move.
type layer struct{}

func (layer) Act(_ module.State, _ module.BotSeat, offers []module.ActionOffer) (module.Action, bool) {
	for _, o := range offers {
		if o.Verb == "lay" && o.Enabled {
			return module.SubmissionFor(o)
		}
	}
	return module.Action{}, false
}

type trapGame struct{}

func (trapGame) Name() string                          { return "trap" }
func (trapGame) Module() module.GameModule             { return trap{} }
func (trapGame) Config(int, string) module.MatchConfig { return module.MatchConfig{} }
func (trapGame) Heuristic() module.Bot                 { return layer{} }
func (trapGame) StateDim() int                         { return 1 }
func (trapGame) CandDim() int                          { return 1 }
func (trapGame) Outcome(module.State, string) (float64, error) {
	return 0, nil
}
func (trapGame) Encode(module.State, string) ([]float32, error) { return []float32{0}, nil }
func (trapGame) Candidates(_ module.State, _ string, offers []module.ActionOffer) ([]Candidate, error) {
	var out []Candidate
	for _, o := range offers {
		if a, ok := module.SubmissionFor(o); ok {
			out = append(out, Candidate{Action: a, Features: []float32{0}})
		}
	}
	return out, nil
}
func (trapGame) Reward(before, after module.State, _ string) (float32, bool, error) {
	return 0, decodeTrap(after).Passes > decodeTrap(before).Passes, nil
}

// TestBenchDoesNotReplayAnUndoneMove is the Canasta wedge in the bench: the
// bot lays, declines, the fallback takes the lay back, and without the guard
// the bot lays again until the action budget runs out.
func TestBenchDoesNotReplayAnUndoneMove(t *testing.T) {
	players := Players(2)
	final, st, err := PlayOut(trap{}, module.MatchConfig{}, players, 1, 1000,
		func(string) (module.Bot, module.Skill) { return layer{}, module.SkillHard })
	if err != nil {
		t.Fatal(err)
	}
	if st.Stalled {
		t.Fatalf("stalled (%s) after %d actions", st.Why, st.Actions)
	}
	// Each turn: lay, undo, pass.
	if st.Actions != 18 || st.Illegal != 0 {
		t.Errorf("actions %d illegal %d, want 18 and 0", st.Actions, st.Illegal)
	}
	if decodeTrap(final).Passes != 6 {
		t.Errorf("final %s", final)
	}
}

// TestEnvDoesNotReplayAnUndoneMove is the same wedge in the environment, where
// it cost a Canasta trainer a 50,000-action stall dozens of times a run.
func TestEnvDoesNotReplayAnUndoneMove(t *testing.T) {
	e, err := NewEnv(trapGame{}, "", []TableSpec{{Plan: []string{Learner, "hard"}}}, 1, 1000)
	if err != nil {
		t.Fatal(err)
	}
	obs, err := e.Observe()
	if err != nil {
		t.Fatal(err)
	}
	for step := 0; step < 20; step++ {
		pass := -1
		for i, c := range e.tables[0].pending {
			if c.Action.Verb == "pass" {
				pass = i
			}
		}
		if pass < 0 {
			t.Fatalf("no pass among the learner's candidates")
		}
		if obs, err = e.Step([]int{pass}); err != nil {
			t.Fatal(err)
		}
	}
	if o := obs[0]; o.Stalls != 0 || o.Illegal != 0 || o.Matches == 0 {
		t.Fatalf("stalls %d (%s) illegal %d matches %d", o.Stalls, o.LastStall, o.Illegal, o.Matches)
	}
}

// TestGuardArmsOnlyAfterAnUndo: repeating a move is ordinary play until the
// turn has taken something back.
func TestGuardArmsOnlyAfterAnUndo(t *testing.T) {
	var g turnGuard
	g.begin("p0")
	lay := botCandidate{action: module.Action{Verb: "lay"}}
	g.note(lay)
	if g.skips(lay) {
		t.Fatal("skipped before any undo")
	}
	g.note(botCandidate{action: module.Action{Verb: "undo"}, undo: true})
	if !g.skips(lay) {
		t.Fatal("replayed a move the turn took back")
	}
	if g.skips(botCandidate{action: module.Action{Verb: "undo"}, undo: true}) {
		t.Fatal("skipped an undo")
	}
	g.begin("p1")
	if g.skips(lay) {
		t.Fatal("the guard outlived the turn")
	}
}
