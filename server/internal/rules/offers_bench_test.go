package rules

import "testing"

// LegalActions runs once per viewer on every broadcast, so its cost is
// multiplied by (players × actions per game). These benchmarks exist to keep
// that honest — the design deliberately trades a handful of state clones for
// the guarantee that offers cannot drift from the engine, and this is where
// that trade stays visible.
//
// The shape that matters: the active player pays for per-card eligibility,
// everyone else pays only the coarse probes. BenchmarkLegalActions_Observer
// should be substantially cheaper than _ActivePlayer; if it ever is not, the
// "only compute card lists for whoever is on turn" optimisation has been
// lost.

func benchState() GameState {
	s := GameState{
		Status:        StatusActive,
		Rules:         ProfileZolikClassic,
		GameNumber:    2,
		Round:         2,
		Phase:         PhaseMeld,
		CurrentTurn:   "p1",
		TurnOrder:     []string{"p1", "p2", "p3", "p4"},
		DealStarterID: "p1",
		DrawPile:      make([]string, 40),
		DiscardPile:   []string{"2C", "3C", "4C", "5C", "6C"},
		Hands: map[string][]string{
			// A full 13-card zolik_classic hand, deliberately rich in cards
			// that fit the melds below so the eligibility scan does real work.
			"p1": {"4H", "9H", "TH", "JH", "QS", "KS", "AS", "2D", "3D", "4D", "7C", "8C", "JOKER1"},
			"p2": make([]string, 13),
			"p3": make([]string, 13),
			"p4": make([]string, 13),
		},
		Melds:       map[string][][]string{},
		MeldMeta:    map[string][]MeldInfo{},
		RoundReqMet: map[string]bool{"p1": true, "p2": true, "p3": true},
		GameScores:  map[string][]int{},
		TotalScores: map[string]int{},
	}
	// Eight melds on the table — a busy late-deal board.
	board := []struct {
		owner string
		cards []string
		typ   MeldType
	}{
		{"p1", []string{"5H", "6H", "7H", "8H"}, MeldRun},
		{"p1", []string{"QH", "QD", "QC"}, MeldSet},
		{"p2", []string{"2S", "3S", "4S", "5S"}, MeldRun},
		{"p2", []string{"9D", "9C", "9S"}, MeldSet},
		{"p3", []string{"TC", "JC", "QC2"}, MeldSet},
		{"p3", []string{"5D", "6D", "7D"}, MeldRun},
		{"p4", []string{"KH", "KD", "KC"}, MeldSet},
		{"p4", []string{"2H", "3H", "JOKER2"}, MeldRun},
	}
	for i, b := range board {
		s.Melds[b.owner] = append(s.Melds[b.owner], b.cards)
		s.MeldMeta[b.owner] = append(s.MeldMeta[b.owner], MeldInfo{
			MeldID: meldIDForBench(i), Type: b.typ, OwnerID: b.owner,
		})
	}
	s.NextMeldSeq = len(board)
	return s
}

func meldIDForBench(i int) string {
	return "meld_" + string(rune('1'+i))
}

func BenchmarkLegalActions_ActivePlayer(b *testing.B) {
	s := benchState()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = LegalActions(s, "p1")
	}
}

func BenchmarkLegalActions_Observer(b *testing.B) {
	s := benchState()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = LegalActions(s, "p2")
	}
}

// TestLegalActions_ObserverIsCheaperThanActivePlayer pins the optimisation
// as a test rather than leaving it to a benchmark nobody runs: an observer
// must not trigger the per-card eligibility scans.
//
// Measured in allocations, not wall time — allocation counts are
// deterministic, so this cannot flake on a loaded CI box the way a timing
// comparison would, and it runs instantly instead of burning a second per
// benchmark.
func TestLegalActions_ObserverIsCheaperThanActivePlayer(t *testing.T) {
	s := benchState()
	active := testing.AllocsPerRun(20, func() { _ = LegalActions(s, "p1") })
	observer := testing.AllocsPerRun(20, func() { _ = LegalActions(s, "p2") })

	if observer >= active {
		t.Errorf("observer (%.0f allocs) is not cheaper than the active player (%.0f allocs) — "+
			"the per-card scans are no longer gated on whose turn it is", observer, active)
	}
	// A busy 4-player, 8-meld board is the realistic worst case. This bound
	// is a tripwire for an accidental O(hand × melds) *state clone* — the
	// design keeps clones to coarse gating only — not a micro-optimisation
	// target; raise it deliberately if the offer set genuinely grows.
	if active > 6000 {
		t.Errorf("active player costs %.0f allocs on a busy board; expected well under 6000 — "+
			"has a per-card path started cloning state?", active)
	}
}
