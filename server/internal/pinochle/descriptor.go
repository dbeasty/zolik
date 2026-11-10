package pinochle

import "zolik/server/internal/module"

var (
	_ module.Ranked     = (*Module)(nil)
	_ module.OpenViewer = (*Module)(nil)
	_ module.Rounded    = (*Module)(nil)
	_ module.Botted     = (*Module)(nil)
	_ module.Seated     = (*Module)(nil)
)

// Descriptor is Pinochle's self-description.
func (m *Module) Descriptor() module.ModuleDescriptor {
	return module.ModuleDescriptor{
		ID:         "pinochle",
		Label:      "Pinochle",
		MinPlayers: 2,
		MaxPlayers: 4,
		Variations: []module.VariationSpec{
			{
				ID:    VarPartnership,
				Label: "Partnership",
				Summary: []module.Fact{
					{LabelKey: "pinochle.summary.partnership"},
					{LabelKey: "pinochle.summary.auction"},
					{LabelKey: "pinochle.summary.meld"},
				},
				Defaults:   defaultsFor(VarPartnership),
				MinPlayers: 4, MaxPlayers: 4,
			},
			{
				ID:    VarCutthroat,
				Label: "Cutthroat",
				Summary: []module.Fact{
					{LabelKey: "pinochle.summary.cutthroat"},
					{LabelKey: "pinochle.summary.widow"},
					{LabelKey: "pinochle.summary.meld"},
				},
				Defaults:   defaultsFor(VarCutthroat),
				MinPlayers: 3, MaxPlayers: 3,
			},
			{
				ID:    VarTwoHanded,
				Label: "Two-handed",
				Summary: []module.Fact{
					{LabelKey: "pinochle.summary.twohanded"},
					{LabelKey: "pinochle.summary.stock"},
					{LabelKey: "pinochle.summary.meldAsYouGo"},
				},
				Defaults:   defaultsFor(VarTwoHanded),
				MinPlayers: 2, MaxPlayers: 2,
			},
			{
				ID:    VarDoubleDeck,
				Label: "Double-deck",
				Summary: []module.Fact{
					{LabelKey: "pinochle.summary.doubledeck"},
					{LabelKey: "pinochle.summary.auction"},
					{LabelKey: "pinochle.summary.bigMelds"},
				},
				Defaults:   defaultsFor(VarDoubleDeck),
				MinPlayers: 4, MaxPlayers: 4,
			},
		},
		Options: []module.OptionSpec{
			{
				Name:  OptScoring,
				Type:  module.OptionEnumInt,
				Label: "Counting cards",
				Help:  "Counters: aces, tens and kings are worth 10 each. Classic: ace 11, ten 10, king 4, queen 3, jack 2. A hand is worth the same either way.",
				Choices: []module.OptionChoice{
					{Value: ScoringCounters, Label: "Counters"},
					{Value: ScoringClassic, Label: "Classic points"},
				},
			},
			{
				Name:  OptMinBid,
				Type:  module.OptionEnumInt,
				Label: "Minimum bid",
				Help:  "The lowest opening bid. Two-handed has no auction.",
				Choices: []module.OptionChoice{
					{Value: 200, Label: "200"},
					{Value: 250, Label: "250"},
					{Value: 300, Label: "300"},
					{Value: 400, Label: "400"},
					{Value: 500, Label: "500"},
					{Value: 600, Label: "600"},
				},
			},
			{
				Name:  OptTarget,
				Type:  module.OptionEnumInt,
				Label: "Play to",
				Help:  "The score that wins the match.",
				Choices: []module.OptionChoice{
					{Value: 1000, Label: "1000"},
					{Value: 1500, Label: "1500"},
					{Value: 2500, Label: "2500"},
					{Value: 5000, Label: "5000"},
				},
			},
			{
				Name:  OptPass,
				Type:  module.OptionEnumInt,
				Label: "Passing cards",
				Help:  "After trumps are named the bidder's partner passes cards to the bidder, who passes the same number back. Partnership tables only.",
				Choices: []module.OptionChoice{
					{Value: 0, Label: "No passing"},
					{Value: 3, Label: "Pass 3"},
					{Value: 4, Label: "Pass 4"},
				},
			},
			{
				Name:  OptMeldNeeds,
				Type:  module.OptionEnumInt,
				Label: "Meld counts if",
				Help:  "What a side has to take in play before its meld scores.",
				Choices: []module.OptionChoice{
					{Value: MeldNeedsTrick, Label: "It takes a trick"},
					{Value: MeldNeedsCounter, Label: "It takes a counter"},
				},
			},
			{
				Name:  OptAllPass,
				Type:  module.OptionEnumInt,
				Label: "If everyone passes",
				Help:  "When nobody bids at all.",
				Choices: []module.OptionChoice{
					{Value: AllPassDealerStuck, Label: "The dealer takes it at the minimum"},
					{Value: AllPassRedeal, Label: "Throw the hand in and deal again"},
				},
			},
			{
				Name:  OptShowCardPoints,
				Type:  module.OptionEnumInt,
				Label: "Show card points",
				Help:  "Show every side's meld and points taken as the hand is played. Your own are always shown to you.",
				Choices: []module.OptionChoice{
					{Value: module.OptOff, Label: "Hidden"},
					{Value: module.OptOn, Label: "Shown"},
				},
			},
			module.BotSkillOption(),
			module.PauseOption(),
			module.StandInOption(),
			module.StepOutOption(),
			module.HintsOption(),
		},
	}
}

