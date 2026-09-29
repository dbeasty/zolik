package ginrummy

import (
	"testing"

	"zolik/server/internal/module"
)

// narrate is every move Apply's events narrate, for the actor's own table.
func narrate(t *testing.T, raw module.State, playerID string, a module.Action) []module.Move {
	t.Helper()
	m := New()
	next, events, err := m.Apply(raw, playerID, a)
	if err != nil {
		t.Fatalf("%s: %v", a.Verb, err)
	}
	var out []module.Move
	for _, ev := range events {
		if mv, ok := m.NarrateEvent(next, ev); ok {
			out = append(out, mv)
		}
	}
	return out
}

// A card taken from the discard pile was face up, so the strip names it. One
// drawn from the stock was not, and is not named.
func TestADrawNamesTheCardOnlyWhenItWasFaceUp(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.Phase = phaseDraw
		s.Hands["p1"] = []string{"AC"}
		s.Interest = map[string][]string{}
	})
	got := narrate(t, raw, "p1", module.Action{OfferID: OfferDrawDiscard, Verb: VerbDraw})
	if len(got) != 1 || got[0].Fact.LabelKey != "ginrummy.move.tookDiscard" || got[0].Fact.Params["card"] != "9S" {
		t.Errorf("a draw from the discard pile narrated as %+v", got)
	}

	got = narrate(t, raw, "p1", module.Action{OfferID: OfferDrawStock, Verb: VerbDraw})
	if len(got) != 1 || got[0].Fact.LabelKey != "ginrummy.move.drewStock" {
		t.Fatalf("a draw from the stock narrated as %+v", got)
	}
	if _, named := got[0].Fact.Params["card"]; named {
		t.Errorf("a stock draw names its card: %v", got[0].Fact.Params)
	}
}

// Laying off is the one move that changes somebody else's meld.
func TestALayOffNamesTheKnockerAndPointsAtTheMeld(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.Phase = phaseLayoff
		s.Current = "p2"
		s.Knocker = "p1"
		s.KnockerMelds = []Meld{{ID: "m0", Kind: "set", Cards: []string{"5C", "5D", "5H"}}}
		s.Hands["p2"] = []string{"5S", "9C"}
	})
	got := narrate(t, raw, "p2", module.Action{Verb: VerbLayOff, Target: "m0", Cards: []string{"5S"}})
	if len(got) != 1 {
		t.Fatalf("want one move, got %+v", got)
	}
	mv := got[0]
	if mv.PlayerID != "p2" || mv.GroupID != "m0" || mv.Fact.LabelKey != "ginrummy.move.laidOff" ||
		mv.Fact.Params["owner"] != "p1" || mv.Fact.Params["card"] != "5S" {
		t.Errorf("lay-off narrated as %+v", mv)
	}
}
