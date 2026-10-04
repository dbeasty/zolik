package holdem

import (
	"math"
	"math/rand"
	"strconv"
	"testing"

	"zolik/server/internal/module"
)

// TestRiverBluffChanceGivesTheEquilibriumShare is the arithmetic: over a range
// spread the way riverValueShare and riverAirShare say, the bluffs come out at
// B/(P+2B) of the bets — a quarter at half the pot, a third at the pot, two
// fifths at twice it.
func TestRiverBluffChanceGivesTheEquilibriumShare(t *testing.T) {
	for _, c := range []struct {
		bet, pot int
		want     float64
	}{{50, 100, 0.25}, {100, 100, 1.0 / 3}, {200, 100, 0.4}} {
		bluffs := riverBluffChance(c.bet, c.pot) * riverAirShare
		if got := bluffs / (bluffs + riverValueShare); math.Abs(got-c.want) > 1e-9 {
			t.Errorf("bet %d into %d: bluffs are %.4f of the bets, want %.4f", c.bet, c.pot, got, c.want)
		}
	}
}

// TestRiverShares re-measures the two constants riverBluffChance divides by:
// the share of river hands at each polar end, by the bot's own estimate.
func TestRiverShares(t *testing.T) {
	r := rand.New(rand.NewSource(7))
	deck := buildDeck()
	value, air, n := 0, 0, 6000
	for i := 0; i < n; i++ {
		r.Shuffle(len(deck), func(a, b int) { deck[a], deck[b] = deck[b], deck[a] })
		switch eq := equity(deck[:2], deck[2:7], 1, rollouts(1), highCard, 0, r); {
		case eq >= riverValue:
			value++
		case eq < riverAir:
			air++
		}
	}
	gotValue, gotAir := float64(value)/float64(n), float64(air)/float64(n)
	if math.Abs(gotValue-riverValueShare) > 0.02 || math.Abs(gotAir-riverAirShare) > 0.02 {
		t.Errorf("river shares measured %.3f value and %.3f air, constants say %.3f and %.3f",
			gotValue, gotAir, riverValueShare, riverAirShare)
	}
}

// checkedToOnTheRiver deals a random heads-up river from r, checked to p1,
// two hundred in the pot and nine hundred behind each, against an opponent
// that has folded to two of four bets after the flop.
func checkedToOnTheRiver(r *rand.Rand, seed int64) (module.State, []string, []string) {
	deck := buildDeck()
	r.Shuffle(len(deck), func(a, b int) { deck[a], deck[b] = deck[b], deck[a] })
	hole, board := deck[:2], deck[4:9]
	return table(2, func(s *GameState) {
		s.Seed = seed
		s.Street = streetRiver
		s.Board = board
		s.Current = 0
		s.Pot = 200
		s.Seats[0].Hole = hole
		s.Seats[0].Committed, s.Seats[0].Stack = 100, 900
		s.Seats[1].Hole = deck[2:4]
		s.Seats[1].Committed, s.Seats[1].Stack = 100, 900
		s.Seats[1].Acted = true
		s.Seats[1].Reads = &SeatReads{Hands: 6, FacedBet: 4, FoldedToBet: 2}
		s.CurrentBet, s.MinRaise = 0, s.BigBlind
	}), hole, board
}

// TestHardRiverBluffsConvergeToTheBetSize: over many seeded river decisions,
// of the bets Hard makes at its polarised size (seventy percent of the pot),
// the share made with a hand that is behind most hands is B/(P+2B) = 0.29.
// Medium, with its flat bluff chance, sits well below it.
func TestHardRiverBluffsConvergeToTheBetSize(t *testing.T) {
	const deals = 3000
	r := rand.New(rand.NewSource(11))
	size := strconv.Itoa(int(math.Round(riverBetSize * 200)))
	bets, bluffs := 0, 0
	for i := 0; i < deals; i++ {
		raw, hole, board := checkedToOnTheRiver(r, int64(i+1))
		a := actAs(t, raw, "p1", module.SkillHard)
		if a.Verb != VerbRaise || a.Params[ParamAmount] != size {
			continue
		}
		bets++
		if equity(hole, board, 1, 2000, highCard, 0, r) < 0.5 {
			bluffs++
		}
	}
	want := riverBetSize / (1 + 2*riverBetSize)
	got := float64(bluffs) / float64(bets)
	se := math.Sqrt(want * (1 - want) / float64(bets))
	t.Logf("%d polarised river bets, %d of them bluffs: %.3f, want %.3f ± %.3f", bets, bluffs, got, want, se)
	if math.Abs(got-want) > 3*se {
		t.Errorf("bluffs are %.3f of Hard's polarised river bets, want %.3f within %.3f", got, want, 3*se)
	}
}

