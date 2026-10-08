package klondike

import "zolik/server/internal/module"

var (
	_ module.RulesProvider     = (*Module)(nil)
	_ module.RuleIndexProvider = (*Module)(nil)
)

// Rules writes this table's rules out, resolved against the same option values
// the engine plays by — see tableOf. Rules that are switched off get a
// sentence too: "may I take a card back down" is a setting here.
func (m *Module) Rules(cfg module.MatchConfig) ([]module.RuleSection, error) {
	t := tableOf(cfg)

	table := []module.RuleItem{
		module.Rule("klondike.rules.solo", nil),
		module.Rule("klondike.rules.deal", nil),
	}
	if t.Draw >= 3 {
		table = append(table, module.Rule("klondike.rules.draw3", nil))
	} else {
		table = append(table, module.Rule("klondike.rules.draw1", nil))
	}
	table = append(table, module.Rule("klondike.rules.stock", nil))
	switch redealRule(t) {
	case "klondike.rules.redealsLimited":
		table = append(table, module.Rule("klondike.rules.redealsLimited", map[string]any{"n": t.Redeals}))
	default:
		table = append(table, module.Rule(redealRule(t), nil))
	}

	play := []module.RuleItem{
		module.Rule("klondike.rules.build", nil),
		module.Rule("klondike.rules.runs", nil),
		module.Rule("klondike.rules.kingOnly", nil),
		module.Rule("klondike.rules.autoFlip", nil),
		module.Rule("klondike.rules.foundation", nil),
	}
	if t.TakeBack {
		play = append(play, module.Rule("klondike.rules.takeBack", nil))
	} else {
		play = append(play, module.Rule("klondike.rules.noTakeBack", nil))
	}
	if t.AutoFinish {
		play = append(play, module.Rule("klondike.rules.autoFinish", nil))
	} else {
		play = append(play, module.Rule("klondike.rules.noAutoFinish", nil))
	}
	play = append(play, module.Rule("klondike.rules.undo", nil))

	var scoring module.RuleSection
	switch t.Scoring {
	case scoreVegas:
		scoring = module.SectionOf("klondike.rules.section.scoring", []module.RuleItem{
			module.Rule("klondike.rules.scoreVegas", nil),
		})
	case scoreNone:
		scoring = module.SectionOf("klondike.rules.section.scoring", []module.RuleItem{
			module.Rule("klondike.rules.scoreNone", nil),
		})
	default:
		recycle := "klondike.rules.scoreRecycle1"
		if t.Draw >= 3 {
			recycle = "klondike.rules.scoreRecycle3"
		}
		scoring = module.SectionOf("klondike.rules.section.scoring", []module.RuleItem{
			module.Rule("klondike.rules.scoreStandard", nil),
			module.Rule(recycle, nil),
		})
	}

	return []module.RuleSection{
		module.SectionOf("klondike.rules.section.table", table),
		module.SectionOf("klondike.rules.section.play", play),
		scoring,
		module.SectionOf("klondike.rules.section.end", []module.RuleItem{
			module.Rule("klondike.rules.won", nil),
			module.Rule("klondike.rules.lost", nil),
			module.Rule("klondike.rules.stuck", map[string]any{"n": idleLimit}),
		}),
	}, nil
}

// ExplainRefusal names the written rules behind a refusal at this table,
// filtered against the rules the table actually states.
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
