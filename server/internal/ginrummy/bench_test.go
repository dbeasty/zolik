package ginrummy

import (
	"testing"

	"zolik/server/internal/module"
)

// BenchmarkDeadwood is the claim in meld.go's package doc held to account: an
// exhaustive search over a ten- or eleven-card hand's candidate melds is
// microseconds, not an algorithm anybody should apologize for.
func BenchmarkDeadwood(b *testing.B) {
	// A pathological hand: two full runs' worth of overlapping candidates in
	// one suit, plus a set, so candidateMelds has as much to enumerate as an
	// eleven-card hand can offer.
	hand := []string{
		"AH", "2H", "3H", "4H", "5H", "6H", "7H",
		"8D", "8C", "8S",
		"KD",
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		Deadwood(hand)
	}
}

// BenchmarkLegalActions covers the phase that does the most probing: the
// discard phase, where every distinct card in an eleven-card hand is tried as
// a candidate knock.
func BenchmarkLegalActions(b *testing.B) {
	m := New()
	state, err := m.NewMatch(module.MatchConfig{}, players(), 1)
	if err != nil {
		b.Fatalf("NewMatch: %v", err)
	}
	s, err := decode(state)
	if err != nil {
		b.Fatalf("decode: %v", err)
	}
	// Force a discard-phase position with a full eleven-card hand, the most
	// expensive shape LegalActions ever has to answer for.
	nonDealer := other(s.Players, s.Dealer)
	s.Phase = phaseDiscard
	s.Current = nonDealer
	if len(s.Hands[nonDealer]) < 11 && len(s.Stock) > 0 {
		s.Hands[nonDealer] = append(s.Hands[nonDealer], s.Stock[len(s.Stock)-1])
		s.Stock = s.Stock[:len(s.Stock)-1]
	}
	raw, err := encode(s)
	if err != nil {
		b.Fatalf("encode: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := m.LegalActions(raw, nonDealer); err != nil {
			b.Fatalf("LegalActions: %v", err)
		}
	}
}
