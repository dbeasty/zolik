package sedma

import "zolik/server/internal/module"

// LegalActions answers "what may this player do right now?". Every
// enabled/disabled decision is the engine's own answer, got by probing Apply.
func (m *Module) LegalActions(raw module.State, playerID string) ([]module.ActionOffer, error) {
	s, err := decode(raw)
	if err != nil {
		return nil, err
	}
	if s.Intermission.Open {
		return s.Intermission.Offers(s.Players, playerID), nil
	}
	if s.Status != "active" || s.seat(playerID) < 0 {
		return nil, nil
	}
	var offers []module.ActionOffer
	add := func(o module.ActionOffer, a module.Action) {
		a.OfferID, a.Verb = o.ID, o.Verb
		o.Enabled, o.WhyNot = probe(m, raw, playerID, a)
		offers = append(offers, o)
	}

	var legal []string
	if s.Current == playerID {
		legal = s.legal(playerID)
	}
	play := module.ActionOffer{ID: OfferPlay, Verb: VerbPlay, LabelKey: "sedma.offer.play"}
	if s.Phase == phaseDecide {
		play.LabelKey = "sedma.offer.continue"
	}
	play.Source = &module.Selector{
		Zone: module.FromHand, OwnerID: playerID, ZoneID: handZoneID(playerID),
		Cards: legal, MinCards: 1, MaxCards: 1,
	}
	play.Target = &module.Selector{Zone: module.FromTable, ZoneID: trickZoneID}
	probeCards := legal
	if len(probeCards) > 1 {
		probeCards = probeCards[:1]
	}
	add(play, module.Action{Cards: probeCards})

	if s.Phase == phaseDecide {
		stop := module.ActionOffer{ID: OfferStop, Verb: VerbStop, LabelKey: "sedma.offer.stop"}
		stop.Facts = []module.Fact{{LabelKey: "sedma.fact.trickPoints", Params: map[string]any{"n": s.trickPoints()}}}
		add(stop, module.Action{})
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
