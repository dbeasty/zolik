package marias

import (
	"math/rand"
	"testing"

	"zolik/server/internal/tricks"
)

// A sampled deal gives every hand its size, never a card anyone has seen,
// and honours what the table showed: voids, can't-beats and promises.
func TestSamplesAreConsistentWithWhatTheTableShowed(t *testing.T) {
	s := playState(t, map[string][]string{
		"p1": {"KH", "9H", "8C", "9C"},
		"p2": {"7D", "8D", "9D", "TD"},
		"p3": {"AC", "TC", "KC", "QC"},
	})
	s.TrumpCard, s.Talon = "9H", []string{"7S", "8S"}
	// Completed: p1 led the král of spades; p2 did not follow and did not
	// trump (void in spades and hearts); p3 followed low without beating,
	// so holds neither the ace nor the ten of spades, which would have.
	s.History = [][]tricks.Play{{{Seat: 0, Card: "KS"}, {Seat: 1, Card: "JD"}, {Seat: 2, Card: "9S"}}}
	// p3 announced a marriage in clubs by playing the svršek earlier.
	s.Announced = []Marriage{{Player: "p3", Suit: "C"}}

	k := s.infer()
	for _, c := range []string{"TS", "AS", "QS", "7H", "AH"} {
		if !k.forbidden["p2"][c] {
			t.Errorf("p2 showed void in spades and hearts, yet may hold %s", c)
		}
	}
	for _, c := range []string{"AS", "TS"} {
		if !k.forbidden["p3"][c] {
			t.Errorf("p3 followed under the král without beating it, yet may hold %s", c)
		}
	}
	r := rand.New(rand.NewSource(1))
	for i := 0; i < 200; i++ {
		deal, ok := s.sample("p1", k, r)
		if !ok {
			t.Fatal("no consistent deal found")
		}
		for _, p := range []string{"p2", "p3"} {
			if len(deal[p]) != len(s.Hands[p]) {
				t.Fatalf("%s dealt %d, holds %d", p, len(deal[p]), len(s.Hands[p]))
			}
			for _, c := range deal[p] {
				if k.forbidden[p][c] {
					t.Fatalf("%s dealt %s, which the table ruled out", p, c)
				}
				if c == "KS" || c == "KH" || c == "7S" {
					t.Fatalf("%s dealt %s, a card p1 has seen", p, c)
				}
			}
		}
		if !hasCard(deal["p3"], "KC") {
			t.Fatal("p3's announced marriage promises the král of clubs")
		}
	}
}
