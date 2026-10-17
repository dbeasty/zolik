package snaps

import (
	"testing"

	"zolik/server/internal/lessontest"
	"zolik/server/internal/module"
)

func TestLessonsWalk(t *testing.T) { lessontest.Check(t, New()) }

func stepByID(t *testing.T, cfg module.MatchConfig, id string) (module.LessonStep, bool) {
	t.Helper()
	ls, err := New().Lessons(cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range ls[0].Steps {
		if s.ID == id {
			return s, true
		}
	}
	return module.LessonStep{}, false
}

func TestLessonsFollowVariation(t *testing.T) {
	snaps := module.MatchConfig{Variation: variationSnaps}
	s66 := module.MatchConfig{Variation: variation66}
	if _, ok := stepByID(t, snaps, "exchange"); !ok {
		t.Error("snaps lesson should teach the spodek exchange")
	}
	if _, ok := stepByID(t, snaps, "exchange66"); ok {
		t.Error("snaps lesson should not teach the 66 exchange")
	}
	if _, ok := stepByID(t, s66, "exchange66"); !ok {
		t.Error("66 lesson should teach the nine exchange")
	}
	if _, ok := stepByID(t, s66, "marriageOpenOnly"); !ok {
		t.Error("66 lesson should teach open-only marriages")
	}
	if _, ok := stepByID(t, snaps, "marriageOpenOnly"); ok {
		t.Error("snaps lesson has no open-only marriage step")
	}
	if _, ok := stepByID(t, snaps, "lastTrick"); !ok {
		t.Error("snaps lesson should teach last-trick-wins")
	}
	if _, ok := stepByID(t, s66, "lastTrick66"); !ok {
		t.Error("66 lesson should teach the last-trick bonus")
	}
	a, _ := stepByID(t, snaps, "setup")
	b, _ := stepByID(t, s66, "setup")
	if a.Params["n"] == b.Params["n"] {
		t.Errorf("hand size should differ: %v vs %v", a.Params, b.Params)
	}
}

func TestLessonsFollowOptions(t *testing.T) {
	a, _ := stepByID(t, module.MatchConfig{Options: module.Options{OptTarget: 5}}, "match")
	b, _ := stepByID(t, module.MatchConfig{Options: module.Options{OptTarget: 9}}, "match")
	if a.Params["n"] == b.Params["n"] {
		t.Errorf("target should flow into the lesson: %v vs %v", a.Params, b.Params)
	}
	if _, ok := stepByID(t, module.MatchConfig{Options: module.Options{OptShowCardPoints: module.OptOn}}, "showPoints"); !ok {
		t.Error("card-point option should add its step")
	}
	if _, ok := stepByID(t, module.MatchConfig{Options: module.Options{OptShowCardPoints: module.OptOff}}, "showPoints"); ok {
		t.Error("no card-point step when the option is off")
	}
}
