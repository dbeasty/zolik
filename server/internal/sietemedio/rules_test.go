package sietemedio

import (
	"reflect"
	"testing"

	"zolik/server/internal/module"
)

// configs is every table this module can be asked to describe: each variation
// on its own, and each variation with one option moved off its default. The
// same sweep the key manifest does, for the same reason — a sentence that
// exists on one setting and not another is only found by walking them.
func configs() []module.MatchConfig {
	d := New().Descriptor()
	out := []module.MatchConfig{{}}
	for _, v := range d.Variations {
		out = append(out, module.MatchConfig{Variation: v.ID})
		for _, opt := range d.Options {
			for _, choice := range opt.Choices {
				out = append(out, module.MatchConfig{
					Variation: v.ID,
					Options:   module.Options{opt.Name: choice.Value},
				})
			}
		}
	}
	return out
}

// TestRules_DescribeTheTableTheMatchWillActuallyBePlayedOn — the split brain
// this codebase closed once already: a rules screen that renders a default
// while the engine plays something else.
func TestRules_DescribeTheTableTheMatchWillActuallyBePlayedOn(t *testing.T) {
	cfg := module.MatchConfig{
		Variation: VarMexicana,
		Options:   module.Options{OptMinStake: 25, OptRounds: 6, OptStartingStack: 200},
	}
	sections, err := New().Rules(cfg)
	if err != nil {
		t.Fatalf("Rules: %v", err)
	}

	want := map[string]int{
		"sietemedio.rules.minStake": 25,
		"sietemedio.rules.rounds":   6,
		"sietemedio.rules.stack":    200,
		"sietemedio.rules.brokeOut": 25,
	}
	for _, sec := range sections {
		for _, item := range sec.Items {
			if n, ok := want[item.ID]; ok {
				if got, _ := item.Params["n"].(int); got != n {
					t.Errorf("%s says %v, but the table is set to %d", item.ID, item.Params["n"], n)
				}
				delete(want, item.ID)
			}
		}
	}
	for id := range want {
		t.Errorf("the rules never state %s", id)
	}

	// And the state the engine will play by agrees with what was just read.
	s := tableOf(cfg)
	if s.MinStake != 25 || s.RoundLimit != 6 || s.StartingStack != 200 {
		t.Errorf("the engine resolved minStake=%d rounds=%d stack=%d",
			s.MinStake, s.RoundLimit, s.StartingStack)
	}
}

// TestRules_EveryHouseRuleIsStatedOneWayOrTheOtherOnEveryTable — the three
// things the sources disagree about are the three a player most needs told,
// and a table that stated neither branch would leave them guessing which game
// they had sat down to.
func TestRules_EveryHouseRuleIsStatedOneWayOrTheOtherOnEveryTable(t *testing.T) {
	pairs := [][2]string{
		{"sietemedio.rules.tiesToBank", "sietemedio.rules.tiesPush"},
		{"sietemedio.rules.sevenPaysDouble", "sietemedio.rules.sevenPaysEven"},
		{"sietemedio.rules.firstCardDown", "sietemedio.rules.allFaceUp"},
	}
	for _, cfg := range configs() {
		sections, err := New().Rules(cfg)
		if err != nil {
			t.Fatalf("Rules(%+v): %v", cfg, err)
		}
		stated := module.RuleIDsIn(sections)
		for _, pair := range pairs {
			if stated[pair[0]] == stated[pair[1]] {
				t.Errorf("config %+v states %q=%v and %q=%v — exactly one should hold",
					cfg, pair[0], stated[pair[0]], pair[1], stated[pair[1]])
			}
		}
		// How the bank moves has three answers rather than two, and exactly
		// one of them belongs on any table.
		moves := 0
		for _, id := range []string{
			"sietemedio.rules.bankOnSeven", "sietemedio.rules.bankRound", "sietemedio.rules.bankFixed",
		} {
			if stated[id] {
				moves++
			}
		}
		if moves != 1 {
			t.Errorf("config %+v states %d rules for how the bank moves, want exactly 1", cfg, moves)
		}
	}
}

