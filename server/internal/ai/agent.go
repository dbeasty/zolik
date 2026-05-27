package ai

import "zolik/server/internal/rules"

// VisibleState is the public board snapshot accessible to agents.
type VisibleState struct {
	Round       int
	Phase       string
	CurrentTurn string

	DiscardPile []string
	Melds       map[string][][]string
	RoundReqMet map[string]bool
	TotalScores map[string]int

	Offer *rules.DiscardOffer
}

type Agent interface {
	ChooseAction(visible VisibleState, hand []string) rules.Action
	Difficulty() string
}

