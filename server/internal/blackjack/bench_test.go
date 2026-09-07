package blackjack

import (
	"encoding/json"
	"strconv"
	"testing"

	"zolik/server/internal/module"
)

// BenchmarkLegalActions is the guard on this module's one real cost.
//
// Every offer is answered by dry-running the real engine, and a dry run
// decodes and re-encodes the whole state — which here means the shoe. That is
// what the deck count measures: the same five offers, from the same crafted
// position, against 52 cards and against 416, where the entire difference is
// serialisation and none of it is blackjack. Even at eight decks it sits
// inside the range the other modules already occupy.
func BenchmarkLegalActions(b *testing.B) {
	for _, decks := range []int{1, 6, 8} {
		b.Run("decks="+strconv.Itoa(decks), func(b *testing.B) { benchLegalActions(b, decks) })
	}
}

func benchLegalActions(b *testing.B, decks int) {
	m := New()
	s := tableOf(module.MatchConfig{Variation: "vegas", Options: module.Options{OptDecks: decks}})
	s.Status = "active"
	s.Phase = phasePlay
	s.RoundNumber = 1
	s.Peeked = true
	s.Current, s.CurrentHand = 0, 0
	s.Dealer = []string{"6S", "TD"}
	s.Shoe = buildShoe(decks, 1)
	for _, id := range []string{"p1", "p2", "p3"} {
		s.Seats = append(s.Seats, Seat{
			PlayerID: id, Stack: 490, Bet: 10, Staked: 10,
			Hands: []Hand{{ID: handID(id, 0), Cards: []string{"8H", "8D"}, Bet: 10}},
		})
	}
	raw, err := json.Marshal(s)
	if err != nil {
		b.Fatalf("marshal: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := m.LegalActions(raw, "p1"); err != nil {
			b.Fatalf("LegalActions: %v", err)
		}
	}
}
