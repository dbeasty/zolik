package blackjack

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

func TestLessonsFollowTheTable(t *testing.T) {
	on := module.Options{OptSurrender: module.OptOn, OptInsurance: module.OptOn, OptMaxSplits: 3, OptBlackjackPays: pays6to5}
	off := module.Options{OptSurrender: module.OptOff, OptInsurance: module.OptOff, OptMaxSplits: 0, OptBlackjackPays: paysEven}
	a := lessonSteps(t, module.MatchConfig{Variation: "vegas", Options: on})
	b := lessonSteps(t, module.MatchConfig{Variation: "vegas", Options: off})
	for _, id := range []string{"surrender", "insurance", "split", "pays6to5"} {
		if _, ok := a[id]; !ok {
			t.Errorf("step %s missing when the rule is on", id)
		}
		if _, ok := b[id]; ok {
			t.Errorf("step %s present when the rule is off", id)
		}
	}
	for _, id := range []string{"noSurrender", "noInsurance", "noSplit", "paysEven"} {
		if _, ok := b[id]; !ok {
			t.Errorf("step %s missing when the rule is off", id)
		}
	}
	if got := a["split"].Params["n"]; got != 3 {
		t.Errorf("split n = %v, want 3", got)
	}
	if a["freeplay"].Takes(VerbSurrender) == b["freeplay"].Takes(VerbSurrender) {
		t.Error("free play should include surrender only when it is allowed")
	}
	h := lessonSteps(t, module.MatchConfig{Variation: "vegas", Options: module.Options{OptDealerHitsSoft17: module.OptOn}})
	s := lessonSteps(t, module.MatchConfig{Variation: "vegas", Options: module.Options{OptDealerHitsSoft17: module.OptOff}})
	if _, ok := h["dealerHits"]; !ok {
		t.Error("soft 17 hit step missing")
	}
	if _, ok := s["dealerStands"]; !ok {
		t.Error("soft 17 stand step missing")
	}
}
