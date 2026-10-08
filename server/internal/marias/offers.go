package marias

import (
	"strconv"

	"zolik/server/internal/module"
)

// LegalActions answers "what may this player do right now?".
//
// Every enabled/disabled decision is the engine's own answer, got by probing
// Apply against the state — nothing here restates a rule. The offers are the
// ones the phase has, for every player: disabled, with the engine's reason,
// for whoever is not the one being waited on.
func (m *Module) LegalActions(raw module.State, playerID string) ([]module.ActionOffer, error) {
	s, err := decode(raw)
	if err != nil {
		return nil, err
	}
	if s.Intermission.Open {
		return s.Intermission.Offers(s.Players, playerID), nil
	}
	if s.Status != "active" || playerID == s.Sitter {
		return nil, nil // a sitter has no say in the deal they sit out
	}

	var offers []module.ActionOffer
	add := func(o module.ActionOffer, a module.Action) {
		a.OfferID, a.Verb = o.ID, o.Verb
		o.Enabled, o.WhyNot = probe(m, raw, playerID, a)
		offers = append(offers, o)
	}
	hand := s.Hands[playerID]
	handSel := func(cards []string, n int) *module.Selector {
		return &module.Selector{
			Zone: module.FromHand, OwnerID: playerID, ZoneID: handZoneID(playerID),
			Cards: cards, MinCards: n, MaxCards: n,
		}
	}

	switch s.Phase {
	case phaseAuction:
		// A bid names any rung above the one standing; the lowest is first,
		// so a press with nothing chosen is the smallest raise.
		var rungs []module.ParamChoice
		for r := s.Rung + 1; r <= topRung; r++ {
			rungs = append(rungs, module.ParamChoice{Value: strconv.Itoa(r), LabelKey: rungKey(r)})
		}
		bid := module.ActionOffer{ID: OfferBid, Verb: VerbBid, LabelKey: "marias.offer.bid"}
		probeBid := module.Action{}
		if len(rungs) > 0 {
			bid.Params = []module.ParamSpec{{Name: "rung", Kind: module.ParamKindChoice, LabelKey: "marias.param.rung", Choices: rungs}}
			probeBid.Params = map[string]string{"rung": rungs[0].Value}
		}
		add(bid, probeBid)
		hold := module.ActionOffer{ID: OfferHold, Verb: VerbHold, LabelKey: "marias.offer.hold"}
		if s.Rung > 0 {
			hold.Facts = []module.Fact{{LabelKey: "marias.fact.rung", Params: map[string]any{"rung": rungKey(s.Rung)}}}
		}
		add(hold, module.Action{})
		add(module.ActionOffer{ID: OfferAuctionPass, Verb: VerbPass, LabelKey: "marias.offer.auctionPass"}, module.Action{})

	case phaseTrump:
		o := module.ActionOffer{ID: OfferTrump, Verb: VerbChooseTrump, LabelKey: "marias.offer.trump"}
		o.Source = handSel(hand, 1)
		add(o, module.Action{Cards: first(hand, 1)})
		if s.ZLidu {
			add(module.ActionOffer{ID: OfferZLidu, Verb: VerbChooseTrump, LabelKey: "marias.offer.zLidu"}, module.Action{})
		}

	case phaseAnnounce:
		if s.licit() {
			m.licitAnnounceOffers(raw, s, playerID, add)
			break
		}
		for _, o := range []module.ActionOffer{
			{ID: OfferHra, Verb: VerbAnnounce, LabelKey: "marias.offer.announce.hra"},
			{ID: OfferHraSedma, Verb: VerbAnnounce, LabelKey: "marias.offer.announce.hraSedma"},
			{ID: OfferSto, Verb: VerbAnnounce, LabelKey: "marias.offer.announce.sto"},
			{ID: OfferStoSedma, Verb: VerbAnnounce, LabelKey: "marias.offer.announce.stoSedma"},
			{ID: OfferBetl, Verb: VerbAnnounce, LabelKey: "marias.offer.announce.betl"},
			{ID: OfferDurch, Verb: VerbAnnounce, LabelKey: "marias.offer.announce.durch"},
		} {
			add(o, module.Action{})
		}

	case phaseTalon:
		// Every card that may go, on its own; any two of them together are a
		// legal talon, since the rules forbid cards, not pairs.
		var allowed []string
		if s.Current == playerID {
			for _, c := range hand {
				if s.talonRefusal(playerID, c) == nil {
					allowed = append(allowed, c)
				}
			}
		}
		o := module.ActionOffer{ID: OfferTalon, Verb: VerbDiscard, LabelKey: "marias.offer.talon"}
		o.Source = handSel(allowed, 2)
		o.Target = &module.Selector{Zone: module.FromTable, ZoneID: talonZoneID}
		add(o, module.Action{Cards: first(allowed, 2)})

	case phaseAnswer:
		add(module.ActionOffer{ID: OfferGood, Verb: VerbGood, LabelKey: "marias.offer.good"}, module.Action{})
		add(module.ActionOffer{ID: OfferBadBetl, Verb: VerbTakeOver, LabelKey: "marias.offer.bad.betl"}, module.Action{})
		add(module.ActionOffer{ID: OfferBadDurch, Verb: VerbTakeOver, LabelKey: "marias.offer.bad.durch"}, module.Action{})

	case phaseFlek:
		// Keyed by part, and written out in full: a key is only in the
		// manifest if it appears as a literal.
		flekOffers := map[string]module.ActionOffer{
			partGame:       {ID: OfferFlekGame, Verb: VerbFlek, LabelKey: "marias.offer.flek.game"},
			partSedma:      {ID: OfferFlekSedma, Verb: VerbFlek, LabelKey: "marias.offer.flek.sedma"},
			partProtiSedma: {ID: OfferFlekPSedma, Verb: VerbFlek, LabelKey: "marias.offer.flek.protiSedma"},
			partProtiSto:   {ID: OfferFlekPSto, Verb: VerbFlek, LabelKey: "marias.offer.flek.protiSto"},
			partSto:        {ID: OfferFlekSto, Verb: VerbFlek, LabelKey: "marias.offer.flek.sto"},
		}
		for _, part := range s.parts() {
			o := flekOffers[part]
			// What the part would be worth doubled once more: ×2 for a flek,
			// ×4 for a re.
			o.Facts = []module.Fact{{LabelKey: "marias.fact.times", Params: map[string]any{"n": 1 << (s.Fleks[part] + 1)}}}
			add(o, module.Action{})
		}
		if s.trumpGame() && !s.licit() {
			add(module.ActionOffer{ID: OfferProtiSedma, Verb: VerbProti, LabelKey: "marias.offer.proti.sedma"}, module.Action{})
			if s.Game == gameHra {
				add(module.ActionOffer{ID: OfferProtiSto, Verb: VerbProti, LabelKey: "marias.offer.proti.sto"}, module.Action{})
			}
		}
		add(module.ActionOffer{ID: OfferPass, Verb: VerbPass, LabelKey: "marias.offer.pass"}, module.Action{})

	case phasePlay:
		var legal []string
		if s.Current == playerID {
			legal = s.legal(playerID)
		}
		o := module.ActionOffer{ID: OfferPlay, Verb: VerbPlay, LabelKey: "marias.offer.play"}
		o.Source = handSel(legal, 1)
		o.Target = &module.Selector{Zone: module.FromTable, ZoneID: trickZoneID}
		add(o, module.Action{Cards: first(legal, 1)})
	}

	m.annotate(raw, s, offers)
	return offers, nil
}

