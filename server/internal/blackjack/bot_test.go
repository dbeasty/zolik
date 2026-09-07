package blackjack

import (
	"testing"

	"zolik/server/internal/module"
)

// planFor is one decision, asked of the chart directly: this hand, that up
// card, at this skill.
func planFor(t *testing.T, skill module.Skill, cards []string, up string, mut func(*GameState)) []string {
	t.Helper()
	s := tableOf(module.MatchConfig{Variation: "vegas"})
	s.Dealer = []string{up, "XX"}
	if mut != nil {
		mut(s)
	}
	return plan(s, &Hand{Cards: cards, Bet: 10}, skill)
}

func first(ids []string) string {
	if len(ids) == 0 {
		return ""
	}
	return ids[0]
}

// TestBot_PlaysBasicStrategy pins the decisions basic strategy is famous for
// — the ones that separate it from what a person guesses.
func TestBot_PlaysBasicStrategy(t *testing.T) {
	for _, tc := range []struct {
		name  string
		cards []string
		up    string
		want  string
	}{
		{"sixteen against a ten is drawn to", []string{"TS", "6H"}, "TD", OfferHit},
		{"sixteen against a six is stood on", []string{"TS", "6H"}, "6D", OfferStand},
		{"twelve against a two is drawn to", []string{"TS", "2H"}, "2D", OfferHit},
		{"twelve against a four is stood on", []string{"TS", "2H"}, "4D", OfferStand},
		{"eleven is doubled", []string{"6S", "5H"}, "TD", OfferDouble},
		{"ten against a ten is not", []string{"6S", "4H"}, "TD", OfferHit},
		{"soft eighteen against a six is doubled", []string{"AS", "7H"}, "6D", OfferDouble},
		{"soft eighteen against a nine is drawn to", []string{"AS", "7H"}, "9D", OfferHit},
		{"soft nineteen is stood on", []string{"AS", "8H"}, "6D", OfferStand},
		{"eights are split against anything", []string{"8S", "8H"}, "TD", OfferSplit},
		{"aces are always split", []string{"AS", "AH"}, "AD", OfferSplit},
		{"tens are never split", []string{"TS", "KH"}, "6D", OfferStand},
		{"fives are a ten, not a pair", []string{"5S", "5H"}, "6D", OfferDouble},
		{"nines stand against a seven", []string{"9S", "9H"}, "7D", OfferStand},
		{"nines split against an eight", []string{"9S", "9H"}, "8D", OfferSplit},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := first(planFor(t, module.SkillHard, tc.cards, tc.up, nil))
			if got != tc.want {
				t.Errorf("%v against %s: chose %q, want %q", tc.cards, tc.up, got, tc.want)
			}
		})
	}
}

// TestBot_SurrendersOnlyTheTwoHandsWorthGivingUp, and only where the table
// offers it — at a table without surrender the same hands are simply played
// out.
func TestBot_SurrendersOnlyTheTwoHandsWorthGivingUp(t *testing.T) {
	late := func(s *GameState) { s.AllowSurrender = true }
	for _, tc := range []struct {
		name  string
		cards []string
		up    string
		want  string
	}{
		{"sixteen against a nine", []string{"TS", "6H"}, "9D", OfferSurrender},
		{"fifteen against a ten", []string{"TS", "5H"}, "TD", OfferSurrender},
		{"sixteen against a six is not given up", []string{"TS", "6H"}, "6D", OfferStand},
		{"fifteen against a nine is not given up", []string{"TS", "5H"}, "9D", OfferHit},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := first(planFor(t, module.SkillHard, tc.cards, tc.up, late)); got != tc.want {
				t.Errorf("%v against %s: chose %q, want %q", tc.cards, tc.up, got, tc.want)
			}
		})
	}

	if got := first(planFor(t, module.SkillHard, []string{"TS", "6H"}, "9D", nil)); got == OfferSurrender {
		t.Error("a table without surrender was offered one anyway")
	}
}

// TestBot_FallsBackWhenTheTableDoesNotAllowTheFirstChoice — "double, else
// hit" and "double, else stand" are different answers, and the difference is
// a real hand: a soft eighteen the table will not let you double is a stand,
// not a draw.
func TestBot_FallsBackWhenTheTableDoesNotAllowTheFirstChoice(t *testing.T) {
	soft18 := planFor(t, module.SkillHard, []string{"AS", "7H"}, "5D", nil)
	if len(soft18) < 2 || soft18[1] != OfferStand {
		t.Errorf("soft eighteen falls back to %v, want a stand behind the double", soft18)
	}
	eleven := planFor(t, module.SkillHard, []string{"6S", "5H"}, "5D", nil)
	if len(eleven) < 2 || eleven[1] != OfferHit {
		t.Errorf("eleven falls back to %v, want a hit behind the double", eleven)
	}
}

// TestBot_FoursOnlySplitWhereTheSplitHandsCanBeDoubled — a house rule
// changing a chart entry, which is the whole reason the rules are options.
func TestBot_FoursOnlySplitWhereTheSplitHandsCanBeDoubled(t *testing.T) {
	with := first(planFor(t, module.SkillHard, []string{"4S", "4H"}, "5D", func(s *GameState) {
		s.DoubleAfterSplit = true
	}))
	without := first(planFor(t, module.SkillHard, []string{"4S", "4H"}, "5D", func(s *GameState) {
		s.DoubleAfterSplit = false
	}))
	if with != OfferSplit {
		t.Errorf("with double-after-split on, fours chose %q", with)
	}
	if without == OfferSplit {
		t.Error("with double-after-split off, fours should not be split")
	}
}

