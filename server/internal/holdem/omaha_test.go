package holdem

import (
	"math/rand"
	"strings"
	"testing"

	"zolik/server/internal/module"
)

var omahaCfg = module.MatchConfig{Variation: VarOmaha, Options: module.Options{OptHandLimit: 10}}

// TestOmahaUsesExactlyTwo pins the rule the whole game is about: two from the
// hand, three from the board, never more or fewer of either.
func TestOmahaUsesExactlyTwo(t *testing.T) {
	cases := []struct {
		name, hole, board string
		want              int
	}{
		// No flush, and no straight either: Q-J would need K-T-9 from the
		// board with no king there, and A-K-Q-J-T takes three from the hand.
		{"four hearts on the board and one in hand is no flush",
			"AH KS QC JD", "2H 5H 9H TH 3C", highCard},
		{"two hearts in hand with three on the board is",
			"AH KH QC JD", "2H 5H 9H TC 3C", flush},
		{"a straight on the board cannot be played: a pair of sevens",
			"2C 2D 7S 7H", "9C TD JH QS KC", pair},
		{"three of a kind in hand plays as a pair",
			"AC AD AH KS", "2C 7D 9H JS 4C", pair},
		{"quads on the board play as trips",
			"AC KD 2S 3H", "9C 9D 9H 9S 4C", trips},
	}
	for _, tc := range cases {
		got := BestUsing(strings.Fields(tc.hole), strings.Fields(tc.board), 2)
		if got.Category != tc.want {
			t.Errorf("%s: category %d (%v), want %d", tc.name, got.Category, got.Cards, tc.want)
		}
		held := 0
		for _, c := range got.Cards {
			if strings.Contains(tc.hole, c) {
				held++
			}
		}
		if held != 2 {
			t.Errorf("%s: best hand %v uses %d hole cards, want exactly 2", tc.name, got.Cards, held)
		}
	}
}

// TestOmahaScoreAgreesWithBestUsing: the bot's fast scorer and the pot's
// evaluator must rank every pair of hands the same way, or the bot would be
// playing a different game from the one it is paid in.
func TestOmahaScoreAgreesWithBestUsing(t *testing.T) {
	rnd := rand.New(rand.NewSource(7))
	deck := buildDeck()
	for trial := 0; trial < 4000; trial++ {
		rnd.Shuffle(len(deck), func(i, j int) { deck[i], deck[j] = deck[j], deck[i] })
		board := deck[:5]
		a, b := deck[5:9], deck[9:13]
		ra, rb := BestUsing(a, board, 2), BestUsing(b, board, 2)
		sa, sb := omahaScore(codesOf(a), codesOf(board)), omahaScore(codesOf(b), codesOf(board))
		if scoreCategory(sa) != ra.Category {
			t.Fatalf("%v on %v: fast category %d, evaluator %d", a, board, scoreCategory(sa), ra.Category)
		}
		cmp := ra.Compare(rb)
		fast := 0
		if sa > sb {
			fast = 1
		} else if sa < sb {
			fast = -1
		}
		if cmp != fast {
			t.Fatalf("%v vs %v on %v: evaluator says %d, fast scorer %d", a, b, board, cmp, fast)
		}
	}
}

func TestOmahaDealsFourAndPaysByTheRule(t *testing.T) {
	m := New()
	raw, err := m.NewMatch(omahaCfg, seatIDs(9), 4)
	if err != nil {
		t.Fatalf("nine seats should deal: %v", err)
	}
	s := mustDecode(t, raw)
	for _, st := range s.Seats {
		if len(st.Hole) != 4 {
			t.Fatalf("%s was dealt %d cards, want 4", st.PlayerID, len(st.Hole))
		}
	}
	if s.Street != streetPreflop {
		t.Errorf("Omaha opens on %q", s.Street)
	}

	// A showdown where a one-heart hand faces a two-heart hand on a
	// four-heart board: the two-heart hand's flush wins, the other has none.
	raw = table(2, func(s *GameState) {
		s.Variation = VarOmaha
		s.Street = streetRiver
		s.Board = []string{"2H", "5H", "9H", "TH", "3C"}
		s.Seats[0].Hole = []string{"AH", "KS", "QC", "JD"}
		s.Seats[1].Hole = []string{"4H", "6H", "7C", "8D"}
		for i := range s.Seats {
			s.Seats[i].Committed, s.Seats[i].Stack, s.Seats[i].Acted = 100, 900, true
		}
		s.Pot = 200
		s.Current = 0
	})
	raw, _, err = m.Apply(raw, "p1", module.Action{Verb: VerbCheck})
	if err != nil {
		t.Fatal(err)
	}
	s = mustDecode(t, raw)
	if s.LastHand == nil || len(s.LastHand.Pots) != 1 {
		t.Fatalf("no showdown: %+v", s.LastHand)
	}
	pot := s.LastHand.Pots[0]
	if len(pot.Winners) != 1 || pot.Winners[0] != "p2" || pot.LabelKey != categoryKey(straightFlush) && pot.LabelKey != categoryKey(flush) {
		t.Errorf("the two-heart hand should win with a flush: %+v", pot)
	}
}

