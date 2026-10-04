package holdem

import (
	"os"
	"testing"

	"zolik/server/internal/module"
)

// TestPushFoldTableIsGenerated holds the committed chart to its generator:
// the solve is deterministic, so regenerating it must reproduce the file byte
// for byte. A hand edit to the table, or a change to the solver that moves it,
// fails here until `go run ./cmd/pushfold` is run and the result committed.
func TestPushFoldTableIsGenerated(t *testing.T) {
	if testing.Short() {
		t.Skip("solving the jam-or-fold game is not a fast test")
	}
	want, err := os.ReadFile("pushfold_table.go")
	if err != nil {
		t.Fatal(err)
	}
	got, summary := PushFoldSource()
	t.Log("\n" + summary)
	if got != string(want) {
		t.Fatal("pushfold_table.go does not match its generator; run `go run ./cmd/pushfold` from server/")
	}
}

// TestHandClassesRoundTrip: every grid cell names one class, and every pair
// of hole cards lands in the cell of its class.
func TestHandClassesRoundTrip(t *testing.T) {
	combos := 0
	for i := 0; i < 169; i++ {
		c := classAt(i)
		if got := classIndex(c.hi, c.lo, c.suited); got != i {
			t.Fatalf("%v: index %d, want %d", c, got, i)
		}
		combos += len(c.combos())
	}
	if combos != 1326 {
		t.Errorf("%d combos over the 169 classes, want 1326", combos)
	}
	for _, c := range []struct {
		hole []string
		name string
	}{
		{[]string{"AS", "KS"}, "AKs"}, {[]string{"KD", "AS"}, "AKo"}, {[]string{"7H", "2C"}, "72o"},
		{[]string{"TC", "TD"}, "TT"}, {[]string{"5H", "4H"}, "54s"},
	} {
		if got := classAt(classOf(c.hole)).String(); got != c.name {
			t.Errorf("%v is %s, want %s", c.hole, got, c.name)
		}
	}
}

// TestPushFoldChartKnownEntries pins the chart against what every published
// heads-up jam-or-fold chart agrees on, so a solver bug that still reproduces
// itself cannot pass: the big pairs and big aces always jam and always call,
// the worst offsuit hands jam only when a fold would cost most of the stack,
// and the small blind jams wider than the big blind calls.
func TestPushFoldChartKnownEntries(t *testing.T) {
	class := func(hole ...string) int { return classOf(hole) }
	for _, c := range []struct {
		name      string
		hand      int
		bb        float64
		jam, call bool
	}{
		{"AA, 50 BB", class("AS", "AH"), 50, true, true},
		{"KK, 50 BB", class("KS", "KH"), 50, true, true},
		{"AKo, 20 BB", class("AS", "KH"), 20, true, true},
		{"22, 10 BB", class("2S", "2H"), 10, true, true},
		{"A2o, 10 BB", class("AS", "2H"), 10, true, true},
		{"K2o, 10 BB", class("KS", "2H"), 10, true, false},
		{"T8s, 10 BB", class("TS", "8S"), 10, true, false},
		{"72o, 10 BB", class("7S", "2H"), 10, false, false},
		{"32o, 5 BB", class("3S", "2H"), 5, false, false},
		{"72o, 1 BB", class("7S", "2H"), 1, true, true},
	} {
		if got := inChart(&pushFoldJam, c.hand, c.bb); got != c.jam {
			t.Errorf("%s: jams %v, want %v", c.name, got, c.jam)
		}
		if got := inChart(&pushFoldCall, c.hand, c.bb); got != c.call {
			t.Errorf("%s: calls %v, want %v", c.name, got, c.call)
		}
	}
	// Ranges shrink as stacks deepen, and past the shallowest stacks jams are
	// wider than calls.
	share := func(row *[169]uint8, bb float64) float64 {
		n := 0
		for i := 0; i < 169; i++ {
			if inChart(row, i, bb) {
				n += len(classAt(i).combos())
			}
		}
		return float64(n) / 1326
	}
	prevJam, prevCall := 1.01, 1.01
	for _, bb := range []float64{2, 5, 8, 10, 12, 15, 20} {
		jam, call := share(&pushFoldJam, bb), share(&pushFoldCall, bb)
		if jam > prevJam || call > prevCall || (bb >= 5 && call > jam) {
			t.Errorf("at %v BB the small blind jams %.2f and the big blind calls %.2f (shallower: %.2f, %.2f)",
				bb, jam, call, prevJam, prevCall)
		}
		prevJam, prevCall = jam, call
	}
}

