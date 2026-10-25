package ferbl

import (
	"testing"

	"zolik/server/internal/lessontest"
	"zolik/server/internal/module"
)

func TestLessonsWalk(t *testing.T) { lessontest.Check(t, New()) }

func TestLessonFollowsOptions(t *testing.T) {
	get := func(vizi, hands int) module.Lesson {
		ls, err := New().Lessons(module.MatchConfig{Options: module.Options{OptVizi: vizi, OptHands: hands}})
		if err != nil {
			t.Fatal(err)
		}
		return ls[0]
	}
	param := func(l module.Lesson, step, key string) any {
		for _, s := range l.Steps {
			if s.ID == step {
				return s.Params[key]
			}
		}
		t.Fatalf("no step %s", step)
		return nil
	}
	a, b := get(1, 10), get(5, 30)
	if param(a, "ante", "n") != 1 || param(b, "ante", "n") != 5 {
		t.Fatal("ante step should carry the vizi")
	}
	if param(a, "round2", "n") != 2 || param(b, "round2", "n") != 10 {
		t.Fatal("round2 step should carry twice the vizi")
	}
	if param(a, "goal", "hands") != 10 || param(b, "goal", "hands") != 30 {
		t.Fatal("goal step should carry the match length")
	}
}
