package rummytiles

import (
	"testing"

	"zolik/server/internal/module"
)

func TestView_NeverShowsAnOpponentsHand(t *testing.T) {
	raw := newMatch(t, 21, "p1", "p2")
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
				t.Errorf("%s can see %s's tiles: %v", viewer, z.OwnerID, z.Cards)
			}
			if z.Count != len(s.Hands[z.OwnerID]) {
				t.Errorf("%s sees %s holding %d tiles, actually %d", viewer, z.OwnerID, z.Count, len(s.Hands[z.OwnerID]))
			}
		}
	}
}

func TestView_TheWorkspaceIsPublicToEveryPlayer(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.InitialMeld["p1"] = true
		s.Sets = []Set{{ID: "s0", Kind: "group", Cards: []string{"7-R", "7-B", "7-O"}}}
		s.Workspace = &Workspace{Sets: cloneSets(s.Sets)}
		s.Hands["p1"] = []string{"7-K"}
	})
	raw, err := apply(t, raw, "p1", moduleAction(VerbAdd, "s0", []string{"7-K"}, nil))
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	m := New()
	for _, viewer := range []string{"p1", "p2"} {
		vm, err := m.View(raw, viewer)
		if err != nil {
			t.Fatalf("View(%s): %v", viewer, err)
		}
		var table *module.Zone
		for i := range vm.Zones {
			if vm.Zones[i].ID == tableZoneID {
				table = &vm.Zones[i]
			}
		}
		if table == nil || len(table.Groups) != 1 || len(table.Groups[0].Cards) != 4 {
			t.Fatalf("viewer %s should see the in-progress four-tile set, got %+v", viewer, table)
		}
	}
}

func TestView_RoundTripDoesNotChangeWhatIsShown(t *testing.T) {
	raw := newMatch(t, 9, "p1", "p2")
	m := New()
	s := stateOf(t, raw)
	before, err := m.View(raw, s.Players[0])
	if err != nil {
		t.Fatalf("View: %v", err)
	}
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