// headsUpPreflop seats two at `eff` chips each, p1 on the button in the small
// blind and to act; p2's hole is never read, and p2 has folded to one of the
// two raises it has faced, which is the evidence an open jam needs.
func headsUpPreflop(hole []string, eff int) module.State {
	return table(2, func(s *GameState) {
		s.Current = 0
		s.Seats[0].Hole = hole
		s.Seats[0].Bet, s.Seats[0].Committed, s.Seats[0].Stack = 10, 10, eff-10
		s.Seats[1].Bet, s.Seats[1].Committed, s.Seats[1].Stack = 20, 20, eff-20
		s.Seats[1].Reads = &SeatReads{Hands: 3, VPIP: 1, FacedRaise: 2, FoldedToRaise: 1}
		s.CurrentBet, s.MinRaise = 20, 20
	})
}

// facingAJam is the same table after p1 has jammed, p2 in the big blind to
// answer.
func facingAJam(hole []string, eff int) module.State {
	return table(2, func(s *GameState) {
		s.Current = 1
		s.Seats[0].Hole = []string{"QC", "JD"}
		s.Seats[0].Bet, s.Seats[0].Committed, s.Seats[0].Stack = eff, eff, 0
		s.Seats[0].Acted, s.Seats[0].AllIn = true, true
		s.Seats[1].Hole = hole
		s.Seats[1].Bet, s.Seats[1].Committed, s.Seats[1].Stack = 20, 20, eff-20
		s.CurrentBet, s.MinRaise = eff, eff-20
	})
}

// TestHardPlaysTheChartShort: at ten big blinds Hard jams or folds from the
// small blind and calls or folds in the big blind, exactly as the chart says;
// deep, and at every other strength, it plays as it did.
func TestHardPlaysTheChartShort(t *testing.T) {
	for _, c := range []struct {
		name string
		hole []string
		want string
	}{
		{"aces", []string{"AS", "AH"}, VerbRaise},
		{"king-deuce offsuit", []string{"KS", "2H"}, VerbRaise},
		{"seven-deuce offsuit", []string{"7S", "2H"}, VerbFold},
	} {
		a := actAs(t, headsUpPreflop(c.hole, 200), "p1", module.SkillHard)
		if a.Verb != c.want {
			t.Errorf("small blind with %s at 10 BB: %s, want %s", c.name, a.Verb, c.want)
		}
		if a.Verb == VerbRaise && a.Params[ParamAmount] != "200" {
			t.Errorf("small blind with %s at 10 BB raised to %s, want all in for 200", c.name, a.Params[ParamAmount])
		}
	}
	for _, c := range []struct {
		name string
		hole []string
		call bool
	}{
		{"ace-deuce offsuit", []string{"AS", "2H"}, true},
		{"king-deuce offsuit", []string{"KS", "2H"}, false},
		{"pocket deuces", []string{"2S", "2H"}, true},
	} {
		v := actAs(t, facingAJam(c.hole, 200), "p2", module.SkillHard).Verb
		if called := v == VerbCall || v == VerbRaise; called != c.call {
			t.Errorf("big blind with %s facing a 10 BB jam: %s, want call %v", c.name, v, c.call)
		}
	}
	// King-deuce from the small blind at fifty big blinds is not a jam, and
	// Medium never consults the chart.
	if a := actAs(t, headsUpPreflop([]string{"KS", "2H"}, 1000), "p1", module.SkillHard); a.Verb == VerbRaise && a.Params[ParamAmount] == "1000" {
		t.Errorf("hard jammed king-deuce at 50 BB")
	}
	if a := actAs(t, headsUpPreflop([]string{"KS", "2H"}, 200), "p1", module.SkillMedium); a.Verb == VerbRaise {
		t.Errorf("medium raised king-deuce at 10 BB: it has no chart")
	}
}

