package blackjack

import (
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
		Variation: "vegas",
		Options:   module.Options{OptDecks: 2, OptMinBet: 25, OptRounds: 5},
	}
	sections, err := New().Rules(cfg)
	if err != nil {
		t.Fatalf("Rules: %v", err)
	}

	want := map[string]int{
		"blackjack.rules.decks":  2,
		"blackjack.rules.minBet": 25,
		"blackjack.rules.rounds": 5,
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
	if s.Decks != 2 || s.MinBet != 25 || s.RoundLimit != 5 {
		t.Errorf("the engine resolved decks=%d minBet=%d rounds=%d", s.Decks, s.MinBet, s.RoundLimit)
	}
}

// TestRules_EveryHouseRuleIsStatedEitherWayRoundOnEveryTable — a rule that is
// off is a rule a player still has to be told about. "No surrender at this
// table" is information; silence is a guess.
func TestRules_EveryHouseRuleIsStatedEitherWayRoundOnEveryTable(t *testing.T) {
	pairs := [][2]string{
		{"blackjack.rules.split", "blackjack.rules.noSplit"},
		{"blackjack.rules.surrender", "blackjack.rules.noSurrender"},
		{"blackjack.rules.insurance", "blackjack.rules.noInsurance"},
		{"blackjack.rules.doubleAfterSplit", "blackjack.rules.noDoubleAfterSplit"},
		{"blackjack.rules.hitsSoft17", "blackjack.rules.standsSoft17"},
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
	}
}

// TestExplainRefusal_OnlyEverPointsAtASentenceThatExists, across every table
// this module can be set to. A refusal explaining itself with a dangling id
// opens an empty page, and only a sweep finds the one option value where that
// happens.
func TestExplainRefusal_OnlyEverPointsAtASentenceThatExists(t *testing.T) {
	m := New()
	codes := []string{
		ErrBetTooSmall, ErrAlreadyBet, ErrCannotDouble, ErrCannotSplit,
		ErrCannotSurrender, ErrInsuranceClosed, ErrNotYourTurn, ErrWrongPhase,
	}
	explained := 0
	for _, cfg := range configs() {
		sections, err := m.Rules(cfg)
		if err != nil {
			t.Fatalf("Rules: %v", err)
		}
		stated := module.RuleIDsIn(sections)
		for _, code := range codes {
			for _, id := range m.ExplainRefusal(cfg, code) {
				if !stated[id] {
					t.Errorf("config %+v: code %s points at %q, which its rules do not state", cfg, code, id)
				}
				explained++
			}
		}
	}
	if explained == 0 {
		t.Error("no refusal was explained by anything at all")
	}
}

// TestExplainRefusal_SaysNothingAboutAMoveTheTableDoesNotHave — pointing a
// player at "you may not split that" when the table never splits anything is
// an explanation of the wrong thing.
func TestExplainRefusal_SaysNothingAboutAMoveTheTableDoesNotHave(t *testing.T) {
	m := New()
	noSplit := module.MatchConfig{Variation: "vegas", Options: module.Options{OptMaxSplits: splitsNone}}
	ids := m.ExplainRefusal(noSplit, ErrCannotSplit)
	if len(ids) != 1 || ids[0] != "blackjack.rules.noSplit" {
		t.Errorf("a no-split table explained a split refusal with %v", ids)
	}
}
