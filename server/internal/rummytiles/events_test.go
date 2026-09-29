package rummytiles

import (
	"strings"
	"testing"

	"zolik/server/internal/module"
)

// Every move on a set points at it, and none names a tile: the client would
// print a tile code as it stands.
func TestSetMovesPointAtTheSetAndNameNoTile(t *testing.T) {
	m := New()
	raw := withState(t, func(s *GameState) {
		s.Hands["p1"] = []string{"10-R", "11-R", "12-R", "13-R", "9-K"}
	})
	steps := []struct {
		a     module.Action
		key   string
		group string
	}{
		{moduleAction(VerbPlace, "", []string{"10-R", "11-R", "12-R"}, nil), "rummytiles.move.placed", "s1"},
		{moduleAction(VerbAdd, "s1", []string{"13-R"}, nil), "rummytiles.move.added", "s1"},
		{moduleAction(VerbCommit, "", nil, nil), "rummytiles.move.committed", ""},
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
		for _, v := range got.Fact.Params {
			if s, _ := v.(string); strings.Contains(s, "-") {
				t.Errorf("%s names a tile: %v", step.a.Verb, got.Fact.Params)
			}
		}
	}
}
