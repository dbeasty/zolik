package lora

import (
	"testing"

	"zolik/server/internal/lessontest"
	"zolik/server/internal/module"
)

func TestLessonsWalk(t *testing.T) { lessontest.Check(t, New()) }

func stepByID(t *testing.T, cfg module.MatchConfig, id string) (module.LessonStep, bool) {
	t.Helper()
	ls, err := New().Lessons(cfg)
	if err != nil || len(ls) == 0 {
		t.Fatalf("lessons: %v", err)
	}
	for _, s := range ls[0].Steps {
		if s.ID == id {
			return s, true
		}
	}
	return module.LessonStep{}, false
}

func TestLessonsFollowOptions(t *testing.T) {
	if _, ok := stepByID(t, module.MatchConfig{Options: module.Options{OptMaturita: module.OptOff}}, "maturita"); ok {
		t.Error("maturita step present with the maturita off")
	}
	if _, ok := stepByID(t, module.MatchConfig{Options: module.Options{OptMaturita: module.OptOn}}, "maturita"); !ok {
		t.Error("maturita step missing with the maturita on")
	}
	s, _ := stepByID(t, module.MatchConfig{Options: module.Options{OptTalie: 2}}, "setup")
	if s.Params["talie"] != 2 {
		t.Errorf("talie param = %v, want 2", s.Params["talie"])
	}
}
