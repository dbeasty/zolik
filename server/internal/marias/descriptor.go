package marias

import "zolik/server/internal/module"

// Descriptor is Mariáš's self-description.
//
// Every house rule that varies between the association's table and a pub's
// is an option here, defaulting to the association's written rule — the one
// source a Czech player can look up.
func (m *Module) Descriptor() module.ModuleDescriptor {
	return module.ModuleDescriptor{
		ID:         "marias",
		Label:      "Mariáš",
		MinPlayers: 3,
		MaxPlayers: 3,
		Deck:       module.DeckGerman,
		Variations: []module.VariationSpec{
			{
				ID:    variationVoleny,
				Label: "Volený",
				Summary: []module.Fact{
					{LabelKey: "marias.rules.deck", Value: "32"},
					{LabelKey: "marias.rules.deal.trump"},
					{LabelKey: "marias.rules.bid.takeover"},
				},
				Defaults: variationDefaults,
			},
		},
		Options: []module.OptionSpec{
			{
				Name:  OptDeals,
				Type:  module.OptionEnumInt,
				Label: "Deals",
				Help:  "How many deals a match lasts. A multiple of three, so everyone chooses trumps equally often.",
				Choices: []module.OptionChoice{
					{Value: 9, Label: "9"},
					{Value: 12, Label: "12"},
					{Value: 18, Label: "18"},
					{Value: 24, Label: "24"},
				},
			},
			{
				Name:  OptTariff,
				Type:  module.OptionEnumInt,
				Label: "Stakes",
				Help:  "What each game is worth. Hra 1, sedma 2 and sto 4 in both; they differ in betl and durch.",
				Choices: []module.OptionChoice{
					{Value: TariffCSM, Label: "Association (betl 15, durch 30)"},
					{Value: TariffPub, Label: "Pub (betl 5, durch 10)"},
				},
			},
			{
				Name:  OptRedDoubles,
				Type:  module.OptionEnumInt,
				Label: "Hearts pay double",
				Help:  "With hearts as trumps, every payment is doubled.",
				Choices: []module.OptionChoice{
					{Value: module.OptOn, Label: "On"},
					{Value: module.OptOff, Label: "Off"},
				},
			},
			{
				Name:  OptFlekLimit,
				Type:  module.OptionEnumInt,
				Label: "Doubling limit",
				Help:  "How many times one game may be doubled: flek, re, tutti, boty, kalhoty…",
				Choices: []module.OptionChoice{
					{Value: FlekUnlimited, Label: "No limit"},
					{Value: 1, Label: "Flek only"},
					{Value: 2, Label: "Up to re"},
					{Value: 4, Label: "Up to boty"},
				},
			},
			{
				Name:  OptZLidu,
				Type:  module.OptionEnumInt,
				Label: "Trumps from the people",
				Help:  "The chooser may take trumps from an unseen card (z lidu) instead of from their first seven.",
				Choices: []module.OptionChoice{
					{Value: module.OptOn, Label: "Allowed"},
					{Value: module.OptOff, Label: "Not allowed"},
				},
			},
			{
				Name:  OptShowCardPoints,
				Type:  module.OptionEnumInt,
				Label: "Show card points",
				Help:  "Show each side's card points as the deal is played, instead of only at the end.",
				Choices: []module.OptionChoice{
					{Value: module.OptOff, Label: "Hidden"},
					{Value: module.OptOn, Label: "Shown"},
				},
			},
			module.BotSkillOption(),
			module.PauseOption(),
		},
	}
}

var variationDefaults = map[string]int{
	OptDeals:                     defaultDeals,
	OptTariff:                    TariffCSM,
	OptRedDoubles:                module.OptOn,
	OptFlekLimit:                 FlekUnlimited,
	OptZLidu:                     module.OptOn,
	OptShowCardPoints:            module.OptOff,
	module.OptBotSkill:           module.SkillOpt(module.SkillMedium),
	module.OptPauseBetweenRounds: module.OptOn,
}

// resolve reads a lobby's config against the variation's defaults.
func resolve(cfg module.MatchConfig) config {
	opt := func(name string) int { return cfg.Opt(name, variationDefaults[name]) }
	return config{
		deals:          opt(OptDeals),
		tariff:         tariffFor(opt(OptTariff)),
		redDoubles:     opt(OptRedDoubles) == module.OptOn,
		flekLimit:      opt(OptFlekLimit),
		zLidu:          opt(OptZLidu) == module.OptOn,
		showCardPoints: opt(OptShowCardPoints) == module.OptOn,
	}
}
