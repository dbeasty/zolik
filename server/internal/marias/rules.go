package marias

import (
	"strings"

	"zolik/server/internal/module"
)

// Rules writes out Mariáš for one lobby's table: its stakes, its doubling
// limit, whether hearts pay double and whether trumps may come z lidu.
//
// Every sentence is a rule id a refusal can point at later, so the ids are
// chosen for the rule they state, not for where they happen to sit.
func (m *Module) Rules(mc module.MatchConfig) ([]module.RuleSection, error) {
	c := resolve(mc)
	t := c.tariff

	deal := []module.RuleItem{
		module.Rule("marias.rules.deal.roles", nil),
		module.Rule("marias.rules.deal.turns", nil),
		module.Rule("marias.rules.deal.trump", nil),
	}
	if c.zLidu {
		deal = append(deal, module.Rule("marias.rules.deal.zLidu", nil))
	}
	deal = append(deal,
		module.Rule("marias.rules.deal.talon", nil),
	)

	bidding := []module.RuleItem{
		module.Rule("marias.rules.bid.announce", nil),
		module.Rule("marias.rules.bid.takeover", nil),
		module.Rule("marias.rules.bid.proti", nil),
		module.Rule("marias.rules.bid.flek", map[string]any{"names": strings.Join(flekNames, ", ")}),
	}
	if c.flekLimit != FlekUnlimited {
		bidding = append(bidding, module.Rule("marias.rules.bid.flekLimit", map[string]any{"n": c.flekLimit}))
	}

	scoring := []module.RuleItem{
		module.Rule("marias.rules.score.parts", nil),
		module.Rule("marias.rules.score.tariff", map[string]any{
			"hra": t.Hra, "sedma": t.Sedma, "sto": t.Sto, "betl": t.Betl, "durch": t.Durch,
		}),
		module.Rule("marias.rules.score.quietSto", nil),
		module.Rule("marias.rules.score.quietSeven", map[string]any{"n": t.quietSeven()}),
		module.Rule("marias.rules.score.flek", nil),
	}
	if c.redDoubles {
		scoring = append(scoring, module.Rule("marias.rules.score.red", nil))
	}

	return []module.RuleSection{
		module.Section("marias.rules.section.goal",
			module.Fact{LabelKey: "marias.rules.goal"},
		),
		module.Section("marias.rules.section.cards",
			module.Fact{LabelKey: "marias.rules.deck", Value: "32"},
			module.Fact{LabelKey: "marias.rules.order.trump"},
			module.Fact{LabelKey: "marias.rules.order.plain"},
			module.Fact{LabelKey: "marias.rules.points", Params: map[string]any{
				"sharp": pointsSharp, "last": pointsLastTrick, "total": pointsTotal,
			}},
		),
		module.SectionOf("marias.rules.section.deal", deal),
		module.Section("marias.rules.section.games",
			module.Fact{LabelKey: "marias.rules.game.hra"},
			module.Fact{LabelKey: "marias.rules.game.sedma"},
			module.Fact{LabelKey: "marias.rules.game.sto", Params: map[string]any{"n": hundred}},
			module.Fact{LabelKey: "marias.rules.game.betl"},
			module.Fact{LabelKey: "marias.rules.game.durch"},
		),
		module.SectionOf("marias.rules.section.bidding", bidding),
		module.Section("marias.rules.section.play",
			module.Fact{LabelKey: "marias.rules.play.lead"},
			module.Fact{LabelKey: "marias.rules.play.follow"},
			module.Fact{LabelKey: "marias.rules.play.trump"},
			module.Fact{LabelKey: "marias.rules.play.seven"},
			module.Fact{LabelKey: "marias.rules.play.marriage", Params: map[string]any{
				"plain": marriagePlain, "trump": marriageTrump,
			}},
			module.Fact{LabelKey: "marias.rules.play.earlyEnd"},
		),
		module.SectionOf("marias.rules.section.scoring", scoring),
		module.Section("marias.rules.section.end",
			module.Fact{LabelKey: "marias.rules.end", Params: map[string]any{"n": c.deals}},
		),
	}, nil
}
