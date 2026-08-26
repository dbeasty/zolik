package module

// RuleSection is one titled group of a game's written rules ("Setup",
// "Melding", "How a match ends").
type RuleSection struct {
	TitleKey string `json:"titleKey"`
	// Items are full-sentence facts — LabelKey names the sentence, Params
	// carries whatever the sentence needs resolved (a deal size, a point
	// floor). Keys, never rendered text, same contract as every other Fact.
	Items []Fact `json:"items"`
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
