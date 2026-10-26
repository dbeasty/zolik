package sietemedio

import "zolik/server/internal/module"

var (
	_ module.RulesProvider     = (*Module)(nil)
	_ module.RuleIndexProvider = (*Module)(nil)
)

// ExplainRefusal names the written rules behind a refusal at this table.
//
// It shares its whole mapping with the offer list (ruleIDsFor), and filters the
// answer against the rules this config actually states — so an id can never
// point at a sentence a player cannot go and read.
func (m *Module) ExplainRefusal(cfg module.MatchConfig, code string) []string {
	stated := module.StatedRuleIDs(m, cfg)
	var out []string
	for _, id := range ruleIDsFor(tableOf(cfg), code) {
		if stated[id] {
			out = append(out, id)
		}
	}
	return out
}
