package sedma

import (
	"testing"

	"zolik/server/internal/lessontest"
	"zolik/server/internal/module"
)

func TestLessonsWalk(t *testing.T) { lessontest.Check(t, New()) }

func TestLessonsFollowOptions(t *testing.T) {
	get := func(opts module.Options) module.Lesson {
		ls, err := New().Lessons(module.MatchConfig{Options: opts})
		if err != nil || len(ls) == 0 {
			t.Fatalf("lessons: %v", err)
		}
		return ls[0]
	}
	find := func(l module.Lesson, id string) (module.LessonStep, bool) {
		for _, s := range l.Steps {
			if s.ID == id {
				return s, true
			}
		}
		return module.LessonStep{}, false
	}
	a, _ := find(get(module.Options{OptTarget: 3}), "target")
	b, _ := find(get(module.Options{OptTarget: 10}), "target")
	if a.Params["n"] != 3 || b.Params["n"] != 10 {
		t.Errorf("target not passed: %v %v", a.Params, b.Params)
	}
	if _, ok := find(get(module.Options{OptShowCardPoints: module.OptOff}), "points"); ok {
		t.Error("points step present with card points hidden")
	}
	if _, ok := find(get(module.Options{OptShowCardPoints: module.OptOn}), "points"); !ok {
		t.Error("points step missing with card points shown")
	}
}
