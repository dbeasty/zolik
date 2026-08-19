package ai

import "zolik/server/internal/rules"

// VisibleState is the public board snapshot accessible to agents.
type VisibleState struct {
	Round       int
	Phase       string
	CurrentTurn string

	DiscardPile        []string
	Melds              map[string][][]string
	MeldMeta           map[string][]rules.MeldInfo
	RoundReqMet        map[string]bool
	TotalScores        map[string]int
	InitialMeldMinimum int
	DiscardDrawMinRound int
	DeckDrawMinRound   int

	Offer *rules.DiscardOffer
}

type Agent interface {
	ChooseAction(visible VisibleState, hand []string) rules.Action
	Difficulty() string
}

