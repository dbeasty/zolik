package ginrummy

import (
	"testing"

	"zolik/server/internal/lessontest"
	"zolik/server/internal/module"
)

func TestLessonsWalk(t *testing.T) { lessontest.Check(t, New()) }

func lessonSteps(t *testing.T, cfg module.MatchConfig) map[string]module.LessonStep {
	t.Helper()
	ls, err := New().Lessons(cfg)
	if err != nil || len(ls) == 0 {
		t.Fatalf("Lessons: %v", err)
	}
	out := map[string]module.LessonStep{}
	for _, s := range ls[0].Steps {
		out[s.ID] = s
	}
	return out
}

func TestLessonsFollowTheRules(t *testing.T) {
	std := lessonSteps(t, module.MatchConfig{Variation: "standard"})
	okl := lessonSteps(t, module.MatchConfig{Variation: "oklahoma"})
	if std["knock"].TextKey == okl["knock"].TextKey {
		t.Fatal("standard and Oklahoma should teach different knocks")
	}
	if std["knock"].Params["n"] != 10 {
		t.Fatalf("knock limit param = %v", std["knock"].Params["n"])
	}
	if _, ok := std["bigGin"]; ok {
		t.Fatal("big gin is off by default")
	}
	on := lessonSteps(t, module.MatchConfig{Variation: "standard", Options: module.Options{OptBigGin: module.OptOn}})
	if _, ok := on["bigGin"]; !ok {
		t.Fatal("big gin on should add its step")
	}
	off := lessonSteps(t, module.MatchConfig{Variation: "standard", Options: module.Options{OptLineBonuses: module.OptOff}})
	if _, ok := off["lineBonuses"]; ok {
		t.Fatal("line bonuses off should drop its step")
	}
	t150 := lessonSteps(t, module.MatchConfig{Variation: "standard", Options: module.Options{OptTargetScore: 150}})
	if t150["target"].Params["n"] != 150 {
		t.Fatalf("target param = %v", t150["target"].Params["n"])
	}
}
