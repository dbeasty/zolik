package zolikmod

import (
	"testing"

	"zolik/server/internal/module"
	"zolik/server/internal/rules"
)

// The two ends of the rule index, checked against each other.
//
// Between them these say: a player cannot be refused by this engine for a
// reason nothing in the written rules explains. That is the whole guarantee,
// and it is the reason the index is worth having rather than a per-code table
// of explanations somebody remembers to update.

// everyConfig is the option space a real lobby can choose from — every
// variation crossed with every declared choice of every option. Small enough
// to enumerate (three options, a handful of choices each) and the only honest
// way to check a config-dependent mapping.
func everyConfig(t *testing.T) []module.MatchConfig {
	t.Helper()
	d := (&Module{}).Descriptor()

	configs := []module.MatchConfig{}
	for _, v := range d.Variations {
		base := module.MatchConfig{Variation: v.ID, Options: module.Options{}}
		configs = append(configs, base)
		for _, opt := range d.Options {
			for _, choice := range opt.Choices {
				next := module.MatchConfig{Variation: v.ID, Options: module.Options{}}
				for k, val := range base.Options {
					next.Options[k] = val
				}
				next.Options[opt.Name] = choice.Value
				configs = append(configs, next)
			}
		}
	}
	if len(configs) < 4 {
		t.Fatalf("option space looks empty: %d configs", len(configs))
	}
	return configs
}

func TestEveryRefusalPointsAtAStatedRule(t *testing.T) {
	m := &Module{}
	emitted, declaredOnly := module.EmittedCodes("../rules")
	if len(emitted) == 0 {
		t.Fatal("no refusal codes found in ../rules — the AST walk is broken, " +
			"which would make this test pass by looking at nothing")
	}

	// Not a failure. A code nothing returns is either a rule waiting to be
	// implemented or a leftover, and only a person can tell which — but it
	// should never be a surprise, so it is reported every run.
	for _, code := range declaredOnly {
		t.Logf("declared, never emitted: %s", code)
	}

	for _, cfg := range everyConfig(t) {
		sections, err := m.Rules(cfg)
		if err != nil {
			t.Fatalf("%+v: Rules: %v", cfg, err)
		}
		stated := module.RuleIDsIn(sections)

		for _, code := range emitted {
			ids := m.ExplainRefusal(cfg, code)
			for _, id := range ids {
				if !stated[id] {
					t.Errorf("%s %v: %s points at %q, which this table's rules never state",
						cfg.Variation, cfg.Options, code, id)
				}
			}
		}
	}
}

// Which codes are allowed to have no rule behind them, and why. Anything not
// on this list must be explained; adding to the list is a deliberate act with
// a reason next to it, rather than a mapping quietly left out.
var refusalsWithNoRule = map[rules.RulesErrorCode]string{
	rules.ErrCardNotInHand: "not a rule — the client and server disagree about the hand",
	rules.ErrNothingToUndo: "not a rule — there is simply nothing to take back",
	rules.ErrGameSuspended: "not a rule — the table is waiting for a player",
	rules.ErrGameNotActive: "not a rule — the match is over or has not started",
}

func TestEveryRefusalIsExplained(t *testing.T) {
	m := &Module{}
	emitted, _ := module.EmittedCodes("../rules")
	cfg := module.MatchConfig{}

	for _, code := range emitted {
		if why, exempt := refusalsWithNoRule[rules.RulesErrorCode(code)]; exempt {
			if len(m.ExplainRefusal(cfg, code)) > 0 {
				t.Errorf("%s is listed as having no rule (%s) but points at one", code, why)
			}
			continue
		}
		if len(m.ExplainRefusal(cfg, code)) == 0 {
			t.Errorf("nothing explains %s — write the rule, or list it in "+
				"refusalsWithNoRule with the reason", code)
		}
	}
}

// Every id is a real address: the id and the label key are one string, so a
// pointer into the index can never resolve to a sentence that is not there.
func TestRuleItemsAreAddressable(t *testing.T) {
	sections, err := (&Module{}).Rules(module.MatchConfig{})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, s := range sections {
		if s.ID != s.TitleKey {
			t.Errorf("section %q: id and title key differ", s.ID)
		}
		for _, it := range s.Items {
			if it.ID != it.LabelKey {
				t.Errorf("item %q: id %q and label key differ", it.LabelKey, it.ID)
			}
			if seen[it.ID] {
				t.Errorf("%q is stated twice — an id must address one sentence", it.ID)
			}
			seen[it.ID] = true
		}
	}
}
