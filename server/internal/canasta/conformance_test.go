package canasta

import (
	"testing"

	"zolik/server/internal/module"
)

// classicRules is the ruleset the unit tests below reason about.
//
// The engine's helpers take a ruleset now that a variation can change the deck,
// the wild limits and every bonus (docs/samba-plan.md §3.1). The tests that
// pinned Canasta's numbers still pin Canasta's numbers — they name the ruleset
// they are asserting about instead of assuming there is only one.
func classicRules() ruleset { return variations["classic"] }

func refs(ids ...string) []module.PlayerRef {
	out := make([]module.PlayerRef, 0, len(ids))
	for _, id := range ids {
		out = append(out, module.PlayerRef{ID: id, Name: id})
	}
	return out
}

// The order a UI shell would pick moves in: build the table, then take the
// pile if you can, then draw, and discard only because a turn has to end.
var driverPrefer = []string{VerbLayMeld, VerbLayOff, VerbTakePile, VerbTakeTop, VerbDraw, VerbDiscard}

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

// TestSambaIsAlsoOfferDriven is the same claim for the variation that had every
// reason to break it.
//
// Sequences are the thing `extensibility-plan.md` §1.1 says cannot be
// enumerated, and Žolíky still cannot be driven from offers for exactly that
// reason. Samba's runs are the exception because they take no wilds: a candidate
// is the maximal block of consecutive ranks in a suit, which is a fact about the
// hand rather than a shape a human composes. If that reasoning were wrong, this
// test is where it would show — the driver would run out of legal moves it could
// see, and the match would not finish.
//
// Every seat count from two to six, because seating is the other thing this work
// changed and a partnership bug at six would otherwise surface in production.
func TestSambaIsAlsoOfferDriven(t *testing.T) {
	m := New()
	for seats := 2; seats <= 6; seats++ {
		players := refs(seatNames(seats)...)
		t.Run(seatLabel(seats), func(t *testing.T) {
			for seed := int64(1); seed <= 8; seed++ {
				cfg := module.MatchConfig{
					Variation: "samba",
					Options:   module.Options{OptTargetScore: 1000},
				}
				state, err := m.NewMatch(cfg, players, seed)
				if err != nil {
					t.Fatalf("seed %d: NewMatch: %v", seed, err)
				}
				final, res, err := module.PlayWithOffers(m, state, players, module.DriverOptions{
					MaxActions: 8000, Prefer: driverPrefer,
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
				s, err := decode(final)
				if err != nil {
					t.Fatalf("seed %d: decode: %v", seed, err)
				}
				if got, want := len(s.Teams), seatsToTeams(seats); got != want {
					t.Errorf("seed %d: %d seats made %d sides, want %d", seed, seats, got, want)
				}
				if s.Teams[s.WinnerTeam].Score < s.TargetScore {
					t.Errorf("seed %d: winner scored %d, below the %d target",
						seed, s.Teams[s.WinnerTeam].Score, s.TargetScore)
				}
			}
		})
	}
}

// A Samba where nobody ever lays a sequence is Canasta with a bigger deck, so
// the run offers have to be reachable in real play rather than merely correct.
//
// What this asserts is that sequences get laid and that the top card gets taken
// onto one — both from offers alone, by a driver that has never heard of a suit.
// It deliberately does *not* assert that a samba completes: only the final deal's
// melds survive on the table, so any count here undercounts by most of the match,
// and a threshold tuned to what the greedy driver happens to manage would be a
// number nobody could defend. That a seven-card sequence scores 1500 and closes
// is pinned deterministically in scoring_test.go instead.
func TestSambasAndSequencesActuallyGetPlayed(t *testing.T) {
	m := New()
	players := refs("p1", "p2", "p3", "p4")

	runsLaid, sambas, topsTaken := 0, 0, 0
	for seed := int64(1); seed <= 12; seed++ {
		cfg := module.MatchConfig{
			Variation: "samba",
			Options:   module.Options{OptTargetScore: 1000},
		}
		state, err := m.NewMatch(cfg, players, seed)
		if err != nil {
			t.Fatalf("NewMatch: %v", err)
		}
		final, res, err := module.PlayWithOffers(m, state, players, module.DriverOptions{
			MaxActions: 8000, Prefer: driverPrefer,
		})
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		topsTaken += res.Verbs[VerbTakeTop]

		s, err := decode(final)
		if err != nil {
			t.Fatalf("seed %d: decode: %v", seed, err)
		}
		// The last deal is still on the table; earlier ones were swept, so this
		// undercounts. It only has to be non-zero.
		for i := range s.Teams {
			for _, mm := range s.Teams[i].Melds {
				if mm.kind() != meldRun {
					continue
				}
				runsLaid++
				if mm.isCanasta() {
					sambas++
				}
			}
		}
	}

	if runsLaid == 0 {
		t.Error("no sequence was ever laid across twelve Samba matches — the run offers are decorative")
	}
	if topsTaken == 0 {
		t.Error("the top card was never taken onto a sequence — take_top is unreachable in real play")
	}
	t.Logf("across twelve matches: %d sequences on the final table, %d of them sambas, %d top cards taken onto a run",
		runsLaid, sambas, topsTaken)
}

func seatNames(n int) []string {
	all := []string{"p1", "p2", "p3", "p4", "p5", "p6"}
	return all[:n]
}

func seatLabel(n int) string {
	if n >= 4 && n%2 == 0 {
		return string(rune('0'+n)) + " seats, partnerships"
	}
	return string(rune('0'+n)) + " seats, everyone for themselves"
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