func first(cards []string, n int) []string {
	if len(cards) < n {
		return cards
	}
	return cards[:n]
}

// probe runs an action through the real engine: accepted, or its reason.
func probe(m *Module, raw module.State, playerID string, a module.Action) (bool, string) {
	if _, _, err := m.Apply(raw, playerID, a); err != nil {
		return false, module.CodeOf(err)
	}
	return true, ""
}

// annotate points each refusal at the written rule behind it.
func (m *Module) annotate(raw module.State, s *GameState, offers []module.ActionOffer) {
	cfg := s.config()
	stated := module.StatedRuleIDs(m, cfg)
	for i := range offers {
		o := &offers[i]
		if o.Enabled || o.WhyNot == "" {
			continue
		}
		o.RuleIDs = explainWith(stated, o.WhyNot)
	}
}

// config rebuilds the lobby config this match was created with, as far as
// the written rules depend on it.
func (s *GameState) config() module.MatchConfig {
	tariff := TariffCSM
	if s.Tariff == tariffs[TariffPub] {
		tariff = TariffPub
	}
	variation := variationVoleny
	if s.licit() {
		variation = variationLicit
	}
	return module.MatchConfig{Variation: variation, Options: module.Options{
		OptDeals:          s.Deals,
		OptTariff:         tariff,
		OptRedDoubles:     module.BoolOpt(s.RedDoubles),
		OptFlekLimit:      s.FlekLimit,
		OptZLidu:          module.BoolOpt(s.ZLidu),
		OptShowCardPoints: module.BoolOpt(s.ShowCardPoints),
		OptOpenBetlDurch:  module.BoolOpt(s.OpenBetlDurch),
	}}
}

