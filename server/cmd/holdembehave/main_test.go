package main

import (
	"testing"

	"zolik/server/internal/learn"
)

// TestCountsBigBets: a maniac overbets most hands it plays, so a few of its
// matches must show big bets, most of them called by the station, and every
// dealt hand counted once per side.
func TestCountsBigBets(t *testing.T) {
	g, err := learn.LookupGame("holdem")
	if err != nil {
		t.Fatal(err)
	}
	bots := map[string]learn.Contender{}
	for label, spec := range map[string]string{labelA: "maniac", labelB: "station"} {
		if bots[label], err = learn.ParseContender(g, spec); err != nil {
			t.Fatal(err)
		}
	}
	total := map[string]*tally{labelA: newTally(), labelB: newTally()}
	for seed := int64(1); seed <= 4; seed++ {
		for _, flip := range []bool{false, true} {
			got, st, err := play(g, 2, seed, 50_000, flip, bots)
			if err != nil || st.Stalled || st.Illegal > 0 {
				t.Fatalf("seed %d: %v %+v", seed, err, st)
			}
			total[labelA].add(got[labelA])
			total[labelB].add(got[labelB])
		}
	}
	a, b := total[labelA], total[labelB]
	if a.Hands == 0 || a.Hands != b.Hands {
		t.Fatalf("hands: A %d, B %d", a.Hands, b.Hands)
	}
	all := a.By["all"]
	if all.Big < a.Hands/4 || all.Called < all.Big/2 || b.By["all"].Big != 0 {
		t.Errorf("maniac %+v over %d hands; station %+v", *all, a.Hands, *b.By["all"])
	}
	if all.Big != a.By["preflop"].Big+a.By["postflop"].Big {
		t.Errorf("street split %d + %d != %d", a.By["preflop"].Big, a.By["postflop"].Big, all.Big)
	}
}
