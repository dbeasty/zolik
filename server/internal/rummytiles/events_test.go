package rummytiles

import (
	"reflect"
	"testing"

	"zolik/server/internal/module"
)

// Every move on a set points at it and names the tiles it moved, which are on
// the table for everyone to see.
func TestSetMovesPointAtTheSetAndNameTheirTiles(t *testing.T) {
	m := New()
	raw := withState(t, func(s *GameState) {
		s.Hands["p1"] = []string{"10-R", "11-R", "12-R", "13-R", "9-K"}
	})
	steps := []struct {
		a     module.Action
		key   string
		group string
		cards []string
	}{
		{moduleAction(VerbPlace, "", []string{"10-R", "11-R", "12-R"}, nil), "rummytiles.move.placed", "s1", []string{"10-R", "11-R", "12-R"}},
		{moduleAction(VerbAdd, "s1", []string{"13-R"}, nil), "rummytiles.move.added", "s1", []string{"13-R"}},
		{moduleAction(VerbCommit, "", nil, nil), "rummytiles.move.committed", "", nil},
	}
	for _, step := range steps {
		next, events, err := m.Apply(raw, "p1", step.a)
		if err != nil {
			t.Fatalf("%s: %v", step.a.Verb, err)
		}
		raw = next
		var got *module.Move
		for _, ev := range events {
			if mv, ok := m.NarrateEvent(next, ev); ok && got == nil {
				got = &mv
			}
		}
		if got == nil {
			t.Fatalf("%s was not narrated", step.a.Verb)
		}
		if got.Fact.LabelKey != step.key || got.GroupID != step.group || got.PlayerID != "p1" {
			t.Errorf("%s narrated as %+v", step.a.Verb, *got)
		}
		if step.cards != nil && !reflect.DeepEqual(got.Fact.Params["cards"], step.cards) {
			t.Errorf("%s names %v, want %v", step.a.Verb, got.Fact.Params["cards"], step.cards)
		}
	}
}

// A joker swap names the tile that went in; the joker it freed is in the tray.
func TestAJokerSwapNamesTheTileThatWentIn(t *testing.T) {
	m := New()
	raw := withState(t, func(s *GameState) {
		s.InitialMeld["p1"] = true
		s.Sets = []Set{{ID: "s0", Kind: "group", Cards: []string{"7-R", "7-B", "JOKER1"}}}
		s.Workspace = &Workspace{Sets: cloneSets(s.Sets)}
		s.Hands["p1"] = []string{"7-O"}
	})
	next, events, err := m.Apply(raw, "p1", moduleAction(VerbSwapJoker, "s0", []string{"7-O"}, nil))
	if err != nil {
		t.Fatalf("swap: %v", err)
	}
	for _, ev := range events {
		if mv, ok := m.NarrateEvent(next, ev); ok {
			if mv.Fact.LabelKey != "rummytiles.move.swappedJoker" || mv.Fact.Params["card"] != "7-O" || mv.GroupID != "s0" {
				t.Errorf("joker swap narrated as %+v", mv)
			}
			return
		}
	}
	t.Fatal("the swap was not narrated")
}
