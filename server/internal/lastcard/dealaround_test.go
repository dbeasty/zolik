package lastcard

import (
	"testing"

	"zolik/server/internal/module"
)

func threeHanded(t *testing.T) module.State {
	t.Helper()
	raw, err := New().NewMatch(module.MatchConfig{}, []module.PlayerRef{{ID: "a"}, {ID: "b"}, {ID: "c"}}, 5)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func mustState(t *testing.T, raw module.State) *GameState {
	t.Helper()
	s, err := decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// A player put down to sit out keeps the deal in play — a bot finishes it —
// and is dealt out of the next: no cards, never on turn, and still on the
// table's standings with the points they had.
func TestASatOutPlayerIsDealtAroundFromTheNextDeal(t *testing.T) {
	raw := threeHanded(t)
	raw, err := New().DealAround(raw, "b", true)
	if err != nil {
		t.Fatal(err)
	}
	s := mustState(t, raw)
	if len(s.Hands["b"]) == 0 || s.dealtOut("b") {
		t.Fatal("the deal in play was taken away from b")
	}
	s.deal()
	if !s.dealtOut("b") || len(s.Hands["b"]) != 0 {
		t.Fatalf("next deal: dealtOut %v, b holds %d cards; want b dealt out", s.DealtOut, len(s.Hands["b"]))
	}
	for i := 0; i < 10; i++ {
		if s.Current == "b" || s.nextPlayer(s.Current) == "b" {
			t.Fatal("the turn reaches a player who was dealt out")
		}
		s.Current = s.nextPlayer(s.Current)
	}
	standings, _ := New().Standings(mustEncode(t, s))
	if len(standings) != 3 {
		t.Errorf("standings = %d seats, want all three", len(standings))
	}

	raw2, _ := New().DealAround(mustEncode(t, s), "b", false)
	s = mustState(t, raw2)
	s.deal()
	if s.dealtOut("b") || len(s.Hands["b"]) == 0 {
		t.Fatal("b is not dealt back in")
	}
}

// Never fewer than two at a deal: with everybody else away, everybody plays.
func TestDealingAroundNeverLeavesOneAtTheTable(t *testing.T) {
	raw := threeHanded(t)
	raw, _ = New().DealAround(raw, "b", true)
	raw, _ = New().DealAround(raw, "c", true)
	s := mustState(t, raw)
	s.deal()
	if len(s.DealtOut) != 0 {
		t.Fatalf("dealt out %v, leaving one player", s.DealtOut)
	}
}

func mustEncode(t *testing.T, s *GameState) module.State {
	t.Helper()
	raw, err := encode(s)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
