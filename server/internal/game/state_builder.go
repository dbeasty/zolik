package game

import (
	"zolik/server/internal/models"
	"zolik/server/internal/rules"
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
	DiscardLocked               bool                     `json:"discardLocked"`
	DiscardDrawnCardPendingMeld string                   `json:"discardDrawnCardPendingMeld,omitempty"`
	CanUndoDiscardDraw          bool                     `json:"canUndoDiscardDraw,omitempty"`
	CanUndoLayOff               bool                     `json:"canUndoLayOff,omitempty"`
	CanUndoLayMeld              bool                     `json:"canUndoLayMeld,omitempty"`
	CanUndoTurn                 bool                     `json:"canUndoTurn,omitempty"`
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
	var canUndoDiscardDraw bool
	var canUndoLayOff bool
	var canUndoLayMeld bool
	var canUndoTurn bool
	if g.CurrentTurn == myPlayerID {
		pendingMeldCard = g.DiscardDrawnCardPendingMeld
		canUndoDiscardDraw = len(g.DiscardDrawnCards) > 0
		canUndoLayOff = g.LastLayOff != nil && g.LastLayOff.PlayerID == myPlayerID
		canUndoLayMeld = g.LastMeldLaid != nil && g.LastMeldLaid.PlayerID == myPlayerID
		// Available any time this player's turn has a snapshot to fall back
		// to, independent of whether the single most-recent action also
		// happens to still be undoable on its own (e.g. after a swap_joker,
		// which isn't individually undoable but is still covered here).
		canUndoTurn = g.TurnMeldSnapshot != nil && g.TurnMeldSnapshot.PlayerID == myPlayerID
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
	// Rule values on the wire come from the game's resolved ruleset, not the
	// legacy scalar columns, so a document written before the ruleset was
	// persisted still reports the same numbers the engine is enforcing.
	cfg := GameRules(g)
	return GameStateMsg{
		Type:                "game_state",
		Status:              g.Status,
		Game:                g.GameNumber,
		Round:               g.Round,
		Phase:               phaseStr,
		CurrentTurn:         g.CurrentTurn,
		MyHand:              myHand,
		DiscardPile:         g.DiscardPile,
		DeckCount:           len(g.DrawPile),
		ReshuffleCount:      g.ReshuffleCount,
		CardCounts:          cardCounts,
		Melds:               g.Melds,
		MeldMeta:            meldMeta,
		Players:             players,
		RoundReqMet:         g.RoundReqMet,
		TotalScores:         g.TotalScores,
		WinnerID:            g.WinnerID,
		IsDraw:              g.IsDraw,
		InitialMeldMinimum:  cfg.InitialMeldMinimum,
		DiscardDrawMinRound: cfg.DiscardDrawMinRound,
		// origin/main's new wire field, but derived from the resolved
		// ruleset like its two neighbours above rather than from the legacy
		// scalar columns, so it agrees with what the engine enforces.
		DiscardLocked:               rules.IsDiscardLocked(g.Round, cfg.DiscardDrawMinRound),
		DiscardDrawnCardPendingMeld: pendingMeldCard,
		CanUndoDiscardDraw:          canUndoDiscardDraw,
		CanUndoLayOff:               canUndoLayOff,
		CanUndoLayMeld:              canUndoLayMeld,
		CanUndoTurn:                 canUndoTurn,
		RulesProfile:                g.RulesProfile,
	}
}
