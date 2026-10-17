package holdem

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
	base := lessonSteps(t, module.MatchConfig{Variation: VarHoldem})
	if _, ok := base["usetwo"]; ok {
		t.Error("Hold'em teaches the Omaha two-card rule")
	}
	if _, ok := base["swap"]; ok {
		t.Error("Hold'em teaches the draw")
	}
	omaha := lessonSteps(t, module.MatchConfig{Variation: VarOmaha})
	if _, ok := omaha["usetwo"]; !ok {
		t.Error("Omaha lesson lacks the two-card rule")
	}
	if _, ok := omaha["allin"]; ok {
		t.Error("pot-limit table teaches no-limit all in")
	}
	draw := lessonSteps(t, module.MatchConfig{Variation: VarDraw})
	if draw["draw"].Params["n"] != drawRules.maxDiscard {
		t.Errorf("draw step params = %v", draw["draw"].Params)
	}
	blinds := lessonSteps(t, module.MatchConfig{Variation: VarHoldem, Options: module.Options{OptBigBlind: 50, OptStartingStack: 200}})
	if blinds["setup"].Params["bb"] != 50 || blinds["setup"].Params["sb"] != 25 || blinds["setup"].Params["n"] != 200 {
		t.Errorf("setup params = %v", blinds["setup"].Params)
	}
	limited := lessonSteps(t, module.MatchConfig{Variation: VarHoldem, Options: module.Options{OptHandLimit: 5}})
	if limited["end"].Params["n"] != 5 || limited["end"].TextKey == base["end"].TextKey {
		t.Errorf("end step = %+v", limited["end"])
	}
	cash := lessonSteps(t, module.MatchConfig{Variation: VarHoldem, Options: module.Options{OptFormat: FormatCash}})
	if cash["table"].TextKey == base["table"].TextKey {
		t.Error("cash and tournament share the table step")
	}
	winners := lessonSteps(t, module.MatchConfig{Variation: VarHoldem, Options: module.Options{OptShowdownReveal: RevealWinners}})
	if winners["reveal"].TextKey == base["reveal"].TextKey {
		t.Error("reveal setting does not change the lesson")
	}
}
