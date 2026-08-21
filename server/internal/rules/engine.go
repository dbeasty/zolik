package rules

import "time"

// ApplyAction validates and applies an action to game state.
// Pure: no DB/network/IO. Any persistence/broadcast is the caller's responsibility.
func ApplyAction(state GameState, playerID string, action Action) (ApplyOutcome, error) {
	if state.GameNumber == 0 {
		state.GameNumber = 1
	}
	if state.Round == 0 {
		state.Round = 1
	}
	if state.DealStarterID == "" && len(state.TurnOrder) > 0 {
		state.DealStarterID = state.TurnOrder[0]
	}
	if state.RoundReqMet == nil {
		state.RoundReqMet = map[string]bool{}
	}
	if state.Hands == nil {
		state.Hands = map[string][]string{}
	}
	if state.Created.IsZero() {
		state.Created = time.Now().UTC()
	}

	var events []StateEvent

	switch action.Type {
	case ActionDrawCard:
		beforeReshuffle := state.ReshuffleCount
		ns, card, _, err := ValidateDraw(state, playerID, action.DrawFrom, action.Card)
		if err != nil {
			return ApplyOutcome{State: state}, err
		}
		if ns.ReshuffleCount > beforeReshuffle {
			events = append(events, ev("reshuffle", map[string]interface{}{
				"reshuffleCount": ns.ReshuffleCount,
			}))
		}
		if action.DrawFrom == DrawFromDeck {
			events = append(events, ev("draw_deck", map[string]interface{}{
				"playerId": playerID,
				"card":     card, // private; redact in replay for other players
			}))
			events = append(events, ev("player_drew", map[string]interface{}{
				"playerId":      playerID,
				"from":          "deck",
				"deckRemaining": len(ns.DrawPile),
			}))
		} else {
			events = append(events, ev("draw_discard", map[string]interface{}{
				"playerId": playerID,
				"card":     card,
			}))
			events = append(events, ev("player_drew", map[string]interface{}{
				"playerId": playerID,
				"from":     "discard",
			}))
		}
		return ApplyOutcome{State: ns, Events: events}, nil

	case ActionUndoDrawDiscard:
		ns, err := ValidateUndoDrawDiscard(state, playerID)
		if err != nil {
			return ApplyOutcome{State: state}, err
		}
		events = append(events, ev("undo_draw_discard", map[string]interface{}{
			"playerId": playerID,
		}))
		return ApplyOutcome{State: ns, Events: events}, nil

	case ActionLayMeld:
		ns, meldID, meldType, err := ValidateMeldAction(state, playerID, action.Cards)
		if err != nil {
			return ApplyOutcome{State: state}, err
		}
		events = append(events, ev("meld_played", map[string]interface{}{
			"playerId": playerID,
			"meldId":   meldID,
			"cards":    action.Cards,
			"meldType": string(meldType),
		}))
		// Melding/laying off away the last card only ends the deal on a
		// profile's final deal — every other deal requires a closing discard
		// (ValidateMeldAction/ValidateLayOff reject an emptying play there),
		// so this must ask the ruleset rather than assume Continental's deal 7.
		if effectiveRules(ns).IsFinalDeal(ns.GameNumber) && ns.RoundReqMet[playerID] && len(ns.Hands[playerID]) == 0 {
			return endGameWithEvents(ns, playerID)
		}
		return ApplyOutcome{State: ns, Events: events}, nil

	case ActionLayOff:
		cards := action.Cards
		if len(cards) == 0 && action.Card != "" {
			cards = []string{action.Card}
		}
		ns, err := ValidateLayOff(state, playerID, action.MeldID, cards, action.Position)
		if err != nil {
			return ApplyOutcome{State: state}, err
		}
		events = append(events, ev("card_laid_off", map[string]interface{}{
			"playerId": playerID,
			"meldId":   action.MeldID,
			"card":     cards[0],
			"cards":    cards,
		}))
		// Melding/laying off away the last card only ends the deal on a
		// profile's final deal — every other deal requires a closing discard
		// (ValidateMeldAction/ValidateLayOff reject an emptying play there),
		// so this must ask the ruleset rather than assume Continental's deal 7.
		if effectiveRules(ns).IsFinalDeal(ns.GameNumber) && ns.RoundReqMet[playerID] && len(ns.Hands[playerID]) == 0 {
			return endGameWithEvents(ns, playerID)
		}
		return ApplyOutcome{State: ns, Events: events}, nil

	case ActionUndoLayOff:
		ns, err := ValidateUndoLayOff(state, playerID)
		if err != nil {
			return ApplyOutcome{State: state}, err
		}
		events = append(events, ev("undo_lay_off", map[string]interface{}{
			"playerId": playerID,
		}))
		return ApplyOutcome{State: ns, Events: events}, nil

	case ActionUndoLayMeld:
		ns, err := ValidateUndoLayMeld(state, playerID)
		if err != nil {
			return ApplyOutcome{State: state}, err
		}
		events = append(events, ev("undo_lay_meld", map[string]interface{}{
			"playerId": playerID,
		}))
		return ApplyOutcome{State: ns, Events: events}, nil

	case ActionUndoTurn:
		ns, err := ValidateUndoTurn(state, playerID)
		if err != nil {
			return ApplyOutcome{State: state}, err
		}
		events = append(events, ev("undo_turn", map[string]interface{}{
			"playerId": playerID,
		}))
		return ApplyOutcome{State: ns, Events: events}, nil

	case ActionSwapJoker:
		ns, err := ValidateSwapJoker(state, playerID, action.MeldID, action.Card)
		if err != nil {
			return ApplyOutcome{State: state}, err
		}
		events = append(events, ev("joker_swapped", map[string]interface{}{
			"playerId": playerID,
			"meldId":   action.MeldID,
			"card":     action.Card,
		}))
		return ApplyOutcome{State: ns, Events: events}, nil

	case ActionDiscard:
		ns, goOut, err := ValidateDiscard(state, playerID, action.Card)
		if err != nil {
			return ApplyOutcome{State: state}, err
		}
		events = append(events, ev("player_discarded", map[string]interface{}{
			"playerId": playerID,
			"card":     action.Card,
		}))
		if goOut {
			return endGameWithEvents(ns, playerID)
		}
		return ApplyOutcome{State: ns, Events: events}, nil

	default:
		return ApplyOutcome{State: state}, RulesError{Code: ErrInvalidMeld, Message: "unknown action"}
	}
}

func endGameWithEvents(state GameState, winnerID string) (ApplyOutcome, error) {
	endedGame := state.GameNumber
	handsAtEnd := allHandsForLog(state)
	ns, err := EndGame(state, winnerID)
	if err != nil {
		return ApplyOutcome{State: state}, err
	}
	events := []StateEvent{ev("deal_ended", map[string]interface{}{
		"winnerId": winnerID,
		"game":     endedGame,
		"scores":   lastRoundScores(ns),
		"allHands": handsAtEnd,
	})}
	if ns.Status == StatusCompleted {
		events = append(events, ev("game_ended", map[string]interface{}{
			"winnerId":    ns.WinnerID,
			"isDraw":      ns.IsDraw,
			"finalScores": ns.TotalScores,
		}))
	}
	return ApplyOutcome{State: ns, Events: events}, nil
}
