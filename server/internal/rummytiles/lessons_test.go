package rummytiles

import (
	"testing"

	"zolik/server/internal/lessontest"
	"zolik/server/internal/module"
)

func TestLessonsWalk(t *testing.T) { lessontest.Check(t, New()) }

func lessonStep(t *testing.T, cfg module.MatchConfig, id string) module.LessonStep {
	t.Helper()
	ls, err := New().Lessons(cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range ls[0].Steps {
		if s.ID == id {
			return s
		}
	}
	t.Fatalf("no step %s", id)
	return module.LessonStep{}
}

func TestLessonFollowsOptions(t *testing.T) {
	target := lessonStep(t, module.MatchConfig{Options: module.Options{OptTargetScore: 300}}, "match")
	if target.TextKey != "guide.rummytiles.target" || target.Params["n"] != 300 {
		t.Fatalf("target step = %+v", target)
	}
	limit := lessonStep(t, module.MatchConfig{Options: module.Options{OptTargetScore: 0, OptRoundLimit: 10}}, "match")
	if limit.TextKey != "guide.rummytiles.roundLimit" || limit.Params["n"] != 10 {
		t.Fatalf("limit step = %+v", limit)
	}
	on := lessonStep(t, module.MatchConfig{Options: module.Options{OptPoolExhaustion: module.OptOn}}, "pool")
	off := lessonStep(t, module.MatchConfig{Options: module.Options{OptPoolExhaustion: module.OptOff}}, "pool")
	if on.TextKey == off.TextKey {
		t.Fatal("pool step should differ with the exhaustion rule")
	}
}
