package sietemedio

import (
	"reflect"
	"testing"

	"zolik/server/internal/module"
)

// planFor is one decision, asked of the bot's own judgement directly: this
// hand, at this skill, in this seat.
func planFor(t *testing.T, skill module.Skill, cards []string, mut func(*GameState)) string {
	t.Helper()
	s := withTable(t, func(s *GameState) {
		s.Seats[1].Cards = cards
		if mut != nil {
			mut(s)
		}
	})
	return plan(s, &s.Seats[1], "p2", skill)
}

// TestBot_StandsWhereTheArithmeticTurns — a hand on 6 survives a draw two
// times in five, which is where drawing stops paying. The strong setting
// stands there; the weaker ones draw past it, which is the commonest way a
// person loses this game.
func TestBot_StandsWhereTheArithmeticTurns(t *testing.T) {
	for _, tc := range []struct {
		name  string
		cards []string
		hard  string
		med   string
		easy  string
	}{
		// 5½: everybody draws.
		{"five and a half", []string{"5D", "JH"}, OfferHit, OfferHit, OfferHit},
		// 6: the strong setting stops here.
		{"six", []string{"6D"}, OfferStand, OfferHit, OfferHit},
		// 6½: medium stops here too.
		{"six and a half", []string{"6D", "JH"}, OfferStand, OfferStand, OfferHit},
		// 7: only the beginner is still drawing, and it is a 70% bust.
		{"seven", []string{"7D"}, OfferStand, OfferStand, OfferStand},
		// A half: nobody stands on a figure.
		{"a half", []string{"JH"}, OfferHit, OfferHit, OfferHit},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := planFor(t, module.SkillHard, tc.cards, nil); got != tc.hard {
				t.Errorf("hard on %s chose %q, want %q", halvesText(Total(tc.cards)), got, tc.hard)
			}
			if got := planFor(t, module.SkillMedium, tc.cards, nil); got != tc.med {
				t.Errorf("medium on %s chose %q, want %q", halvesText(Total(tc.cards)), got, tc.med)
			}
			if got := planFor(t, module.SkillEasy, tc.cards, nil); got != tc.easy {
				t.Errorf("easy on %s chose %q, want %q", halvesText(Total(tc.cards)), got, tc.easy)
			}
		})
	}
}

// TestBot_TheStrongBankDrawsToWhatItHasToBeat — the bank plays last, so what
// it needs is on the table rather than a number picked in advance. That is the
// one thing the strong setting does that the other two do not.
func TestBot_TheStrongBankDrawsToWhatItHasToBeat(t *testing.T) {
	bankOn := func(skill module.Skill, bank []string, player []string) string {
		t.Helper()
		s := withTable(t, func(s *GameState) {
			s.Phase, s.Current = phaseBank, 0
			s.Seats[0].Cards = bank
			s.Seats[0].Shown = true
			s.Seats[1].Cards = player
			s.Seats[1].Stood, s.Seats[1].Shown = true, true
			s.Seats[2].Stake = 0
			s.Seats[2].Cards = nil
		})
		return plan(s, &s.Seats[0], "p1", skill)
	}

	// A player standing on 7½, all face up: the bank has to get there or lose.
	if got := bankOn(module.SkillHard, []string{"6D", "JH"}, []string{"7S", "QC"}); got != OfferHit {
		t.Errorf("the strong bank stood on 6½ against a visible 7½: %q", got)
	}
	// The same bank hand against a player showing a half: nothing to chase.
	if got := bankOn(module.SkillHard, []string{"6D", "JH"}, []string{"JS", "QC"}); got != OfferStand {
		t.Errorf("the strong bank drew on 6½ with only a 1 to beat: %q", got)
	}
	// Medium banks to a number it brought with it, whatever is on the table.
	if got := bankOn(module.SkillMedium, []string{"6D", "JH"}, []string{"7S", "QC"}); got != OfferStand {
		t.Errorf("medium read the table: %q — that is the strong setting's move", got)
	}
}

// TestBot_TheBankNeverStandsBelowTheFloor — standing on five and a half with
// the whole table still to be paid is giving the round away, however weak the
// hands opposite look.
func TestBot_TheBankNeverStandsBelowTheFloor(t *testing.T) {
	s := withTable(t, func(s *GameState) {
		s.Phase, s.Current = phaseBank, 0
		s.Seats[0].Cards = []string{"5D", "JH"} // 5½
		s.Seats[1].Cards = []string{"JS", "QC"} // 1, and face up
		s.Seats[1].Stood, s.Seats[1].Shown = true, true
		s.Seats[2].Stake = 0
		s.Seats[2].Cards = nil
	})
	if got := plan(s, &s.Seats[0], "p1", module.SkillHard); got != OfferHit {
		t.Errorf("the bank stood on 5½: %q", got)
	}
}

