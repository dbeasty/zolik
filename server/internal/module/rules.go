package module

// RuleSection is one titled group of a game's written rules ("Setup",
// "Melding", "How a match ends").
type RuleSection struct {
	// ID is the section's stable address — its own TitleKey. See RuleItem.ID.
	ID       string `json:"id"`
	TitleKey string `json:"titleKey"`
	// Items are full-sentence facts — LabelKey names the sentence, Params
	// carries whatever the sentence needs resolved (a deal size, a point
	// floor). Keys, never rendered text, same contract as every other Fact.
	Items []RuleItem `json:"items"`
}

// RuleItem is one written rule, addressable.
//
// The address is the sentence's own LabelKey, written out as a field of its
// own because everything that points at a rule — a refusal's RuleIDs, a deep
// link, an anchor on a rules screen — should be able to say "this id" without
// knowing that ids happen to be label keys.
//
// That identity is what makes the rules a real index. A sentence built by
// Rules() is already resolved against the table actually being played and
// already carries that table's own numbers, so a refusal that points at one
// gets a config-accurate explanation for free, in whatever locale the reader
// has, identical to what the rules screen prints.
type RuleItem struct {
	ID string `json:"id"`
	Fact
}

// Rule builds an item from the fact that names it. The id is the label key:
// one string, so the two can never disagree.
func Rule(labelKey string, params map[string]any) RuleItem {
	return RuleOf(Fact{LabelKey: labelKey, Params: params})
}

// RuleOf addresses a fact that is already built, for a sentence carrying a
// Value as well as params.
func RuleOf(f Fact) RuleItem { return RuleItem{ID: f.LabelKey, Fact: f} }

// Section is a titled group of sentences, addressed by its own title key.
func Section(titleKey string, items ...Fact) RuleSection {
	sec := RuleSection{ID: titleKey, TitleKey: titleKey}
	for _, f := range items {
		sec.Items = append(sec.Items, RuleOf(f))
	}
	return sec
}

// SectionOf is Section for a list of items built up conditionally.
func SectionOf(titleKey string, items []RuleItem) RuleSection {
	return RuleSection{ID: titleKey, TitleKey: titleKey, Items: items}
}

// RulesProvider is implemented by a module that can state its rules in
// writing, resolved against one lobby's actual variation and option choices —
// so the text a player reads describes the table they are at, not a default
// they may have changed.
//
// Optional, the same way Ranked and Bot are: a module that has not written
// its rules yet simply has none to give, rather than the runtime inventing
// placeholder prose on its behalf.
type RulesProvider interface {
	Rules(cfg MatchConfig) ([]RuleSection, error)
}

// RuleIndexProvider maps a refusal code to the written rules that justify it.
//
// Optional like RulesProvider, and for the same reason: a module with no
// written rules has no index to point into.
//
// What this is not is a second opinion about legality. It says which sentence
// *explains* a refusal, never whether the refusal is correct — that stays
// where it has always been, in the validator the offer list already probes.
// A wrong mapping here mislabels an explanation; it cannot make an illegal
// move legal.
type RuleIndexProvider interface {
	ExplainRefusal(cfg MatchConfig, code string) []string
}

// RulesFor returns a module's written rules for a config, or nil if it has
// none to give.
func RulesFor(m GameModule, cfg MatchConfig) []RuleSection {
	p, ok := m.(RulesProvider)
	if !ok {
		return nil
	}
	out, err := p.Rules(cfg)
	if err != nil {
		return nil
	}
	return out
}

// ExplainRefusalFor returns the rule ids behind a code, or nil if the module
// has no index.
func ExplainRefusalFor(m GameModule, cfg MatchConfig, code string) []string {
	p, ok := m.(RuleIndexProvider)
	if !ok || code == "" {
		return nil
	}
	return p.ExplainRefusal(cfg, code)
}

// RuleIDsIn is every id a rule listing states, for callers checking that a
// pointer into the index resolves.
func RuleIDsIn(sections []RuleSection) map[string]bool {
	out := map[string]bool{}
	for _, s := range sections {
		out[s.ID] = true
		for _, it := range s.Items {
			out[it.ID] = true
		}
	}
	return out
}
