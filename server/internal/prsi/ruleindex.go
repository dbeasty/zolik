package prsi

import "zolik/server/internal/module"

var _ module.RuleIndexProvider = (*Module)(nil)

// ExplainRefusal names the written rules behind each code this engine can
// return.
//
// Prší states the same rules at every table — the one option is how many cards
// are dealt — so nothing here branches on the config. It is still filtered
// against what Rules() states, because an id that stops being written is a
// pointer into nothing, and that should fail the build rather than reach a
// player.
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
	case ErrNotYourTurn, ErrCardDoesNotFit:
		return []string{"prsi.rules.turn.match"}
	case ErrNothingToDraw:
		return []string{"prsi.rules.turn.draw"}

	// The three special cards, each of which is a rule a player meets first as
	// a refusal: a seven you have to answer, an ace you have to take, a queen
	// that will not go down until you say what follows it.
	case ErrMustAnswerDraw:
		return []string{"prsi.rules.sevens"}
	case ErrNothingToSkip:
		return []string{"prsi.rules.aces"}
	case ErrSuitRequired, ErrUnknownSuit:
		return []string{"prsi.rules.queens"}

	// --- not about the rules at all ---------------------------------------
	case ErrCardNotInHand, ErrGameNotActive, ErrUnknownAction, "TOO_FEW_PLAYERS":
		return nil
	}
	return nil
}