// TestDefendsTheMinimum is the other side: facing B into P, a hand in the
// top P/(P+B) of all hands continues.
func TestDefendsTheMinimum(t *testing.T) {
	for _, c := range []struct {
		anyHand   float64
		owed, pot int
		want      bool
	}{
		{0.55, 100, 200, true},  // pot-sized: the top half
		{0.45, 100, 200, false}, //
		{0.40, 50, 150, true},   // half pot: the top two thirds
		{0.30, 50, 150, false},  //
		{0.80, 300, 400, true},  // three pots: the top quarter
		{0.70, 300, 400, false}, //
		{0.70, 0, 400, false},   // nothing to defend against
	} {
		if got := defends(c.anyHand, c.owed, c.pot); got != c.want {
			t.Errorf("hand %.2f facing %d with %d on the table: defends %v, want %v", c.anyHand, c.owed, c.pot, got, c.want)
		}
	}
}

// TestHardDefendsAgainstAShownBluffer: half the pot on a K-9-5-3-2 river, and
// a pair of threes that the claim-read (a pair held now) puts well behind. It
// is in the top two thirds of all hands, so against a bettor that has turned
// over a big bet made with nothing it calls, every seed; against one that has
// not, the read stands and it folds. Queen high is below the line either way.
func TestHardDefendsAgainstAShownBluffer(t *testing.T) {
	board := []string{"KH", "9D", "5C", "3S", "2H"}
	for _, c := range []struct {
		name  string
		hole  []string
		reads SeatReads
		call  bool
	}{
		{"pair of threes, bettor shown bluffing", []string{"3C", "7D"}, SeatReads{Hands: 5, BigBetsShown: 2, BluffsShown: 1}, true},
		{"pair of threes, bettor never shown", []string{"3C", "7D"}, SeatReads{Hands: 5, BigBetsShown: 2}, false},
		{"queen high, bettor shown bluffing", []string{"QS", "JD"}, SeatReads{Hands: 5, BigBetsShown: 2, BluffsShown: 1}, false},
	} {
		for seed := int64(1); seed <= 10; seed++ {
			s, err := decode(facing(c.hole, board, 100))
			if err != nil {
				t.Fatal(err)
			}
			s.Seed = seed
			s.Seats[1].Reads = &c.reads
			raw, err := encode(s)
			if err != nil {
				t.Fatal(err)
			}
			if v := actAs(t, raw, "p1", module.SkillHard).Verb; (v == VerbCall) != c.call {
				t.Errorf("%s, seed %d: %s, want call %v", c.name, seed, v, c.call)
			}
		}
	}
}

// TestHardDoesNotBluffAStation: against an opponent that has answered five
// bets after the flop and folded none — or that has not yet been seen to fold
// at all — the equilibrium share is only chips given away, so the air end of
// the range checks every time; the value end still bets.
func TestHardDoesNotBluffAStation(t *testing.T) {
	for _, reads := range []*SeatReads{{Hands: 6, FacedBet: 5}, nil} {
		hardDoesNotBluff(t, reads)
	}
}

func hardDoesNotBluff(t *testing.T, reads *SeatReads) {
	r := rand.New(rand.NewSource(13))
	size := strconv.Itoa(int(math.Round(riverBetSize * 200)))
	bets, bluffs := 0, 0
	for i := 0; i < 600; i++ {
		raw, hole, board := checkedToOnTheRiver(r, int64(i+1))
		s, err := decode(raw)
		if err != nil {
			t.Fatal(err)
		}
		s.Seats[1].Reads = reads
		if raw, err = encode(s); err != nil {
			t.Fatal(err)
		}
		a := actAs(t, raw, "p1", module.SkillHard)
		if a.Verb != VerbRaise || a.Params[ParamAmount] != size {
			continue
		}
		bets++
		if equity(hole, board, 1, 2000, highCard, 0, r) < 0.5 {
			bluffs++
		}
	}
	if bets == 0 || bluffs > 0 {
		t.Errorf("against reads %+v: %d polarised river bets, %d of them bluffs; want value bets and no bluffs", reads, bets, bluffs)
	}
}
