package okobere

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
		return s.Intermission.Offers(s.players(), playerID), nil
	}
	st := s.seat(playerID)
	if s.Status != "active" || st == nil {
		return nil, nil
	}
	var offers []module.ActionOffer
	add := func(o module.ActionOffer, a module.Action) {
		a.OfferID, a.Verb = o.ID, o.Verb
		o.Enabled, o.WhyNot = probe(m, raw, playerID, a)
		offers = append(offers, o)
	}

	punterToBet := s.Phase == phasePunters && s.Current == playerID && st.Bet == 0
	if punterToBet {
		bet := module.ActionOffer{ID: OfferBet, Verb: VerbBet, LabelKey: "okobere.offer.bet"}
		top := s.maxBet(st)
		bet.Params = []module.ParamSpec{{
			Name: ParamAmount, Kind: module.ParamKindInt, LabelKey: "okobere.param.stake",
			Min: s.MinBet, Max: top, Step: 1, Default: s.MinBet, Headline: true,
			Choices: quickStakes(s.MinBet, top, s.bankFree(), st.Stack),
		}}
		bet.Facts = []module.Fact{{LabelKey: "okobere.fact.bankFree", Params: map[string]any{"n": s.bankFree()}}}
		add(bet, module.Action{Params: map[string]string{ParamAmount: strconv.Itoa(s.MinBet)}})
	} else {
		add(module.ActionOffer{ID: OfferCard, Verb: VerbCard, LabelKey: "okobere.offer.card"}, module.Action{})
		add(module.ActionOffer{ID: OfferStand, Verb: VerbStand, LabelKey: "okobere.offer.stand"}, module.Action{})
	}

	stated := module.StatedRuleIDs(m, s.config())
	for i := range offers {
		if o := &offers[i]; !o.Enabled && o.WhyNot != "" {
			o.RuleIDs = explainWith(stated, o.WhyNot)
		}
	}
	return offers, nil
}

// quickStakes are the buttons beside the stake's range: double the minimum,
// and the most the seat may stake — "va banque" when that is the whole bank.
func quickStakes(minBet, top, bankFree, stack int) []module.ParamChoice {
	var out []module.ParamChoice
	if d := minBet * 2; d < top {
		out = append(out, module.ParamChoice{Value: strconv.Itoa(d), LabelKey: "okobere.quick.double"})
	}
	switch {
	case top <= minBet:
	case top == bankFree && bankFree <= stack:
		out = append(out, module.ParamChoice{Value: strconv.Itoa(top), LabelKey: "okobere.quick.vaBanque"})
	default:
		out = append(out, module.ParamChoice{Value: strconv.Itoa(top), LabelKey: "okobere.quick.allIn"})
	}
	return out
}

func probe(m *Module, raw module.State, playerID string, a module.Action) (bool, string) {
	if _, _, err := m.Apply(raw, playerID, a); err != nil {
		return false, module.CodeOf(err)
	}
	return true, ""
}
