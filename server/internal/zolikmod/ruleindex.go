package zolikmod

import (
	"zolik/server/internal/module"
	"zolik/server/internal/rules"
)

var _ module.RuleIndexProvider = (*Module)(nil)

// ExplainRefusal names the written rules behind each code the Žolíky
// validators can return.
//
// A declared table rather than something derived, because which rule causes a
// refusal is an editorial judgement — but it is checked at both ends, and that
// is what keeps it honest. Every code the engine can emit must appear here,
// and every id it returns must be a sentence ruleSections actually builds for
// the config in force; ruleindex_test.go fails the build otherwise.
//
// It decides nothing about legality. A wrong entry here mislabels an
// explanation; it cannot make an illegal move legal, because the refusal has
// already happened by the time anything asks.
func (m *Module) ExplainRefusal(mc module.MatchConfig, code string) []string {
	return explainRefusal(resolveConfig(mc), code)
}

func explainRefusal(cfg rules.RulesConfig, code string) []string {
	return nonEmpty(refusalRules(cfg, code))
}

// nonEmpty drops the placeholders the helpers below return when a table
// states no sentence of that kind at all — a config with no contract has no
// contract rule to point at, and a pointer at nothing is worse than one
// fewer rule.
func nonEmpty(ids []string) []string {
	out := ids[:0]
	for _, id := range ids {
		if id != "" {
			out = append(out, id)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func refusalRules(cfg rules.RulesConfig, code string) []string {
	switch rules.RulesErrorCode(code) {

	// --- whose turn it is, and how far into it ---------------------------
	case rules.ErrNotYourTurn, rules.ErrWrongPhase, rules.ErrMustDrawFirst:
		return []string{"zolik.rules.turn.draw", "zolik.rules.turn.discard"}

	// --- drawing ----------------------------------------------------------
	case rules.ErrDiscardLocked:
		// Only ever returned while the pile is locked, which is the only
		// config in which the sentence naming the round exists.
		if cfg.DiscardDrawMinRound > 1 {
			return []string{"zolik.rules.pickup.locked"}
		}
		return []string{"zolik.rules.pickup.open"}
	case rules.ErrDiscardPileEmpty:
		return []string{pickupModeRule(cfg)}
	case rules.ErrNoCardsLeft:
		return []string{"zolik.rules.deck.reshuffle"}

	// --- what a pickup obliges you to do ----------------------------------
	case rules.ErrDiscardCardNotMelded:
		return []string{"zolik.rules.pickup.obligation", pickupModeRule(cfg)}
	case rules.ErrDiscardTakenCard:
		return []string{"zolik.rules.pickup.noReturn"}
	case rules.ErrIncompleteInitialMeld:
		return []string{"zolik.rules.pickup.obligation", "zolik.rules.turn.discard"}

	// --- meld shapes ------------------------------------------------------
	case rules.ErrInvalidMeld:
		return []string{"zolik.rules.meldShapes"}
	case rules.ErrTooManyWilds:
		return []string{"zolik.rules.wilds.setLimit"}
	case rules.ErrSetTooLarge:
		return []string{"zolik.rules.set.maxSize"}
	case rules.ErrRunTooLong:
		return []string{"zolik.rules.run.maxLength"}
	case rules.ErrAceBridge:
		return []string{"zolik.rules.run.aceBridge"}
	case rules.ErrAdjacentWilds:
		// Declared in the engine's vocabulary and returned from nowhere. It
		// keeps a mapping so that implementing the rule is a one-line change
		// here rather than a test failure at the far end of the build.
		return []string{"zolik.rules.wilds.setLimit"}

	// --- going down -------------------------------------------------------
	case rules.ErrMeldBelowMinimum:
		if cfg.InitialMeldMinimum > 0 {
			return []string{"zolik.rules.meldFloor.on"}
		}
		return []string{"zolik.rules.meldFloor.off"}
	case rules.ErrNeedCleanRun:
		if cfg.ContractFor(1).RequireCleanRun {
			return []string{"zolik.rules.cleanRun.on"}
		}
		return []string{"zolik.rules.cleanRun.off"}
	case rules.ErrMeldNoContribution:
		return []string{"zolik.rules.contracts.contribution", contractRule(cfg)}

	// --- touching what is already on the table ----------------------------
	case rules.ErrRoundReqNotMet:
		return []string{"zolik.rules.layoff.afterDown", contractRule(cfg)}
	case rules.ErrWrongRunEnd:
		return []string{"zolik.rules.layoff.runEnds"}
	case rules.ErrNoJokerInMeld, rules.ErrJokerSwapMismatch:
		return []string{"zolik.rules.jokers.swap"}

	// --- discarding -------------------------------------------------------
	case rules.ErrJokerDiscard:
		if cfg.JokerDiscardRestricted {
			return []string{"zolik.rules.jokers.restricted"}
		}
		return []string{"zolik.rules.turn.discard"}

	// --- not about the rules at all ---------------------------------------
	//
	// A card you are not holding, an undo with nothing to undo, a table that
	// is paused: real refusals, but no written rule explains them and none
	// should be invented to. Nil, not a guess — the sheet shows the reason
	// and the remedy and simply has no rule to offer.
	case rules.ErrCardNotInHand, rules.ErrNothingToUndo,
		rules.ErrGameSuspended, rules.ErrGameNotActive:
		return nil
	}
	return nil
}

// pickupModeRule is whichever of the two discard-pile sentences this table
// states, so a refusal about the pile points at the rule that describes it
// rather than at one describing the other house rule.
func pickupModeRule(cfg rules.RulesConfig) string {
	if cfg.DiscardPickupMode == rules.DiscardPickupTopOnly {
		return "zolik.rules.pickup.topOnly"
	}
	return "zolik.rules.pickup.anyFromPile"
}

// contractRule is the sentence stating what this deal asks for — a different
// sentence depending on whether the contract rotates, and no sentence at all
// on a table that asks for nothing in particular.
func contractRule(cfg rules.RulesConfig) string {
	if cfg.FixedDealCount > 0 {
		return "zolik.rules.contracts.rotating"
	}
	if c := cfg.ContractFor(1); c.Sets > 0 || c.Runs > 0 {
		return "zolik.rules.contracts.static"
	}
	return ""
}
