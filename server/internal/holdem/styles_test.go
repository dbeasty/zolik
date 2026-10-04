package holdem

import (
	"testing"

	"zolik/server/internal/learn"
	"zolik/server/internal/module"
)

// TestSolidPlaysWholeLegalMatches: the solid style at every table size the
// pool deals, beside Hard and against itself, every move applied as given.
func TestSolidPlaysWholeLegalMatches(t *testing.T) {
	b := learnGame{}.Styles()["solid"]
	for n := 2; n <= 6; n++ {
		for seed := int64(1); seed <= 2; seed++ {
			players := learn.Players(n)
			seats := botSeats(players, bot{})
			for i, p := range players {
				if seed == 2 || i%2 == 0 {
					seats[p.ID] = b
				}
			}
			state, err := New().NewMatch(learnGame{}.Config(n, ""), players, seed*100+int64(n))
			if err != nil {
				t.Fatal(err)
			}
			if final := playBots(t, state, players, seats, 20000); final.Status != "completed" {
				t.Errorf("%d seats, seed %d: %s", n, seed, final.Status)
			}
		}
	}
}

// shovedInto seats two on a flop with two hundred in the pot and the
// opponent all in for `shove` more, seat 0 to answer with nine hundred behind.
func shovedInto(hole, board []string, shove int) module.State {
	return table(2, func(s *GameState) {
		s.Street = streetFlop
		s.Board = board
		s.Current = 0
		s.Pot = 200
		s.Seats[0].Hole = hole
		s.Seats[0].Committed, s.Seats[0].Stack = 100, 900
		s.Seats[1].Hole = []string{"5C", "5D"}
		s.Seats[1].Committed, s.Seats[1].Stack = 100+shove, 0
		s.Seats[1].Bet, s.Seats[1].Acted, s.Seats[1].AllIn = shove, true, true
		s.CurrentBet, s.MinRaise = shove, s.BigBlind
	})
}

// TestSolidCallsAShoveWithTopPairAndFoldsNothing is the style's reason to
// exist, pinned: a pot-sized shove on a dry board, and one of twice the pot,
// is called by top pair with a good kicker and by better, every time, and
// folded by a hand with nothing. A call here is a call or a raise — facing an
// all-in the engine still offers a raise, and either puts the chips in.
// Hard's answers are logged beside it: it reads a shove past the pot as a
// claim to two pair, and lets top pair go to the bigger one.
func TestSolidCallsAShoveWithTopPairAndFoldsNothing(t *testing.T) {
	board := []string{"KH", "7D", "2C"}
	solidBot := learnGame{}.Styles()["solid"]
	act := func(b module.Bot, raw module.State, skill module.Skill) string {
		offers, err := New().LegalActions(raw, "p1")
		if err != nil {
			t.Fatal(err)
		}
		a, ok := b.Act(raw, module.BotSeat{PlayerID: "p1", Skill: skill}, offers)
		if !ok {
			t.Fatal("no move")
		}
		return a.Verb
	}
	cases := []struct {
		name string
		hole []string
		call bool
	}{
		{"top pair, queen kicker", []string{"KS", "QD"}, true},
		{"top pair, ace kicker", []string{"AS", "KD"}, true},
		{"top pair, jack kicker", []string{"KC", "JS"}, true},
		{"overpair", []string{"AH", "AD"}, true},
		{"set", []string{"7S", "7H"}, true},
		{"queen high", []string{"QS", "9D"}, false},
		{"jack high", []string{"JS", "4D"}, false},
		{"ten high", []string{"TH", "8S"}, false},
	}
	for _, shove := range []int{200, 400} {
		for _, c := range cases {
			calls, hardCalls := 0, 0
			const seeds = 20
			for seed := int64(1); seed <= seeds; seed++ {
				raw := atSeed(t, shovedInto(c.hole, board, shove), seed)
				v := act(solidBot, raw, "")
				if v == VerbCall || v == VerbRaise {
					calls++
				}
				if hv := act(bot{}, raw, module.SkillHard); hv == VerbCall || hv == VerbRaise {
					hardCalls++
				}
			}
			t.Logf("shove %d: %-24s solid calls %2d/%d, hard %2d/%d", shove, c.name, calls, seeds, hardCalls, seeds)
			if c.call && calls != seeds {
				t.Errorf("%s facing a shove of %d on %v: solid called %d of %d", c.name, shove, board, calls, seeds)
			}
			if !c.call && calls != 0 {
				t.Errorf("%s facing a shove of %d on %v: solid called %d of %d, want none", c.name, shove, board, calls, seeds)
			}
		}
	}
}

// TestPusherPlaysShortStackedMatches: the pusher at the short-stacked
// training table, at every size the pool deals, every move applied as given;
// and heads-up against a station, where Hard gives the chart up because the
// station never folds, the pusher keeps jamming.
func TestPusherPlaysShortStackedMatches(t *testing.T) {
	g := learnGame{}
	b := g.Styles()["pusher"]
	for n := 2; n <= 6; n++ {
		players := learn.Players(n)
		seats := botSeats(players, bot{})
		seats[players[0].ID] = b
		seats[players[n-1].ID] = b
		state, err := New().NewMatch(g.ShortConfig(n, "", 0), players, int64(300+n))
		if err != nil {
			t.Fatal(err)
		}
		if final := playBots(t, state, players, seats, 20000); final.Status != "completed" {
			t.Errorf("%d seats: %s", n, final.Status)
		}
	}
	pfr := func(x module.Bot) (int, int) {
		raises, hands := 0, 0
		players := refs("x", "y")
		for seed := int64(1); seed <= 6; seed++ {
			seats := map[string]module.Bot{"x": x, "y": station{}}
			state, _ := New().NewMatch(g.ShortConfig(2, "", 0), players, seed)
			r := playBots(t, state, players, seats, 20000).seat("x").Reads
			raises += r.PFR
			hands += r.Hands
		}
		return raises, hands
	}
	pr, ph := pfr(b)
	hardProfile := profiles[module.SkillHard]
	hr, hh := pfr(bot{tuning: &hardProfile})
	t.Logf("raises before the flop vs a station at 15 bb: pusher %d/%d, hard %d/%d", pr, ph, hr, hh)
	if float64(pr)/float64(ph) <= float64(hr)/float64(hh) {
		t.Errorf("the pusher raised no more often than Hard against a station")
	}
}

// TestShortConfigIsFifteenToTwentyFiveBigBlinds pins the training depth.
func TestShortConfigIsFifteenToTwentyFiveBigBlinds(t *testing.T) {
	g := learnGame{}
	for _, c := range []struct {
		roll float64
		bb   int
	}{{0, 15}, {0.5, 20}, {0.999, 25}} {
		o := g.ShortConfig(2, "", c.roll).Options
		if got := o[OptStartingStack] / o[OptBigBlind]; got != c.bb {
			t.Errorf("roll %.3f: %d bb, want %d", c.roll, got, c.bb)
		}
	}
	if o := g.Config(2, "").Options; o[OptStartingStack] != 1000 {
		t.Errorf("ShortConfig changed the usual table: %v", o)
	}
}
