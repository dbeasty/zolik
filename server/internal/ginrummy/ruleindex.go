package ginrummy

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

// explainWith filters the mapping against the sentences a table actually
// states, so an id can never point at a rule the player cannot go and read —
// Oklahoma's variable knock limit and a fixed one are both listed for
// DEADWOOD_TOO_HIGH, and each table keeps the one it has.
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

	// --- when in the turn -------------------------------------------------
	//
	// WRONG_PHASE covers the upcard dance, the ordinary draw-then-discard, and
	// the lay-off that follows a knock — three different answers to "why not
	// now?", which is exactly why the code alone cannot be the explanation.
	case ErrNotYourTurn, ErrWrongPhase:
		return []string{
			"ginrummy.rules.drawDiscard",
			"ginrummy.rules.upcardDance",
			"ginrummy.rules.layoffDescription",
		}
	case ErrUpcardDeclined:
		return []string{"ginrummy.rules.upcardDance"}
	case ErrNothingToDraw:
		return []string{"ginrummy.rules.drawDiscard", "ginrummy.rules.deadHandDescription"}

	// --- knocking ---------------------------------------------------------
	case ErrDeadwoodTooHigh:
		return []string{
			"ginrummy.rules.knockLimit", "ginrummy.rules.oklahoma",
			"ginrummy.rules.gin", "ginrummy.rules.bigGinBonus",
		}
	case ErrCardDoesNotFit:
		// The catch-all for a submission that is not one card when one card is
		// what the move takes — a knock, a discard, a lay-off.
		return []string{"ginrummy.rules.drawDiscard", "ginrummy.rules.knockOnDiscard"}

	// --- laying off -------------------------------------------------------
	case ErrCardDoesNotExtendMeld:
		return []string{"ginrummy.rules.layoffDescription", "ginrummy.rules.setsAndRuns"}

	// --- not about the rules at all ---------------------------------------
	case ErrCardNotInHand, ErrNoSuchMeld, ErrGameNotActive,
		ErrUnknownAction, ErrWrongPlayerCount:
		return nil
	}
	return nil
}
