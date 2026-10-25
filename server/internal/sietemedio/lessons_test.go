package sietemedio

import (
	"testing"

	"zolik/server/internal/lessontest"
	"zolik/server/internal/module"
)

func TestLessonsWalk(t *testing.T) { lessontest.Check(t, New()) }

// TestLessonFollowsOptions — the four rules the sources disagree about are
// settings, so the guide has to teach whichever one this table plays. A lesson
// that taught the default while the engine played something else is the split
// brain the written rules exist to avoid.
func TestLessonFollowsOptions(t *testing.T) {
	get := func(opts module.Options) module.Lesson {
		t.Helper()
		ls, err := New().Lessons(module.MatchConfig{Options: opts})
		if err != nil || len(ls) == 0 {
			t.Fatalf("lessons: %v", err)
		}
		return ls[0]
	}
	step := func(l module.Lesson, id string) module.LessonStep {
		t.Helper()
		for _, s := range l.Steps {
			if s.ID == id {
				return s
			}
		}
		t.Fatalf("no step %s", id)
		return module.LessonStep{}
	}

	for _, tc := range []struct {
		name string
		id   string
		a, b module.Options
	}{
		{"a level hand", "ties", module.Options{OptTies: tiesToBank}, module.Options{OptTies: tiesPush}},
		{"the bonus", "seven", module.Options{OptSevenPays: sevenPaysDouble}, module.Options{OptSevenPays: sevenPaysEven}},
		{"the opening card", "deal", module.Options{OptFirstCardDown: module.OptOn}, module.Options{OptFirstCardDown: module.OptOff}},
		{"how the bank moves", "bankMoves", module.Options{OptBank: bankOnSeven}, module.Options{OptBank: bankRound}},
	} {
		if a, b := step(get(tc.a), tc.id), step(get(tc.b), tc.id); a.TextKey == b.TextKey {
			t.Errorf("%s: the step should differ between the two settings, and both say %q", tc.name, a.TextKey)
		}
	}

	// A fixed bank is the third spelling of a step with three, and the one a
	// two-way check would miss.
	if got := step(get(module.Options{OptBank: bankFixed}), "bankMoves").TextKey; got != "guide.sietemedio.bankFixed" {
		t.Errorf("a fixed bank is taught as %q", got)
	}

	if got := step(get(module.Options{OptMinStake: 25}), "stake").Params["n"]; got != 25 {
		t.Errorf("stake n = %v, want 25", got)
	}
	if got := step(get(module.Options{OptRounds: 24}), "end").Params["n"]; got != 24 {
		t.Errorf("end n = %v, want 24", got)
	}
}
