package zolikmod

import (
	"testing"

	"zolik/server/internal/module"
	"zolik/server/internal/rules"
)

// findFact reports the params of the first item across all sections whose
// LabelKey matches, and whether it was found at all.
func findFact(t *testing.T, sections []module.RuleSection, key string) (map[string]any, bool) {
	t.Helper()
	for _, s := range sections {
		for _, item := range s.Items {
			if item.LabelKey == key {
				return item.Params, true
			}
		}
	}
	return nil, false
}

// TestRulesReflectTheSelectedOptions is the drift guard: NewMatch and Rules
// both go through resolveConfig, so a table's written rules must say exactly
// what its match will play like.
func TestRulesReflectTheSelectedOptions(t *testing.T) {
	m := New()

	t.Run("classic default", func(t *testing.T) {
		// ProfileZolikClassic ships InitialMeldMinimum: 0 (no floor) — the
		// 35-point floor is Continental's default, not Classic's.
		sections, err := m.Rules(module.MatchConfig{Variation: "zolik_classic"})
		if err != nil {
			t.Fatalf("Rules: %v", err)
		}
		if _, ok := findFact(t, sections, "zolik.rules.meldFloor.off"); !ok {
			t.Error("expected zolik.rules.meldFloor.off")
		}
		if _, ok := findFact(t, sections, "zolik.rules.cleanRun.on"); !ok {
			t.Error("expected zolik.rules.cleanRun.on")
		}
		if params, ok := findFact(t, sections, "zolik.rules.end.atScore"); !ok {
			t.Error("expected zolik.rules.end.atScore")
		} else if params["n"] != 200 {
			t.Errorf("end.atScore n = %v, want 200", params["n"])
		}
	})

	t.Run("meld floor on via option", func(t *testing.T) {
		sections, err := m.Rules(module.MatchConfig{
			Variation: "zolik_classic",
			Options:   module.Options{"initialMeldMinimum": 50},
		})
		if err != nil {
			t.Fatalf("Rules: %v", err)
		}
		if params, ok := findFact(t, sections, "zolik.rules.meldFloor.on"); !ok {
			t.Error("expected zolik.rules.meldFloor.on")
		} else if params["n"] != 50 {
			t.Errorf("meldFloor.on n = %v, want 50", params["n"])
		}
		if _, ok := findFact(t, sections, "zolik.rules.meldFloor.off"); ok {
			t.Error("meldFloor.off should not appear when the option is on")
		}
	})

	t.Run("clean run off", func(t *testing.T) {
		sections, err := m.Rules(module.MatchConfig{
			Variation: "zolik_classic",
			Options:   module.Options{"requireCleanRun": 0},
		})
		if err != nil {
			t.Fatalf("Rules: %v", err)
		}
		if _, ok := findFact(t, sections, "zolik.rules.cleanRun.off"); !ok {
			t.Error("expected zolik.rules.cleanRun.off")
		}
		if _, ok := findFact(t, sections, "zolik.rules.cleanRun.on"); ok {
			t.Error("cleanRun.on should not appear when the option is off")
		}
	})

	t.Run("continental", func(t *testing.T) {
		sections, err := m.Rules(module.MatchConfig{Variation: "continental"})
		if err != nil {
			t.Fatalf("Rules: %v", err)
		}
		if params, ok := findFact(t, sections, "zolik.rules.contracts.rotating"); !ok {
			t.Error("expected zolik.rules.contracts.rotating")
		} else if params["n"] != 7 {
			t.Errorf("contracts.rotating n = %v, want 7", params["n"])
		}
		if params, ok := findFact(t, sections, "zolik.rules.end.afterDeals"); !ok {
			t.Error("expected zolik.rules.end.afterDeals")
		} else if params["n"] != 7 {
			t.Errorf("end.afterDeals n = %v, want 7", params["n"])
		}
		if _, ok := findFact(t, sections, "zolik.rules.end.atScore"); ok {
			t.Error("end.atScore should not appear for a fixed-deal-count profile")
		}
	})
}

// TestResolveConfigMatchesNewMatch is the guard itself, stated directly: the
// config Rules writes from must be the exact config NewMatch deals from.
func TestResolveConfigMatchesNewMatch(t *testing.T) {
	mc := module.MatchConfig{
		Variation: "zolik_classic",
		Options:   module.Options{"initialMeldMinimum": 50, "requireCleanRun": 0},
	}
	cfg := resolveConfig(mc)
	if cfg.InitialMeldMinimum != 50 {
		t.Errorf("InitialMeldMinimum = %d, want 50", cfg.InitialMeldMinimum)
	}
	if cfg.StaticContract.RequireCleanRun {
		t.Error("StaticContract.RequireCleanRun should be false")
	}

	state, err := New().NewMatch(mc, refs("p1", "p2"), 1)
	if err != nil {
		t.Fatalf("NewMatch: %v", err)
	}
	gs, err := RulesStateOf(state)
	if err != nil {
		t.Fatalf("RulesStateOf: %v", err)
	}
	if gs.Rules.InitialMeldMinimum != cfg.InitialMeldMinimum {
		t.Errorf("dealt match InitialMeldMinimum = %d, want %d matching resolveConfig",
			gs.Rules.InitialMeldMinimum, cfg.InitialMeldMinimum)
	}
}

// Who wins and what the deck is were never written: the lowest total wins,
// a tie goes to fewer deals won (rules.DetermineMatchWinner), and the pack
// count follows the table size (rules.DeckCountForPlayers).
func TestRulesStateTheWinnerAndThePacks(t *testing.T) {
	for _, variation := range []string{"zolik_classic", "continental"} {
		t.Run(variation, func(t *testing.T) {
			sections, err := New().Rules(module.MatchConfig{Variation: variation})
			if err != nil {
				t.Fatal(err)
			}
			for _, key := range []string{"zolik.rules.end.lowestWins", "zolik.rules.end.tieBreak"} {
				if _, ok := findFact(t, sections, key); !ok {
					t.Errorf("expected %s", key)
				}
			}
			params, ok := findFact(t, sections, "zolik.rules.deck.packs")
			if !ok {
				t.Fatal("expected zolik.rules.deck.packs")
			}
			if params["small"] != 2 || params["mid"] != 3 || params["large"] != 4 {
				t.Errorf("packs = %v, want 2/3/4", params)
			}
		})
	}
	// The tie-break the rule states is the one the engine applies.
	w, draw := rules.DetermineMatchWinner([]string{"a", "b"},
		map[string]int{"a": 40, "b": 40},
		map[string][]int{"a": {0, 0, 40}, "b": {20, 10, 10}}) // a won two deals, b one
	if draw || w != "b" {
		t.Errorf("a tie on total goes to fewer deals won: got %q draw=%v, want b", w, draw)
	}
}