func TestPotLimitCapsTheRaise(t *testing.T) {
	m := New()
	// Preflop at 10/20, first to act: pot-sized is to 70.
	raw := table(4, func(s *GameState) {
		s.Variation = VarOmaha
		s.Button = 0
		s.Seats[1].Bet, s.Seats[1].Committed, s.Seats[1].Stack = 10, 10, 990
		s.Seats[2].Bet, s.Seats[2].Committed, s.Seats[2].Stack = 20, 20, 980
		s.CurrentBet = 20
		s.Current = 3
	})
	offers, err := m.LegalActions(raw, "p4")
	if err != nil {
		t.Fatal(err)
	}
	var raise module.ActionOffer
	for _, o := range offers {
		if o.Verb == VerbRaise {
			raise = o
		}
	}
	if !raise.Enabled || raise.Params[0].Max != 70 || raise.Params[0].Min != 40 {
		t.Fatalf("raise range %+v, want 40..70", raise.Params)
	}
	var labels []string
	for _, c := range raise.Params[0].Choices {
		labels = append(labels, c.LabelKey+"="+c.Value)
		if c.LabelKey == "holdem.quick.allIn" {
			t.Errorf("a pot-limit raise capped below the stack offers %q", c.LabelKey)
		}
	}
	if strings.Join(labels, " ") != "holdem.quick.halfPot=45 holdem.quick.pot=70" {
		t.Errorf("quick choices %v, want half pot 45 and pot 70", labels)
	}
	if _, _, err := m.Apply(raw, "p4", raiseTo(71)); module.CodeOf(err) != ErrOverPotLimit {
		t.Errorf("a raise to 71 got %v, want %s", err, ErrOverPotLimit)
	}
	if _, _, err := m.Apply(raw, "p4", raiseTo(70)); err != nil {
		t.Errorf("a pot-sized raise was refused: %v", err)
	}

	// A short stack under the cap may still go all in, for less than a full
	// raise, and the button says so.
	short := table(2, func(s *GameState) {
		s.Variation = VarOmaha
		s.Seats[0].Stack = 25
		s.Seats[1].Bet, s.Seats[1].Committed, s.Seats[1].Stack = 20, 20, 980
		s.CurrentBet = 20
		s.Pot = 40
		s.Current = 0
	})
	if _, _, err := m.Apply(short, "p1", raiseTo(25)); err != nil {
		t.Errorf("an all-in under the pot cap was refused: %v", err)
	}
	offers, _ = m.LegalActions(short, "p1")
	for _, o := range offers {
		if o.Verb == VerbRaise && o.Enabled {
			found := false
			for _, c := range o.Params[0].Choices {
				found = found || c.LabelKey == "holdem.quick.allIn"
			}
			if !found {
				t.Error("a stack under the pot cap is not offered all-in")
			}
		}
	}

	// No-limit Hold'em is untouched: the same spot goes to the whole stack.
	nl := table(4, func(s *GameState) {
		s.Seats[1].Bet, s.Seats[1].Committed, s.Seats[1].Stack = 10, 10, 990
		s.Seats[2].Bet, s.Seats[2].Committed, s.Seats[2].Stack = 20, 20, 980
		s.CurrentBet = 20
		s.Current = 3
	})
	if _, _, err := m.Apply(nl, "p4", raiseTo(1000)); err != nil {
		t.Errorf("Hold'em should still allow a shove: %v", err)
	}
}

func TestOmahaBotPlaysOnlyLegalMovesToTheEnd(t *testing.T) {
	m := New()
	for _, seats := range []int{2, 3, 6, 9} {
		players := seatIDs(seats)
		for seed := int64(1); seed <= 2; seed++ {
			state, err := m.NewMatch(module.MatchConfig{
				Variation: VarOmaha,
				Options:   module.Options{OptStartingStack: 400, OptBigBlind: 20, OptHandLimit: 6},
			}, players, seed)
			if err != nil {
				t.Fatal(err)
			}
			skills := []module.Skill{module.SkillEasy, module.SkillMedium, module.SkillHard}
			raises := 0
			final := playSkilled(t, state, players, func(i int) module.Skill { return skills[(i+int(seed))%3] }, 6000,
				func(a module.Action) {
					if a.Verb == VerbRaise {
						raises++
					}
				})
			total := 0
			for i := range final.Seats {
				total += final.Seats[i].Stack
			}
			if want := 400 * seats; total != want {
				t.Errorf("%d seats seed %d: %d chips, want %d", seats, seed, total, want)
			}
			if raises == 0 {
				t.Errorf("%d seats seed %d: nobody ever raised", seats, seed)
			}
		}
	}
}

