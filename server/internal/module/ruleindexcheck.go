package module

import "fmt"

// The two ends of a rule index, checked against each other.
//
// Between them these say: a player cannot be refused by this engine for a
// reason nothing in the written rules explains. That is the whole guarantee,
// and it is what makes an index worth having rather than a per-code table of
// explanations somebody remembers to update.
//
// It lives here rather than in each game's test file for the reason
// conformance.go does: every module's suite runs it, the check is identical in
// all of them, and six copies of it would drift the first time one game's
// index grew an idea the others did not get.

// RuleIndexCheck is one module's index, asked to prove itself.
type RuleIndexCheck struct {
	// Module is the game under test; it must implement both RulesProvider and
	// RuleIndexProvider, or the check reports that as the first problem.
	Module GameModule
	// Dirs are the source directories whose validators emit this module's
	// refusal codes, relative to the test's own package — usually "." and,
	// for a module built on a shared engine, that engine's directory too.
	Dirs []string
	// Exempt lists codes that are allowed to have no rule behind them, each
	// with the reason. A card you are not holding, an action name the client
	// invented, a table that is not running: real refusals, but no written
	// rule explains them and none should be invented to.
	//
	// Adding to this map is a deliberate act with a reason next to it, which
	// is the point of it being a map rather than a list.
	Exempt map[string]string
}

// Run reports every way the index and the written rules disagree, plus notes
// that are worth printing but are not failures.
//
// Problems, in the order they are worth reading:
//
//   - a code nothing explains — the gap this whole mechanism exists to close;
//   - an id pointing at a sentence the table in question never states, which
//     reaches a player as a rule reference that resolves to nothing;
//   - an exempt code that points at a rule anyway, meaning the exemption is
//     stale;
//   - a rules listing whose ids are not addresses: an item whose id and label
//     key differ, or two items sharing one id.
func (c RuleIndexCheck) Run() (problems, notes []string) {
	rules, ok := c.Module.(RulesProvider)
	if !ok {
		return []string{"module states no written rules (RulesProvider)"}, nil
	}
	index, ok := c.Module.(RuleIndexProvider)
	if !ok {
		return []string{"module has no rule index (RuleIndexProvider)"}, nil
	}

	emitted, declaredOnly := EmittedCodes(c.Dirs...)
	if len(emitted) == 0 {
		return []string{fmt.Sprintf(
			"no refusal codes found in %v — the source walk is broken, which "+
				"would make this check pass by looking at nothing", c.Dirs)}, nil
	}
	// Not a failure. A code nothing returns is either a rule waiting to be
	// implemented or a leftover, and only a person can tell which — but it
	// should never be a surprise, so it is reported every run.
	for _, code := range declaredOnly {
		notes = append(notes, "declared, never emitted: "+code)
	}

	// A code counts as explained if *some* table explains it, not if every
	// one does. Half the refusals in a configurable game only exist on some
	// settings — a sequence cannot be malformed at a table with no sequences
	// — and demanding an explanation from a table that can never emit the
	// code would force a mapping to point somewhere untrue.
	explained := map[string]bool{}

	for _, cfg := range EveryConfig(c.Module) {
		sections, err := rules.Rules(cfg)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: Rules: %v", describe(cfg), err))
			continue
		}
		problems = append(problems, addressable(describe(cfg), sections)...)
		stated := RuleIDsIn(sections)

		for _, code := range emitted {
			ids := index.ExplainRefusal(cfg, code)
			if len(ids) > 0 {
				explained[code] = true
			}
			for _, id := range ids {
				if !stated[id] {
					problems = append(problems, fmt.Sprintf(
						"%s: %s points at %q, which this table's rules never state",
						describe(cfg), code, id))
				}
			}
		}
	}

	for _, code := range emitted {
		why, exempt := c.Exempt[code]
		switch {
		case exempt && explained[code]:
			problems = append(problems, fmt.Sprintf(
				"%s is listed as having no rule (%s) but points at one", code, why))
		case !exempt && !explained[code]:
			problems = append(problems, fmt.Sprintf(
				"nothing explains %s at any table — write the rule, or list it "+
					"in Exempt with the reason", code))
		}
	}
	return problems, notes
}

// addressable checks that a rules listing is a real index: every id is the
// label key of the sentence it names, and no id names two sentences. Without
// both, a pointer into the index can resolve to the wrong rule or to none.
func addressable(where string, sections []RuleSection) (problems []string) {
	seen := map[string]bool{}
	for _, s := range sections {
		if s.ID != s.TitleKey {
			problems = append(problems, fmt.Sprintf(
				"%s: section %q: id and title key differ", where, s.ID))
		}
		for _, it := range s.Items {
			if it.ID != it.LabelKey {
				problems = append(problems, fmt.Sprintf(
					"%s: item %q: id %q and label key differ", where, it.LabelKey, it.ID))
			}
			if seen[it.ID] {
				problems = append(problems, fmt.Sprintf(
					"%s: %q is stated twice — an id must address one sentence", where, it.ID))
			}
			seen[it.ID] = true
		}
	}
	return problems
}

// EveryConfig is the option space a lobby can actually choose from: every
// variation on its own, and every variation with each declared choice of each
// option applied.
//
// Not the full cross product, which is exponential in the number of options
// and buys little — a rule index branches on one option at a time, and an
// interaction between two of them is a rules bug rather than an index one.
func EveryConfig(m GameModule) []MatchConfig {
	d := m.Descriptor()
	configs := []MatchConfig{{}}
	for _, v := range d.Variations {
		configs = append(configs, MatchConfig{Variation: v.ID, Options: Options{}})
		for _, opt := range d.Options {
			for _, choice := range opt.Choices {
				configs = append(configs, MatchConfig{
					Variation: v.ID,
					Options:   Options{opt.Name: choice.Value},
				})
			}
		}
	}
	return configs
}

func describe(cfg MatchConfig) string {
	if cfg.Variation == "" && len(cfg.Options) == 0 {
		return "default table"
	}
	if len(cfg.Options) == 0 {
		return cfg.Variation
	}
	return fmt.Sprintf("%s %v", cfg.Variation, cfg.Options)
}