// TestBot_TheWeakSettingsMakeTheMistakesTheyAreNamedFor. A weaker bot is not
// a bot that plays randomly; it is one that plays the way people actually get
// this game wrong.
func TestBot_TheWeakSettingsMakeTheMistakesTheyAreNamedFor(t *testing.T) {
	// Easy mimics the dealer: it stands on a sixteen only when the total says
	// so, never because of what the dealer is showing.
	if got := first(planFor(t, module.SkillEasy, []string{"TS", "6H"}, "6D", nil)); got != OfferHit {
		t.Errorf("easy stood on sixteen against a six: %q — that is the chart, not a mimic", got)
	}
	if got := first(planFor(t, module.SkillEasy, []string{"TS", "7H"}, "TD", nil)); got != OfferStand {
		t.Errorf("easy drew to seventeen: %q", got)
	}
	if got := first(planFor(t, module.SkillEasy, []string{"8S", "8H"}, "TD", nil)); got == OfferSplit {
		t.Error("easy split a pair; it is not supposed to know how")
	}

	// Medium plays the hit/stand chart and the two doubles, but no soft
	// doubling and no surrender.
	if got := first(planFor(t, module.SkillMedium, []string{"TS", "6H"}, "6D", nil)); got != OfferStand {
		t.Errorf("medium drew to sixteen against a six: %q", got)
	}
	if got := first(planFor(t, module.SkillMedium, []string{"AS", "7H"}, "6D", nil)); got == OfferDouble {
		t.Error("medium doubled a soft hand; that is the hard setting's move")
	}
	if got := first(planFor(t, module.SkillMedium, []string{"TS", "6H"}, "9D", nil)); got == OfferSurrender {
		t.Error("medium surrendered; that is the hard setting's move")
	}
}

// TestBot_OnlyTheBeginnerTakesInsurance — insurance is a 2:1 bet on something
// that comes in less than a third of the time, so declining it is not taste.
func TestBot_OnlyTheBeginnerTakesInsurance(t *testing.T) {
	raw := withTable(t, func(s *GameState) {
		s.Phase = phaseInsurance
		s.Peeked = false
		s.Current, s.CurrentHand = -1, -1
		s.Dealer = []string{"AS", "TD"}
	})
	m := New()
	offers, err := m.LegalActions(raw, "p1")
	if err != nil {
		t.Fatalf("LegalActions: %v", err)
	}

	for _, tc := range []struct {
		skill module.Skill
		want  string
	}{
		{module.SkillEasy, VerbInsure},
		{module.SkillMedium, VerbDecline},
		{module.SkillHard, VerbDecline},
	} {
		a, ok := bot{}.Act(raw, module.BotSeat{PlayerID: "p1", Skill: tc.skill}, offers)
		if !ok {
			t.Fatalf("%s had no answer to an insurance decision", tc.skill)
		}
		if a.Verb != tc.want {
			t.Errorf("%s answered insurance with %q, want %q", tc.skill, a.Verb, tc.want)
		}
	}
}

// TestBot_StakesWithinTheOffersOwnRange — the bet is the one number this game
// asks for, and a bot that named one outside the declared range would be a
// control the engine then refuses.
func TestBot_StakesWithinTheOffersOwnRange(t *testing.T) {
	m := New()
	raw := newMatch(t, 9, module.MatchConfig{Options: module.Options{OptStartingStack: 200, OptMinBet: 25}})
	offers, err := m.LegalActions(raw, "p1")
	if err != nil {
		t.Fatalf("LegalActions: %v", err)
	}
	for _, skill := range module.Skills {
		a, ok := bot{}.Act(raw, module.BotSeat{PlayerID: "p1", Skill: skill, Seed: 42}, offers)
		if !ok {
			t.Fatalf("%s would not stake anything", skill)
		}
		if _, _, err := m.Apply(raw, "p1", a); err != nil {
			t.Errorf("%s staked %v and the engine refused it: %v", skill, a.Params, err)
		}
	}
}

// TestBot_TheSkillsAreOrderedByWhatTheyWin is the honest version of a
// difficulty dial: the stronger setting has to actually beat the weaker one,
// measured, at the same table off the same shoe.
//
// Blackjack makes that unusually testable — the three seats share a dealer, so
// this is a paired comparison rather than three separate samples — and
// unusually necessary, because every one of these bots loses money in the
// long run. "Loses less" is the entire claim, and only a measurement supports
// it.
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

	for seed := int64(1); seed <= 120; seed++ {
		cfg := module.MatchConfig{
			Variation: "vegas",
			Options: module.Options{
				OptRounds: 20, OptStartingStack: 2000, OptMinBet: 5,
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
				Seed: module.SeatSeed(seed, actor, "blackjack"),
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

	t.Logf("chips after 120 matches: hard=%d medium=%d easy=%d", totals["p1"], totals["p2"], totals["p3"])
	if totals["p1"] <= totals["p3"] {
		t.Errorf("the hard setting finished on %d chips and the easy one on %d — the dial is not honest",
			totals["p1"], totals["p3"])
	}
	if totals["p2"] <= totals["p3"] {
		t.Errorf("the medium setting finished on %d chips and the easy one on %d",
			totals["p2"], totals["p3"])
	}
}
