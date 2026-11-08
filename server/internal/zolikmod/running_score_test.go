package zolikmod

import (
	"testing"

	"zolik/server/internal/module"
	"zolik/server/internal/rules"
)

// A hand's penalty is the deal's running score, and a sum over hidden cards:
// its owner sees it in the header, nobody else sees it anywhere. It used to
// ride on the scoreboard, which is the same for every viewer, so each player
// could read how heavy everyone else's hand was — a joker moves it by 50.
func TestOnlyTheOwnerSeesTheirHandPenalty(t *testing.T) {
	gs := rules.GameState{
		Status:      rules.StatusActive,
		GameNumber:  1,
		TurnOrder:   []string{"p1", "p2"},
		CurrentTurn: "p1",
		Hands:       map[string][]string{"p1": {"4C", "KH"}, "p2": {"5C"}},
	}
	raw, err := encode(&matchState{Rules: gs, Players: refs("p1", "p2")})
	if err != nil {
		t.Fatal(err)
	}
	yourHand := func(vm module.ViewModel) (int, bool) {
		for _, f := range vm.Header {
			if f.LabelKey == "zolik.header.yourHand" {
				return f.Params["n"].(int), true
			}
		}
		return 0, false
	}
	for _, viewer := range []string{"p1", "p2"} {
		vm, err := New().View(raw, viewer)
		if err != nil {
			t.Fatal(err)
		}
		n, ok := yourHand(vm)
		if want := handPenalty(gs, viewer); !ok || n != want {
			t.Errorf("%s: header says %d (%v), want their own %d", viewer, n, ok, want)
		}
	}
	if handPenalty(gs, "p1") == handPenalty(gs, "p2") {
		t.Fatal("the hands must differ for this test to tell them apart")
	}
	spectator, _ := New().View(raw, "")
	if _, ok := yourHand(spectator); ok {
		t.Errorf("an onlooker is told a hand's penalty: %+v", spectator.Header)
	}
	for _, s := range module.StandingsFor(New(), raw) {
		for _, f := range s.Facts {
			if f.LabelKey == "zolik.standing.inHand" {
				t.Errorf("the scoreboard prices %s's hidden hand: %+v", s.PlayerID, f)
			}
		}
	}
}
