package canasta

import (
	"testing"

	"zolik/server/internal/module"
)

// Melds belong to a partnership, so the board cannot say which partner laid a
// card on one. The strip has to.
func TestALayOffSaysWhichPartnerMadeIt(t *testing.T) {
	raw := fourHanded(func(s *GameState) {
		s.Current = "p3"
		s.Teams[0].HasMelded = true
		s.Teams[0].Melds = []Meld{{
			ID: meldID(0, "K"), TeamID: 0, Kind: meldSet, Rank: "K",
			Cards: []string{"KH", "KD", "KS"},
		}}
		s.Hands["p3"] = []string{"KC", "8C", "9C"}
	})
	m := New()
	next, events, err := m.Apply(raw, "p3", module.Action{
		Verb: VerbLayOff, Target: meldID(0, "K"), Cards: []string{"KC"},
	})
	if err != nil {
		t.Fatalf("lay off: %v", err)
	}
	var got []module.Move
	for _, ev := range events {
		if mv, ok := m.NarrateEvent(next, ev); ok {
			got = append(got, mv)
		}
	}
	if len(got) != 1 {
		t.Fatalf("want one move, got %+v", got)
	}
	mv := got[0]
	if mv.PlayerID != "p3" || mv.Fact.Params["player"] != "p3" || mv.Fact.LabelKey != "canasta.move.laidOff" {
		t.Errorf("lay-off narrated as %+v", mv)
	}
	if mv.GroupID != meldID(0, "K") {
		t.Errorf("the move should point at the meld it touched, got %q", mv.GroupID)
	}
}

// A draw from the stock never names what was drawn.
func TestADrawNamesNoCard(t *testing.T) {
	raw := twoHanded(nil)
	m := New()
	next, events, err := m.Apply(raw, "p1", module.Action{Verb: VerbDraw})
	if err != nil {
		t.Fatalf("draw: %v", err)
	}
	for _, ev := range events {
		mv, ok := m.NarrateEvent(next, ev)
		if !ok || ev.Type != "cards_drawn" {
			continue
		}
		if mv.Fact.LabelKey != "canasta.move.drewStock" {
			t.Errorf("a draw narrated as %s", mv.Fact.LabelKey)
		}
		for k := range mv.Fact.Params {
			if k != "player" {
				t.Errorf("a draw carries %s: %v", k, mv.Fact.Params)
			}
		}
		return
	}
	t.Fatal("the draw was not narrated")
}
