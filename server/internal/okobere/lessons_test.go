package okobere

import (
	"testing"

	"zolik/server/internal/lessontest"
	"zolik/server/internal/module"
)

func TestLessonsWalk(t *testing.T) { lessontest.Check(t, New()) }

func TestLessonFollowsOptions(t *testing.T) {
	get := func(opts module.Options) module.Lesson {
		ls, err := New().Lessons(module.MatchConfig{Options: opts})
		if err != nil || len(ls) == 0 {
			t.Fatalf("lessons: %v", err)
		}
		return ls[0]
	}
	step := func(l module.Lesson, id string) module.LessonStep {
		for _, s := range l.Steps {
			if s.ID == id {
				return s
			}
		}
		t.Fatalf("no step %s", id)
		return module.LessonStep{}
	}
	if a, b := step(get(module.Options{OptOkoPays: 1}), "oko"), step(get(module.Options{OptOkoPays: 3}), "oko"); a.TextKey == b.TextKey {
		t.Errorf("oko step should differ between payouts: %s", a.TextKey)
	}
	if got := step(get(module.Options{OptMinBet: 5}), "stake").Params["n"]; got != 5 {
		t.Errorf("stake n = %v, want 5", got)
	}
	if got := step(get(module.Options{OptRoundsPerBank: 8}), "rotation").Params["n"]; got != 8 {
		t.Errorf("rotation n = %v, want 8", got)
	}
}
