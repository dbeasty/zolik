package blackjack

import (
	"testing"

	"zolik/server/internal/module"
)

func zoneOf(t *testing.T, vm module.ViewModel, id string) *module.Zone {
	t.Helper()
	for i := range vm.Zones {
		if vm.Zones[i].ID == id {
			return &vm.Zones[i]
		}
	}
	t.Fatalf("no zone %q in %+v", id, vm.Zones)
	return nil
}

// TestView_TheHoleCardIsTheOnlyThingHidden, and it is hidden from everybody —
// including the seat whose turn it is, which is the whole reason the game is
// interesting.
func TestView_TheHoleCardIsTheOnlyThingHidden(t *testing.T) {
	raw := withTable(t, func(s *GameState) {
		s.HoleShown = false
		s.Dealer = []string{"6S", "TD"}
	})
	m := New()

	for _, viewer := range []string{"p1", "p2"} {
		vm, err := m.View(raw, viewer)
		if err != nil {
			t.Fatalf("View(%s): %v", viewer, err)
		}
		dealer := zoneOf(t, vm, dealerZoneID)
		if len(dealer.Cards) != 1 || dealer.Cards[0].Card != "6S" {
			t.Errorf("%s sees the dealer holding %+v, want the up card alone", viewer, dealer.Cards)
		}
		if dealer.Count != 2 {
			t.Errorf("%s sees %d dealer cards counted, want 2", viewer, dealer.Count)
		}
		// Every player card is face up in this game, so a box shows its cards
		// to the whole table.
		box := zoneOf(t, vm, boxZoneID("p2"))
		if len(box.Groups) != 1 || len(box.Groups[0].Cards) != 2 {
			t.Errorf("%s cannot see p2's face-up hand: %+v", viewer, box.Groups)
		}
	}
}

func TestView_TheHoleCardIsShownOnceItIsTurned(t *testing.T) {
	raw := withTable(t, func(s *GameState) {
		s.Seats[1].Hands[0].Done = true
		s.Dealer = []string{"9S", "8D"}
	})
	next, err := apply(t, raw, "p1", VerbStand)
	if err != nil {
		t.Fatalf("stand: %v", err)
	}
	vm, err := New().View(next, "p2")
	if err != nil {
		t.Fatalf("View: %v", err)
	}
	if got := len(zoneOf(t, vm, dealerZoneID).Cards); got < 2 {
		t.Errorf("after the dealer plays, the table sees %d of its cards", got)
	}
}

// TestView_ASplitBoxIsTwoGroupsWithTheHandInPlayMarked — a client draws the
// boxes from this, and "which of my two hands am I being asked about" is a
// fact only the module has.
func TestView_ASplitBoxIsTwoGroupsWithTheHandInPlayMarked(t *testing.T) {
	raw := withTable(t, func(s *GameState) {
		s.Seats[0].Hands[0].Cards = []string{"8H", "8D"}
		s.Shoe = stack("3C", "9S")
	})
	next, err := apply(t, raw, "p1", VerbSplit)
	if err != nil {
		t.Fatalf("split: %v", err)
	}
	vm, err := New().View(next, "p1")
	if err != nil {
		t.Fatalf("View: %v", err)
	}
	box := zoneOf(t, vm, boxZoneID("p1"))
	if len(box.Groups) != 2 {
		t.Fatalf("a split box drew %d groups, want 2", len(box.Groups))
	}
	marked := 0
	for _, g := range box.Groups {
		for _, b := range g.BadgeKeys {
			if b == "blackjack.badge.inPlay" {
				marked++
			}
		}
	}
	if marked != 1 {
		t.Errorf("%d of the two hands is marked as the one in play, want exactly 1", marked)
	}
}

// TestView_AHandsTotalIsPushedRatherThanLeftToBeAddedUp — an ace is eleven
// only while it fits, which is a rule. A client working it out would be
// re-implementing one.
func TestView_AHandsTotalIsPushedRatherThanLeftToBeAddedUp(t *testing.T) {
	raw := withTable(t, func(s *GameState) {
		s.Seats[0].Hands[0].Cards = []string{"AH", "6D"}
	})
	vm, err := New().View(raw, "p1")
	if err != nil {
		t.Fatalf("View: %v", err)
	}
	seat := vm.SeatOf("p1")
	if seat == nil {
		t.Fatal("no seat for p1")
	}
	found := false
	for _, f := range seat.Facts {
		if f.LabelKey == "blackjack.seat.softTotal" && f.Value == "17" {
			found = true
		}
	}
	if !found {
		t.Errorf("a soft seventeen was not reported as one: %+v", seat.Facts)
	}
}

// TestView_EveryPhaseWaitsOnTheSeatsItIsAskingSomethingOf — betting and
// insurance ask the whole table at once, play asks one seat.
func TestView_EveryPhaseWaitsOnTheSeatsItIsAskingSomethingOf(t *testing.T) {
	betting := stateOf(t, newMatch(t, 5, module.MatchConfig{}))
	if !awaiting(betting, 0) || !awaiting(betting, 1) {
		t.Error("betting should wait on every seat that can stake")
	}

	playing := stateOf(t, withTable(t, func(s *GameState) {}))
	if !awaiting(playing, 0) || awaiting(playing, 1) {
		t.Error("play should wait on the seat whose hand it is, and nobody else")
	}
}
