package holdem

import "zolik/server/internal/module"

var _ module.RuleIndexProvider = (*Module)(nil)

// ExplainRefusal names the written rules behind each code this engine can
// return.
//
// Hold'em's refusals are unusually rule-shaped: every one of them except the
// two about a malformed amount is a betting rule stated in the section above,
// and a player told "you can't raise here" is being told the consequence of a
// rule nobody has shown them. This is the mapping from one to the other.
//
// It decides nothing about legality. The refusal has already happened by the
// time anything asks.
func (m *Module) ExplainRefusal(cfg module.MatchConfig, code string) []string {
	return explainWith(module.StatedRuleIDs(m, cfg), code)
}

func explainWith(stated map[string]bool, code string) []string {
	var out []string
	for _, id := range refusalRules(code) {
		if stated[id] {
			out = append(out, id)
		}
	}
	return out
}

func refusalRules(code string) []string {
	switch code {
	case ErrNotYourTurn:
		return []string{"holdem.rules.streets"}
	case ErrSeatNotInHand:
		return []string{"holdem.rules.foldedOut"}

	// --- what you may do when a bet is owed, and when none is -------------
	case ErrCannotCheck, ErrNothingToCall:
		return []string{"holdem.rules.checkOrCall"}

	// --- raising ----------------------------------------------------------
	case ErrRaiseTooSmall:
		return []string{"holdem.rules.minRaise", "holdem.rules.allIn"}
	case ErrCannotRaise, ErrNotEnoughChips:
		return []string{"holdem.rules.allIn"}
	case ErrAmountRequired:
		// A no-limit raise is a figure the player names, which is the rule
		// this refusal is the consequence of.
		return []string{"holdem.rules.noLimit"}

	// --- not about the rules at all ---------------------------------------
	//
	// An amount that is not a number is a client that sent the wrong thing,
	// not a player who broke a rule; wording a poker rule for it would be
	// explaining a bug as a convention.
	case ErrAmountNotNumber, ErrGameNotActive, ErrUnknownAction, "WRONG_PLAYER_COUNT":
		return nil
	}
	return nil
}
