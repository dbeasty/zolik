package snaps

import (
	"strconv"
	"testing"

	"zolik/server/internal/module"
)

// At a table dealt with the defaults, every seat shows its card points while
// the hand is still being played — the running score — and not only once it
// has been settled. Counting the other side's points used to be left to
// memory unless a table switched the option on; it is on by default now.
func TestTheDefaultTableShowsEverySeatsPointsMidHand(t *testing.T) {
	var players []module.PlayerRef
	for i := 1; i <= 2; i++ {
		id := "p" + strconv.Itoa(i)
		players = append(players, module.PlayerRef{ID: id, Name: id})
	}
	m := New()
	raw, err := m.NewMatch(module.MatchConfig{}, players, 3)
	if err != nil {
		t.Fatal(err)
	}
	for step := 0; step < 400; step++ {
		s, err := decode(raw)
		if err != nil {
			t.Fatal(err)
		}
		if midHand(s) {
			vm, err := m.View(raw, players[0].ID)
			if err != nil {
				t.Fatal(err)
			}
			for _, seat := range vm.Seats {
				if sitsOut(s, seat.PlayerID) {
					continue
				}
				found := false
				for _, f := range seat.Facts {
					found = found || f.LabelKey == "snaps.seat.points"
				}
				if !found {
					t.Errorf("%s shows no points mid-hand: %+v", seat.PlayerID, seat.Facts)
				}
			}
			return
		}
		moved := false
		for _, p := range players {
			offers, _ := m.LegalActions(raw, p.ID)
			if a, ok := module.ChooseAction(offers, nil); ok {
				if next, _, err := m.Apply(raw, p.ID, a); err == nil {
					raw, moved = next, true
					break
				}
			}
		}
		if !moved {
			break
		}
	}
	t.Fatal("never reached the play with a trick taken")
}

func midHand(s *GameState) bool {
	return s.Status == "active" && !s.Intermission.Open && len(s.History) > 0 && len(s.Trick) == 0
}

func sitsOut(*GameState, string) bool { return false }
