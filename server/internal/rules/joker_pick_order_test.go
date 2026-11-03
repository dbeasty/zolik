package rules

import (
	"slices"
	"testing"
)

// Where a joker goes in a run that could hold it at either end. A player who
// laid 5-6-7 and a joker picked the joker's place when they picked the cards:
// after the seven it is the eight, before the five it is the four. Once laid,
// that place is the joker's for good — the meld growing, the joker being
// bought back, and what it counts towards the floor all read it from there.

// pickOrderState is "me" in the meld phase of deal one, holding hand, with
// karel down and table on the table as karel's run m1.
func pickOrderState(hand []string, table []string) GameState {
	st := GameState{
		Status:      StatusActive,
		Rules:       continentalNoFloor(),
		GameNumber:  1,
		Round:       1,
		Phase:       PhaseMeld,
		CurrentTurn: "me",
		TurnOrder:   []string{"me", "karel"},
		Hands:       map[string][]string{"me": hand, "karel": {"2D", "2H"}},
		Melds:       map[string][][]string{},
		MeldMeta:    map[string][]MeldInfo{},
		RoundReqMet: map[string]bool{"me": true, "karel": true},
		GameScores:  map[string][]int{},
		TotalScores: map[string]int{},
	}
	if table != nil {
		st.Melds["karel"] = [][]string{table}
		st.MeldMeta["karel"] = []MeldInfo{{MeldID: "m1", Type: MeldRun, OwnerID: "karel", WildCount: 1}}
	}
	return st
}

func layInOrder(t *testing.T, cards []string, pickOrder bool) []string {
	t.Helper()
	st := pickOrderState(append(append([]string(nil), cards...), "KD", "KH"), nil)
	out, err := ApplyAction(st, "me", Action{Type: ActionLayMeld, Cards: cards, PickOrder: pickOrder})
	if err != nil {
		t.Fatalf("lay %v: %v", cards, err)
	}
	return out.State.Melds["me"][0]
}

func TestLayMeldPutsTheJokerWherePickOrderSays(t *testing.T) {
	for _, tc := range []struct {
		name   string
		picked []string
		want   []string
	}{
		{"picked last, upwards: the eight", []string{"5S", "6S", "7S", "JOKER1"}, []string{"5S", "6S", "7S", "JOKER1"}},
		{"picked first, upwards: the four", []string{"JOKER1", "5S", "6S", "7S"}, []string{"JOKER1", "5S", "6S", "7S"}},
		{"picked last, downwards: the four", []string{"7S", "6S", "5S", "JOKER1"}, []string{"JOKER1", "5S", "6S", "7S"}},
		{"picked first, downwards: the eight", []string{"JOKER1", "7S", "6S", "5S"}, []string{"5S", "6S", "7S", "JOKER1"}},
		{"naturals out of order, joker last: the eight", []string{"5S", "7S", "6S", "JOKER1"}, []string{"5S", "6S", "7S", "JOKER1"}},
		{"a gap leaves it no choice", []string{"JOKER1", "5S", "7S", "8S"}, []string{"5S", "JOKER1", "7S", "8S"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := layInOrder(t, tc.picked, true); !slices.Equal(got, tc.want) {
				t.Fatalf("laid %v as %v, want %v", tc.picked, got, tc.want)
			}
		})
	}
}

// Without a pick order the cards' order means nothing and the joker takes the
// slot worth more, as it always has — which is what bots still get.
func TestLayMeldWithoutPickOrderStillPrefersValue(t *testing.T) {
	got := layInOrder(t, []string{"JOKER1", "5S", "6S", "7S"}, false)
	if want := []string{"5S", "6S", "7S", "JOKER1"}; !slices.Equal(got, want) {
		t.Fatalf("laid as %v, want %v", got, want)
	}
}

func TestLaidJokerKeepsItsSlotWhenTheRunGrows(t *testing.T) {
	st := pickOrderState([]string{"8S", "KD", "KH"}, []string{"JOKER1", "5S", "6S", "7S"})
	ns, err := ValidateLayOff(st, "me", "m1", []string{"8S"}, "")
	if err != nil {
		t.Fatalf("lay off 8S: %v", err)
	}
	if got, want := ns.Melds["karel"][0], []string{"JOKER1", "5S", "6S", "7S", "8S"}; !slices.Equal(got, want) {
		t.Fatalf("run is %v, want %v — the joker moved off the four", got, want)
	}

	// A joker dropped onto the front of a run still goes to the front.
	st = pickOrderState([]string{"JOKER2", "KD", "KH"}, []string{"5S", "6S", "7S", "8S"})
	st.MeldMeta["karel"][0].WildCount = 0
	ns, err = ValidateLayOff(st, "me", "m1", []string{"JOKER2"}, "front")
	if err != nil {
		t.Fatalf("lay off a joker at the front: %v", err)
	}
	if got, want := ns.Melds["karel"][0], []string{"JOKER2", "5S", "6S", "7S", "8S"}; !slices.Equal(got, want) {
		t.Fatalf("run is %v, want %v", got, want)
	}
}

func TestOnlyTheJokersOwnCardBuysItBack(t *testing.T) {
	table := []string{"JOKER1", "5S", "6S", "7S"} // the joker is the four
	if _, err := ValidateSwapJoker(pickOrderState([]string{"8S", "KD"}, table), "me", "m1", "8S"); err == nil {
		t.Fatal("8S bought back a joker laid as the four")
	}
	ns, err := ValidateSwapJoker(pickOrderState([]string{"4S", "KD"}, table), "me", "m1", "4S")
	if err != nil {
		t.Fatalf("4S: %v", err)
	}
	if got, want := ns.Melds["karel"][0], []string{"4S", "5S", "6S", "7S"}; !slices.Equal(got, want) {
		t.Fatalf("run is %v, want %v", got, want)
	}

	// The lay-off that reclaims a joker on the way asks the same question:
	// 8S extends the run instead of taking the four's place.
	ns, err = ValidateLayOff(pickOrderState([]string{"8S", "KD", "KH"}, table), "me", "m1", []string{"8S"}, "")
	if err != nil {
		t.Fatalf("lay off 8S: %v", err)
	}
	if got := ns.Hands["me"]; slices.Contains(got, "JOKER1") {
		t.Fatalf("8S reclaimed the four's joker: hand %v", got)
	}
}

func TestLaidJokerCountsAsItsOwnSlot(t *testing.T) {
	st := pickOrderState(nil, nil)
	st.Melds["me"] = [][]string{{"JOKER1", "5S", "6S", "7S"}}
	low := PlayerInitialMeldNaturalValue(st, "me")
	st.Melds["me"] = [][]string{{"5S", "6S", "7S", "JOKER1"}}
	high := PlayerInitialMeldNaturalValue(st, "me")
	if high-low != 4 {
		t.Fatalf("joker as the four scored %d, as the eight %d; want them 4 apart", low, high)
	}
}
