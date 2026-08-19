package game

import (
	"zolik/server/internal/models"
)

type PlayerMsg struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	IsAI         bool   `json:"isAI"`
	AIDifficulty string `json:"aiDifficulty,omitempty"`
}

type MeldMetaMsg struct {
	MeldID string `json:"meldId"`
	Type   string `json:"type"`
}

type GameStateMsg struct {
	Type                        string                   `json:"type"`
	Status                      string                   `json:"status"`
	Game                        int                      `json:"game"`
	Round                       int                      `json:"round"`
	Phase                       string                   `json:"phase"`
	CurrentTurn                 string                   `json:"currentTurn"`
	MyHand                      []string                 `json:"myHand"`
	DiscardPile                 []string                 `json:"discardPile"`
	DeckCount                   int                      `json:"deckCount"`
	ReshuffleCount              int                      `json:"reshuffleCount"`
	CardCounts                  map[string]int           `json:"cardCounts"`
	Melds                       map[string][][]string    `json:"melds"`
	MeldMeta                    map[string][]MeldMetaMsg `json:"meldMeta"`
	Players                     []PlayerMsg              `json:"players"`
	RoundReqMet                 map[string]bool          `json:"roundReqMet"`
	TotalScores                 map[string]int           `json:"totalScores"`
	WinnerID                    string                   `json:"winnerId,omitempty"`
	IsDraw                      bool                     `json:"isDraw,omitempty"`
	InitialMeldMinimum          int                      `json:"initialMeldMinimum"`
	DiscardDrawMinRound         int                      `json:"discardDrawMinRound"`
	DiscardDrawnCardPendingMeld string                   `json:"discardDrawnCardPendingMeld,omitempty"`
	RulesProfile                string                   `json:"rulesProfile"`
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

	var pendingMeldCard string
	if g.CurrentTurn == myPlayerID {
		pendingMeldCard = g.DiscardDrawnCardPendingMeld
	}

	players := make([]PlayerMsg, 0, len(g.Players))
	for _, p := range g.Players {
		players = append(players, PlayerMsg{
			ID:           p.ID,
			Name:         p.Name,
			IsAI:         p.IsAI,
			AIDifficulty: p.AIDifficulty,
		})
	}

	meldMeta := map[string][]MeldMetaMsg{}
	for owner, metas := range g.MeldMeta {
		out := make([]MeldMetaMsg, 0, len(metas))
		for _, mi := range metas {
			out = append(out, MeldMetaMsg{MeldID: mi.MeldID, Type: mi.Type})
		}
		meldMeta[owner] = out
	}

	phaseStr := g.Phase
	return GameStateMsg{
		Type:                        "game_state",
		Status:                      g.Status,
		Game:                        g.GameNumber,
		Round:                       g.Round,
		Phase:                       phaseStr,
		CurrentTurn:                 g.CurrentTurn,
		MyHand:                      myHand,
		DiscardPile:                 g.DiscardPile,
		DeckCount:                   len(g.DrawPile),
		ReshuffleCount:              g.ReshuffleCount,
		CardCounts:                  cardCounts,
		Melds:                       g.Melds,
		MeldMeta:                    meldMeta,
		Players:                     players,
		RoundReqMet:                 g.RoundReqMet,
		TotalScores:                 g.TotalScores,
		WinnerID:                    g.WinnerID,
		IsDraw:                      g.IsDraw,
		InitialMeldMinimum:          g.InitialMeldMinimum,
		DiscardDrawMinRound:         g.DiscardDrawMinRound,
		DiscardDrawnCardPendingMeld: pendingMeldCard,
		RulesProfile:                g.RulesProfile,
	}
}
