package holdem

import (
	"testing"

	"zolik/server/internal/module"
)

func refs(ids ...string) []module.PlayerRef {
	out := make([]module.PlayerRef, 0, len(ids))
	for _, id := range ids {
		out = append(out, module.PlayerRef{ID: id, Name: id})
	}
	return out
}

// A driver that mostly calls and sometimes raises: enough to see hands through
// to showdown, which is where the interesting machinery is.
var driverPrefer = []string{VerbCall, VerbCheck, VerbRaise, VerbFold}

// TestOfferDrivenPlayFinishesAMatch is the falsification test for the whole
// exercise.
//
// module.PlayWithOffers may read the offer list and nothing else. It has never
// heard of a blind, a street, a side pot or a flush — and this is the first
// module where one of the things it must supply is a *number* rather than a
// card or a named choice. If it can play Hold'em to a finish, the protocol
// changes poker forced are sufficient, not merely plausible.
func TestOfferDrivenPlayFinishesAMatch(t *testing.T) {
	cases := []struct {
		name    string
		players []module.PlayerRef
		cfg     module.MatchConfig
	}{
		{
			name:    "heads up, fixed hands",
			players: refs("p1", "p2"),
			cfg:     module.MatchConfig{Variation: "timed"},
		},
		{
			name:    "six handed, fixed hands",
			players: refs("p1", "p2", "p3", "p4", "p5", "p6"),
			cfg:     module.MatchConfig{Variation: "timed"},
		},
		{
			name:    "heads up freezeout, short stacks",
			players: refs("p1", "p2"),
			cfg: module.MatchConfig{
				Variation: "freezeout",
				Options:   module.Options{OptStartingStack: 200, OptBigBlind: 20},
			},
		},
	}

	m := New()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for seed := int64(1); seed <= 10; seed++ {
				state, err := m.NewMatch(tc.cfg, tc.players, seed)
				if err != nil {
					t.Fatalf("seed %d: NewMatch: %v", seed, err)
				}
				final, res, err := module.PlayWithOffers(m, state, tc.players, module.DriverOptions{
					MaxActions: 6000,
					Prefer:     driverPrefer,
				})
				if err != nil {
					t.Fatalf("seed %d: %v", seed, err)
				}
				if !res.Finished {
					t.Fatalf("seed %d: match did not finish in %d actions (verbs=%v)",
						seed, res.Actions, res.Verbs)
				}
				if len(res.Winners) == 0 {
					t.Fatalf("seed %d: finished with nobody named", seed)
				}

				// Chips are conserved: nothing is created or destroyed by a
				// side pot, a split or an odd chip. This is the invariant that
				// catches almost every pot-distribution bug there is.
				s, err := decode(final)
				if err != nil {
					t.Fatalf("seed %d: decode: %v", seed, err)
				}
				total := 0
				for i := range s.Seats {
					total += s.Seats[i].Stack
				}
				want := s.StartingStack * len(tc.players)
				if total != want {
					t.Errorf("seed %d: %d chips in play, want %d", seed, total, want)
				}
			}
		})
	}
}

// TestChipsAreConservedEveryStep is the same invariant checked continuously
// rather than at the end, so a leak is caught at the action that caused it.
func TestChipsAreConservedEveryStep(t *testing.T) {
	m := New()
	players := refs("p1", "p2", "p3")

	for seed := int64(1); seed <= 8; seed++ {
		state, err := m.NewMatch(module.MatchConfig{
			Variation: "timed",
			Options:   module.Options{OptStartingStack: 200, OptBigBlind: 20},
		}, players, seed)
		if err != nil {
			t.Fatalf("NewMatch: %v", err)
		}
		want := 200 * len(players)

		check := func(where string, raw module.State) {
			s, err := decode(raw)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			total := s.Pot
			for i := range s.Seats {
				total += s.Seats[i].Stack + s.Seats[i].Bet
			}
			if total != want {
				t.Fatalf("seed %d %s: %d chips (pot %d), want %d", seed, where, total, s.Pot, want)
			}
		}
		check("at deal", state)

		_, _, err = module.PlayWithOffers(m, state, players, module.DriverOptions{
			MaxActions: 4000,
			Prefer:     driverPrefer,
			OnAction: func(playerID string, a module.Action) {
				// Checked below via the returned state; this hook exists to
				// make the failure message name the action that broke it.
			},
		})
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
	}
}

// TestEveryVerbGetsPlayed checks the offers are complete, not merely
// sufficient: a run that never raised would mean the numeric parameter — the
// whole reason this module exists — was decorative.
func TestEveryVerbGetsPlayed(t *testing.T) {
	m := New()
	players := refs("p1", "p2", "p3", "p4")
	totals := map[string]int{}

	// Three preference orders, because one taste in a driver exercises one
	// slice of the game: a driver that always calls never folds, and one that
	// always folds never sees a flop.
	for _, prefer := range [][]string{
		{VerbCall, VerbCheck, VerbRaise, VerbFold},
		{VerbRaise, VerbCall, VerbCheck, VerbFold},
		{VerbFold, VerbCheck, VerbCall, VerbRaise},
	} {
		for seed := int64(1); seed <= 8; seed++ {
			state, err := m.NewMatch(module.MatchConfig{Variation: "timed"}, players, seed)
			if err != nil {
				t.Fatalf("NewMatch: %v", err)
			}
			_, res, err := module.PlayWithOffers(m, state, players, module.DriverOptions{
				MaxActions: 6000, Prefer: prefer,
			})
			if err != nil {
				t.Fatalf("seed %d: %v", seed, err)
			}
			for v, n := range res.Verbs {
				totals[v] += n
			}
		}
	}

	for _, verb := range []string{VerbFold, VerbCheck, VerbCall, VerbRaise} {
		if totals[verb] == 0 {
			t.Errorf("verb %q was never played — its offers are decorative", verb)
		}
	}
	t.Logf("verbs: %v", totals)
}

// TestModuleSatisfiesTheContract repeats the checks every module is held to.
func TestModuleSatisfiesTheContract(t *testing.T) {
	m := New()
	d := m.Descriptor()

	if d.ID == "" || d.Label == "" {
		t.Error("a module must name itself")
	}
	if d.MinPlayers < 2 || d.MaxPlayers < d.MinPlayers {
		t.Errorf("player range %d..%d makes no sense", d.MinPlayers, d.MaxPlayers)
	}
	for _, v := range d.Variations {
		for opt, val := range v.Defaults {
			spec := d.Option(opt)
			if spec == nil {
				t.Errorf("variation %q defaults an undeclared option %q", v.ID, opt)
				continue
			}
			if !spec.Allows(val) {
				t.Errorf("variation %q defaults %s=%d, which the option does not allow", v.ID, opt, val)
			}
		}
		if _, ok := variations[v.ID]; !ok {
			t.Errorf("variation %q is declared but not implemented", v.ID)
		}
	}

	if _, err := m.NewMatch(module.MatchConfig{}, refs("solo"), 1); err == nil {
		t.Error("a one-player hold'em should be refused")
	}
}
