package klondike

import (
	"testing"

	"zolik/server/internal/lessontest"
	"zolik/server/internal/module"
)

func TestLessonsWalk(t *testing.T) { lessontest.Check(t, New()) }

// The lesson follows the table: the stock, redeal, scoring and house-rule
// steps change with the options.
func TestLessonsFollowOptions(t *testing.T) {
	textOf := func(cfg module.MatchConfig, id string) (string, map[string]any) {
		ls, err := New().Lessons(cfg)
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range ls[0].Steps {
			if s.ID == id {
				return s.TextKey, s.Params
			}
		}
		t.Fatalf("no step %s", id)
		return "", nil
	}
	if k, _ := textOf(module.MatchConfig{Variation: "classic"}, "stock"); k != "guide.klondike.stock1" {
		t.Errorf("classic stock: %s", k)
	}
	if k, p := textOf(module.MatchConfig{Variation: "draw3"}, "stock"); k != "guide.klondike.stock3" || p["n"] != 3 {
		t.Errorf("draw3 stock: %s %v", k, p)
	}
	vegas := module.MatchConfig{Variation: "vegas"}
	if k, _ := textOf(vegas, "scoring"); k != "guide.klondike.scoreVegas" {
		t.Errorf("vegas scoring: %s", k)
	}
	if k, _ := textOf(vegas, "redeals"); k != "guide.klondike.redealsNone" {
		t.Errorf("vegas redeals: %s", k)
	}
	two := module.MatchConfig{Variation: "classic", Options: module.Options{OptRedeals: 2}}
	if k, p := textOf(two, "redeals"); k != "guide.klondike.redealsLimited" || p["n"] != 2 {
		t.Errorf("limited redeals: %s %v", k, p)
	}
	off := module.MatchConfig{Variation: "classic", Options: module.Options{OptTakeBack: module.OptOff, OptAutoFinish: module.OptOff}}
	if k, _ := textOf(off, "takeback"); k != "guide.klondike.noTakeBack" {
		t.Errorf("takeback: %s", k)
	}
	if k, _ := textOf(off, "finish"); k != "guide.klondike.noAutoFinish" {
		t.Errorf("finish: %s", k)
	}
}
