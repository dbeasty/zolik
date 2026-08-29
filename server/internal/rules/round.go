package rules

import (
	"fmt"
	"time"
)

// EndGame scores the deal that just ended and deals the next one, unless the
// match is over.
//
// It is ScoreDeal followed by StartNextGame, and it keeps that shape — and its
// signature — because it is what every existing caller and test means by "the
// deal ended". A caller that needs to stop in between, to put the settlement in
// front of the players before the table is wiped, calls the two halves itself.
func EndGame(state GameState, winnerID string) (GameState, error) {
	ns, over, err := ScoreDeal(state, winnerID)
	if err != nil || over {
		return ns, err
	}
	return StartNextGame(ns, NextDealStarter(ns, effectiveRules(ns), winnerID))
}

// NextDealStarter decides who leads the deal after the one that was just
// scored, per the resolved config's DealStarterMode.
//
// Rotate reads DealStarterID rather than CurrentTurn or TurnOrder[0]: it is
// the marker the deal just finishing recorded as its own leader
// (dealNewGame sets both together), so "the seat after whoever led" is
// correct even when the table has looped past TurnOrder's start. An unknown
// or empty DealStarterID (a state from before this field existed) falls back
// to TurnOrder[0], same as ResumeAfterIntermission already does.
func NextDealStarter(state GameState, cfg RulesConfig, winnerID string) string {
	if cfg.DealStarter == DealStarterWinner {
		return winnerID
	}
	for i, pid := range state.TurnOrder {
		if pid == state.DealStarterID {
			return state.TurnOrder[(i+1)%len(state.TurnOrder)]
		}
	}
	if len(state.TurnOrder) > 0 {
		return state.TurnOrder[0]
	}
	return winnerID
}

// ScoreDeal settles the deal that just ended and stops there, reporting whether
// that finished the match.
func ScoreDeal(state GameState, winnerID string) (GameState, bool, error) {
	if state.GameNumber < 1 {
		return state, false, RulesError{Code: ErrInvalidMeld, Message: "invalid game"}
	}
	if state.TurnOrder == nil {
		return state, false, RulesError{Code: ErrInvalidMeld, Message: "missing turn order"}
	}
	if state.Hands == nil {
		return state, false, RulesError{Code: ErrInvalidMeld, Message: "missing hands"}
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

	if matchIsOver(state, cfg) {
		state.Status = StatusCompleted
		state.Phase = Phase("completed")
		winner, draw := DetermineGameWinner(state)
		state.WinnerID = winner
		state.IsDraw = draw
		return state, true, nil
	}
	return state, false, nil
}

// PauseAfterDeal parks a scored match between deals: nobody is on turn, and the
// board is left exactly as the deal ended so the settlement it is made of can
// still be looked at.
//
// nextStarter is remembered rather than recomputed on resume, because who leads
// the next deal is decided by who went out of this one and that fact is gone as
// soon as the table is dealt again.
func PauseAfterDeal(state GameState, nextStarter string) GameState {
	state.Phase = PhaseIntermission
	state.PendingDealStarter = nextStarter
	state.CurrentTurn = ""
	return state
}

// ResumeAfterIntermission deals the deal the table has been waiting for.
func ResumeAfterIntermission(state GameState) (GameState, error) {
	starter := state.PendingDealStarter
	if starter == "" && len(state.TurnOrder) > 0 {
		starter = state.TurnOrder[0]
	}
	state.PendingDealStarter = ""
	return StartNextGame(state, starter)
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
// order as given, starterID leading. It is the counterpart to StartNextGame
// (which advances an in-progress match to its next deal) and both funnel
// through dealNewGame, so there is exactly one implementation of "shuffle,
// deal, turn the first discard, reset per-deal state". Callers must not
// hand-roll this — that is how the two paths drifted apart before.
//
// starterID picks who leads the opening deal; an empty string falls back to
// turnOrder[0] so a caller with no opinion — a test, a replay of an old
// match — keeps meaning exactly what it always meant. A lobby's own opening
// seat is a runtime concern (see module.StartingSeat), not a rule this
// package decides on its own.
func StartMatch(cfg RulesConfig, turnOrder []string, seed int64, starterID string) (GameState, error) {
	if len(turnOrder) == 0 {
		return GameState{}, fmt.Errorf("no turn order")
	}
	if starterID == "" {
		starterID = turnOrder[0]
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
	return dealNewGame(state, starterID)
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
	state.WentOutByDiscard = false

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
