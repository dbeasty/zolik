package pinochle

import (
	"testing"

	"zolik/server/internal/lessontest"
	"zolik/server/internal/module"
)

// The falsification test: a driver that knows nothing about Pinochle beyond
// its offer list must play whole matches of every variation to a named
// winner. Two tastes, so the auction is both won and left to stick the
// dealer, and the two-handed meld is both laid and passed over.
var driverStyles = map[string][]string{
	"passive": {VerbPass, VerbTrump, VerbBury, VerbGive, VerbDraw, VerbPlay},
	"melder":  {VerbMeld, VerbTrump, VerbBury, VerbGive, VerbPass, VerbPlay},
}

func TestConformance_PlaysWholeMatchesToAWinner(t *testing.T) {
	m := New()
	for _, v := range allVariations {
		for style, prefer := range driverStyles {
			t.Run(v+"/"+style, func(t *testing.T) {
				for seed := int64(1); seed <= 4; seed++ {
					players := refs(seatsOf(v))
					state, err := m.NewMatch(cfgOf(v, module.Options{OptPass: 3}), players, seed)
					if err != nil {
						t.Fatal(err)
					}
					final, res, err := module.PlayWithOffers(m, state, players, module.DriverOptions{MaxActions: 60000, Prefer: prefer})
					if err != nil {
						t.Fatalf("seed %d: %v", seed, err)
					}
					if !res.Finished || len(res.Winners) == 0 {
						t.Fatalf("seed %d: finished=%v winners=%v after %d", seed, res.Finished, res.Winners, res.Actions)
					}
					done, winners, err := m.Finished(final)
					if err != nil || !done || len(winners) == 0 {
						t.Errorf("seed %d: Finished disagrees: %v %v %v", seed, done, winners, err)
					}
				}
			})
		}
	}
}

// Every verb is reachable from the offers alone.
func TestConformance_EveryVerbIsReachable(t *testing.T) {
	m := New()
	verbs := map[string]int{}
	for _, v := range allVariations {
		for _, prefer := range driverStyles {
			for seed := int64(1); seed <= 3; seed++ {
				players := refs(seatsOf(v))
				state, _ := m.NewMatch(cfgOf(v, module.Options{OptPass: 4}), players, seed)
				_, res, err := module.PlayWithOffers(m, state, players, module.DriverOptions{MaxActions: 60000, Prefer: prefer})
				if err != nil {
					t.Fatal(err)
				}
				for verb, n := range res.Verbs {
					verbs[verb] += n
				}
			}
		}
	}
	// A bidder who always concedes: the throw-in is a real move too. Such a
	// table never reaches its target, so it plays a few hands and stops.
	{
		players := refs(4)
		state, _ := m.NewMatch(cfgOf(VarPartnership, nil), players, 1)
		_, res, err := module.PlayWithOffers(m, state, players, module.DriverOptions{
			MaxActions: 400, Prefer: []string{VerbThrowIn, VerbPass, VerbTrump, VerbPlay},
		})
		if err != nil {
			t.Fatal(err)
		}
		for verb, n := range res.Verbs {
			verbs[verb] += n
		}
	}
	// A bot, too, so the auction is won by a bid and a bidder throws in.
	for _, v := range allVariations {
		for seed := int64(1); seed <= 3; seed++ {
			playMatchVerbs(t, v, seed, verbs)
		}
	}
	for _, verb := range []string{VerbBid, VerbPass, VerbTrump, VerbBury, VerbGive, VerbPlay, VerbMeld, VerbDraw, VerbThrowIn} {
		if verbs[verb] == 0 {
			t.Errorf("verb %s was never played", verb)
		}
	}
}

