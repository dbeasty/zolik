package blackpeter

import "zolik/server/internal/module"

const (
	minPlayers = 2
	maxPlayers = 6
)

var defaults = map[string]int{
	OptPairs:                     pairsByRank,
	OptRounds:                    5,
	module.OptBotSkill:           module.SkillOpt(module.SkillMedium),
	module.OptPauseBetweenRounds: module.OptOn,
	module.OptStandInAfter:       module.StandInAfterDefault,
}

// tableOf resolves a lobby's options into the table a match is played on.
// NewMatch freezes it onto the match; Rules and ExplainRefusal read the same
// answer, so the rules screen never describes a table nobody is sitting at.
func tableOf(cfg module.MatchConfig) *GameState {
	opt := func(name string) int { return cfg.Opt(name, defaults[name]) }
	return &GameState{
		ByColour:   opt(OptPairs) == pairsByColour,
		RoundLimit: max(1, opt(OptRounds)),
		Pause:      cfg.PauseBetweenRounds(true),
	}
}

// config is the match's own table, as options, for the rules it states.
func (s *GameState) config() module.MatchConfig {
	pairs := pairsByRank
	if s.ByColour {
		pairs = pairsByColour
	}
	return module.MatchConfig{Options: module.Options{OptPairs: pairs, OptRounds: s.RoundLimit}}
}

// Descriptor is Black Peter's self-description.
func (m *Module) Descriptor() module.ModuleDescriptor {
	return module.ModuleDescriptor{
		ID:             "blackpeter",
		Label:          "Black Peter",
		MinPlayers:     minPlayers,
		MaxPlayers:     maxPlayers,
		DefaultPlayers: 4,
		Variations: []module.VariationSpec{{
			ID:    "classic",
			Label: "Classic",
			Summary: []module.Fact{
				{LabelKey: "blackpeter.rules.goal"},
				{LabelKey: "blackpeter.rules.deck"},
				{LabelKey: "blackpeter.rules.draw"},
			},
			Defaults: defaults,
		}},
		Options: []module.OptionSpec{
			{
				Name: OptPairs, Type: module.OptionEnumInt, Label: "Pairs",
				Help: "What makes a pair: any two cards of the same rank, or the same rank and the same colour.",
				Choices: []module.OptionChoice{
					{Value: pairsByRank, Label: "Same rank"},
					{Value: pairsByColour, Label: "Same rank and colour"},
				},
			},
			{
				Name: OptRounds, Type: module.OptionEnumInt, Label: "Rounds",
				Help:    "How many rounds the match lasts. The fewest marks at the end win.",
				Choices: []module.OptionChoice{{Value: 1, Label: "1"}, {Value: 3, Label: "3"}, {Value: 5, Label: "5"}, {Value: 10, Label: "10"}},
			},
			module.BotSkillOption(),
			module.PauseOption(),
			module.StandInOption(),
			module.StepOutOption(),
			module.HintsOption(),
		},
	}
}
