package rummytiles

import (
	"testing"

	"zolik/server/internal/module"
)

// BenchmarkLegalActions covers the most expensive shape LegalActions answers:
// a mid-turn workspace with several sets on the table, each probed for
// add/take/split/swap_joker eligibility.
func BenchmarkLegalActions(b *testing.B) {
	m := New()
	raw := module.State(nil)
	{
		s := &GameState{
			Status:      "active",
			Players:     []string{"p1", "p2"},
			Current:     "p1",
			Hands:       map[string][]string{"p1": {"1-R", "2-R", "3-B", "4-K", "5-O"}, "p2": {}},
			Pool:        []string{"9-R"},
			InitialMeld: map[string]bool{"p1": true, "p2": true},
			NextSetID:   4,
			Scores:      map[string]int{"p1": 0, "p2": 0},
			TargetScore: 200,
			Sets: []Set{
				{ID: "s0", Kind: "group", Cards: []string{"7-R", "7-B", "7-O"}},
				{ID: "s1", Kind: "run", Cards: []string{"3-R", "4-R", "5-R", "6-R"}},
				{ID: "s2", Kind: "group", Cards: []string{"9-R", "9-B", "9-O", "9-K"}},
			},
		}
		s.Workspace = &Workspace{Sets: cloneSets(s.Sets)}
		var err error
		raw, err = encode(s)
		if err != nil {
			b.Fatalf("encode: %v", err)
		}
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := m.LegalActions(raw, "p1"); err != nil {
			b.Fatalf("LegalActions: %v", err)
		}
	}
}