func playMatchVerbs(t *testing.T, v string, seed int64, verbs map[string]int) {
	t.Helper()
	m := New()
	players := refs(seatsOf(v))
	state, _ := m.NewMatch(cfgOf(v, module.Options{OptPass: 3}), players, seed)
	for step := 0; step < 20000; step++ {
		s, _ := decode(state)
		if s.Status != "active" {
			return
		}
		offers, _ := m.LegalActions(state, s.Current)
		a, ok := m.Bot().Act(state, module.BotSeat{PlayerID: s.Current, Skill: module.SkillMedium, Seed: seed}, offers)
		if !ok {
			t.Fatalf("%s seed %d: no move for %s", v, seed, s.Current)
		}
		next, _, err := m.Apply(state, s.Current, a)
		if err != nil {
			t.Fatalf("%s seed %d: %s's %s refused: %v", v, seed, s.Current, a.Verb, err)
		}
		verbs[a.Verb]++
		state = next
	}
}

// No turn ends in a dead position: whoever is on always has an enabled
// offer.
func TestNoDeadPositions(t *testing.T) {
	m := New()
	for _, v := range allVariations {
		players := refs(seatsOf(v))
		for seed := int64(1); seed <= 3; seed++ {
			state, _ := m.NewMatch(cfgOf(v, module.Options{OptPass: 4}), players, seed)
			_, _, err := module.PlayWithOffers(m, state, players, module.DriverOptions{
				MaxActions: 60000, Prefer: driverStyles["melder"],
				OnState: func(st module.State) {
					s, _ := decode(st)
					if s.Status != "active" || s.Intermission.Open {
						return
					}
					offers, _ := m.LegalActions(st, s.Current)
					for _, o := range offers {
						if o.Enabled {
							return
						}
					}
					t.Fatalf("%s seed %d: %s has nothing to do in %s", v, seed, s.Current, s.Phase)
				},
			})
			if err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestRuleIndex(t *testing.T) {
	problems, notes := module.RuleIndexCheck{
		Module: New(),
		Dirs:   []string{"."},
		Exempt: map[string]string{
			ErrCardNotInHand: "not a rule — the client and server disagree about the hand",
			ErrGameNotActive: "not a rule — the game is over or has not started",
			ErrUnknownAction: "not a rule — a verb this module does not have",
			ErrWrongPlayers:  "not a rule of play — the table could not be seated",
			ErrNotANumber:    "not a rule — the client sent something that is not a figure",
			ErrWrongPhase:    "not a rule — a move for another stage of the hand",
		},
	}.Run()
	for _, n := range notes {
		t.Log(n)
	}
	for _, p := range problems {
		t.Error(p)
	}
}

func TestLessonsWalk(t *testing.T) { lessontest.Check(t, New()) }

func TestRulesFollowTheTable(t *testing.T) {
	ids := func(cfg module.MatchConfig) map[string]bool {
		secs, err := New().Rules(cfg)
		if err != nil {
			t.Fatal(err)
		}
		return module.RuleIDsIn(secs)
	}
	two := ids(module.MatchConfig{Variation: VarTwoHanded})
	if two["pinochle.rules.auction"] || !two["pinochle.rules.stockFree"] {
		t.Error("two-handed rules talk about an auction, or not about the stock")
	}
	cut := ids(module.MatchConfig{Variation: VarCutthroat})
	if !cut["pinochle.rules.bury"] || cut["pinochle.rules.partners"] {
		t.Error("cutthroat rules miss the bury, or mention partners")
	}
	dd := ids(module.MatchConfig{Variation: VarDoubleDeck})
	if dd["pinochle.rules.meld.dix"] || !dd["pinochle.rules.meld.quadruples"] {
		t.Error("double-deck rules have a dix, or no quadruple melds")
	}
	if !ids(module.MatchConfig{Options: module.Options{OptScoring: ScoringClassic}})["pinochle.rules.points.classic"] {
		t.Error("classic scoring not stated")
	}
	if !ids(module.MatchConfig{Options: module.Options{OptPass: 4}})["pinochle.rules.pass"] {
		t.Error("the pass not stated")
	}
}
