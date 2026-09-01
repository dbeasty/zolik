package ginrummy

import (
	"testing"

	"zolik/server/internal/module"
)

func TestView_NeverShowsTheOpponentsHandBeforeAKnock(t *testing.T) {
	raw := newMatch(t, 21, module.MatchConfig{})
	s := stateOf(t, raw)
	m := New()

	for _, viewer := range s.Players {
		vm, err := m.View(raw, viewer)
		if err != nil {
			t.Fatalf("View: %v", err)
		}
		for _, z := range vm.Zones {
			if z.Kind != module.ZoneHand || z.OwnerID == viewer {
				continue
			}
			if len(z.Cards) != 0 {
				t.Errorf("%s can see %s's cards: %v", viewer, z.OwnerID, z.Cards)
			}
			if z.Count != len(s.Hands[z.OwnerID]) {
				t.Errorf("%s sees %s holding %d cards, actually %d", viewer, z.OwnerID, z.Count, len(s.Hands[z.OwnerID]))
			}
		}
	}
}

func TestView_AKnockerHandIsPublicToBothPlayers(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.Phase = phaseLayoff
		s.Current = "p2"
		s.Knocker = "p1"
		s.KnockerDeadwood = 3
		s.KnockerMelds = []Meld{{ID: "m0", Kind: "set", Cards: []string{"5C", "5D", "5H"}}}
		s.Hands["p1"] = []string{"5C", "5D", "5H", "AH", "2D"}
		s.Hands["p2"] = []string{"9C", "TD", "JC"}
	})
	m := New()

	for _, viewer := range []string{"p1", "p2"} {
		vm, err := m.View(raw, viewer)
		if err != nil {
			t.Fatalf("View(%s): %v", viewer, err)
		}
		var knockerZone *module.Zone
		for i := range vm.Zones {
			if vm.Zones[i].ID == handZoneID("p1") {
				knockerZone = &vm.Zones[i]
			}
		}
		if knockerZone == nil || len(knockerZone.Cards) != 5 {
			t.Fatalf("viewer %s should see the knocker's full hand, got %+v", viewer, knockerZone)
		}
	}

	// The defender's hand must still be hidden from the knocker.
	vm, _ := m.View(raw, "p1")
	for _, z := range vm.Zones {
		if z.ID == handZoneID("p2") && len(z.Cards) != 0 {
			t.Errorf("the defender's hand leaked to the knocker: %v", z.Cards)
		}
	}
}

func TestView_MeldsZoneAppearsOnlyAfterAKnock(t *testing.T) {
	raw := newMatch(t, 5, module.MatchConfig{})
	m := New()
	s := stateOf(t, raw)
	vm, err := m.View(raw, s.Players[0])
	if err != nil {
		t.Fatalf("View: %v", err)
	}
	for _, z := range vm.Zones {
		if z.ID == meldsZoneID {
			t.Fatal("no melds should be visible before a knock has happened")
		}
	}
}

func TestView_RoundTripDoesNotChangeWhatIsShown(t *testing.T) {
	raw := newMatch(t, 9, module.MatchConfig{})
	m := New()
	s := stateOf(t, raw)
	before, err := m.View(raw, s.Players[0])
	if err != nil {
		t.Fatalf("View: %v", err)
	}

	// Round-trip through JSON, exactly as module.State travels.
	roundTripped, err := encode(stateOf(t, raw))
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	after, err := m.View(roundTripped, s.Players[0])
	if err != nil {
		t.Fatalf("View after round-trip: %v", err)
	}
	if len(before.Zones) != len(after.Zones) {
		t.Errorf("zone count changed across a round trip: %d vs %d", len(before.Zones), len(after.Zones))
	}
}
