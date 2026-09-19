package canasta

import "zolik/server/internal/module"

var _ module.RuleIndexProvider = (*Module)(nil)

// ExplainRefusal names the written rules behind each code this engine can
// return.
//
// A declared table rather than something derived, because which rule causes a
// refusal is an editorial judgement — but it is checked at both ends, and that
// is what keeps it honest. Every code the engine can emit must appear here,
// and every id it hands back must be a sentence Rules() actually builds for
// the variation in force; ruleindex_test.go fails the build otherwise.
//
// It decides nothing about legality. The refusal has already happened by the
// time anything asks; a wrong entry here mislabels an explanation, it cannot
// make an illegal move legal.
func (m *Module) ExplainRefusal(cfg module.MatchConfig, code string) []string {
	return explainWith(module.StatedRuleIDs(m, cfg), resolveVariation(cfg.Variation), code)
}

// explainWith is the mapping filtered against the sentences a table actually
// states, so an id can never point at a rule the player cannot go and read.
//
// That filter is what lets the table below name the ideal sentence for a
// refusal without every arm re-deriving which variation states it — the
// permanent freeze and the buried-wild freeze are both listed for PILE_FROZEN,
// and each table keeps the one it has.
//
// The stated set is passed in rather than rebuilt because the offer list asks
// this once per disabled offer, and rebuilding a whole rules listing per
// refusal would put that on every broadcast.
func explainWith(stated map[string]bool, v ruleset, code string) []string {
	var out []string
	for _, id := range refusalRules(v, code) {
		if id != "" && stated[id] {
			out = append(out, id)
		}
	}
	return out
}

// statedRules is the id set for the table a deal is actually being played
// under, reconstructed from the state's own resolved numbers rather than from
// the lobby's options — a match keeps the rules it was dealt under, and the
// rules a refusal points at have to be those same ones.
func (m *Module) statedRules(s *GameState) map[string]bool {
	return module.StatedRuleIDs(m, module.MatchConfig{
		Variation: s.Variation,
		Options: module.Options{
			OptHandSize:        s.HandSize,
			OptTargetScore:     s.TargetScore,
			OptCanastasToGoOut: s.CanastasToGoOut,
		},
	})
}

func refusalRules(v ruleset, code string) []string {
	switch code {

	// --- whose turn it is, and how far into it ----------------------------
	//
	// The single largest group, and the one this index was written for.
	// WRONG_PHASE is the refusal a player meets most and the one a bare code
	// explains least: it means "you have not drawn yet" and "you have already
	// drawn" at the same table, ten seconds apart.
	case ErrNotYourTurn, ErrWrongPhase:
		return []string{"canasta.rules.turn", "canasta.rules.turnDiscard"}
	case ErrNothingToDraw, ErrPileEmpty:
		return []string{"canasta.rules.turn"}

	// --- getting into the discard pile ------------------------------------
	case ErrTopCardUnusable:
		return []string{"canasta.rules.pileTopCard"}
	case ErrPileBlocked:
		return []string{"canasta.rules.pileBlocked"}
	case ErrPileFrozen:
		// Whichever freeze this table has. A variation frozen all deal states
		// only the permanent sentence, and the filter drops the other.
		return []string{"canasta.rules.pileAlwaysFrozen", "canasta.rules.pileFrozenByWild"}
	case ErrMeldCaptureNotAllowed:
		return []string{"canasta.rules.pileNoMeldCapture"}
	case ErrCaptureNeedsTwo:
		// What the capture costs, in whichever of the three sentences this
		// variation states it.
		return []string{
			"canasta.rules.pileNoMeldCapture", "canasta.rules.pileAlwaysFrozen",
			"canasta.rules.pileOntoMeld", "canasta.rules.pileTopCard",
		}

	// --- what a meld is ---------------------------------------------------
	case ErrMeldTooSmall, ErrMeldMixedRanks:
		return []string{"canasta.rules.meldShape"}
	case ErrWrongRank:
		return []string{"canasta.rules.meldShape", "canasta.rules.sequences"}
	case ErrMeldTooLarge, ErrMeldClosed:
		// A group closes at seven where a canasta closes, and a sequence
		// always does — so a table with sambas states the samba sentence and
		// one without states the canasta one.
		return []string{"canasta.rules.canastaCloses", "canasta.rules.samba", "canasta.rules.canasta"}
	case ErrTooManyWilds, ErrNotEnoughNaturals:
		return []string{"canasta.rules.wildLimit", "canasta.rules.wildRatio"}
	case ErrRankAlreadyMelded:
		return []string{"canasta.rules.oneMeldPerRank"}
	case ErrSequenceNoWilds, ErrSequenceNeedsOneSuit, ErrRunNotConsecutive:
		return []string{"canasta.rules.sequences"}

	// --- threes -----------------------------------------------------------
	//
	// Both codes point at whichever black-three sentence this variation
	// states, and CANNOT_MELD_THREE adds the red-three rule: a player refused
	// for trying to meld a three is holding one kind or the other, and the
	// two are refused for entirely different reasons.
	case ErrCannotMeldThree:
		return []string{
			"canasta.rules.redThrees",
			"canasta.rules.blackThreesNeverMeld", "canasta.rules.blackThreesGoOut",
		}
	case ErrBlackThreeGoOutOnly:
		return []string{"canasta.rules.blackThreesGoOut", "canasta.rules.blackThreesNeverMeld"}
	case ErrCannotDiscardThree:
		return []string{"canasta.rules.redThrees"}

	// --- whose melds ------------------------------------------------------
	case ErrNotYourMeld:
		return []string{"canasta.rules.meldsAreShared"}

	// --- opening, and going out -------------------------------------------
	case ErrInitialMeldNotMet:
		return []string{"canasta.rules.meldFloorBands", "canasta.rules.meldFloorBandsFive"}
	case ErrMustMeldFirst:
		return []string{
			"canasta.rules.layOffAfterOpening",
			"canasta.rules.meldFloorBands", "canasta.rules.meldFloorBandsFive",
		}
	case ErrCannotGoOutYet:
		return []string{
			"canasta.rules.oneCanastaToGoOut", "canasta.rules.twoCanastasToGoOut",
			"canasta.rules.goOutKeepsACard",
		}
	case ErrMustKeepACard:
		return []string{"canasta.rules.goOutKeepsACard", "canasta.rules.turnDiscard"}

	// --- not about the rules at all ---------------------------------------
	//
	// A card you are not holding, a meld id that is not on the table, an undo
	// with nothing behind it or asked for out of order, a table that is not
	// running. Real refusals, but no written rule explains them and none should
	// be invented to — the sheet shows the reason and the remedy and simply has
	// no rule to offer. Taking a move back is this implementation's affordance
	// rather than a rule of Canasta, so the order the moves come back off in is
	// not one either.
	case ErrCardNotInHand, ErrNoSuchMeld, ErrNothingToUndo, ErrUndoMeldsFirst,
		ErrGameNotActive, ErrUnknownAction, "WRONG_PLAYER_COUNT":
		return nil
	}
	return nil
}