// TestBot_NeverDrawsOnSevenAndAHalf — the engine stands such a hand rather
// than offering it a draw, so this can only be reached by a state change. A
// bot that would draw on it is one change away from drawing itself out of a
// won round.
func TestBot_NeverDrawsOnSevenAndAHalf(t *testing.T) {
	for _, skill := range module.Skills {
		if got := planFor(t, skill, []string{"7D", "JH"}, nil); got != OfferStand {
			t.Errorf("%s drew on 7½: %q", skill, got)
		}
	}
}

// TestBot_StakesWithinTheOffersOwnRange — the stake is the one number this game
// asks for, and its top is another seat's stack. A bot that named a figure
// outside the declared range would be pressing a control the engine refuses.
func TestBot_StakesWithinTheOffersOwnRange(t *testing.T) {
	m := New()
	raw, err := m.NewMatch(module.MatchConfig{
		Options: module.Options{OptStartingStack: 200, OptMinStake: 25},
	}, table(), 9)
	if err != nil {
		t.Fatalf("NewMatch: %v", err)
	}
	s := stateOf(t, raw)
	// Whichever seat is not banking.
	actor := s.Seats[(s.Banker+1)%len(s.Seats)].PlayerID

	offers, err := m.LegalActions(raw, actor)
	if err != nil {
		t.Fatalf("LegalActions: %v", err)
	}
	for _, skill := range module.Skills {
		a, ok := bot{}.Act(raw, module.BotSeat{PlayerID: actor, Skill: skill, Seed: 42}, offers)
		if !ok {
			t.Fatalf("%s would not stake anything", skill)
		}
		if _, _, err := m.Apply(raw, actor, a); err != nil {
			t.Errorf("%s staked %v and the engine refused it: %v", skill, a.Params, err)
		}
	}
}

// TestBot_TheSkillsAreOrderedByWhatTheyWin is the honest version of a
// difficulty dial: the stronger setting has to actually beat the weaker one,
// measured, at the same table off the same shuffle.
//
// The Cantina table is used rather than the standard one because the bank goes
// round it: over twelve rounds each of three seats banks four times, so this is
// a paired comparison of three players at one table rather than a measurement
// of whoever happened to win the cut.
func TestBot_TheSkillsAreOrderedByWhatTheyWin(t *testing.T) {
	if testing.Short() {
		t.Skip("self-play measurement")
	}
	m := New()
	skills := map[string]module.Skill{
		"p1": module.SkillHard,
		"p2": module.SkillMedium,
		"p3": module.SkillEasy,
	}
	totals := map[string]int{}

	for seed := int64(1); seed <= 200; seed++ {
		cfg := module.MatchConfig{
			Variation: VarCantina,
			Options: module.Options{
				OptRounds: 12, OptStartingStack: 2000, OptMinStake: 5,
				module.OptPauseBetweenRounds: module.OptOff,
			},
		}
		state, err := m.NewMatch(cfg, table(), seed)
		if err != nil {
			t.Fatalf("NewMatch: %v", err)
		}
		for step := 0; step < 4000; step++ {
			done, _, err := m.Finished(state)
			if err != nil || done {
				break
			}
			awaited := module.AwaitedSeats(m, state, "p1", table())
			if len(awaited) == 0 {
				break
			}
			actor := awaited[0]
			offers, err := m.LegalActions(state, actor)
			if err != nil {
				t.Fatalf("LegalActions: %v", err)
			}
			a, ok := bot{}.Act(state, module.BotSeat{
				PlayerID: actor, Skill: skills[actor],
				Seed: module.SeatSeed(seed, actor, "sietemedio"),
			}, offers)
			if !ok {
				t.Fatalf("%s had no move", actor)
			}
			next, _, err := m.Apply(state, actor, a)
			if err != nil {
				t.Fatalf("%s was refused %+v: %v", actor, a, err)
			}
			state = next
		}
		s := stateOf(t, state)
		for i := range s.Seats {
			totals[s.Seats[i].PlayerID] += s.Seats[i].Stack
		}
	}

	t.Logf("chips after 200 matches: hard=%d medium=%d easy=%d", totals["p1"], totals["p2"], totals["p3"])
	if totals["p1"] <= totals["p3"] {
		t.Errorf("the hard setting finished on %d chips and the easy one on %d — the dial is not honest",
			totals["p1"], totals["p3"])
	}
	if totals["p2"] <= totals["p3"] {
		t.Errorf("the medium setting finished on %d chips and the easy one on %d",
			totals["p2"], totals["p3"])
	}
	if totals["p1"] <= totals["p2"] {
		t.Errorf("the hard setting finished on %d chips and the medium one on %d",
			totals["p1"], totals["p2"])
	}
}

