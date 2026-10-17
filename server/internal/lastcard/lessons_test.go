package lastcard

import (
	"testing"

	"zolik/server/internal/lessontest"
	"zolik/server/internal/module"
)

func TestLessonsWalk(t *testing.T) { lessontest.Check(t, New()) }

func lessonSteps(t *testing.T, opts map[string]int) map[string]module.LessonStep {
	t.Helper()
	ls, err := New().Lessons(module.MatchConfig{Options: opts})
	if err != nil || len(ls) == 0 {
		t.Fatalf("lessons: %v", err)
	}
	m := map[string]module.LessonStep{}
	for _, s := range ls[0].Steps {
		m[s.ID] = s
	}
	return m
}

func TestLessonFollowsOptions(t *testing.T) {
	base := lessonSteps(t, nil)
	if _, ok := base["stacking"]; ok {
		t.Fatal("stacking step present with stacking off")
	}
	if _, ok := base["call"]; !ok {
		t.Fatal("call step missing with the call on")
	}
	got := lessonSteps(t, map[string]int{OptStacking: stackAny, OptSevenZero: module.OptOn,
		OptLastCardCall: module.OptOff, OptDrawFourChallenge: module.OptOff, OptHandSize: 5})
	for _, id := range []string{"stacking", "sevenZero"} {
		if _, ok := got[id]; !ok {
			t.Fatalf("%s step missing", id)
		}
	}
	for _, id := range []string{"call", "challenge"} {
		if _, ok := got[id]; ok {
			t.Fatalf("%s step present with the rule off", id)
		}
	}
	if got["goal"].Params["n"] != 5 {
		t.Fatalf("hand size not passed: %v", got["goal"].Params)
	}
	low := lessonSteps(t, map[string]int{OptScoring: scoreLowest})
	if low["scoring"].TextKey == base["scoring"].TextKey {
		t.Fatal("scoring step ignores the scoring option")
	}
	single := lessonSteps(t, map[string]int{OptTargetScore: 0})
	if single["scoring"].TextKey == base["scoring"].TextKey {
		t.Fatal("scoring step ignores a single-deal match")
	}
}
