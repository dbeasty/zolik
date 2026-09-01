package ginrummy

import (
	"strconv"
	"strings"

	"zolik/server/internal/module"
)

// Offer IDs. draw:stock and draw:discard share the verb "draw" — a client
// tells them apart by LabelKey, and Apply tells them apart by which OfferID
// came back, exactly as zolikmod's own upcard-vs-stock draw already does.
// discard is a single aggregate offer, canasta's pattern: any card in hand is
// equally legal to discard, so there is nothing per-card to enumerate. knock
// and gin are the opposite — one offer per legal discard, each carrying its
// own resulting deadwood, because that number is the only reason to prefer
// one over another and a player cannot compute it by eye.
const (
	OfferDrawStock    = "draw:stock"
	OfferDrawDiscard  = "draw:discard"
	OfferPassUpcard   = "pass_upcard"
	OfferDiscard      = "discard"
	OfferBigGin       = "big_gin"
	OfferFinishLayoff = "finish_layoff"
)

// LegalActions answers "what may this player do right now?", built entirely
// by probing the real engine — never restating a rule, so the offer list
// cannot drift from Apply.
func (m *Module) LegalActions(raw module.State, playerID string) ([]module.ActionOffer, error) {
	s, err := decode(raw)
	if err != nil {
		return nil, err
	}

	if s.Intermission.Open {
		return s.Intermission.Offers(s.Players, playerID), nil
	}
	if s.Status != "active" || s.Current != playerID {
		why := ErrNotYourTurn
		if s.Status != "active" {
			why = ErrGameNotActive
		}
		return placeholderOffers(why), nil
	}

	switch s.Phase {
	case phaseUpcardNonDealer, phaseUpcardDealer:
		return m.upcardOffers(raw, s, playerID), nil
	case phaseDraw:
		return m.drawOffers(raw, s, playerID), nil
	case phaseDiscard:
		return m.discardPhaseOffers(raw, s, playerID), nil
	case phaseLayoff:
		return m.layoffOffers(raw, s, playerID), nil
	}
	return placeholderOffers(ErrGameNotActive), nil
}

func placeholderOffers(why string) []module.ActionOffer {
	return []module.ActionOffer{
		{ID: OfferDrawStock, Verb: VerbDraw, LabelKey: "ginrummy.offer.drawStock", WhyNot: why},
		{ID: OfferDrawDiscard, Verb: VerbDraw, LabelKey: "ginrummy.offer.drawDiscard", WhyNot: why},
		{ID: OfferPassUpcard, Verb: VerbPass, LabelKey: "ginrummy.offer.passUpcard", WhyNot: why},
		{ID: OfferDiscard, Verb: VerbDiscard, LabelKey: "ginrummy.offer.discard", WhyNot: why},
		{ID: OfferFinishLayoff, Verb: VerbFinishLayoff, LabelKey: "ginrummy.offer.finishLayoff", WhyNot: why},
	}
}

func (m *Module) probe(raw module.State, playerID string, a module.Action) (bool, string) {
	if _, _, err := m.Apply(raw, playerID, a); err != nil {
		return false, module.CodeOf(err)
	}
	return true, ""
}

func (m *Module) upcardOffers(raw module.State, s *GameState, playerID string) []module.ActionOffer {
	take := module.ActionOffer{
		ID: OfferDrawDiscard, Verb: VerbDraw, LabelKey: "ginrummy.offer.takeUpcard",
		Source: &module.Selector{Zone: module.FromDiscardPile, ZoneID: discardZoneID, Cards: append([]string(nil), s.DiscardPile...)},
	}
	take.Enabled, take.WhyNot = m.probe(raw, playerID, module.Action{OfferID: take.ID, Verb: VerbDraw})

	pass := module.ActionOffer{ID: OfferPassUpcard, Verb: VerbPass, LabelKey: "ginrummy.offer.passUpcard"}
	pass.Enabled, pass.WhyNot = m.probe(raw, playerID, module.Action{OfferID: pass.ID, Verb: VerbPass})

	return []module.ActionOffer{take, pass}
}

func (m *Module) drawOffers(raw module.State, s *GameState, playerID string) []module.ActionOffer {
	stock := module.ActionOffer{
		ID: OfferDrawStock, Verb: VerbDraw, LabelKey: "ginrummy.offer.drawStock",
		Source: &module.Selector{Zone: module.FromDeck, ZoneID: stockZoneID},
	}
	stock.Enabled, stock.WhyNot = m.probe(raw, playerID, module.Action{OfferID: stock.ID, Verb: VerbDraw})

	discard := module.ActionOffer{
		ID: OfferDrawDiscard, Verb: VerbDraw, LabelKey: "ginrummy.offer.drawDiscard",
		Source: &module.Selector{Zone: module.FromDiscardPile, ZoneID: discardZoneID, Cards: append([]string(nil), s.DiscardPile...)},
	}
	discard.Enabled, discard.WhyNot = m.probe(raw, playerID, module.Action{OfferID: discard.ID, Verb: VerbDraw})

	return []module.ActionOffer{stock, discard}
}

