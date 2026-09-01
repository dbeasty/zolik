package rummytiles

import "testing"

// TestNoDeadEnds is the guarantee §3.3 of the plan calls structural rather
// than predictive: over a corpus of mid-turn workspaces at 2, 3 and 4 seats,
// the player on turn always has an enabled offer, and reset_turn always
// returns the table to exactly what it was before the turn began.
func TestNoDeadEnds(t *testing.T) {
	m := New()
	for _, seats := range []int{2, 3, 4} {
		t.Run(seatsLabel(seats), func(t *testing.T) {
			for _, state := range collectStates(t, seats, 4) {
				s, err := decode(state)
				if err != nil {
					t.Fatalf("decode: %v", err)
				}
				if s.Status != "active" || s.Intermission.Open || s.Workspace == nil {
					continue
				}

				offers, err := m.LegalActions(state, s.Current)
				if err != nil {
					t.Fatalf("LegalActions: %v", err)
				}
				anyEnabled := false
				var resetOffer *struct {
					enabled bool
					id      string
				}
				for _, o := range offers {
					if o.Enabled {
						anyEnabled = true
					}
					if o.ID == OfferResetTurn {
						resetOffer = &struct {
							enabled bool
							id      string
						}{o.Enabled, o.ID}
					}
				}
				if !anyEnabled {
					t.Fatalf("player on turn has no enabled offer at all: %+v", offers)
				}
				if resetOffer == nil || !resetOffer.enabled {
					t.Fatalf("reset_turn is not enabled mid-turn, which is what keeps every other move safe to try")
				}

				// reset_turn must return the table to exactly the committed
				// state — not to whatever the workspace happened to be.
				before := cloneSets(s.Sets)
				beforeHandLen := len(s.Hands[s.Current])
				next, _, err := m.Apply(state, s.Current, moduleAction(VerbResetTurn, "", nil, nil))
				if err != nil {
					t.Fatalf("reset_turn: %v", err)
				}
				after := stateOf(t, next)
				if len(after.Workspace.Sets) != len(before) {
					t.Fatalf("reset_turn left %d sets, want %d (the committed table)", len(after.Workspace.Sets), len(before))
				}
				for i, set := range before {
					if !sameTiles(set.Cards, after.Workspace.Sets[i].Cards) {
						t.Errorf("set %d after reset = %v, want %v", i, after.Workspace.Sets[i].Cards, set.Cards)
					}
				}
				if len(after.Hands[s.Current]) < beforeHandLen {
					t.Errorf("reset_turn should never leave the hand smaller than it started, got %d < %d",
						len(after.Hands[s.Current]), beforeHandLen)
				}
				if len(after.Workspace.Tray) != 0 {
					t.Errorf("reset_turn should leave an empty tray, got %v", after.Workspace.Tray)
				}
			}
		})
	}
}

func seatsLabel(n int) string {
	switch n {
	case 2:
		return "2-seats"
	case 3:
		return "3-seats"
	default:
		return "4-seats"
	}
}

func sameTiles(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	count := map[string]int{}
	for _, t := range a {
		count[t]++
	}
	for _, t := range b {
		count[t]--
	}
	for _, n := range count {
		if n != 0 {
			return false
		}
	}
	return true
}
