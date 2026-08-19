package ai

import "zolik/server/internal/rules"

// VisibleState is the public board snapshot accessible to agents.
type VisibleState struct {
	// GameNumber is which deal of the match this is (1-7).
	GameNumber int
	// Round is the lap-around-the-table counter within the current deal;
	// gates DiscardDrawMinRound below.
	Round       int
	Phase       string
	CurrentTurn string

	DiscardPile         []string
	Melds               map[string][][]string
	MeldMeta            map[string][]rules.MeldInfo
	RoundReqMet         map[string]bool
	TotalScores         map[string]int
	InitialMeldMinimum  int
	DiscardDrawMinRound int
	// Rules is the game's resolved ruleset — see rules.RulesConfig. Agents
	// must use this instead of any hardcoded set/run size or contract
	// assumption so they behave correctly under every profile.
	Rules rules.RulesConfig
}

type Agent interface {
	ChooseAction(visible VisibleState, hand []string) rules.Action
	Difficulty() string
}
