package holdem

import (
	"testing"

	"zolik/server/internal/module"
)

// A cash table takes a new player mid-hand: folded out of the hand in play,
// with no cards, and dealt in at the next one with the starting stack.
func TestACashTableSeatsSomebodyAtTheNextHand(t *testing.T) {
	raw := cashMatch(t, people("p1", "p2")...)
	raw, _, err := New().SeatLate(raw, module.PlayerRef{ID: "p3", Name: "p3"})
	if err != nil {
		t.Fatalf("SeatLate: %v", err)
	}
	s := mustDecode(t, raw)
	p3 := s.seat("p3")
	if p3 == nil || !p3.Joining || !p3.Folded || len(p3.Hole) != 0 || p3.Stack != s.StartingStack {
		t.Fatalf("new seat = %+v, want joining, folded, no cards, the starting stack", p3)
	}
	for _, id := range order(s) {
		if id == "p3" {
			t.Error("a seat joining at the next hand is waited on at this showdown")
		}
	}
	// Fold the hand out; the next one deals p3 in.
	for i := 0; i < 6 && s.HandNumber == 1; i++ {
		if s.Break.Open {
			for _, id := range order(s) {
				raw, _ = apply(t, raw, id, module.Action{Verb: module.VerbContinue})
			}
		} else {
			raw, _ = apply(t, raw, s.Seats[s.Current].PlayerID, module.Action{Verb: VerbFold})
		}
		s = mustDecode(t, raw)
	}
	p3 = s.seat("p3")
	if s.HandNumber != 2 || p3.Joining || len(p3.Hole) != 2 {
		t.Fatalf("hand %d, p3 = %+v: want p3 dealt into hand 2", s.HandNumber, p3)
	}
}

// A tournament takes nobody once it is under way, and a full table nobody.
func TestNoLateSeatAtATournamentOrAFullTable(t *testing.T) {
	raw, _ := New().NewMatch(module.MatchConfig{}, people("p1", "p2"), 7)
	if _, _, err := New().SeatLate(raw, module.PlayerRef{ID: "p3"}); module.CodeOf(err) != ErrLateJoinTournament {
		t.Errorf("tournament: %v, want %s", err, ErrLateJoinTournament)
	}
	ids := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i"}
	full := cashMatch(t, people(ids...)...)
	if _, _, err := New().SeatLate(full, module.PlayerRef{ID: "j"}); module.CodeOf(err) != ErrTableFull {
		t.Errorf("full: %v, want %s", err, ErrTableFull)
	}
}
