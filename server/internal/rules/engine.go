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
		//
		// The events accumulated so far are prepended to the end-of-deal ones:
		// the play that emptied the hand is itself an event a client needs, and
		// returning only endGameWithEvents' events silently dropped it.
		if effectiveRules(ns).IsFinalDeal(ns.GameNumber) && ns.RoundReqMet[playerID] && len(ns.Hands[playerID]) == 0 {
			outcome, err := endGameWithEvents(ns, playerID)
			if err != nil {
				return outcome, err
			}
			outcome.Events = append(events, outcome.Events...)
			return outcome, nil
		}
		return ApplyOutcome{State: ns, Events: events}, nil

	case ActionLayOff:
		cards := action.Cards
		if len(cards) == 0 && action.Card != "" {
			cards = []string{action.Card}
		}
		// A single natural card dropped onto a meld that holds a joker is
		// tried as a joker swap first, ahead of an ordinary lay-off. Two
		// reasons: a set happily accepts a redundant natural alongside the
		// joker it duplicates (lay-off would silently "succeed" while
		// leaving the joker stuck — no error to react to), and a run can
		// often satisfy a gap-filling card by re-resolving the wild onto a
		// different end instead of releasing it. Both leave the player
		// unable to reclaim a joker they clearly meant to take back, which
		// is exactly what dragging the exact matching card onto the meld
		// (rather than using the explicit "Swap joker here" button) is
		// this drag-and-drop shortcut for. Only single-card drops qualify —
		// a multi-card lay-off is unambiguous and shouldn't silently become
		// something else.
		if len(cards) == 1 && !IsJoker(cards[0]) {
			if swapNs, swapErr := ValidateSwapJoker(state, playerID, action.MeldID, cards[0]); swapErr == nil {
				events = append(events, ev("joker_swapped", map[string]interface{}{
					"playerId": playerID,
					"meldId":   action.MeldID,
					"card":     cards[0],
				}))
				return ApplyOutcome{State: swapNs, Events: events}, nil
			}
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
		//
		// The events accumulated so far are prepended to the end-of-deal ones:
		// the play that emptied the hand is itself an event a client needs, and
		// returning only endGameWithEvents' events silently dropped it.
		if effectiveRules(ns).IsFinalDeal(ns.GameNumber) && ns.RoundReqMet[playerID] && len(ns.Hands[playerID]) == 0 {
			outcome, err := endGameWithEvents(ns, playerID)
			if err != nil {
				return outcome, err
			}
			outcome.Events = append(events, outcome.Events...)
			return outcome, nil
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
		ns, goOut, err := ValidateDiscard(state, playerID, action.Card, action.CardIndex)
		if err != nil {
			return ApplyOutcome{State: state}, err
		}
		events = append(events, ev("player_discarded", map[string]interface{}{
			"playerId": playerID,
			"card":     action.Card,
		}))
		if goOut {
			outcome, err := endGameWithEvents(ns, playerID)
			if err != nil {
				return outcome, err
			}
			outcome.Events = append(events, outcome.Events...)
			return outcome, nil
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
