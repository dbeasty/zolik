package marias

import "zolik/server/internal/module"

var _ module.LessonProvider = (*Module)(nil)

// Lessons is Mariáš's guided walk-through. It is written against the same
// resolved config as Rules(cfg): volený and licitovaný teach different ways
// to become the declarer, and the options that change a rule (doubling limit,
// hearts paying double, trumps z lidu, open betl and durch, the stakes) add,
// drop or refill the matching step.
func (m *Module) Lessons(cfg module.MatchConfig) ([]module.Lesson, error) {
	c := resolve(cfg)
	t := c.tariff
	licit := c.variation == variationLicit

	steps := []module.LessonStep{
		module.Say("goal", "guide.marias.goal", map[string]any{"n": c.deals}, module.AnchorScore).
			About("marias.rules.goal", "marias.rules.end"),
		module.Say("cards", "guide.marias.cards", nil, module.AnchorHand).
			About("marias.rules.deck", "marias.rules.points"),
	}

	if licit {
		steps = append(steps,
			module.Say("deal", "guide.marias.licit.deal", nil, module.AnchorHand).
				About("marias.rules.licit.deal"),
			module.Say("auction", "guide.marias.licit.auction", nil, module.AnchorActionBar).
				About("marias.rules.licit.auction", "marias.rules.licit.middle", "marias.rules.licit.nobody"),
			module.Say("ladder", "guide.marias.licit.ladder", nil, module.AnchorActionBar).
				About("marias.rules.licit.ladder", "marias.rules.licit.contract"),
			module.Say("dveSedmy", "guide.marias.licit.dveSedmy", nil, module.AnchorActionBar).
				About("marias.rules.game.dveSedmy", "marias.rules.licit.bluff", "marias.rules.licit.omyl"),
		)
	} else {
		steps = append(steps,
			module.Say("deal", "guide.marias.deal", nil, module.AnchorHand).
				About("marias.rules.deal.roles", "marias.rules.deal.turns"),
		)
		if c.zLidu {
			steps = append(steps,
				module.Say("trump", "guide.marias.trump.zLidu", nil, module.AnchorActionBar).
					About("marias.rules.deal.trump", "marias.rules.deal.zLidu"),
			)
		} else {
			steps = append(steps,
				module.Say("trump", "guide.marias.trump", nil, module.AnchorActionBar).
					About("marias.rules.deal.trump"),
			)
		}
		steps = append(steps,
			module.Say("announce", "guide.marias.announce", nil, module.AnchorActionBar).
				About("marias.rules.bid.announce", "marias.rules.deal.talon"),
			module.Say("takeover", "guide.marias.takeover", nil, module.AnchorActionBar).
				About("marias.rules.bid.reveal", "marias.rules.bid.takeover"),
		)
	}

	steps = append(steps,
		module.Say("games", "guide.marias.games", nil, module.AnchorActionBar).
			About("marias.rules.game.hra", "marias.rules.game.sedma", "marias.rules.game.betl", "marias.rules.game.durch"),
		module.Do("play", "guide.marias.play", nil, []string{VerbPlay},
			module.AnchorHand, module.AnchorTable).Times(2).
			About("marias.rules.play.follow", "marias.rules.play.trump"),
		module.Say("marriage", "guide.marias.marriage", map[string]any{"plain": marriagePlain, "trump": marriageTrump}, module.AnchorHand).
			About("marias.rules.play.marriage"),
	)

	if c.openBetlDurch {
		steps = append(steps,
			module.Say("openHand", "guide.marias.openHand", nil, module.AnchorTable).
				About("marias.rules.play.openHand"),
		)
	}

	if licit {
		steps = append(steps,
			module.Say("defenders", "guide.marias.licit.defenders", nil, module.AnchorActionBar).
				About("marias.rules.licit.noProti"),
		)
	} else {
		steps = append(steps,
			module.Say("defenders", "guide.marias.defenders", nil, module.AnchorActionBar).
				About("marias.rules.bid.proti"),
		)
	}

	if c.flekLimit != FlekUnlimited {
		steps = append(steps,
			module.Say("flek", "guide.marias.flek.limit", map[string]any{"n": c.flekLimit}, module.AnchorActionBar).
				About("marias.rules.bid.flek", "marias.rules.bid.flekLimit"),
		)
	} else {
		steps = append(steps,
			module.Say("flek", "guide.marias.flek", nil, module.AnchorActionBar).
				About("marias.rules.bid.flek"),
		)
	}

	tariffKey, tariffRule := "guide.marias.tariff", "marias.rules.score.tariff"
	params := map[string]any{"hra": t.Hra, "sedma": t.Sedma, "sto": t.Sto, "betl": t.Betl, "durch": t.Durch}
	if licit {
		tariffKey, tariffRule = "guide.marias.licit.tariff", "marias.rules.score.tariffLicit"
		params["dveSedmy"] = t.DveSedmy
	}
	steps = append(steps,
		module.Say("tariff", tariffKey, params, module.AnchorScore).
			About("marias.rules.score.parts", tariffRule),
	)
	if c.redDoubles {
		steps = append(steps,
			module.Say("red", "guide.marias.red", nil, module.AnchorScore).
				About("marias.rules.score.red"),
		)
	}

	verbs := []string{VerbPlay, VerbChooseTrump, VerbAnnounce, VerbDiscard, VerbGood, VerbTakeOver,
		VerbFlek, VerbProti, VerbPass, VerbBid, VerbHold, VerbFold}
	steps = append(steps,
		module.Do("freeplay", "guide.marias.freeplay", nil, verbs,
			module.AnchorHand, module.AnchorActionBar).Times(4),
		module.Say("hint", "guide.marias.hint", nil, module.AnchorHint),
		module.Say("done", "guide.marias.done", nil),
	)

	return []module.Lesson{{
		ID:         "basics",
		TitleKey:   "guide.marias.basics.title",
		SummaryKey: "guide.marias.basics.summary",
		Seed:       1,
		Bots:       2,
		Steps:      steps,
	}}, nil
}