func defaultsFor(variation string) map[string]int {
	t := tableOf(variation)
	return map[string]int{
		OptScoring:                   ScoringCounters,
		OptMinBid:                    max(t.minBid, 250),
		OptTarget:                    t.target,
		OptPass:                      0,
		OptMeldNeeds:                 MeldNeedsTrick,
		OptAllPass:                   AllPassDealerStuck,
		OptShowCardPoints:            module.OptOn,
		module.OptBotSkill:           module.SkillOpt(module.SkillMedium),
		module.OptPauseBetweenRounds: module.OptOn,
		module.OptStandInAfter:       module.StandInAfterDefault,
	}
}

type config struct {
	variation      string
	scoring        int
	minBid         int
	target         int
	pass           int
	meldNeeds      int
	allPass        int
	showCardPoints bool
}

func resolve(cfg module.MatchConfig) config {
	v := cfg.Variation
	if !knownVariation(v) {
		v = VarPartnership
	}
	d := defaultsFor(v)
	opt := func(name string) int { return cfg.Opt(name, d[name]) }
	c := config{
		variation:      v,
		scoring:        opt(OptScoring),
		minBid:         opt(OptMinBid),
		target:         max(100, opt(OptTarget)),
		pass:           opt(OptPass),
		meldNeeds:      opt(OptMeldNeeds),
		allPass:        opt(OptAllPass),
		showCardPoints: opt(OptShowCardPoints) == module.OptOn,
	}
	if c.scoring != ScoringClassic {
		c.scoring = ScoringCounters
	}
	if c.minBid < bidStep || c.minBid%bidStep != 0 {
		c.minBid = d[OptMinBid]
	}
	if c.pass != 3 && c.pass != 4 || !tableOf(v).partners {
		c.pass = 0
	}
	if v == VarTwoHanded {
		c.minBid = 0
	}
	return c
}

// config is the match's options as a lobby would send them.
func (s *GameState) config() module.MatchConfig {
	return module.MatchConfig{Variation: s.Variation, Options: module.Options{
		OptScoring:        s.Scoring,
		OptMinBid:         s.MinBid,
		OptTarget:         s.Target,
		OptPass:           s.PassCount,
		OptMeldNeeds:      s.MeldNeeds,
		OptAllPass:        s.AllPass,
		OptShowCardPoints: module.BoolOpt(!s.HideCardPoints),
	}}
}

// Sides are the partnerships at a four-handed table, partners opposite; nil
// at a table where everyone plays alone.
func (m *Module) Sides(cfg module.MatchConfig, players []module.PlayerRef) [][]string {
	if len(players) != 4 {
		return nil
	}
	return [][]string{{players[0].ID, players[2].ID}, {players[1].ID, players[3].ID}}
}

// Bot is Pinochle's own bot — see bot.go.
func (m *Module) Bot() module.Bot { return bot{} }
