package canasta

import (
	"reflect"
	"testing"

	"zolik/server/internal/learn"
	"zolik/server/internal/module"
)

// An AI seat at a CanastaX partnership table plays exactly what Hard plays,
// and at a table the model was trained for it plays the model.
func TestAISeatPlaysHardWhereTheModelIsUntrained(t *testing.T) {
	if _, err := learn.SetHardModel("canasta", true); err != nil {
		t.Fatalf("switch the model on: %v", err)
	}
	t.Cleanup(func() { _, _ = learn.SetHardModel("canasta", false) })

	m := New()
	live := m.Bot()
	if _, ok := live.(interface{ Heuristic() module.Bot }); !ok {
		t.Fatalf("Bot() with the model on is %T, want a model-backed bot", live)
	}
	net := learn.HardModel(learnGame{}, bot{})

	cases := []struct {
		variation string
		seats     int
		wantHard  bool
	}{
		{variationCanastaX, 4, true},
		{variationCanastaX, 6, true},
		{variationCanastaX, 2, false},
		{variationCanastaX, 3, false},
		{"classic", 4, false},
		{"samba", 4, false},
	}
	ids := []string{"p1", "p2", "p3", "p4", "p5", "p6"}
	for _, c := range cases {
		state, err := m.NewMatch(module.MatchConfig{Variation: c.variation}, refs(ids[:c.seats]...), 11)
		if err != nil {
			t.Fatalf("%s/%d: NewMatch: %v", c.variation, c.seats, err)
		}
		differs := false
		// Walk the first stretch of the deal, comparing the live AI seat's
		// choice with Hard's and with the model's at every decision.
		for step := 0; step < 60; step++ {
			s, err := decode(state)
			if err != nil {
				t.Fatal(err)
			}
			if s.Status != "active" || s.Break.Open {
				break
			}
			who := s.Current
			offers, err := m.LegalActions(state, who)
			if err != nil {
				t.Fatal(err)
			}
			ai := module.BotSeat{PlayerID: who, Skill: module.SkillAI, Seed: int64(step)}
			hard := ai
			hard.Skill = module.SkillHard
			got, ok := live.Act(state, ai, offers)
			if !ok {
				t.Fatalf("%s/%d step %d: no move", c.variation, c.seats, step)
			}
			want, _ := bot{}.Act(state, hard, offers)
			fromNet, _ := net.Act(state, ai, offers)
			if c.wantHard && !reflect.DeepEqual(got, want) {
				t.Fatalf("%s/%d step %d: AI seat played %+v, Hard plays %+v", c.variation, c.seats, step, got, want)
			}
			if !c.wantHard && !reflect.DeepEqual(got, fromNet) {
				t.Fatalf("%s/%d step %d: AI seat played %+v, the model plays %+v", c.variation, c.seats, step, got, fromNet)
			}
			differs = differs || !reflect.DeepEqual(want, fromNet)
			state, _, err = m.Apply(state, who, got)
			if err != nil {
				t.Fatalf("%s/%d step %d: %v", c.variation, c.seats, step, err)
			}
		}
		// Otherwise the comparison above proved nothing about which bot moved.
		if !differs {
			t.Errorf("%s/%d: Hard and the model agreed on every move; pick another seed", c.variation, c.seats)
		}
	}
}
