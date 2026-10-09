package ferbl

import "zolik/server/internal/module"

// LegalActions answers "what may this player do right now?". Every
// enabled/disabled decision is the engine's own answer, got by probing Apply.
func (m *Module) LegalActions(raw module.State, playerID string) ([]module.ActionOffer, error) {
	s, err := decode(raw)
	if err != nil {
		return nil, err
	}
	if s.Intermission.Open {
		return s.Intermission.Offers(s.players(), playerID), nil
	}
	st := s.seat(playerID)
	if s.Status != "active" || st == nil || !st.live() {
		return nil, nil
	}
	var offers []module.ActionOffer
	add := func(o module.ActionOffer) {
		o.Enabled, o.WhyNot = probe(m, raw, playerID, module.Action{OfferID: o.ID, Verb: o.Verb})
		offers = append(offers, o)
	}
	owe := s.toCall(st)
	if owe == 0 {
		add(module.ActionOffer{ID: VerbCheck, Verb: VerbCheck, LabelKey: "ferbl.offer.check"})
		add(module.ActionOffer{ID: VerbBet, Verb: VerbBet, LabelKey: "ferbl.offer.bet",
			Facts: []module.Fact{{LabelKey: "ferbl.fact.chips", Params: map[string]any{"n": min(s.unit(), st.Stack)}}}})
	} else {
		add(module.ActionOffer{ID: VerbFold, Verb: VerbFold, LabelKey: "ferbl.offer.fold"})
		add(module.ActionOffer{ID: VerbCall, Verb: VerbCall, LabelKey: "ferbl.offer.call",
			Facts: []module.Fact{{LabelKey: "ferbl.fact.chips", Params: map[string]any{"n": min(owe, st.Stack)}}}})
		add(module.ActionOffer{ID: VerbRaise, Verb: VerbRaise, LabelKey: "ferbl.offer.raise",
			Facts: []module.Fact{{LabelKey: "ferbl.fact.chips", Params: map[string]any{"n": min(owe+s.unit(), st.Stack)}}}})
	}
	stated := module.StatedRuleIDs(m, s.config())
	for i := range offers {
		if o := &offers[i]; !o.Enabled && o.WhyNot != "" {
			o.RuleIDs = explainWith(stated, o.WhyNot)
		}
	}
	return offers, nil
}

func probe(m *Module, raw module.State, playerID string, a module.Action) (bool, string) {
	if _, _, err := m.Apply(raw, playerID, a); err != nil {
		return false, module.CodeOf(err)
	}
	return true, ""
}
