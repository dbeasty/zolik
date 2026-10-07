package lastcard

import "zolik/server/internal/module"

var _ module.RuleIndexProvider = (*Module)(nil)

// ExplainRefusal names the written rules behind each code this engine can
// return, filtered against what Rules() states so an id that stops being
// written fails the build rather than reaching a player.
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
	case ErrNotYourTurn, ErrCardDoesNotMatch:
		return []string{"lastcard.rules.turn.match"}
	case ErrOnlyDrawnCard, ErrAlreadyDrew, ErrNothingToKeep:
		return []string{"lastcard.rules.turn.draw"}
	case ErrDrawFourHeld:
		return []string{"lastcard.rules.wildDrawFour"}
	case ErrColourRequired, ErrUnknownColour:
		return []string{"lastcard.rules.wild"}
	case ErrAnswerDrawFour, ErrNoDrawFour:
		return []string{"lastcard.rules.challenge"}
	case ErrNothingToCatch, ErrCallNotNow, ErrAlreadyCalled:
		return []string{"lastcard.rules.call", "lastcard.rules.catch"}

	// --- not about the rules at all ---------------------------------------
	case ErrCardNotInHand, ErrGameNotActive, ErrUnknownAction, ErrTooFewPlayers,
		module.ErrNotPaused, module.ErrAlreadyReady, module.ErrNotSeated:
		return nil
	}
	return nil
}
