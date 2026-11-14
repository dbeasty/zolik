package okobere

import (
	"math/rand"
	"strconv"
	"testing"

	"zolik/server/internal/module"
)

// TestOkoLeavesTheOtherStakesCovered: an oko is paid out of what the bank
// holds beyond the stakes still on the table, so the bank can still pay
// them when the banker loses.
func TestOkoLeavesTheOtherStakesCovered(t *testing.T) {
	s := rigged(t, map[string][]string{"p1": {"7H"}, "p2": {"TH"}, "p3": {"AD"}}, []string{"9S", "AC", "8S", "7C"})
	s.Seats[0].Stack, s.Bank = 0, 100
	before := chips(s)
	s = must(t, s, "p2", betOf(30))
	s = must(t, s, "p2", cardAction) // 19
	s = must(t, s, "p2", standAction)
	s = must(t, s, "p3", betOf(50)) // the bank has 70 free
	s = must(t, s, "p3", cardAction)
	// Three times 50 is 150, but 30 of the bank is p2's to win: oko collects 70.
	if st := s.seat("p3"); st.Result != resultOko || st.Delta != 70 || st.Stack != 170 || s.Bank != 30 {
		t.Fatalf("after the oko: %+v, bank %d", st, s.Bank)
	}
	s = must(t, s, "p1", cardAction) // 15
	s = must(t, s, "p1", standAction)
	// p2 wins 30 from the 30 left: the bank is empty, broken, and passes to p2.
	if st := s.seat("p2"); st.Result != resultWon || st.Delta != 30 || s.worth("p2") != 130 {
		t.Fatalf("p2 against a bank of 15: %+v, worth %d", st, s.worth("p2"))
	}
	if !s.Rounds[len(s.Rounds)-1].BankBroken || s.bankerIndex() != 1 {
		t.Fatalf("an empty bank should be broken and pass on; banker is seat %d", s.bankerIndex())
	}
	if b := s.seat("p1"); b.Stack != 0 || s.Rounds[len(s.Rounds)-1].Stacks["p1"] != 0 {
		t.Fatalf("the old banker should be left with 0, has %d", b.Stack)
	}
	checkBank(t, s, before, "after the round")
	if chips(s) != before {
		t.Fatalf("chips went from %d to %d", before, chips(s))
	}
}

// checkBank is the money invariant: nothing negative, the bank able to pay
// every stake still on the table, and not a chip made or lost.
func checkBank(t *testing.T, s *GameState, want int, where string) {
	t.Helper()
	if s.Bank < 0 {
		t.Fatalf("%s: bank %d", where, s.Bank)
	}
	for _, st := range s.Seats {
		if st.Stack < 0 || st.Bet < 0 {
			t.Fatalf("%s: %s has stack %d, stake %d", where, st.PlayerID, st.Stack, st.Bet)
		}
	}
	if s.Status == "active" && s.committed() > s.Bank {
		t.Fatalf("%s: %d staked against a bank of %d", where, s.committed(), s.Bank)
	}
	if got := chips(s); got != want {
		t.Fatalf("%s: %d chips at the table, want %d", where, got, want)
	}
}

// randomMove is a reckless punter: any enabled offer, and a stake anywhere
// in its range, the top of it most often.
func randomMove(offers []module.ActionOffer, r *rand.Rand) (module.Action, bool) {
	var on []module.ActionOffer
	for _, o := range offers {
		if o.Enabled {
			on = append(on, o)
		}
	}
	if len(on) == 0 {
		return module.Action{}, false
	}
	o := on[r.Intn(len(on))]
	a := module.Action{OfferID: o.ID, Verb: o.Verb}
	if len(o.Params) > 0 {
		p := o.Params[0]
		n := p.Max
		if r.Intn(3) == 0 {
			n = p.Min + r.Intn(p.Max-p.Min+1)
		}
		a.Params = map[string]string{p.Name: strconv.Itoa(n)}
	}
	return a, true
}

// TestTheBankNeverGoesNegative plays many whole matches, bots and reckless
// players together, across the table options, checking the money after
// every move.
func TestTheBankNeverGoesNegative(t *testing.T) {
	m := New()
	skills := []module.Skill{module.SkillEasy, module.SkillMedium, module.SkillHard}
	seeds := 200
	if testing.Short() {
		seeds = 60
	}
	for seed := int64(1); seed <= int64(seeds); seed++ {
		r := rand.New(rand.NewSource(seed))
		n := 2 + r.Intn(5)
		opts := module.Options{module.OptPauseBetweenRounds: module.OptOff}
		if seed%4 != 0 { // every fourth match keeps the defaults
			opts[OptStartingStack] = []int{100, 200, 500}[r.Intn(3)]
			opts[OptMinBet] = []int{1, 2, 5}[r.Intn(3)]
			opts[OptBank] = []int{20, 50, 100}[r.Intn(3)]
			opts[OptRoundsPerBank] = []int{3, 5, 8}[r.Intn(3)]
			opts[OptBankTurns] = []int{1, 2}[r.Intn(2)]
			opts[OptOkoPays] = []int{1, 3}[r.Intn(2)]
		}
		state, err := m.NewMatch(module.MatchConfig{Options: opts}, refs(n), seed)
		if err != nil {
			t.Fatal(err)
		}
		bots := map[string]module.Skill{}
		for _, p := range refs(n) {
			if r.Intn(2) == 0 {
				bots[p.ID] = skills[r.Intn(len(skills))]
			}
		}
		s, _ := decode(state)
		want := n * s.StartingStack
		for step := 0; ; step++ {
			s, _ = decode(state)
			where := "seed " + strconv.FormatInt(seed, 10) + " step " + strconv.Itoa(step)
			checkBank(t, s, want, where)
			if s.Status != "active" {
				break
			}
			if step > 20000 {
				t.Fatalf("%s: the match did not finish", where)
			}
			actor := s.Current
			offers, err := m.LegalActions(state, actor)
			if err != nil {
				t.Fatal(err)
			}
			var a module.Action
			var ok bool
			if skill, isBot := bots[actor]; isBot {
				a, ok = m.Bot().Act(state, module.BotSeat{PlayerID: actor, Skill: skill, Seed: seed}, offers)
			} else {
				a, ok = randomMove(offers, r)
			}
			if !ok {
				t.Fatalf("%s: %s had no move", where, actor)
			}
			next, _, err := m.Apply(state, actor, a)
			if err != nil {
				t.Fatalf("%s: %s %s %v refused: %v", where, actor, a.Verb, a.Params, err)
			}
			state = next
		}
	}
}
