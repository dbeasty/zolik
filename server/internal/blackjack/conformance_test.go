package blackjack

import (
	"testing"

	"zolik/server/internal/module"
)

// The falsification test for this module: a driver that knows nothing about
// blackjack beyond its offer list must be able to play whole matches to a
// named winner.
//
// It is worth more here than in most modules, because this is the first game
// whose opponent is not a seat. If the offers are sufficient, then a client
// never has to know that the dealer exists as anything other than a zone with
// cards in it — which is the claim the whole module protocol makes.

// Three tastes rather than one. A driver that always stands never doubles or
// splits, and a game whose optional moves are never exercised has not been
// driven at all.
var driverStyles = map[string][]string{
	"cautious":  {"bet", "decline_insurance", "stand", "hit"},
	"reckless":  {"bet", "decline_insurance", "hit", "stand"},
	"splitting": {"bet", "decline_insurance", "split", "double", "stand", "hit"},
}

func table() []module.PlayerRef { return seats("p1", "p2", "p3") }

func TestConformance_PlaysWholeMatchesToAWinner(t *testing.T) {
	m := New()
	for _, variation := range []string{"vegas", "atlantic", "single"} {
		for style, prefer := range driverStyles {
			t.Run(variation+"/"+style, func(t *testing.T) {
				for seed := int64(1); seed <= 12; seed++ {
					cfg := module.MatchConfig{
						Variation: variation,
						Options:   module.Options{OptRounds: 10},
					}
					state, err := m.NewMatch(cfg, table(), seed)
					if err != nil {
						t.Fatalf("seed %d: NewMatch: %v", seed, err)
					}
					final, res, err := module.PlayWithOffers(m, state, table(), module.DriverOptions{
						MaxActions: 8000, Prefer: prefer,
					})
					if err != nil {
						t.Fatalf("seed %d: %v", seed, err)
					}
					if !res.Finished {
						t.Fatalf("seed %d: did not finish in %d actions", seed, res.Actions)
					}
					if len(res.Winners) == 0 {
						t.Fatalf("seed %d: finished naming nobody", seed)
					}
					if res.Verbs[VerbBet] == 0 {
						t.Errorf("seed %d: nobody ever staked anything", seed)
					}
					done, winners, err := m.Finished(final)
					if err != nil || !done || len(winners) == 0 {
						t.Errorf("seed %d: Finished disagrees with the driver: done=%v winners=%v err=%v",
							seed, done, winners, err)
					}
				}
			})
		}
	}
}

// TestConformance_TheOptionalMovesAreReachableFromTheOffersAlone — hit and
// stand would play this game on their own, which is exactly why they are not
// enough to prove anything. Every optional move has to be reachable by a
// driver that reads the offers and nothing else.
//
// One preference order cannot show that, and the reason is itself worth
// recording: doubling and surrendering are offered on the same decision, so an
// order that prefers one starves the other. So this sweeps several tastes and
// asks that between them every verb gets played.
func TestConformance_TheOptionalMovesAreReachableFromTheOffersAlone(t *testing.T) {
	m := New()
	orders := [][]string{
		{"bet", "insure", "split", "stand", "hit"},
		{"bet", "decline_insurance", "double", "stand", "hit"},
		{"bet", "decline_insurance", "surrender", "stand", "hit"},
		{"bet", "decline_insurance", "hit", "stand"},
	}

	verbs := map[string]int{}
	for _, prefer := range orders {
		for seed := int64(1); seed <= 12; seed++ {
			cfg := module.MatchConfig{Variation: "atlantic", Options: module.Options{OptRounds: 20}}
			state, err := m.NewMatch(cfg, table(), seed)
			if err != nil {
				t.Fatalf("seed %d: NewMatch: %v", seed, err)
			}
			_, res, err := module.PlayWithOffers(m, state, table(), module.DriverOptions{
				MaxActions: 8000, Prefer: prefer,
			})
			if err != nil {
				t.Fatalf("seed %d: %v", seed, err)
			}
			for v, n := range res.Verbs {
				verbs[v] += n
			}
		}
	}
	for _, verb := range []string{
		VerbBet, VerbHit, VerbStand, VerbDouble, VerbSplit, VerbSurrender,
		VerbInsure, VerbDecline, module.VerbContinue,
	} {
		if verbs[verb] == 0 {
			t.Errorf("no driver ever played %q from the offer list alone", verb)
		}
	}
}

// TestConformance_ChipsAreConserved — every chip a seat loses is a chip the
// house took, and every chip it wins came from there. Nothing else creates or
// destroys one, so a stack that has drifted is an arithmetic bug in the
// settlement, which is precisely the sort of thing a scoreboard will not show
// you.
func TestConformance_ChipsAreConserved(t *testing.T) {
	m := New()
	for seed := int64(1); seed <= 10; seed++ {
		cfg := module.MatchConfig{Variation: "vegas", Options: module.Options{OptRounds: 10}}
		state, err := m.NewMatch(cfg, table(), seed)
		if err != nil {
			t.Fatalf("NewMatch: %v", err)
		}
		final, _, err := module.PlayWithOffers(m, state, table(), module.DriverOptions{
			MaxActions: 8000, Prefer: driverStyles["splitting"],
		})
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}

		s := stateOf(t, final)
		for i := range s.Seats {
			pid := s.Seats[i].PlayerID
			running := s.StartingStack
			for _, r := range s.Rounds {
				running += r.Deltas[pid]
				if got := r.Stacks[pid]; got != running {
					t.Fatalf("seed %d: %s's round %d says %d chips, the deltas say %d",
						seed, pid, r.Number, got, running)
				}
			}
			if s.Seats[i].Stack != running {
				t.Errorf("seed %d: %s finished on %d chips, the round table says %d",
					seed, pid, s.Seats[i].Stack, running)
			}
		}
	}
}

// TestConformance_ASeatIsNeverLeftWithNothingToDo — the dead-position rule.
// A live match always has somebody it is waiting on, and every seat it is
// waiting on has something it can actually press.
func TestConformance_ASeatIsNeverLeftWithNothingToDo(t *testing.T) {
	m := New()
	for seed := int64(1); seed <= 8; seed++ {
		cfg := module.MatchConfig{Variation: "atlantic", Options: module.Options{OptRounds: 10}}
		state, err := m.NewMatch(cfg, table(), seed)
		if err != nil {
			t.Fatalf("NewMatch: %v", err)
		}
		for step := 0; step < 4000; step++ {
			done, _, err := m.Finished(state)
			if err != nil {
				t.Fatalf("Finished: %v", err)
			}
			if done {
				break
			}
			awaited := module.AwaitedSeats(m, state, "p1", table())
			if len(awaited) == 0 {
				t.Fatalf("seed %d step %d: a live match is waiting on nobody", seed, step)
			}
			for _, id := range awaited {
				offers, err := m.LegalActions(state, id)
				if err != nil {
					t.Fatalf("LegalActions: %v", err)
				}
				if _, ok := module.ChooseAction(offers, nil); !ok {
					t.Fatalf("seed %d step %d: %s is waited on with nothing to press:\n%s",
						seed, step, id, module.DescribeOffers(offers))
				}
			}
			offers, _ := m.LegalActions(state, awaited[0])
			a, _ := module.ChooseAction(offers, driverStyles["splitting"])
			next, _, err := m.Apply(state, awaited[0], a)
			if err != nil {
				t.Fatalf("seed %d step %d: offered %+v but refused: %v", seed, step, a, err)
			}
			state = next
		}
	}
}
