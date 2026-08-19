package rules

import (
	"fmt"
)

func EndGame(state GameState, winnerID string) (GameState, error) {
	if state.GameNumber < 1 {
		return state, RulesError{Code: ErrInvalidMeld, Message: "invalid game"}
	}
	if state.TurnOrder == nil {
		return state, RulesError{Code: ErrInvalidMeld, Message: "missing turn order"}
	}
	if state.Hands == nil {
		return state, RulesError{Code: ErrInvalidMeld, Message: "missing hands"}
	}

	if state.GameScores == nil {
		state.GameScores = map[string][]int{}
	}
	if state.TotalScores == nil {
		state.TotalScores = map[string]int{}
	}

	tableMelds := AllTableMelds(state)
	gameScore := map[string]int{}
	for _, pid := range state.TurnOrder {
		if pid == winnerID {
			gameScore[pid] = 0
			continue
		}
		gameScore[pid] = HandPenaltyTotalWithMelds(state.Hands[pid], tableMelds)
	}

	for _, pid := range state.TurnOrder {
		state.GameScores[pid] = append(state.GameScores[pid], gameScore[pid])
		state.TotalScores[pid] += gameScore[pid]
	}

	// Advance or end the match.
	if state.GameNumber >= 7 {
		state.Status = StatusCompleted
		state.Phase = Phase("completed")
		winner, draw := DetermineGameWinner(state)
		state.WinnerID = winner
		state.IsDraw = draw
		return state, nil
	}

	return StartNextGame(state, winnerID)
}

func StartNextGame(state GameState, nextTurnID string) (GameState, error) {
	state.GameNumber++
	state.Round = 1
	state.DealStarterID = nextTurnID
	state.Phase = PhaseDraw
	state.ReshuffleCount = 0
	state.DiscardPile = nil
	state.DrawPile = nil

	// Reset per-game state.
	state.Melds = map[string][][]string{}
	state.MeldMeta = map[string][]MeldInfo{}
	state.RoundReqMet = map[string]bool{}
	state.NextMeldSeq = 0
	state.MeldsLaidThisTurn = 0
	state.DiscardDrawnCardPendingMeld = ""

	for _, pid := range state.TurnOrder {
		state.Hands[pid] = nil
		state.RoundReqMet[pid] = false
		state.Melds[pid] = nil
	}

	// Build new deck & deal.
	deck := BuildDeck(len(state.TurnOrder))
	seed := state.DeckSeed + int64(state.GameNumber)*9973
	state.DrawPile = Shuffle(deck, seed)

	// Deal 12.
	var err error
	state, err = Deal12(state)
	if err != nil {
		return state, err
	}
	if len(state.DrawPile) == 0 {
		return state, RulesError{Code: ErrNoCardsLeft, Message: "no cards for initial discard"}
	}
	// Initialize discard with one top card.
	top := state.DrawPile[len(state.DrawPile)-1]
	state.DrawPile = state.DrawPile[:len(state.DrawPile)-1]
	state.DiscardPile = []string{top}

	// Next turn begins with the last winner (simplified choice).
	state.CurrentTurn = nextTurnID

	return state, nil
}

func (s GameState) String() string {
	return fmt.Sprintf("game=%d round=%d phase=%s turn=%s", s.GameNumber, s.Round, s.Phase, s.CurrentTurn)
}
