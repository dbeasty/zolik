package prsi

import (
	"testing"

	"zolik/server/internal/module"
)

func narrateMove(t *testing.T, raw module.State, pid string, a module.Action) []module.Fact {
	t.Helper()
	m := New()
	out, events, err := m.Apply(raw, pid, a)
	if err != nil {
		t.Fatalf("%s %v: %v", pid, a, err)
	}
	var lines []module.Fact
	for _, ev := range events {
		if mv, ok := module.NarrateEvent(m, out, ev); ok {
			lines = append(lines, mv.Fact)
		}
	}
	return lines
}

// Each special card's line says what it now asks, and of whom.
func TestEachPlayIsOneLineThatSaysWhatItAsks(t *testing.T) {
	seven := withState(t, func(s *GameState) { s.DiscardPile = []string{"9C"}; s.Hands["p1"] = []string{"7C", "KS"} })
	if l := narrateMove(t, seven, "p1", module.Action{Verb: VerbPlay, Cards: []string{"7C"}}); len(l) != 1 ||
		l[0].LabelKey != "prsi.move.playedSeven" || l[0].Params["target"] != "p2" || l[0].Params["n"] != 2 {
		t.Errorf("seven: %+v", l)
	}
	ace := withState(t, func(s *GameState) { s.DiscardPile = []string{"9C"}; s.Hands["p1"] = []string{"AC", "KS"} })
	if l := narrateMove(t, ace, "p1", module.Action{Verb: VerbPlay, Cards: []string{"AC"}}); len(l) != 1 || l[0].LabelKey != "prsi.move.playedAce" {
		t.Errorf("ace: %+v", l)
	}
	queen := withState(t, func(s *GameState) { s.Hands["p1"] = []string{"QH", "KS"} })
	if l := narrateMove(t, queen, "p1", module.Action{Verb: VerbPlay, Cards: []string{"QH"}, Params: map[string]string{"suit": "C"}}); len(l) != 1 ||
		l[0].LabelKey != "prsi.move.playedQueen" || l[0].Params["suit"] != "suit.german.C" {
		t.Errorf("queen: %+v", l)
	}
	taken := withState(t, func(s *GameState) { s.PendingDraw = 4; s.DiscardPile = []string{"7S"} })
	if l := narrateMove(t, taken, "p1", module.Action{Verb: VerbDraw}); len(l) != 1 || l[0].LabelKey != "prsi.move.tookMany" || l[0].Params["n"] != 3 {
		// The draw pile in withState holds three cards, so taking four gets three.
		t.Errorf("take: %+v", l)
	}
}