// TestRules_EveryVariationSummaryPointsAtARuleItStates — the lobby shows a
// variation's summary before a match exists, so each line of it has to be a
// rule that variation's own table actually states.
func TestRules_EveryVariationSummaryPointsAtARuleItStates(t *testing.T) {
	m := New()
	for _, v := range m.Descriptor().Variations {
		sections, err := m.Rules(module.MatchConfig{Variation: v.ID})
		if err != nil {
			t.Fatalf("Rules(%s): %v", v.ID, err)
		}
		stated := module.RuleIDsIn(sections)
		for _, f := range v.Summary {
			if !stated[f.LabelKey] {
				t.Errorf("variation %q is summarised with %q, which its table does not state", v.ID, f.LabelKey)
			}
		}
	}
}

// TestRules_EveryVariationDefaultIsADeclaredChoice — a lobby renders the form
// from the descriptor and posts what it was given back, so a default that is
// not one of the option's own choices is a form that cannot be submitted
// unchanged.
func TestRules_EveryVariationDefaultIsADeclaredChoice(t *testing.T) {
	d := New().Descriptor()
	for _, v := range d.Variations {
		for name, value := range v.Defaults {
			opt := d.Option(name)
			if opt == nil {
				t.Errorf("variation %q defaults %q, which is not a declared option", v.ID, name)
				continue
			}
			if !opt.Allows(value) {
				t.Errorf("variation %q defaults %q to %d, which is not one of its choices", v.ID, name, value)
			}
		}
		// And the other way round: an option with no default in a variation is
		// an option a lobby has to guess at.
		for _, opt := range d.Options {
			if _, ok := v.Defaults[opt.Name]; !ok {
				t.Errorf("variation %q says nothing about option %q", v.ID, opt.Name)
			}
		}
	}
}

// TestRules_TheVariationDefaultsAreTheTableTheEngineBuilds — the descriptor
// tells a lobby what it is about to create. If tableOf then resolved something
// else, the lobby would be advertising a table nobody sits at.
func TestRules_TheVariationDefaultsAreTheTableTheEngineBuilds(t *testing.T) {
	d := New().Descriptor()
	for _, v := range d.Variations {
		bare := tableOf(module.MatchConfig{Variation: v.ID})
		spelled := tableOf(module.MatchConfig{Variation: v.ID, Options: optionsOf(v.Defaults)})
		if !reflect.DeepEqual(bare, spelled) {
			t.Errorf("variation %q resolves differently from its own advertised defaults:\n bare: %+v\nspelled: %+v",
				v.ID, bare, spelled)
		}
	}
}

func optionsOf(defaults map[string]int) module.Options {
	out := module.Options{}
	for k, v := range defaults {
		out[k] = v
	}
	return out
}

// TestExplainRefusal_SaysNothingAboutARuleTheTableDoesNotState — pointing a
// player at a sentence their rules screen does not contain is a "why not?"
// that opens an empty page.
func TestExplainRefusal_SaysNothingAboutARuleTheTableDoesNotState(t *testing.T) {
	m := New()
	for _, cfg := range configs() {
		sections, err := m.Rules(cfg)
		if err != nil {
			t.Fatalf("Rules(%+v): %v", cfg, err)
		}
		stated := module.RuleIDsIn(sections)
		for _, code := range []string{
			ErrWrongPhase, ErrNotYourTurn, ErrNotInRound, ErrStakeTooSmall, ErrAlreadyStaked,
			ErrNotEnoughChips, ErrAmountRequired, ErrBankCannotCover, ErrBankerDoesNotStake,
			ErrAlreadyLeft,
		} {
			ids := m.ExplainRefusal(cfg, code)
			if len(ids) == 0 {
				t.Errorf("config %+v explains %s with nothing at all", cfg, code)
			}
			for _, id := range ids {
				if !stated[id] {
					t.Errorf("config %+v explains %s with %q, which it does not state", cfg, code, id)
				}
			}
		}
	}
}
