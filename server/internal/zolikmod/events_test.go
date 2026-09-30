package zolikmod

import (
	"testing"

	"zolik/server/internal/module"
	"zolik/server/internal/rules"
)

// The strip over the table has to say who added a card to whose meld. The
// board cannot: one player laying off onto another's run looks the same as
// the owner extending it.
func TestALayOffOntoSomebodyElsesMeldNamesBoth(t *testing.T) {
	m := &Module{}
	gs := rules.GameState{
		Status:      rules.StatusActive,
		Rules:       resolveConfig(module.MatchConfig{}),
		GameNumber:  1,
		Round:       2,
		Phase:       rules.PhaseDraw,
		CurrentTurn: "p1",
		TurnOrder:   []string{"p1", "p2"},
		Hands:       map[string][]string{"p1": {"9C", "KS", "2H"}, "p2": {"2D", "3H"}},
		Melds:       map[string][][]string{"p1": {{"KD", "KH", "KC"}}, "p2": {{"5C", "6C", "7C", "8C"}}},
		MeldMeta: map[string][]rules.MeldInfo{
			"p1": {{MeldID: "m0", Type: rules.MeldSet, OwnerID: "p1"}},
			"p2": {{MeldID: "m1", Type: rules.MeldRun, OwnerID: "p2"}},
		},
		RoundReqMet: map[string]bool{"p1": true, "p2": true},
		DrawPile:    []string{"3S", "4D"},
		DiscardPile: []string{"9D"},
		GameScores:  map[string][]int{},
		TotalScores: map[string]int{},
		NextMeldSeq: 2,
	}
	st, err := encode(&matchState{Rules: gs, Players: []module.PlayerRef{{ID: "p1", Name: "A"}, {ID: "p2", Name: "B"}}})
	if err != nil {
		t.Fatal(err)
	}

	st, events, err := m.Apply(st, "p1", module.Action{Verb: "draw", OfferID: "draw:deck"})
	if err != nil {
		t.Fatalf("draw: %v", err)
	}
	// The opponent learns that a card was drawn, never which.
	for _, ev := range events {
		seen, ok := m.ProjectEvent(ev, "p2")
		if !ok {
			continue
		}
		if mv, ok := m.NarrateEvent(st, seen); ok {
			if mv.Fact.LabelKey != "zolik.move.drewStock" {
				t.Errorf("a blind draw narrated as %s", mv.Fact.LabelKey)
			}
			if _, named := mv.Fact.Params["card"]; named {
				t.Errorf("a blind draw names its card to the opponent: %v", mv.Fact.Params)
			}
		}
	}

	st, events, err = m.Apply(st, "p1", module.Action{
		Verb: "lay_off", OfferID: "lay_off:m1", Target: "m1", Cards: []string{"9C"},
	})
	if err != nil {
		t.Fatalf("lay off: %v", err)
	}
	var got []module.Move
	for _, ev := range events {
		if mv, ok := m.NarrateEvent(st, ev); ok {
			got = append(got, mv)
		}
	}
	if len(got) != 1 {
		t.Fatalf("want one narrated move for the lay-off, got %+v", got)
	}
	mv := got[0]
	if mv.Fact.LabelKey != "zolik.move.laidOff" || mv.Fact.Params["owner"] != "p2" || mv.PlayerID != "p1" {
		t.Errorf("lay-off onto p2's run narrated as %+v", mv)
	}
	if mv.GroupID != "m1" {
		t.Errorf("the move should point at the meld it touched, got %q", mv.GroupID)
	}
}

// The discard that closes a deal lies face down in the classic profile, but
// the strip still names it, and says it went out: a bare "discarded" would
// read like any other turn while the table has in fact stopped.
func TestTheClosingDiscardIsNarratedAsGoingOut(t *testing.T) {
	m := &Module{}
	gs := rules.GameState{
		Status:      rules.StatusActive,
		Rules:       resolveConfig(module.MatchConfig{}),
		GameNumber:  1,
		Round:       3,
		Phase:       rules.PhaseDiscard,
		CurrentTurn: "p1",
		TurnOrder:   []string{"p1", "p2"},
		Hands:       map[string][]string{"p1": {"9C", "4S"}, "p2": {"2D", "3H"}},
		Melds:       map[string][][]string{"p1": {{"KD", "KH", "KC"}}, "p2": {}},
		MeldMeta: map[string][]rules.MeldInfo{
			"p1": {{MeldID: "m0", Type: rules.MeldSet, OwnerID: "p1"}},
		},
		RoundReqMet: map[string]bool{"p1": true, "p2": true},
		DrawPile:    []string{"3S", "4D", "8H"},
		DiscardPile: []string{"9D"},
		GameScores:  map[string][]int{},
		TotalScores: map[string]int{},
		NextMeldSeq: 1,
	}
	if !gs.Rules.GoOutDiscardFaceDown {
		t.Fatalf("the default profile should lay the closing discard face down")
	}
	st, err := encode(&matchState{Rules: gs, Players: []module.PlayerRef{{ID: "p1", Name: "A"}, {ID: "p2", Name: "B"}}})
	if err != nil {
		t.Fatal(err)
	}

	narrate := func(st module.State, events []module.Event, viewer string) []module.Move {
		var got []module.Move
		for _, ev := range events {
			seen, ok := m.ProjectEvent(ev, viewer)
			if !ok {
				continue
			}
			if mv, ok := m.NarrateEvent(st, seen); ok {
				got = append(got, mv)
			}
		}
		return got
	}

	// An ordinary discard is just a discard.
	st, events, err := m.Apply(st, "p1", module.Action{Verb: string(rules.VerbDiscard), Cards: []string{"4S"}})
	if err != nil {
		t.Fatalf("discard: %v", err)
	}
	if got := narrate(st, events, "p2"); len(got) != 1 || got[0].Fact.LabelKey != "zolik.move.discarded" {
		t.Fatalf("a mid-deal discard narrated as %+v", got)
	}

	// Back to p1 with one card left: draw, lay nothing, discard, out.
	s, err := decode(st)
	if err != nil {
		t.Fatal(err)
	}
	s.Rules.CurrentTurn = "p1"
	s.Rules.Phase = rules.PhaseDiscard
	if st, err = encode(s); err != nil {
		t.Fatal(err)
	}
	st, events, err = m.Apply(st, "p1", module.Action{Verb: string(rules.VerbDiscard), Cards: []string{"9C"}})
	if err != nil {
		t.Fatalf("closing discard: %v", err)
	}
	for _, viewer := range []string{"p1", "p2"} {
		got := narrate(st, events, viewer)
		if len(got) != 1 {
			t.Fatalf("%s: want one narrated move for the closing discard, got %+v", viewer, got)
		}
		f := got[0].Fact
		if f.LabelKey != "zolik.move.wentOut" || f.Params["player"] != "p1" || f.Params["card"] != "9C" {
			t.Errorf("%s: the closing discard narrated as %+v", viewer, f)
		}
	}
}