func TestOmahaBotDoesNotPeek(t *testing.T) {
	for _, street := range []string{streetPreflop, streetFlop, streetRiver} {
		for _, skill := range []module.Skill{module.SkillEasy, module.SkillMedium, module.SkillHard} {
			build := func(theirs []string, reversed bool) module.State {
				return table(2, func(s *GameState) {
					s.Variation = VarOmaha
					s.Street = street
					s.Board = map[string][]string{
						streetPreflop: nil,
						streetFlop:    {"7H", "8D", "2C"},
						streetRiver:   {"7H", "8D", "2C", "KS", "3H"},
					}[street]
					s.Seats[0].Hole = []string{"AC", "AD", "9H", "TS"}
					s.Seats[1].Hole = theirs
					s.Seats[1].Bet, s.Seats[1].Acted, s.Seats[1].Committed = 60, true, 60
					s.Seats[0].Committed = 20
					s.CurrentBet, s.Pot = 60, 40
					s.Current = 0
					if reversed {
						for i, j := 0, len(s.Deck)-1; i < j; i, j = i+1, j-1 {
							s.Deck[i], s.Deck[j] = s.Deck[j], s.Deck[i]
						}
					}
				})
			}
			act := func(raw module.State) module.Action {
				offers, err := New().LegalActions(raw, "p1")
				if err != nil {
					t.Fatal(err)
				}
				a, ok := New().Bot().Act(raw, module.BotSeat{PlayerID: "p1", Skill: skill}, offers)
				if !ok {
					t.Fatalf("%s %s: no move", street, skill)
				}
				return a
			}
			a := act(build([]string{"5C", "6D", "JH", "QS"}, false))
			for _, rigged := range []module.State{
				build([]string{"KC", "KD", "8H", "8S"}, false),
				build([]string{"5C", "6D", "JH", "QS"}, true),
			} {
				if b := act(rigged); a.Verb != b.Verb || a.Params[ParamAmount] != b.Params[ParamAmount] {
					t.Errorf("%s %s: played %+v, and %+v when hidden cards changed", street, skill, a, b)
				}
			}
		}
	}
}

func TestOmahaRulesAreWrittenForOmaha(t *testing.T) {
	stated := func(cfg module.MatchConfig) map[string]bool {
		secs, err := New().Rules(cfg)
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]bool{}
		for _, sec := range secs {
			for _, it := range sec.Items {
				out[it.LabelKey] = true
			}
		}
		return out
	}
	omaha, holdem := stated(omahaCfg), stated(module.MatchConfig{Variation: VarHoldem})
	for _, key := range []string{"holdem.rules.omaha.deal", "holdem.rules.omaha.useTwo", "holdem.rules.potLimit", "holdem.rules.omaha.allIn"} {
		if !omaha[key] {
			t.Errorf("Omaha's rules leave out %s", key)
		}
		if holdem[key] {
			t.Errorf("Hold'em's rules state %s", key)
		}
	}
	for _, key := range []string{"holdem.rules.noLimit", "holdem.rules.allIn"} {
		if omaha[key] {
			t.Errorf("Omaha's rules state %s, which is false at a pot-limit table", key)
		}
	}
	if got := New().ExplainRefusal(omahaCfg, ErrOverPotLimit); len(got) == 0 || got[0] != "holdem.rules.potLimit" {
		t.Errorf("OVER_POT_LIMIT explains as %v", got)
	}
}

// TestOmahaBotLadderIsOrdered: each skill beats a calling station, and the
// ladder is in order, on fixed seeds with both seatings.
func TestOmahaBotLadderIsOrdered(t *testing.T) {
	if testing.Short() {
		t.Skip("a strength sweep is not a fast test")
	}
	m := New()
	station := module.OfferBot(VerbCall, VerbCheck, VerbRaise, VerbFold)
	type side struct {
		bot   module.Bot
		skill module.Skill
	}
	hard, medium, easy := side{m.Bot(), module.SkillHard}, side{m.Bot(), module.SkillMedium}, side{m.Bot(), module.SkillEasy}
	const seeds = 30
	net := func(a, b side) int {
		total := 0
		for seed := int64(1); seed <= seeds; seed++ {
			for _, ids := range [][]string{{"A", "B"}, {"B", "A"}} {
				players := refs(ids...)
				state, err := m.NewMatch(module.MatchConfig{Variation: VarOmaha, Options: module.Options{
					OptStartingStack: 1000, OptBigBlind: 20, OptHandLimit: 15,
				}}, players, seed)
				if err != nil {
					t.Fatal(err)
				}
				final := playSkilledBy(t, state, players, map[string]side{"A": a, "B": b}, 8000,
					func(s side) (module.Bot, module.Skill) { return s.bot, s.skill })
				total += final.seat("A").Stack - 1000
			}
		}
		return total
	}
	for _, c := range []struct {
		name string
		a, b side
	}{
		{"hard vs station", hard, side{station, ""}},
		{"medium vs station", medium, side{station, ""}},
		{"easy vs station", easy, side{station, ""}},
		{"medium vs easy", medium, easy},
		{"hard vs medium", hard, medium},
	} {
		got := net(c.a, c.b)
		t.Logf("%s: %+.1f big blinds a match", c.name, float64(got)/float64(2*seeds)/20)
		if got <= 0 {
			t.Errorf("%s: %+d chips over %d matches, want a profit", c.name, got, 2*seeds)
		}
	}
}
