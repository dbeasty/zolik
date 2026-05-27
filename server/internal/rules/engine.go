package rules

import "time"

// ApplyAction validates and applies an action to game state.
// Pure: no DB/network/IO. Any persistence/broadcast is the caller's responsibility.
func ApplyAction(state GameState, playerID string, action Action) (GameState, error) {
	if state.Round == 0 {
		state.Round = 1
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

	switch action.Type {
	case ActionDrawCard:
		ns, _, _, err := ValidateDraw(state, playerID, action.DrawFrom)
		return ns, err

	case ActionLayMeld:
		ns, _, err := ValidateMeldAction(state, playerID, action.Cards)
		if err != nil {
			return state, err
		}
		if ns.Round == 7 && ns.RoundReqMet[playerID] && len(ns.Hands[playerID]) == 0 {
			return EndRound(ns, playerID)
		}
		return ns, nil

	case ActionLayOff:
		ns, err := ValidateLayOff(state, playerID, action.MeldID, action.Card)
		if err != nil {
			return state, err
		}
		if ns.Round == 7 && ns.RoundReqMet[playerID] && len(ns.Hands[playerID]) == 0 {
			return EndRound(ns, playerID)
		}
		return ns, nil

	case ActionDiscard:
		ns, goOut, err := ValidateDiscard(state, playerID, action.Card)
		if err != nil {
			return state, err
		}
		if goOut {
			return EndRound(ns, playerID)
		}
		return ns, nil

	case ActionAcceptOffer:
		ns, _, _, err := ValidateAcceptOffer(state, playerID)
		if err != nil {
			return state, err
		}
		return ns, nil

	case ActionDeclineOffer:
		ns, err := ValidateDeclineOffer(state, playerID)
		if err != nil {
			return state, err
		}
		return ns, nil

	default:
		return state, RulesError{Code: ErrInvalidMeld, Message: "unknown action"}
	}
}