// TestPushFoldTightensWithMorePlayers: a hand the small blind jams heads-up
// is folded first to act with five players behind.
func TestPushFoldTightensWithMorePlayers(t *testing.T) {
	hole := []string{"KS", "2H"}
	raw := table(6, func(s *GameState) {
		s.Current = 3
		for i := range s.Seats {
			s.Seats[i].Stack = 200
		}
		s.Seats[3].Hole = hole
		s.Seats[1].Bet, s.Seats[1].Committed, s.Seats[1].Stack = 10, 10, 190
		s.Seats[2].Bet, s.Seats[2].Committed, s.Seats[2].Stack = 20, 20, 180
		s.CurrentBet, s.MinRaise = 20, 20
	})
	if v := actAs(t, raw, "p4", module.SkillHard).Verb; v != VerbFold {
		t.Errorf("king-deuce at 10 BB with five to act behind: %s, want fold", v)
	}
	raw = table(6, func(s *GameState) {
		s.Current = 3
		for i := range s.Seats {
			s.Seats[i].Stack = 200
		}
		s.Seats[3].Hole = []string{"AS", "AH"}
		s.Seats[1].Bet, s.Seats[1].Committed, s.Seats[1].Stack = 10, 10, 190
		s.Seats[2].Bet, s.Seats[2].Committed, s.Seats[2].Stack = 20, 20, 180
		s.CurrentBet, s.MinRaise = 20, 20
	})
	if a := actAs(t, raw, "p4", module.SkillHard); a.Verb != VerbRaise || a.Params[ParamAmount] != "200" {
		t.Errorf("aces at 10 BB with five to act behind: %v, want all in", a)
	}
}

// TestReadsCountFoldsToARaise: answering a raise before the flop counts, and
// a fold to one counts as a fold; answering the big blind alone is neither.
func TestReadsCountFoldsToARaise(t *testing.T) {
	s := &GameState{Street: streetPreflop, BigBlind: 20, Seats: []Seat{{PlayerID: "p1"}, {PlayerID: "p2"}}}
	record(s, VerbCall, decisionAt{seat: 0, owed: 10, bet: 20})
	record(s, VerbFold, decisionAt{seat: 1, owed: 180, bet: 200})
	record(s, VerbCall, decisionAt{seat: 0, owed: 400, bet: 600})
	if r := s.Seats[0].Reads; r.FacedRaise != 1 || r.FoldedToRaise != 0 {
		t.Errorf("p1 limped then called a raise: %+v, want one raise faced and no folds", *r)
	}
	if r := s.Seats[1].Reads; r.FacedRaise != 1 || r.FoldedToRaise != 1 {
		t.Errorf("p2 folded to a raise: %+v, want one faced and one folded", *r)
	}
}

// TestTheChartGivesWayToANonFolder: heads-up against a player who has
// answered four raises and folded none, or who has not been seen to fold to a
// raise yet, king-deuce is not jammed from the chart — there is no fold
// equity in it that anyone has seen — while against one who folded two of
// four it is. Aces are raised either way.
func TestTheChartGivesWayToANonFolder(t *testing.T) {
	at := func(hole []string, reads *SeatReads) module.Action {
		s, err := decode(headsUpPreflop(hole, 200))
		if err != nil {
			t.Fatal(err)
		}
		s.Seats[1].Reads = reads
		raw, err := encode(s)
		if err != nil {
			t.Fatal(err)
		}
		return actAs(t, raw, "p1", module.SkillHard)
	}
	station := &SeatReads{Hands: 5, VPIP: 5, FacedRaise: 4}
	folder := &SeatReads{Hands: 5, VPIP: 2, FacedRaise: 4, FoldedToRaise: 2}
	for _, r := range []*SeatReads{station, nil} {
		if a := at([]string{"KS", "2H"}, r); a.Verb == VerbRaise && a.Params[ParamAmount] == "200" {
			t.Errorf("king-deuce jammed into a player who has never folded to a raise (%+v)", r)
		}
	}
	if a := at([]string{"KS", "2H"}, folder); a.Verb != VerbRaise || a.Params[ParamAmount] != "200" {
		t.Errorf("king-deuce against a player who folds to raises: %v, want the chart's jam", a)
	}
	if a := at([]string{"AS", "AH"}, station); a.Verb != VerbRaise {
		t.Errorf("aces against a non-folder: %s, want a raise", a.Verb)
	}
}
