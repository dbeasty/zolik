package ginrummy

import (
	"testing"

	"zolik/server/internal/module"
)

// prefer puts knocking first: a driver that preferred discarding would never
// knock at all, since the aggregate discard offer is enabled every turn a
// knock offer also is. lay_off and finish_layoff come next so the defender's
// sub-phase always resolves, then the ordinary draw/discard/pass fill in
// whatever is left.
var driverPrefer = []string{"knock", "lay_off", "finish_layoff", "draw", "discard", "pass"}

func players() []module.PlayerRef {
	return []module.PlayerRef{{ID: "p1", Name: "p1"}, {ID: "p2", Name: "p2"}}
}

// TestConformance_PlaysWholeMatchesToAWinner is the falsification test: a
// driver that knows nothing about Gin Rummy beyond its offers must be able to
// play a whole match — knocks and gins included — to a named winner, across
// seeds and both variations.
func TestConformance_PlaysWholeMatchesToAWinner(t *testing.T) {
	m := New()
	for _, variation := range []string{"standard", "oklahoma"} {
		t.Run(variation, func(t *testing.T) {
			for seed := int64(1); seed <= 20; seed++ {
				cfg := module.MatchConfig{Variation: variation, Options: module.Options{OptTargetScore: 100}}
				state, err := m.NewMatch(cfg, players(), seed)
				if err != nil {
					t.Fatalf("seed %d: NewMatch: %v", seed, err)
				}
				// Oklahoma's upcard-set knock limit is sometimes very low, which
				// makes dead hands (no score, redeal) far more frequent than the
				// standard variation and lengthens a match considerably against
				// a driver with no game knowledge — not a stall, just a harsher
				// ruleset met with a naive player. 16000 clears every seed tried
				// with headroom.
				maxActions := 4000
				if variation == "oklahoma" {
					maxActions = 16000
				}
				final, res, err := module.PlayWithOffers(m, state, players(), module.DriverOptions{
					MaxActions: maxActions, Prefer: driverPrefer,
				})
				if err != nil {
					t.Fatalf("seed %d: %v", seed, err)
				}
				if !res.Finished {
					t.Fatalf("seed %d: did not finish in %d actions", seed, res.Actions)
				}
				if len(res.Winners) != 1 {
					t.Fatalf("seed %d: expected exactly one winner, got %v", seed, res.Winners)
				}
				if res.Verbs["knock"] == 0 {
					t.Errorf("seed %d: match never knocked — the whole point of this game was never exercised", seed)
				}
				done, winners, err := m.Finished(final)
				if err != nil || !done || len(winners) != 1 {
					t.Errorf("seed %d: Finished disagrees with the driver: done=%v winners=%v err=%v", seed, done, winners, err)
				}
			}
		})
	}
}

// TestKnock_BigGinDeclaresAllElevenCardsWithNoDiscard is the engine-level
// check for the path a random driver could never reach on its own: an
// eleven-card hand that already melds completely needs no discard at all,
// which is exactly why applyKnock treats an empty card list as the
// declaration, and why this is a hand-crafted test rather than a conformance
// sweep — the odds of drawing into a natural eleven-card gin by undirected
// play are far too long to wait out.
func TestKnock_BigGinDeclaresAllElevenCardsWithNoDiscard(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.BigGin = true
		// A four-card set plus a seven-card run: eleven cards, nothing over.
		s.Hands["p1"] = []string{
			"2H", "2D", "2C", "2S",
			"5H", "6H", "7H", "8H", "9H", "TH", "JH",
		}
	})
	next, err := apply(t, raw, "p1", module.Action{Verb: VerbKnock})
	if err != nil {
		t.Fatalf("big gin: %v", err)
	}
	s := stateOf(t, next)
	if len(s.Rounds) != 1 || s.Rounds[0].Kind != "gin" {
		t.Fatalf("expected big gin to score as a gin, got %+v", s.Rounds)
	}
}

func TestKnock_BigGinRefusedWhenTheOptionIsOff(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.BigGin = false
		s.Hands["p1"] = []string{
			"2H", "2D", "2C", "2S",
			"5H", "6H", "7H", "8H", "9H", "TH", "JH",
		}
	})
	if _, err := apply(t, raw, "p1", module.Action{Verb: VerbKnock}); err == nil {
		t.Fatal("expected big gin to be refused when the option is off")
	}
}
