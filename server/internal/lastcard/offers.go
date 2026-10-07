package lastcard

import (
	"sort"

	"zolik/server/internal/module"
)

// Offer IDs.
const (
	OfferPlay = "play_card"
	OfferDraw = "draw"
	OfferPass = "pass"
)

// LegalActions answers "what may this player do right now?", built the way
// Prší's is: every enabled/disabled decision is the real engine's answer,
// probed against the state, so the offer list cannot drift from Apply.
func (m *Module) LegalActions(raw module.State, playerID string) ([]module.ActionOffer, error) {
	s, err := decode(raw)
	if err != nil {
		return nil, err
	}
	offers := make([]module.ActionOffer, 0, 3)

	// --- play a card ---------------------------------------------------------
	play := module.ActionOffer{ID: OfferPlay, Verb: VerbPlay}
	playable := m.playableCards(raw, s, playerID)
	if len(playable) > 0 {
		play.Enabled = true
	} else {
		play.Enabled, play.WhyNot = m.probeFirst(raw, s, playerID)
	}
	play.Source = &module.Selector{
		Zone: module.FromHand, OwnerID: playerID, ZoneID: handZoneID(playerID),
		Cards: playable, MinCards: 1, MaxCards: 1,
	}
	play.Target = &module.Selector{Zone: module.FromDiscardPile, ZoneID: discardZoneID}
	if containsWild(playable) {
		play.Params = []module.ParamSpec{colourParam()}
	}
	offers = append(offers, play)

	// --- draw ----------------------------------------------------------------
	draw := module.ActionOffer{ID: OfferDraw, Verb: VerbDraw}
	draw.Enabled, draw.WhyNot = probe(m, raw, playerID, module.Action{Verb: VerbDraw})
	draw.Source = &module.Selector{Zone: module.FromDeck, ZoneID: drawZoneID}
	draw.Target = &module.Selector{Zone: module.FromHand, OwnerID: playerID, ZoneID: handZoneID(playerID)}
	offers = append(offers, draw)

	// --- keep the card just drawn -------------------------------------------
	// Labelled for what it does here: the card just drawn stays in hand.
	pass := module.ActionOffer{ID: OfferPass, Verb: VerbPass, LabelKey: "lastcard.offer.keep"}
	pass.Enabled, pass.WhyNot = probe(m, raw, playerID, module.Action{Verb: VerbPass})
	offers = append(offers, pass)

	m.annotate(module.MatchConfig{}, s, offers)
	return offers, nil
}

// playableCards lists which cards in hand the engine would actually accept,
// probing each with a concrete colour so a wild's missing-colour refusal is
// not mistaken for the card being unplayable.
func (m *Module) playableCards(raw module.State, s *GameState, playerID string) []string {
	seen := map[string]bool{}
	var out []string
	for _, card := range s.Hands[playerID] {
		if seen[card] {
			continue
		}
		seen[card] = true
		if ok, _ := probe(m, raw, playerID, playAction(card)); ok {
			out = append(out, card)
		}
	}
	sort.Strings(out)
	return out
}

// probeFirst reports the engine's reason a card cannot be played, preferring
// the most specific one in hand: a Wild Draw Four held back by its colour
// says more than "doesn't match", and is the refusal a player holding one
// will actually wonder about.
func (m *Module) probeFirst(raw module.State, s *GameState, playerID string) (bool, string) {
	hand := s.Hands[playerID]
	if len(hand) == 0 {
		return probe(m, raw, playerID, module.Action{Verb: VerbPlay})
	}
	why := ""
	for _, card := range hand {
		ok, code := probe(m, raw, playerID, playAction(card))
		if ok {
			return true, ""
		}
		if why == "" || code == ErrDrawFourHeld {
			why = code
		}
	}
	return false, why
}

func playAction(card string) module.Action {
	a := module.Action{Verb: VerbPlay, Cards: []string{card}}
	if isWild(card) {
		a.Params = map[string]string{"colour": colours[0]}
	}
	return a
}

// probe runs an action through the real engine and reports whether it would
// be accepted, plus the engine's own reason if not.
func probe(m *Module, raw module.State, playerID string, a module.Action) (bool, string) {
	if _, _, err := m.Apply(raw, playerID, a); err != nil {
		return false, module.CodeOf(err)
	}
	return true, ""
}

func containsWild(cards []string) bool {
	for _, c := range cards {
		if isWild(c) {
			return true
		}
	}
	return false
}

// colourParam declares the choice a wild demands: which colour follows it.
// The four choices are written out so each key is a literal the key manifest
// can see.
func colourParam() module.ParamSpec {
	return module.ParamSpec{Name: "colour", LabelKey: "lastcard.prompt.chooseColour", Choices: []module.ParamChoice{
		{Value: "C", LabelKey: "lastcard.colour.C"},
		{Value: "T", LabelKey: "lastcard.colour.T"},
		{Value: "V", LabelKey: "lastcard.colour.V"},
		{Value: "A", LabelKey: "lastcard.colour.A"},
	}}
}
