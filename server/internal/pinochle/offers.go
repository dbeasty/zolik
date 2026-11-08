package pinochle

import (
	"strconv"

	"zolik/server/internal/module"
)

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
	mine := s.Current == playerID
	hand := s.Hands[playerID]
	handSel := func(cards []string, n int) *module.Selector {
		return &module.Selector{
			Zone: module.FromHand, OwnerID: playerID, ZoneID: handZoneID(playerID),
			Cards: cards, MinCards: n, MaxCards: n,
		}
	}

	switch s.Phase {
	case phaseAuction:
		next := s.minNext()
		bid := module.ActionOffer{ID: OfferBid, Verb: VerbBid, LabelKey: "pinochle.offer.bid"}
		bid.Params = []module.ParamSpec{{
			Name: ParamAmount, Kind: module.ParamKindInt, LabelKey: "pinochle.param.bid",
			Min: next, Max: maxBidTop, Step: bidStep, Default: next, Headline: true,
		}}
		if s.Bid > 0 {
			bid.Facts = []module.Fact{{LabelKey: "pinochle.fact.standingBid", Value: strconv.Itoa(s.Bid),
				Params: map[string]any{"n": s.Bid}}}
		}
		add(bid, module.Action{Params: map[string]string{ParamAmount: strconv.Itoa(next)}})
		add(module.ActionOffer{ID: OfferPass, Verb: VerbPass, LabelKey: "pinochle.offer.pass"}, module.Action{})

	case phaseTrump:
		o := module.ActionOffer{ID: OfferTrump, Verb: VerbTrump, LabelKey: "pinochle.offer.trump"}
		o.Params = []module.ParamSpec{{
			Name: ParamSuit, Kind: module.ParamKindChoice, LabelKey: "pinochle.param.trump",
			Choices: []module.ParamChoice{
				{Value: "H", LabelKey: "suit.H"},
				{Value: "C", LabelKey: "suit.C"},
				{Value: "D", LabelKey: "suit.D"},
				{Value: "S", LabelKey: "suit.S"},
			},
			DefaultChoice: bestTrumpFor(hand, s.table()),
		}}
		add(o, module.Action{Params: map[string]string{ParamSuit: bestTrumpFor(hand, s.table())}})

	case phaseBury:
		n := s.table().widow
		o := module.ActionOffer{ID: OfferBury, Verb: VerbBury, LabelKey: "pinochle.offer.bury"}
		o.Facts = []module.Fact{{LabelKey: "pinochle.fact.cards", Value: strconv.Itoa(n), Params: map[string]any{"n": n}}}
		o.Source = handSel(probeable(mine, hand), n)
		add(o, module.Action{Cards: firstN(hand, n)})

	case phasePassIn, phasePassBack:
		n := s.PassCount
		o := module.ActionOffer{ID: OfferGive, Verb: VerbGive, LabelKey: "pinochle.offer.give"}
		o.Facts = []module.Fact{{LabelKey: "pinochle.fact.cards", Value: strconv.Itoa(n), Params: map[string]any{"n": n}}}
		o.Source = handSel(probeable(mine, hand), n)
		add(o, module.Action{Cards: firstN(hand, n)})

	case phasePlay:
		var legal []string
		if mine {
			legal = s.legal(playerID)
		}
		play := module.ActionOffer{ID: OfferPlay, Verb: VerbPlay, LabelKey: "pinochle.offer.play"}
		play.Source = handSel(legal, 1)
		play.Target = &module.Selector{Zone: module.FromTable, ZoneID: trickZoneID}
		probeCards := legal
		if len(probeCards) > 1 {
			probeCards = probeCards[:1]
		}
		add(play, module.Action{Cards: probeCards})
		if s.table().auction && playerID == s.Bidder && len(s.History) == 0 && len(s.Trick) == 0 {
			o := module.ActionOffer{ID: OfferThrowIn, Verb: VerbThrowIn, LabelKey: "pinochle.offer.throwIn"}
			o.Facts = []module.Fact{{LabelKey: "pinochle.fact.loses", Value: strconv.Itoa(s.Bid), Params: map[string]any{"n": s.Bid}}}
			add(o, module.Action{})
		}

	case phaseMeld:
		if mine {
			for _, o := range twoHandOptions(hand, s.Used[playerID], s.trump()) {
				add(meldOffer(o), module.Action{})
			}
			if s.canExchange(playerID) {
				o := module.ActionOffer{ID: OfferExchange, Verb: VerbMeld, LabelKey: "pinochle.offer.exchange"}
				o.Facts = []module.Fact{{LabelKey: "pinochle.fact.points", Value: strconv.Itoa(pointsDix), Params: map[string]any{"n": pointsDix}}}
				add(o, module.Action{})
			}
			if s.canDeclareDix(playerID) {
				o := module.ActionOffer{ID: OfferDix, Verb: VerbMeld, LabelKey: "pinochle.offer.dix"}
				o.Facts = []module.Fact{{LabelKey: "pinochle.fact.points", Value: strconv.Itoa(pointsDix), Params: map[string]any{"n": pointsDix}}}
				add(o, module.Action{})
			}
		}
		add(module.ActionOffer{ID: OfferDraw, Verb: VerbDraw, LabelKey: "pinochle.offer.draw"}, module.Action{})
	}

	stated := module.StatedRuleIDs(m, s.config())
	for i := range offers {
		if o := &offers[i]; !o.Enabled && o.WhyNot != "" {
			o.RuleIDs = explainWith(stated, o.WhyNot)
			o.Remedy = remedyFor(s, playerID, o.WhyNot)
		}
	}
	return offers, nil
}

// meldOffer is one two-handed meld as a button: its points on it, and the
// cards it lays named so a client can show them.
func meldOffer(o Option) module.ActionOffer {
	out := module.ActionOffer{ID: o.ID, Verb: VerbMeld, LabelKey: meldLabelKey(o.Kind)}
	out.Facts = []module.Fact{{LabelKey: "pinochle.fact.points", Value: strconv.Itoa(o.Points), Params: map[string]any{"n": o.Points}}}
	if o.Suit != "" {
		out.Facts = append(out.Facts, module.Fact{LabelKey: suitKey(o.Suit)})
	}
	if o.Times > 1 {
		out.Facts = append(out.Facts, module.Fact{LabelKey: "pinochle.fact.double"})
	}
	return out
}

// probeable is the hand as a selector's card list, for the seat whose move
// it is; nothing for anyone else. Every copy is listed, since burying or
// passing both nines of a suit is two cards, not one.
func probeable(mine bool, hand []string) []string {
	if !mine {
		return nil
	}
	return append([]string(nil), hand...)
}

func firstN(hand []string, n int) []string {
	if len(hand) < n {
		return append([]string(nil), hand...)
	}
	return append([]string(nil), hand[:n]...)
}

func probe(m *Module, raw module.State, playerID string, a module.Action) (bool, string) {
	if _, _, err := m.Apply(raw, playerID, a); err != nil {
		return false, module.CodeOf(err)
	}
	return true, ""
}

// bestTrumpFor is the suit a hand would make most of: its meld with that
// suit as trumps, plus ten a card for length. The offer's starting choice,
// and the bots' first guess.
func bestTrumpFor(hand []string, t table) string {
	best, bestV := "H", -1
	for _, su := range suits {
		v := BestMeld(hand, byte(su), t).Total
		for _, c := range hand {
			if c[1] == byte(su) {
				v += 10
			}
		}
		if v > bestV {
			best, bestV = string(su), v
		}
	}
	return best
}
