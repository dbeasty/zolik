package marias

import "zolik/server/internal/module"

var (
	_ module.RulesProvider     = (*Module)(nil)
	_ module.RuleIndexProvider = (*Module)(nil)
)

// ExplainRefusal names the written rules behind each code this engine can
// return, filtered to the ones this table actually states.
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
	// Each code lists every sentence that could explain it; explainWith keeps
	// the ones this table actually states, so a volený-only sentence simply
	// drops out at a licitovaný table and the other way round.
	switch code {
	case ErrNoZLidu:
		return []string{"marias.rules.deal.trump", "marias.rules.licit.contract"}
	case ErrPickOneCard:
		return []string{"marias.rules.deal.trump", "marias.rules.play.follow"}
	case ErrNoSeven:
		return []string{"marias.rules.game.sedma", "marias.rules.bid.proti", "marias.rules.licit.bluff"}
	case ErrDiscardTwo, ErrSharpInTalon:
		return []string{"marias.rules.deal.talon"}
	case ErrSevenInTalon:
		return []string{"marias.rules.game.sedma", "marias.rules.deal.talon"}
	case ErrNotHigher:
		return []string{"marias.rules.bid.takeover", "marias.rules.licit.contract"}
	case ErrNotYoursToFlek, ErrNoSuchPart:
		return []string{"marias.rules.bid.flek"}
	case ErrFlekLimit:
		return []string{"marias.rules.bid.flekLimit"}
	case ErrProtiNotNow:
		return []string{"marias.rules.bid.proti", "marias.rules.licit.noProti"}
	case ErrMustFollow, ErrMustBeat:
		return []string{"marias.rules.play.follow"}
	case ErrMustTrump, ErrMustOvertrump:
		return []string{"marias.rules.play.trump"}
	case ErrKeepSeven:
		return []string{"marias.rules.play.seven"}
	case ErrBidTooLow:
		return []string{"marias.rules.licit.ladder"}
	case ErrYouHold, ErrYouBid:
		return []string{"marias.rules.licit.auction"}
	case ErrBelowTheBid, ErrUnknownSuit:
		return []string{"marias.rules.licit.contract"}
	case ErrHelperIsTrump:
		return []string{"marias.rules.game.dveSedmy"}
	case ErrNoOmyl:
		return []string{"marias.rules.licit.omyl"}

	case ErrNotYourTurn, ErrNotNow:
		return []string{"marias.rules.deal.turns"}

	// --- not about the rules at all ---------------------------------------
	case ErrGameNotActive, ErrUnknownAction, ErrCardNotInHand, "TOO_FEW_PLAYERS":
		return nil
	}
	return nil
}