// TestBotDoesNotPeek.
//
// The bot is handed the whole state — every seat's face-down card and the pack
// in the order it will deal — because that is what the runtime has. Nothing
// but this test stops it reading them, and the hint button shows a person
// whatever the bot would do, so a bot that peeked would leak the table's cards
// to them too.
//
// Each position is built with every combination of the other seats' hidden
// cards and pack order; the decision has to come out the same across all of
// them, at every skill. The offers are asked for per variant, the way the
// runtime asks, so a leak through the offer list would fail here as well.
func TestBotDoesNotPeek(t *testing.T) {
	// Same length, entirely different cards: a bot that knew the next card was
	// a figure would draw on anything, and a bot that knew the seats opposite
	// were sitting on sevens would chase them.
	packs := [][]string{
		pack("7C", "7H", "7S", "6C", "6D", "6H", "5S", "5C"),
		pack("JC", "QH", "KS", "JD", "QD", "KH", "JS", "QC"),
	}
	// What the two seats opposite might be holding face down.
	hidden := [][]string{
		{"7H", "7S"},
		{"JH", "QS"},
	}

	type position struct {
		name   string
		phase  string
		banker int
		hand   []string // the seat under test; nil in the staking phase
	}
	var positions []position
	for _, hand := range [][]string{
		{"5D"},       // five: everybody draws
		{"6D"},       // six: where the strong setting stops
		{"6D", "JH"}, // six and a half
		{"4H", "2C"}, // six, made of two cards
	} {
		positions = append(positions,
			position{"a player's hand", phasePlay, 0, hand},
			position{"the bank's hand", phaseBank, 1, hand},
		)
	}
	positions = append(positions, position{name: "staking", phase: phaseStakes, banker: 0})

	build := func(p position, secret []string, deck []string) *GameState {
		return withTable(t, func(s *GameState) {
			s.Phase = p.phase
			s.Banker = p.banker
			s.Deck = append([]string(nil), deck...)
			switch p.phase {
			case phaseStakes:
				s.Current = -1
				for i := range s.Seats {
					s.Seats[i].Cards = nil
					s.Seats[i].Stake = 0
				}
			default:
				// The seat under test is p2; the other two hold the cards
				// being varied, face down and stood.
				s.Current = 1
				s.Seats[1].Cards = p.hand
				s.Seats[0].Cards = []string{secret[0]}
				s.Seats[2].Cards = []string{secret[1]}
				s.Seats[0].Stood, s.Seats[2].Stood = true, true
				if p.phase == phaseBank {
					// p2 banks: it may see its own hand and nobody else's.
					s.Banker = 1
					s.Seats[1].Shown = true
					s.Seats[0].Stake, s.Seats[2].Stake = 10, 10
				}
			}
		})
	}

	m := New()
	act := func(s *GameState, skill module.Skill) module.Action {
		t.Helper()
		raw := rawOf(t, s)
		offers, err := m.LegalActions(raw, "p2")
		if err != nil {
			t.Fatalf("LegalActions: %v", err)
		}
		a, ok := bot{}.Act(raw, module.BotSeat{PlayerID: "p2", Skill: skill, Seed: 7}, offers)
		if !ok {
			t.Fatalf("%s had no move", skill)
		}
		return a
	}

	for _, p := range positions {
		for _, skill := range module.Skills {
			var want module.Action
			for i, secret := range hidden {
				for j, deck := range packs {
					got := act(build(p, secret, deck), skill)
					if i == 0 && j == 0 {
						want = got
						continue
					}
					if !reflect.DeepEqual(got, want) {
						t.Errorf("%s, %v, %s: played %+v with hidden %v and pack %d, but %+v with hidden %v and pack 0 — it is reading cards it cannot see",
							p.name, p.hand, skill, got, secret, j, want, hidden[0])
					}
				}
			}
		}
	}
}
