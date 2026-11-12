package zolikmod

import (
	"reflect"
	"testing"

	"zolik/server/internal/learn"
	"zolik/server/internal/module"
	"zolik/server/internal/rules"
)

// An AI seat at a Continental table plays exactly what Hard plays, and at a
// classic table it plays the model.
func TestAISeatPlaysHardAtContinental(t *testing.T) {
	if _, err := learn.SetHardModel("zolik", true); err != nil {
		t.Fatalf("switch the model on: %v", err)
	}
	t.Cleanup(func() { _, _ = learn.SetHardModel("zolik", false) })

	m := New()
	live := m.Bot()
	if _, ok := live.(interface{ Heuristic() module.Bot }); !ok {
		t.Fatalf("Bot() with the model on is %T, want a model-backed bot", live)
	}
	net := learn.HardModel(learnGame{}, heuristicBot{})

	for _, c := range []struct {
		variation string
		wantHard  bool
	}{
		{varContinental, true},
		{varClassic, false},
		{varFloor35, false},
	} {
		raw, err := m.NewMatch(learnGame{}.Config(4, c.variation), learn.Players(4), 7)
		if err != nil {
			t.Fatal(err)
		}
		differs := false
		for step := 0; step < 120; step++ {
			s, err := decode(raw)
			if err != nil {
				t.Fatal(err)
			}
			if s.Rules.Status != rules.StatusActive {
				break
			}
			who := s.Rules.CurrentTurn
			offers, err := m.LegalActions(raw, who)
			if err != nil {
				t.Fatal(err)
			}
			ai := module.BotSeat{PlayerID: who, Skill: module.SkillAI, Seed: int64(step)}
			hard := ai
			hard.Skill = module.SkillHard
			got, ok := live.Act(raw, ai, offers)
			want, wantOK := heuristicBot{}.Act(raw, hard, offers)
			if !ok || !wantOK {
				// Between deals, where the engine moves for everyone.
				break
			}
			fromNet, _ := net.Act(raw, ai, offers)
			if c.wantHard && !reflect.DeepEqual(got, want) {
				t.Fatalf("%s step %d: AI seat played %+v, Hard plays %+v", c.variation, step, got, want)
			}
			if !c.wantHard && !reflect.DeepEqual(got, fromNet) {
				t.Fatalf("%s step %d: AI seat played %+v, the model plays %+v", c.variation, step, got, fromNet)
			}
			differs = differs || !reflect.DeepEqual(want, fromNet)
			if raw, _, err = m.Apply(raw, who, got); err != nil {
				t.Fatalf("%s step %d: %v", c.variation, step, err)
			}
		}
		if !differs {
			t.Errorf("%s: Hard and the model agreed on every move; pick another seed", c.variation)
		}
	}
}
