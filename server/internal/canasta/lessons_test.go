package canasta

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
		t.Fatalf("Lessons(%v): %v", cfg, err)
	}
	out := map[string]module.LessonStep{}
	for _, s := range ls[0].Steps {
		out[s.ID] = s
	}
	return out
}

func TestLessonFollowsTheVariation(t *testing.T) {
	classic := lessonSteps(t, module.MatchConfig{Variation: "classic"})
	modern := lessonSteps(t, module.MatchConfig{Variation: "modern_american"})
	samba := lessonSteps(t, module.MatchConfig{Variation: "samba"})

	if classic["pile"].TextKey == modern["pile"].TextKey || classic["pile"].TextKey == samba["pile"].TextKey {
		t.Errorf("the pile step should differ per variation")
	}
	if classic["blackThrees"].TextKey == modern["blackThrees"].TextKey {
		t.Errorf("Modern American never melds black threes")
	}
	if classic["goOut"].TextKey == modern["goOut"].TextKey {
		t.Errorf("one canasta versus two should change the go-out step")
	}
	if classic["setup"].Params["n"] == modern["setup"].Params["n"] {
		t.Errorf("hand size should differ: %v", classic["setup"].Params)
	}
	if _, ok := classic["sequences"]; ok {
		t.Errorf("Classic has no sequences")
	}
	if _, ok := samba["sequences"]; !ok {
		t.Errorf("Samba teaches sequences")
	}
	if samba["draw"].TextKey == classic["draw"].TextKey || samba["draw"].Params["n"] != 2 {
		t.Errorf("Samba draws two: %v", samba["draw"])
	}
	if samba["goOutBonus"].TextKey == classic["goOutBonus"].TextKey {
		t.Errorf("Samba has no concealed bonus")
	}
}

func TestLessonFollowsTheOptions(t *testing.T) {
	base := lessonSteps(t, module.MatchConfig{Variation: "samba"})
	for _, id := range []string{"topOnly", "wildMeld", "rearrange", "poach"} {
		if _, ok := base[id]; ok {
			t.Errorf("plain Samba should not teach %s", id)
		}
	}
	x := lessonSteps(t, module.MatchConfig{Variation: "canastax"})
	for _, id := range []string{"topOnly", "wildMeld", "rearrange", "poach"} {
		if _, ok := x[id]; !ok {
			t.Errorf("CanastaX should teach %s", id)
		}
	}
	if x["sequences"].TextKey == base["sequences"].TextKey {
		t.Errorf("dirty sequences should change the sequences step")
	}
	hand := lessonSteps(t, module.MatchConfig{Variation: "classic", Options: module.Options{OptHandSize: 15}})
	if hand["setup"].Params["n"] != 15 {
		t.Errorf("hand size option not passed: %v", hand["setup"].Params)
	}
	target := lessonSteps(t, module.MatchConfig{Variation: "classic", Options: module.Options{OptTargetScore: 1000}})
	if target["goal"].Params["n"] != 1000 {
		t.Errorf("target option not passed: %v", target["goal"].Params)
	}
	two := lessonSteps(t, module.MatchConfig{Variation: "classic", Options: module.Options{OptCanastasToGoOut: 2}})
	if two["goOut"].TextKey == lessonSteps(t, module.MatchConfig{Variation: "classic"})["goOut"].TextKey {
		t.Errorf("canastas-to-go-out option should change the go-out step")
	}
}
