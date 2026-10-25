package sietemedio

import (
	"testing"

	"zolik/server/internal/module"
)

// The falsification test for this module: a driver that knows nothing about
// Siete y Medio beyond its offer list must be able to play whole matches to a
// named winner.
//
// It is worth more here than in most modules, because of how much of the game
// is in state a driver is never told about in so many words: the bank moves
// during the match, every stake is answered out of another player's stack, and
// a seat the bank cannot cover is dropped from the round. If the offers are
// sufficient, then a client never has to know any of it, which is the claim
// the whole module protocol makes. (Oko bere banks round a table too, and its
// own suite makes the same demand of it.)

// Two tastes rather than one. A driver that always stands never goes over, and
// a game whose bust rule is never exercised has not been driven at all.
var driverStyles = map[string][]string{
	"cautious": {"stake", "stand", "hit"},
	"reckless": {"stake", "hit", "stand"},
}

func table() []module.PlayerRef { return seats("p1", "p2", "p3") }

func TestConformance_PlaysWholeMatchesToAWinner(t *testing.T) {
	m := New()
	for _, variation := range []string{VarMexicana, VarCantina, VarAbierta} {
		for style, prefer := range driverStyles {
			t.Run(variation+"/"+style, func(t *testing.T) {
				for seed := int64(1); seed <= 12; seed++ {
					cfg := module.MatchConfig{
						Variation: variation,
						Options:   module.Options{OptRounds: 12},
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
					if res.Verbs[VerbStake] == 0 {
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

// TestConformance_EveryVerbIsReachableFromTheOffersAlone — three verbs and an
// intermission, and a driver reading the offer list has to be able to find all
// of them.
func TestConformance_EveryVerbIsReachableFromTheOffersAlone(t *testing.T) {
	m := New()
	verbs := map[string]int{}
	for _, prefer := range [][]string{
		{"stake", "hit", "stand"},
		{"stake", "stand", "hit"},
	} {
		for seed := int64(1); seed <= 12; seed++ {
			cfg := module.MatchConfig{Variation: VarMexicana, Options: module.Options{OptRounds: 12}}
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
	for _, verb := range []string{VerbStake, VerbHit, VerbStand, module.VerbContinue} {
		if verbs[verb] == 0 {
			t.Errorf("no driver ever played %q from the offer list alone", verb)
		}
	}
}

// TestConformance_TheRoundTableAgreesWithTheStacks — the round log is what a
// player reads when they want to know where their chips went, and a drifted
// one is the sort of arithmetic bug a scoreboard will never show you.
func TestConformance_TheRoundTableAgreesWithTheStacks(t *testing.T) {
	m := New()
	for seed := int64(1); seed <= 12; seed++ {
		cfg := module.MatchConfig{Variation: VarMexicana, Options: module.Options{OptRounds: 12}}
		state, err := m.NewMatch(cfg, table(), seed)
		if err != nil {
			t.Fatalf("NewMatch: %v", err)
		}
		final, _, err := module.PlayWithOffers(m, state, table(), module.DriverOptions{
			MaxActions: 8000, Prefer: driverStyles["reckless"],
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

// TestConformance_EveryRoundSumsToZero — the bank is a seat, so a round is a
// transfer rather than a payout. The whole table's deltas have to cancel.
func TestConformance_EveryRoundSumsToZero(t *testing.T) {
	m := New()
	for seed := int64(1); seed <= 12; seed++ {
		cfg := module.MatchConfig{Variation: VarAbierta, Options: module.Options{OptRounds: 12}}
		state, err := m.NewMatch(cfg, table(), seed)
		if err != nil {
			t.Fatalf("NewMatch: %v", err)
		}
		final, _, err := module.PlayWithOffers(m, state, table(), module.DriverOptions{
			MaxActions: 8000, Prefer: driverStyles["cautious"],
		})
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		s := stateOf(t, final)
		for _, r := range s.Rounds {
			sum := 0
			for _, d := range r.Deltas {
				sum += d
			}
			if sum != 0 {
				t.Errorf("seed %d round %d: the deltas sum to %d, not zero", seed, r.Number, sum)
			}
		}
	}
}

// TestConformance_ASeatIsNeverLeftWithNothingToDo — the dead-position rule. A
// live match always has somebody it is waiting on, and every seat it is waiting
// on has something it can actually press.
//
// The rule earns its keep twice over in this game: a hand on 7½ has no card it
// could draw, and a seat the bank cannot cover has no stake it could make.
func TestConformance_ASeatIsNeverLeftWithNothingToDo(t *testing.T) {
	m := New()
	for seed := int64(1); seed <= 10; seed++ {
		cfg := module.MatchConfig{
			Variation: VarMexicana,
			// A short stack and a big minimum, so the bank runs out of cover
			// often rather than never.
			Options: module.Options{OptRounds: 12, OptStartingStack: 200, OptMinStake: 25},
		}
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
			a, _ := module.ChooseAction(offers, driverStyles["reckless"])
			next, _, err := m.Apply(state, awaited[0], a)
			if err != nil {
				t.Fatalf("seed %d step %d: offered %+v but refused: %v", seed, step, a, err)
			}
			state = next
		}
	}
}
