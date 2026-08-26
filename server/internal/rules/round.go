package rules

import (
	"fmt"
	"time"
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

	cfg := effectiveRules(state)
	tableMelds := AllTableMelds(state)
	gameScore := map[string]int{}
	for _, pid := range state.TurnOrder {
		if pid == winnerID {
			gameScore[pid] = 0
			continue
		}
		gameScore[pid] = HandPenaltyTotalWithMelds(state.Hands[pid], tableMelds, cfg)
	}

	for _, pid := range state.TurnOrder {
		state.GameScores[pid] = append(state.GameScores[pid], gameScore[pid])
		state.TotalScores[pid] += gameScore[pid]
	}
	// Recorded alongside the scores because it cannot be read back out of
	// them — see GameState.DealWinners.
	state.DealWinners = append(state.DealWinners, winnerID)

	// Advance or end the match.
	if matchIsOver(state, cfg) {
		state.Status = StatusCompleted
		state.Phase = Phase("completed")
		winner, draw := DetermineGameWinner(state)
		state.WinnerID = winner
		state.IsDraw = draw
		return state, nil
	}

	return StartNextGame(state, winnerID)
}

// matchIsOver checks the current profile's match-end condition after a deal
// has just finished scoring (state.TotalScores already updated).
func matchIsOver(state GameState, cfg RulesConfig) bool {
	switch cfg.MatchEndMode {
	case MatchEndAtScore:
		for _, pid := range state.TurnOrder {
			if state.TotalScores[pid] >= cfg.TargetScore {
				return true
			}
		}
		return false
	default: // MatchEndAfterDeals
		return state.GameNumber >= cfg.FixedDealCount
	}
}

// StartMatch builds the opening state for a brand-new match: deal 1, turn
// order as given, first player to act leading. It is the counterpart to
// StartNextGame (which advances an in-progress match to its next deal) and
// both funnel through dealNewGame, so there is exactly one implementation of
// "shuffle, deal, turn the first discard, reset per-deal state". Callers must
// not hand-roll this — that is how the two paths drifted apart before.
func StartMatch(cfg RulesConfig, turnOrder []string, seed int64) (GameState, error) {
	if len(turnOrder) == 0 {
		return GameState{}, fmt.Errorf("no turn order")
	}
	state := GameState{
		Status:      StatusActive,
		Rules:       ResolveConfig(cfg),
		GameNumber:  0, // dealNewGame increments to 1
		Created:     time.Now().UTC(),
		TurnOrder:   append([]string(nil), turnOrder...),
		DeckSeed:    seed,
		Hands:       map[string][]string{},
		Melds:       map[string][][]string{},
		MeldMeta:    map[string][]MeldInfo{},
		RoundReqMet: map[string]bool{},
		GameScores:  map[string][]int{},
		TotalScores: map[string]int{},
	}
	for _, pid := range turnOrder {
		state.GameScores[pid] = []int{}
		state.TotalScores[pid] = 0
	}
	return dealNewGame(state, turnOrder[0])
}

func StartNextGame(state GameState, nextTurnID string) (GameState, error) {
	return dealNewGame(state, nextTurnID)
}

// dealNewGame advances state to its next deal: bumps the deal counter, wipes
// everything that is per-deal, then shuffles a fresh deck, deals every hand
// and turns the first discard. nextTurnID both leads the deal and becomes its
// DealStarterID (the marker the lap counter compares against).
func dealNewGame(state GameState, nextTurnID string) (GameState, error) {
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
	state.DiscardTakenCard = ""
	state.DiscardDrawnCards = nil
	state.LastLayOff = nil
	state.LastMeldLaid = nil
	state.TurnMeldSnapshot = nil

	if state.Hands == nil {
		state.Hands = map[string][]string{}
	}
	for _, pid := range state.TurnOrder {
		state.Hands[pid] = nil
		state.RoundReqMet[pid] = false
		state.Melds[pid] = nil
	}

	cfg := effectiveRules(state)

	// Build new deck & deal.
	deck := BuildDeck(len(state.TurnOrder))
	seed := state.DeckSeed + int64(state.GameNumber)*9973
	state.DrawPile = Shuffle(deck, seed)

	var err error
	state, err = DealHand(state, cfg)
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