func (m *Module) discardPhaseOffers(raw module.State, s *GameState, playerID string) []module.ActionOffer {
	hand := s.Hands[playerID]
	offers := make([]module.ActionOffer, 0, len(hand)+2)

	discard := module.ActionOffer{
		ID: OfferDiscard, Verb: VerbDiscard, LabelKey: "ginrummy.offer.discard",
		Source: &module.Selector{Zone: module.FromHand, OwnerID: playerID, ZoneID: handZoneID(playerID),
			Cards: append([]string(nil), hand...), MinCards: 1},
	}
	if len(hand) > 0 {
		discard.Enabled, discard.WhyNot = m.probe(raw, playerID, module.Action{
			OfferID: discard.ID, Verb: VerbDiscard, Cards: hand[:1],
		})
	} else {
		discard.WhyNot = ErrCardNotInHand
	}
	offers = append(offers, discard)

	// One offer per card that would knock or gin — not per card in hand.
	// Advertising fifty-two disabled "Knock" buttons would be noise; a card
	// that cannot knock simply has no knock offer, exactly as a card that
	// cannot be played has no offer in prsi.
	seen := map[string]bool{}
	for _, card := range hand {
		if seen[card] {
			continue
		}
		seen[card] = true
		rest := removeCard(hand, card)
		deadwood, _ := Deadwood(rest)
		if deadwood > s.KnockLimit {
			continue
		}
		source := &module.Selector{Zone: module.FromHand, OwnerID: playerID, ZoneID: handZoneID(playerID),
			Cards: []string{card}, MinCards: 1}
		facts := []module.Fact{
			{LabelKey: "ginrummy.fact.deadwood", Value: strconv.Itoa(deadwood), Params: map[string]any{"n": deadwood}},
			{LabelKey: "ginrummy.fact.discardCard", Value: card},
		}
		var o module.ActionOffer
		if deadwood == 0 {
			o = module.ActionOffer{ID: "gin:" + card, Verb: VerbKnock, LabelKey: "ginrummy.offer.gin", Source: source, Facts: facts}
		} else {
			o = module.ActionOffer{ID: "knock:" + card, Verb: VerbKnock, LabelKey: "ginrummy.offer.knock", Source: source, Facts: facts}
		}
		o.Enabled, o.WhyNot = m.probe(raw, playerID, module.Action{OfferID: o.ID, Verb: VerbKnock, Cards: []string{card}})
		offers = append(offers, o)
	}

	if s.BigGin && len(hand) == 11 {
		if deadwood, _ := Deadwood(hand); deadwood == 0 {
			o := module.ActionOffer{ID: OfferBigGin, Verb: VerbKnock, LabelKey: "ginrummy.offer.bigGin"}
			o.Enabled, o.WhyNot = m.probe(raw, playerID, module.Action{OfferID: o.ID, Verb: VerbKnock})
			offers = append(offers, o)
		}
	}

	return offers
}

func (m *Module) layoffOffers(raw module.State, s *GameState, playerID string) []module.ActionOffer {
	finish := module.ActionOffer{ID: OfferFinishLayoff, Verb: VerbFinishLayoff, LabelKey: "ginrummy.offer.finishLayoff"}
	finish.Enabled, finish.WhyNot = m.probe(raw, playerID, module.Action{OfferID: finish.ID, Verb: VerbFinishLayoff})
	offers := []module.ActionOffer{finish}

	if playerID != other(s.Players, s.Knocker) {
		return offers
	}

	hand := s.Hands[playerID]
	for _, meld := range s.KnockerMelds {
		var eligible []string
		seen := map[string]bool{}
		for _, card := range hand {
			if seen[card] {
				continue
			}
			seen[card] = true
			if ok, _ := m.probe(raw, playerID, module.Action{
				OfferID: "lay_off:" + meld.ID, Verb: VerbLayOff, Target: meld.ID, Cards: []string{card},
			}); ok {
				eligible = append(eligible, card)
			}
		}
		o := module.ActionOffer{
			ID: "lay_off:" + meld.ID, Verb: VerbLayOff, LabelKey: "ginrummy.offer.layOff",
			Target: &module.Selector{MeldID: meld.ID},
			Facts:  []module.Fact{{LabelKey: "ginrummy.fact.meldCards", Value: strings.Join(meld.Cards, ",")}},
		}
		if len(eligible) > 0 {
			o.Source = &module.Selector{Zone: module.FromHand, OwnerID: playerID, ZoneID: handZoneID(playerID),
				Cards: eligible, MinCards: 1}
			o.Enabled = true
		} else {
			o.WhyNot = ErrCardDoesNotExtendMeld
		}
		offers = append(offers, o)
	}
	return offers
}
