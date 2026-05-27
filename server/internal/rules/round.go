package rules

import (
	"fmt"
)

func EndRound(state GameState, winnerID string) (GameState, error) {
	if state.Round < 1 {
		return state, RulesError{Code: ErrInvalidMeld, Message: "invalid round"}
	}
	if state.TurnOrder == nil {
		return state, RulesError{Code: ErrInvalidMeld, Message: "missing turn order"}
	}
	if state.Hands == nil {
		return state, RulesError{Code: ErrInvalidMeld, Message: "missing hands"}
	}

	if state.RoundScores == nil {
		state.RoundScores = map[string][]int{}
	}
	if state.TotalScores == nil {
		state.TotalScores = map[string]int{}
	}

	roundScore := map[string]int{}
	for _, pid := range state.TurnOrder {
		if pid == winnerID {
			roundScore[pid] = 0
			continue
		}
		roundScore[pid] = HandPenaltyTotal(state.Hands[pid])
	}

	for _, pid := range state.TurnOrder {
		state.RoundScores[pid] = append(state.RoundScores[pid], roundScore[pid])
		state.TotalScores[pid] += roundScore[pid]
	}

	// Advance or end game.
	if state.Round >= 7 {
		state.Status = StatusCompleted
		state.Phase = Phase("completed")
		return state, nil
	}

	return StartNextRound(state, winnerID)
}

func StartNextRound(state GameState, nextTurnID string) (GameState, error) {
	state.Round++
	state.Phase = PhaseDraw
	state.Offer = nil
	state.ReshuffleCount = 0
	state.DiscardPile = nil
	state.DrawPile = nil

	// Reset per-round state.
	state.Melds = map[string][][]string{}
	state.MeldMeta = map[string][]MeldInfo{}
	state.RoundReqMet = map[string]bool{}
	state.NextMeldSeq = 0

	for _, pid := range state.TurnOrder {
		state.Hands[pid] = nil
		state.RoundReqMet[pid] = false
		state.Melds[pid] = nil
	}

	// Build new deck & deal.
	deck := BuildDeck(len(state.TurnOrder))
	seed := state.DeckSeed + int64(state.Round)*9973
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
	return fmt.Sprintf("round=%d phase=%s turn=%s", s.Round, s.Phase, s.CurrentTurn)
}

