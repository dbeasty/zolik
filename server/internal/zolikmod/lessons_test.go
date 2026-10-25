package zolikmod

import (
	"testing"

	"zolik/server/internal/lessontest"
	"zolik/server/internal/module"
	"zolik/server/internal/rules"
)

func TestLessonsWalk(t *testing.T) { lessontest.Check(t, New()) }

func lessonSteps(t *testing.T, mc module.MatchConfig) map[string]module.LessonStep {
	t.Helper()
	ls, err := New().Lessons(mc)
	if err != nil || len(ls) == 0 {
		t.Fatalf("lessons: %v", err)
	}
	out := map[string]module.LessonStep{}
	for _, s := range ls[0].Steps {
		out[s.ID] = s
	}
	return out
}

func TestLessonsFollowTheOptions(t *testing.T) {
	on := lessonSteps(t, module.MatchConfig{Options: module.Options{rules.OptInitialMeldMinimum: 51, rules.OptRequireCleanRun: rules.OptOn}})
	if s, ok := on["floor"]; !ok || s.Params["n"] != 51 {
		t.Errorf("floor step = %+v, want n=51", s)
	}
	if _, ok := on["cleanRun"]; !ok {
		t.Error("clean-run step missing when the option is on")
	}
	off := lessonSteps(t, module.MatchConfig{Options: module.Options{rules.OptInitialMeldMinimum: 0, rules.OptRequireCleanRun: rules.OptOff}})
	if _, ok := off["floor"]; ok {
		t.Error("floor step present with no minimum")
	}
	if _, ok := off["cleanRun"]; ok {
		t.Error("clean-run step present when the option is off")
	}
	must := lessonSteps(t, module.MatchConfig{Options: module.Options{rules.OptJokerReclaimMustPlay: rules.OptOn}})
	keep := lessonSteps(t, module.MatchConfig{Options: module.Options{rules.OptJokerReclaimMustPlay: rules.OptOff}})
	if must["jokers"].TextKey == keep["jokers"].TextKey {
		t.Error("joker reclaim option does not change the joker step")
	}
}

func TestLessonsFollowTheVariation(t *testing.T) {
	d := New().Descriptor()
	if len(d.Variations) < 2 {
		t.Skip("one variation")
	}
	seen := map[string]bool{}
	for _, v := range d.Variations {
		ls, _ := New().Lessons(module.MatchConfig{Variation: v.ID})
		sig := ""
		for _, s := range ls[0].Steps {
			sig += s.ID + ":" + s.TextKey + ";"
		}
		seen[sig] = true
	}
	if len(seen) < 2 {
		t.Error("every variation gets the same lesson")
	}
}
