package snaps

import "zolik/server/internal/module"

// LegalActions answers "what may this player do right now?".
//
// Every enabled/disabled decision is the engine's own answer, got by probing
// Apply against the state — nothing here restates a rule.
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
	play := module.ActionOffer{ID: OfferPlay, Verb: VerbPlay, LabelKey: "snaps.offer.play"}
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

	// Exchanging and closing are the leader's, and only while the talon is
	// open: offered then, so a refusal names a rule rather than a phase.
	if s.open() && len(s.Trick) == 0 {
		ex := module.ActionOffer{ID: OfferExchange, Verb: VerbExchange, LabelKey: "snaps.offer.exchange"}
		ex.Facts = []module.Fact{{LabelKey: "snaps.fact.exchange", Params: map[string]any{
			"card": exchangeKey(s.rules().exchange),
		}}}
		add(ex, module.Action{})
		add(module.ActionOffer{ID: OfferClose, Verb: VerbClose, LabelKey: "snaps.offer.close"}, module.Action{})
	}

	m.annotate(s, offers)
	return offers, nil
}

func exchangeKey(rank byte) string {
	if rank == '9' {
		return "snaps.card.nine"
	}
	return "snaps.card.spodek"
}

// probe runs an action through the real engine: accepted, or its reason.
func probe(m *Module, raw module.State, playerID string, a module.Action) (bool, string) {
	if _, _, err := m.Apply(raw, playerID, a); err != nil {
		return false, module.CodeOf(err)
	}
	return true, ""
}

// annotate points each refusal at the written rule behind it.
func (m *Module) annotate(s *GameState, offers []module.ActionOffer) {
	stated := module.StatedRuleIDs(m, s.config())
	for i := range offers {
		o := &offers[i]
		if o.Enabled || o.WhyNot == "" {
			continue
		}
		o.RuleIDs = explainWith(stated, o.WhyNot)
	}
}
