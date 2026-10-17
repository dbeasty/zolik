package marias

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
		t.Fatalf("lessons: %v", err)
	}
	out := map[string]module.LessonStep{}
	for _, s := range ls[0].Steps {
		out[s.ID] = s
	}
	return out
}

func TestLessonsFollowTheRules(t *testing.T) {
	v := lessonSteps(t, module.MatchConfig{Variation: variationVoleny})
	l := lessonSteps(t, module.MatchConfig{Variation: variationLicit})
	if _, ok := v["auction"]; ok {
		t.Error("volený has no auction")
	}
	if _, ok := l["auction"]; !ok {
		t.Error("licitovaný lesson needs the auction")
	}
	if _, ok := l["takeover"]; ok {
		t.Error("licitovaný has no take-over")
	}

	opt := func(name string, val int) map[string]module.LessonStep {
		return lessonSteps(t, module.MatchConfig{Variation: variationVoleny, Options: module.Options{name: val}})
	}
	if _, ok := opt(OptRedDoubles, module.OptOff)["red"]; ok {
		t.Error("hearts-double step should be gone when off")
	}
	if _, ok := opt(OptRedDoubles, module.OptOn)["red"]; !ok {
		t.Error("hearts-double step missing when on")
	}
	if _, ok := opt(OptOpenBetlDurch, module.OptOn)["openHand"]; !ok {
		t.Error("open-hand step missing when on")
	}
	if _, ok := opt(OptOpenBetlDurch, module.OptOff)["openHand"]; ok {
		t.Error("open-hand step should be gone when off")
	}
	if opt(OptZLidu, module.OptOn)["trump"].TextKey == opt(OptZLidu, module.OptOff)["trump"].TextKey {
		t.Error("z lidu should change the trump step")
	}
	if got := opt(OptFlekLimit, 2)["flek"].Params["n"]; got != 2 {
		t.Errorf("flek limit param = %v", got)
	}
	if opt(OptTariff, TariffCSM)["tariff"].Params["betl"] == opt(OptTariff, TariffPub)["tariff"].Params["betl"] {
		t.Error("stakes should change the tariff numbers")
	}
	if opt(OptDeals, 12)["goal"].Params["n"] != 12 {
		t.Error("deals should reach the goal step")
	}
}
