package lessontest_test

import (
	"strings"
	"testing"

	"zolik/server/internal/lessontest"
	"zolik/server/internal/module"
	"zolik/server/internal/prsi"
)

// The walk is only worth having if it can fail. A lesson that waits for a
// move the game never offers must be caught, not played past.
func TestWalkFailsWhenAStepCanNeverBeDone(t *testing.T) {
	m := prsi.New()
	l := module.Lesson{
		ID: "impossible", TitleKey: "t", Seed: 3, Bots: 1,
		Steps: []module.LessonStep{
			module.Do("never", "k", nil, []string{"no_such_verb"}).Held("w"),
		},
	}
	err := lessontest.Walk(m, module.MatchConfig{}, l)
	if err == nil || !strings.Contains(err.Error(), "never") {
		t.Fatalf("Walk = %v, want a failure naming the step", err)
	}
}

func TestWalkRefusesAnEmptyOrWrongSizedTable(t *testing.T) {
	m := prsi.New()
	l := module.Lesson{ID: "alone", TitleKey: "t", Seed: 3, Bots: 0,
		Steps: []module.LessonStep{module.Say("a", "k", nil)}}
	if err := lessontest.Walk(m, module.MatchConfig{}, l); err == nil {
		t.Fatal("a lesson that seats one player at a two-player game should fail")
	}
}

func TestValidateCatchesAuthoringMistakes(t *testing.T) {
	cases := map[string]module.Lesson{
		"no steps":          {ID: "a", TitleKey: "t"},
		"repeated step id":  {ID: "a", TitleKey: "t", Steps: []module.LessonStep{module.Say("x", "k", nil), module.Say("x", "k", nil)}},
		"act without verbs": {ID: "a", TitleKey: "t", Steps: []module.LessonStep{{ID: "x", Kind: module.StepAct, TextKey: "k"}}},
		"strict, no words":  {ID: "a", TitleKey: "t", Steps: []module.LessonStep{{ID: "x", Kind: module.StepAct, TextKey: "k", Verbs: []string{"v"}, Strict: true}}},
		"unknown kind":      {ID: "a", TitleKey: "t", Steps: []module.LessonStep{{ID: "x", Kind: "dance", TextKey: "k"}}},
	}
	for name, l := range cases {
		if err := l.Validate(); err == nil {
			t.Errorf("%s: Validate accepted it", name)
		}
	}
}

func TestConfigsCoverEveryVariationAndOptionChoice(t *testing.T) {
	m := prsi.New()
	cfgs := lessontest.Configs(m)
	d := m.Descriptor()
	want := 1
	for _, o := range d.Options {
		want += len(o.Choices)
	}
	if len(cfgs) < want {
		t.Errorf("Configs = %d, want at least %d (defaults + every option choice)", len(cfgs), want)
	}
}
