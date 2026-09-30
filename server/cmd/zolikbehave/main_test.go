package main

import (
	"testing"

	"zolik/server/internal/learn"
)

// One whole four-seat match: every deal is counted once per seat, exactly one
// seat goes out of each, and every seat that did not is caught.
func TestWatcherCountsEveryDealOnce(t *testing.T) {
	g, err := learn.Lookup("zolik")
	if err != nil {
		t.Fatal(err)
	}
	hard, _ := learn.ParseContender(g, "hard")
	medium, _ := learn.ParseContender(g, "medium")
	players := learn.Players(4)
	w, st, err := play(g, g.Config(4, "zolik_classic+floor35"), players, 7, 50_000, map[string]bool{"p1": true}, hard, medium)
	if err != nil || st.Stalled || st.Illegal > 0 {
		t.Fatalf("%v %+v", err, st)
	}
	deals := w.a.SeatDeals
	if deals < 1 || w.b.SeatDeals != 3*deals {
		t.Fatalf("A saw %d seat-deals, B %d", deals, w.b.SeatDeals)
	}
	if w.a.Out+w.b.Out != deals || w.a.Caught+w.b.Caught != 3*deals {
		t.Errorf("out %d+%d, caught %d+%d over %d deals", w.a.Out, w.b.Out, w.a.Caught, w.b.Caught, deals)
	}
	if len(w.a.Match) != 1 || len(w.b.Match) != 3 {
		t.Errorf("match totals %v %v", w.a.Match, w.b.Match)
	}
	sum := 0
	for _, p := range append(append([]int(nil), w.a.Penalty...), w.b.Penalty...) {
		sum += p
	}
	total := w.a.Match[0] + w.b.Match[0] + w.b.Match[1] + w.b.Match[2]
	if sum != total {
		t.Errorf("deal penalties add to %d, match totals to %d", sum, total)
	}
}

func TestCleanRunAndSetMaterial(t *testing.T) {
	for _, c := range []struct {
		hand      []string
		run, sets bool
	}{
		{[]string{"5S", "6S", "7S", "KD"}, true, false},
		{[]string{"AS", "2S", "3S"}, true, false},
		{[]string{"QH", "KH", "AH"}, true, false},
		{[]string{"6S", "JOKER1", "8S", "4D", "5D"}, false, false},
		{[]string{"9S", "9D", "JOKER2", "3C"}, false, true},
		{[]string{"9S", "9D", "9C", "3C"}, false, true},
		{[]string{"9S", "9S", "JOKER2"}, false, false},
	} {
		if got := cleanRun(c.hand, 3); got != c.run {
			t.Errorf("%v: clean run %v", c.hand, got)
		}
		if got := setMaterial(c.hand); got != c.sets {
			t.Errorf("%v: set material %v", c.hand, got)
		}
	}
}
