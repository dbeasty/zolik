package lora

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

	if s.Phase == phaseChoose {
		if playerID != s.Current {
			return nil, nil
		}
		o := module.ActionOffer{ID: OfferChoose, Verb: VerbChoose, LabelKey: "lora.offer.choose"}
		var games []module.ParamChoice
		for _, g := range sequence {
			if s.choosable(g) {
				games = append(games, module.ParamChoice{Value: g, LabelKey: gameKey(g)})
			}
		}
		o.Params = []module.ParamSpec{{Name: ParamGame, Kind: module.ParamKindChoice, LabelKey: "lora.param.game", Choices: games}}
		add(o, module.Action{Params: map[string]string{ParamGame: games[0].Value}})
		return offers, nil
	}

	var legal []string
	if s.Current == playerID {
		legal = s.legal(playerID)
	}
	play := module.ActionOffer{ID: OfferPlay, Verb: VerbPlay, LabelKey: "lora.offer.play"}
	target := trickZoneID
	switch s.Phase {
	case phaseQuart:
		play.LabelKey, target = "lora.offer.quart", quartZoneID
	case phaseTens:
		play.LabelKey, target = "lora.offer.lay", rowsZoneID
	}
	play.Source = &module.Selector{
		Zone: module.FromHand, OwnerID: playerID, ZoneID: handZoneID(playerID),
		Cards: legal, MinCards: 1, MaxCards: 1,
	}
	play.Target = &module.Selector{Zone: module.FromTable, ZoneID: target}
	probeCards := legal
	if len(probeCards) > 1 {
		probeCards = probeCards[:1]
	}
	add(play, module.Action{Cards: probeCards})

	if s.Phase == phaseTens {
		add(module.ActionOffer{ID: OfferDone, Verb: VerbDone, LabelKey: "lora.offer.done"}, module.Action{})
		add(module.ActionOffer{ID: OfferTuk, Verb: VerbTuk, LabelKey: "lora.offer.tuk"}, module.Action{})
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
