package blackpeter

import "zolik/server/internal/module"

// LegalActions answers "what may this player do right now?". There is one
// move in the game — take a card from the left — and every seat holding cards
// is shown it, enabled or greyed with the engine's own reason.
func (m *Module) LegalActions(raw module.State, playerID string) ([]module.ActionOffer, error) {
	s, err := decode(raw)
	if err != nil {
		return nil, err
	}
	if s.Intermission.Open {
		return s.Intermission.Offers(s.players(), playerID), nil
	}
	st := s.seat(playerID)
	if s.Status != "active" || st == nil {
		return nil, nil
	}
	o := module.ActionOffer{ID: OfferDraw, Verb: VerbDraw, LabelKey: "blackpeter.offer.draw"}
	if from := s.source(playerID); from >= 0 {
		src := s.Seats[from].PlayerID
		// From a hand that is not the taker's: the client lets the player
		// point at one of its backs, and sends which under module.PickParam.
		o.Source = &module.Selector{Zone: module.FromHand, OwnerID: src, ZoneID: handZoneID(src)}
		o.Target = &module.Selector{Zone: module.FromHand, OwnerID: playerID, ZoneID: handZoneID(playerID)}
		o.Facts = []module.Fact{{LabelKey: "blackpeter.fact.from", Params: map[string]any{"player": src, "n": len(s.Seats[from].Hand)}}}
	}
	if err := s.drawRefusal(playerID); err != nil {
		o.WhyNot = module.CodeOf(err)
		o.RuleIDs = explainWith(module.StatedRuleIDs(m, s.config()), o.WhyNot)
	} else {
		o.Enabled = true
	}
	return []module.ActionOffer{o}, nil
}
