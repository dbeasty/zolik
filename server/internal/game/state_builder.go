package game

import (
	"zolik/server/internal/models"
)

type GameStateMsg struct {
	Type               string                     `json:"type"`
	Round              int                        `json:"round"`
	Phase              string                     `json:"phase"`
	CurrentTurn        string                     `json:"currentTurn"`
	MyHand             []string                  `json:"myHand"`
	DiscardPile        []string                  `json:"discardPile"`
	ReshuffleCount     int                        `json:"reshuffleCount"`
	CardCounts         map[string]int           `json:"cardCounts"`
	Melds              map[string][][]string    `json:"melds"`
	RoundReqMet        map[string]bool          `json:"roundReqMet"`
	TotalScores        map[string]int           `json:"totalScores"`
	WinnerID           string                   `json:"winnerId,omitempty"`
	IsDraw             bool                     `json:"isDraw,omitempty"`
	InitialMeldMinimum int                      `json:"initialMeldMinimum"`
	Offer              *OfferMsg                 `json:"offer"`
}

type OfferMsg struct {
	Card string `json:"card"`
}

func BuildGameStateMsg(g models.Game, myPlayerID string) GameStateMsg {
	cardCounts := map[string]int{}
	for _, p := range g.Players {
		if p.ID == myPlayerID {
			continue
		}
		cardCounts[p.ID] = len(g.Hands[p.ID])
	}

	var myHand []string
	if g.Hands != nil {
		myHand = g.Hands[myPlayerID]
	}

	var offer *OfferMsg
	if g.Offer != nil && g.Offer.OfferedTo == myPlayerID {
		offer = &OfferMsg{Card: g.Offer.Card}
	}

	phaseStr := g.Phase
	return GameStateMsg{
		Type:               "game_state",
		Round:              g.Round,
		Phase:              phaseStr,
		CurrentTurn:        g.CurrentTurn,
		MyHand:             myHand,
		DiscardPile:        g.DiscardPile,
		ReshuffleCount:     g.ReshuffleCount,
		CardCounts:         cardCounts,
		Melds:              g.Melds,
		RoundReqMet:        g.RoundReqMet,
		TotalScores:        g.TotalScores,
		WinnerID:           g.WinnerID,
		IsDraw:             g.IsDraw,
		InitialMeldMinimum: g.InitialMeldMinimum,
		Offer:              offer,
	}
}