// licitAnnounceOffers are licitovaný's contract buttons: one per kind of
// contract, each carrying the trumps (and for dvě sedmy the helper suit)
// that would put it at or above the rung won. A kind no suit can lift that
// high is offered disabled, with the engine's reason.
func (m *Module) licitAnnounceOffers(raw module.State, s *GameState, playerID string, add func(module.ActionOffer, module.Action)) {
	hand := s.Hands[playerID]
	for _, o := range []module.ActionOffer{
		{ID: OfferLSedma, Verb: VerbAnnounce, LabelKey: "marias.offer.licit.sedma"},
		{ID: OfferLSto, Verb: VerbAnnounce, LabelKey: "marias.offer.licit.sto"},
		{ID: OfferLStoSedma, Verb: VerbAnnounce, LabelKey: "marias.offer.licit.stoSedma"},
		{ID: OfferLBetl, Verb: VerbAnnounce, LabelKey: "marias.offer.licit.betl"},
		{ID: OfferLDurch, Verb: VerbAnnounce, LabelKey: "marias.offer.licit.durch"},
		{ID: OfferLDveSedmy, Verb: VerbAnnounce, LabelKey: "marias.offer.licit.dveSedmy"},
		{ID: OfferLDveSedmySto, Verb: VerbAnnounce, LabelKey: "marias.offer.licit.dveSedmySto"},
	} {
		kind := kindOfOffer(o.ID)
		probeA := module.Action{}
		if trumpKind(kind) {
			var suits []module.ParamChoice
			candidates := []string{"H", "D", "C", "S"}
			if kind == kindSedma {
				candidates = suitsHolding(hand)
			}
			for _, suit := range candidates {
				if contractRung(kind, suit == string(suitRed)) >= s.Rung {
					suits = append(suits, module.ParamChoice{Value: suit, LabelKey: module.GermanSuitKey(suit)})
				}
			}
			// With no suit to offer, hearts is probed for the reason: the
			// highest a kind can reach, so it reads "below the bid" when
			// that is why, and "no trump seven" when plain sedma lacks one.
			probeTrump := "H"
			if len(suits) > 0 {
				probeTrump = suits[0].Value
				o.Params = []module.ParamSpec{{Name: "trump", Kind: module.ParamKindChoice, LabelKey: "marias.param.trump", Choices: suits}}
			}
			probeA.Params = map[string]string{"trump": probeTrump}
			if kind == kindDveSedmy || kind == kindDveSedmySto {
				// A helper suit too, so a kind that cannot reach the rung
				// is refused for that and not for a missing suit.
				probeA.Params["helper"] = "D"
			}
			if (kind == kindDveSedmy || kind == kindDveSedmySto) && len(suits) > 0 {
				// The helper suit's list starts away from the first trump,
				// so a press with nothing chosen is a legal pair.
				var helpers []module.ParamChoice
				for _, suit := range []string{"D", "C", "S", "H"} {
					helpers = append(helpers, module.ParamChoice{Value: suit, LabelKey: module.GermanSuitKey(suit)})
				}
				if helpers[0].Value == probeTrump {
					helpers = append(helpers[1:], helpers[0])
				}
				o.Params = append(o.Params, module.ParamSpec{Name: "helper", Kind: module.ParamKindChoice, LabelKey: "marias.param.helper", Choices: helpers})
				probeA.Params["helper"] = helpers[0].Value
			}
		}
		add(o, probeA)
	}
	omyl := module.ActionOffer{ID: OfferOmyl, Verb: VerbFold, LabelKey: "marias.offer.omyl"}
	omyl.Facts = []module.Fact{{LabelKey: "marias.fact.units", Params: map[string]any{"n": s.Tariff.Omyl}}}
	add(omyl, module.Action{})
}
