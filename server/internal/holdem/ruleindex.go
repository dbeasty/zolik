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
		// Whichever of the two games is being played states one of these.
		return []string{"holdem.rules.streets", "holdem.rules.draw.streets", "holdem.rules.draw.draw"}

	// --- the draw ---------------------------------------------------------
	case ErrDrawing, ErrNotDrawing:
		return []string{"holdem.rules.draw.streets", "holdem.rules.draw.draw"}
	case ErrDrawTooMany, ErrDrawEmpty, ErrCardNotInHand:
		return []string{"holdem.rules.draw.draw"}
	case ErrSeatNotInHand:
		return []string{"holdem.rules.foldedOut"}

	// --- what you may do when a bet is owed, and when none is -------------
	case ErrCannotCheck, ErrNothingToCall:
		return []string{"holdem.rules.checkOrCall"}

	// --- raising ----------------------------------------------------------
	case ErrRaiseTooSmall:
		return []string{"holdem.rules.minRaise", "holdem.rules.allIn", "holdem.rules.omaha.allIn"}
	case ErrCannotRaise, ErrNotEnoughChips:
		return []string{"holdem.rules.allIn", "holdem.rules.omaha.allIn"}
	case ErrOverPotLimit:
		return []string{"holdem.rules.potLimit", "holdem.rules.omaha.allIn"}
	case ErrAmountRequired:
		// A raise is a figure the player names, within the limit the table
		// plays — which is the rule this refusal is the consequence of.
		return []string{"holdem.rules.noLimit", "holdem.rules.potLimit"}

	// --- showing a hand ---------------------------------------------------
	//
	// Both point at the right to show, because both are refusals of it: one
	// says the hand is already face up, the other that there is no hand to
	// turn over. The "once" sentence carries the first of those.
	case ErrAlreadyShown:
		return []string{"holdem.rules.showOnce", "holdem.rules.showYourOwn"}
	case ErrNothingToShow:
		return []string{"holdem.rules.showYourOwn"}

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
