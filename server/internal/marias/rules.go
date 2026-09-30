package marias

import (
	"strings"

	"zolik/server/internal/module"
)

// Rules writes out Mariáš for one lobby's table: its variation, its stakes,
// its doubling limit, whether hearts pay double and whether trumps may come
// z lidu.
//
// Every sentence is a rule id a refusal can point at later, so the ids are
// chosen for the rule they state, not for where they happen to sit. The two
// variations share every sentence about the cards, the play and the scoring;
// they differ in how a declarer and a contract come about.
func (m *Module) Rules(mc module.MatchConfig) ([]module.RuleSection, error) {
	c := resolve(mc)
	t := c.tariff
	licit := c.variation == variationLicit

	cards := module.Section("marias.rules.section.cards",
		module.Fact{LabelKey: "marias.rules.deck", Value: "32"},
		module.Fact{LabelKey: "marias.rules.order.trump"},
		module.Fact{LabelKey: "marias.rules.order.plain"},
		module.Fact{LabelKey: "marias.rules.points", Params: map[string]any{
			"sharp": pointsSharp, "last": pointsLastTrick, "total": pointsTotal,
		}},
	)

	flek := []module.RuleItem{
		module.Rule("marias.rules.bid.flek", map[string]any{"names": strings.Join(flekNames, ", ")}),
	}
	if c.flekLimit != FlekUnlimited {
		flek = append(flek, module.Rule("marias.rules.bid.flekLimit", map[string]any{"n": c.flekLimit}))
	}

	lead := module.Rule("marias.rules.play.lead", nil)
	if licit {
		lead = module.Rule("marias.rules.licit.lead", nil)
	}
	play := module.SectionOf("marias.rules.section.play", []module.RuleItem{
		lead,
		module.Rule("marias.rules.play.follow", nil),
		module.Rule("marias.rules.play.trump", nil),
		module.Rule("marias.rules.play.seven", nil),
		module.Rule("marias.rules.play.marriage", map[string]any{"plain": marriagePlain, "trump": marriageTrump}),
		module.Rule("marias.rules.play.earlyEnd", nil),
	})

	scoring := []module.RuleItem{module.Rule("marias.rules.score.parts", nil)}
	if licit {
		scoring = append(scoring, module.Rule("marias.rules.score.tariffLicit", map[string]any{
			"hra": t.Hra, "sedma": t.Sedma, "sto": t.Sto, "betl": t.Betl, "durch": t.Durch, "dveSedmy": t.DveSedmy,
		}))
	} else {
		scoring = append(scoring, module.Rule("marias.rules.score.tariff", map[string]any{
			"hra": t.Hra, "sedma": t.Sedma, "sto": t.Sto, "betl": t.Betl, "durch": t.Durch,
		}))
	}
	scoring = append(scoring,
		module.Rule("marias.rules.score.sto", nil),
		module.Rule("marias.rules.score.quietSto", nil),
		module.Rule("marias.rules.score.quietSeven", map[string]any{"n": t.quietSeven()}),
		module.Rule("marias.rules.score.flek", nil),
	)
	if c.redDoubles {
		scoring = append(scoring, module.Rule("marias.rules.score.red", nil))
	}
	scoring = append(scoring, module.Rule("marias.rules.score.limit", map[string]any{"n": payLimit}))

	goal := module.Section("marias.rules.section.goal", module.Fact{LabelKey: "marias.rules.goal"})
	end := module.Section("marias.rules.section.end",
		module.Fact{LabelKey: "marias.rules.end", Params: map[string]any{"n": c.deals}},
	)

	if licit {
		bidding := append([]module.RuleItem{module.Rule("marias.rules.licit.noProti", nil)}, flek...)
		return []module.RuleSection{
			goal,
			cards,
			module.Section("marias.rules.section.deal",
				module.Fact{LabelKey: "marias.rules.licit.deal"},
				module.Fact{LabelKey: "marias.rules.deal.turns"},
			),
			module.Section("marias.rules.section.auction",
				module.Fact{LabelKey: "marias.rules.licit.auction"},
				module.Fact{LabelKey: "marias.rules.licit.middle"},
				module.Fact{LabelKey: "marias.rules.licit.nobody"},
				module.Fact{LabelKey: "marias.rules.licit.ladder"},
				module.Fact{LabelKey: "marias.rules.licit.contract"},
				module.Fact{LabelKey: "marias.rules.deal.talon"},
			),
			module.Section("marias.rules.section.games",
				module.Fact{LabelKey: "marias.rules.game.hra"},
				module.Fact{LabelKey: "marias.rules.game.sedma"},
				module.Fact{LabelKey: "marias.rules.game.sto", Params: map[string]any{"n": hundred}},
				module.Fact{LabelKey: "marias.rules.game.betl"},
				module.Fact{LabelKey: "marias.rules.game.durch"},
				module.Fact{LabelKey: "marias.rules.game.dveSedmy"},
				module.Fact{LabelKey: "marias.rules.licit.bluff"},
				module.Fact{LabelKey: "marias.rules.licit.omyl", Params: map[string]any{"n": t.Omyl}},
			),
			module.SectionOf("marias.rules.section.bidding", bidding),
			play,
			module.SectionOf("marias.rules.section.scoring", scoring),
			end,
		}, nil
	}

	deal := []module.RuleItem{
		module.Rule("marias.rules.deal.roles", nil),
		module.Rule("marias.rules.deal.turns", nil),
		module.Rule("marias.rules.deal.trump", nil),
	}
	if c.zLidu {
		deal = append(deal, module.Rule("marias.rules.deal.zLidu", nil))
	}
	deal = append(deal, module.Rule("marias.rules.deal.talon", nil))

	bidding := append([]module.RuleItem{
		module.Rule("marias.rules.bid.announce", nil),
		module.Rule("marias.rules.bid.takeover", nil),
		module.Rule("marias.rules.bid.proti", nil),
	}, flek...)

	return []module.RuleSection{
		goal,
		cards,
		module.SectionOf("marias.rules.section.deal", deal),
		module.Section("marias.rules.section.games",
			module.Fact{LabelKey: "marias.rules.game.hra"},
			module.Fact{LabelKey: "marias.rules.game.sedma"},
			module.Fact{LabelKey: "marias.rules.game.sto", Params: map[string]any{"n": hundred}},
			module.Fact{LabelKey: "marias.rules.game.betl"},
			module.Fact{LabelKey: "marias.rules.game.durch"},
		),
		module.SectionOf("marias.rules.section.bidding", bidding),
		play,
		module.SectionOf("marias.rules.section.scoring", scoring),
		end,
	}, nil
}
