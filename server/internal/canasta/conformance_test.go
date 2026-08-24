package canasta

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

// The order a UI shell would pick moves in: build the table, then take the
// pile if you can, then draw, and discard only because a turn has to end.
var driverPrefer = []string{VerbLayMeld, VerbLayOff, VerbTakePile, VerbDraw, VerbDiscard}

// TestOfferDrivenPlayFinishesAMatch is the strongest claim this module makes.
//
// module.PlayWithOffers may read the offer list and the ViewModel and nothing
// else — it never decodes State, never names a rank or a suit, and has never
// heard of a canasta, a red three or a frozen pile. That it can play whole
// Canasta matches *to a winner* is what closes the limitation
// extensibility-plan.md §4.x records for Žolíky, where going out needs a meld
// shape the offer protocol deliberately does not enumerate. Canasta's melds are
// enumerable, so the offers carry the shape and the driver needs no rules.
func TestOfferDrivenPlayFinishesAMatch(t *testing.T) {
	cases := []struct {
		name    string
		players []module.PlayerRef
		cfg     module.MatchConfig
	}{
		{
			name:    "two players, classic",
			players: refs("p1", "p2"),
			cfg: module.MatchConfig{
				Variation: "classic",
				Options:   module.Options{OptTargetScore: 500},
			},
		},
		{
			name:    "four players, partnerships",
			players: refs("p1", "p2", "p3", "p4"),
			cfg: module.MatchConfig{
				Variation: "classic",
				Options:   module.Options{OptTargetScore: 500},
			},
		},
		{
			name:    "modern american, two canastas to go out",
			players: refs("p1", "p2", "p3", "p4"),
			cfg: module.MatchConfig{
				Variation: "modern_american",
				Options:   module.Options{OptTargetScore: 500},
			},
		},
	}

	m := New()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for seed := int64(1); seed <= 12; seed++ {
				state, err := m.NewMatch(tc.cfg, tc.players, seed)
				if err != nil {
					t.Fatalf("seed %d: NewMatch: %v", seed, err)
				}
				final, res, err := module.PlayWithOffers(m, state, tc.players, module.DriverOptions{
					MaxActions: 4000,
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
					t.Fatalf("seed %d: finished with no winner", seed)
				}

				// A match, not a stalemate scored on hand penalties: somebody
				// has to have built canastas and gone out for this to be
				// Canasta rather than a shuffling exercise.
				if res.Verbs[VerbLayMeld] == 0 {
					t.Errorf("seed %d: nobody ever melded (verbs=%v)", seed, res.Verbs)
				}
				s, err := decode(final)
				if err != nil {
					t.Fatalf("seed %d: decode: %v", seed, err)
				}
				if s.Teams[s.WinnerTeam].Score < s.TargetScore {
					t.Errorf("seed %d: winner scored %d, below the %d target",
						seed, s.Teams[s.WinnerTeam].Score, s.TargetScore)
				}
			}
		})
	}
}

// TestDriverExercisesEveryVerb checks the offers are not merely sufficient but
// complete: a run that never took a pile would mean the capture offers were
// decorative.
func TestDriverExercisesEveryVerb(t *testing.T) {
	m := New()
	players := refs("p1", "p2", "p3", "p4")
	totals := map[string]int{}

	for seed := int64(1); seed <= 12; seed++ {
		state, err := m.NewMatch(module.MatchConfig{
			Options: module.Options{OptTargetScore: 500},
		}, players, seed)
		if err != nil {
			t.Fatalf("NewMatch: %v", err)
		}
		_, res, err := module.PlayWithOffers(m, state, players, module.DriverOptions{
			MaxActions: 4000, Prefer: driverPrefer,
		})
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		for v, n := range res.Verbs {
			totals[v] += n
		}
	}

	for _, verb := range []string{VerbDraw, VerbTakePile, VerbLayMeld, VerbLayOff, VerbDiscard} {
		if totals[verb] == 0 {
			t.Errorf("verb %q was never played across twelve matches — its offers are decorative", verb)
		}
	}
	t.Logf("verbs across twelve matches: %v", totals)
}

// TestModuleSatisfiesTheContract repeats the checks zolikmod's suite makes of
// every module, so a third game cannot quietly break a shared assumption.
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
				t.Errorf("variation %q defaults an option %q the descriptor does not declare", v.ID, opt)
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

	if _, err := m.NewMatch(module.MatchConfig{}, refs("only-one"), 1); err == nil {
		t.Error("a one-player canasta should be refused")
	}
	if _, err := m.NewMatch(module.MatchConfig{}, refs("a", "b", "c", "d", "e"), 1); err == nil {
		t.Error("a five-player canasta should be refused")
	}
}
