package rummytiles

import "zolik/server/internal/module"

var _ module.RuleIndexProvider = (*Module)(nil)

// ExplainRefusal names the written rules behind each code this engine can
// return.
//
// The mapping is an editorial judgement — which sentence explains a refusal is
// not something a machine can decide — but it is checked at both ends:
// ruleindex_test.go walks every table this lobby can create and fails the
// build if a code goes unexplained or an id names a sentence the table never
// states.
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
		return []string{"rummytiles.rules.turnDescription"}

	// --- what a turn has to add up to -------------------------------------
	//
	// This game's refusals are nearly all one rule: a turn must end with the
	// whole table valid, every tile out of the tray, and at least one tile
	// played from hand. Three codes, one sentence.
	case ErrTrayNotEmpty, ErrNothingPlayed:
		return []string{"rummytiles.rules.turnDescription", "rummytiles.rules.noDiscard"}
	case ErrTableNotValid:
		return []string{
			"rummytiles.rules.turnDescription",
			"rummytiles.rules.group", "rummytiles.rules.run", "rummytiles.rules.noWrap",
		}

	// --- the initial meld -------------------------------------------------
	case ErrInitialMeldLow, ErrInitialMeldOnly:
		return []string{"rummytiles.rules.initialMeldDescription"}

	// --- what fits where --------------------------------------------------
	case ErrTileDoesNotFit:
		return []string{"rummytiles.rules.group", "rummytiles.rules.run", "rummytiles.rules.noWrap"}
	case ErrNotARun, ErrBadSplitPosition:
		return []string{"rummytiles.rules.run"}

	// --- jokers -----------------------------------------------------------
	case ErrNoJokerInSet, ErrJokerSwapMismatch:
		return []string{"rummytiles.rules.jokerTakingDescription", "rummytiles.rules.joker"}

	// --- drawing ----------------------------------------------------------
	//
	// Declared and currently unreachable — an empty pool ends the round
	// rather than refusing the draw. Mapped anyway so that making it
	// reachable is a one-line change here rather than a test failure at the
	// far end of the build.
	case ErrNothingToDraw:
		return []string{"rummytiles.rules.noDiscard"}

	// --- not about the rules at all ---------------------------------------
	case ErrTileNotInHand, ErrNoSuchSet, ErrGameNotActive,
		ErrUnknownAction, ErrWrongPlayerCount:
		return nil
	}
	return nil
}
