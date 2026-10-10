package blackjack

import (
	"testing"

	"zolik/server/internal/module"
)

// While stakes are being taken a newcomer can put one up at once; once the
// cards are out they are in from the next round.
func TestBlackjackSeatsALatePlayer(t *testing.T) {
	raw := newMatch(t, 11, module.MatchConfig{})
	raw, _, err := New().SeatLate(raw, module.PlayerRef{ID: "p3", Name: "p3"})
	if err != nil {
		t.Fatalf("SeatLate while betting: %v", err)
	}
	if p3 := seatOf(t, stateOf(t, raw), "p3"); p3.Joining || p3.Out {
		t.Fatalf("p3 = %+v, want in this round", p3)
	}

	raw = newMatch(t, 12, module.MatchConfig{Options: module.Options{module.OptPauseBetweenRounds: module.OptOff}})
	raw, _ = bet(t, raw, "p1", 50)
	raw, _ = bet(t, raw, "p2", 50)
	raw, _, err = New().SeatLate(raw, module.PlayerRef{ID: "p3", Name: "p3"})
	if err != nil {
		t.Fatalf("SeatLate mid-round: %v", err)
	}
	if p3 := seatOf(t, stateOf(t, raw), "p3"); !p3.Joining || !p3.Out {
		t.Fatalf("p3 = %+v, want joining at the next round", p3)
	}
	for i := 0; i < 20; i++ {
		s := stateOf(t, raw)
		if s.Phase == phaseBets || s.Status != "active" {
			break
		}
		for _, id := range []string{"p1", "p2"} {
			if next, err := apply(t, raw, id, VerbStand); err == nil {
				raw = next
			}
			if next, err := apply(t, raw, id, VerbDecline); err == nil {
				raw = next
			}
		}
	}
	if p3 := seatOf(t, stateOf(t, raw), "p3"); p3.Joining || p3.Out {
		t.Fatalf("after the round p3 = %+v, want in", p3)
	}
}
