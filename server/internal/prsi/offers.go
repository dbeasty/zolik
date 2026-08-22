package prsi

import (
	"sort"

	"zolik/server/internal/module"
)

// Offer IDs. Stable, and few — Prší has three verbs to Žolíky's nine.
const (
	OfferPlay = "play_card"
	OfferDraw = "draw"
	OfferPass = "pass"
)

// LegalActions answers "what may this player do right now?".
//
// Built the same way Žolíky's is, and for the same reason: every
// enabled/disabled decision comes from *probing the real engine* against a
// copy of the state and reading back its error code. Nothing here restates a
// rule, so the offer list cannot drift from Apply — it is Apply's own answer,
// asked in advance. That the identical construction works for a game with no
// melds and no phases is the point of the exercise.
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
		// Report the engine's reason for the first card in hand, which is the
		// reason a player would actually hit. An empty hand cannot happen on
		// a live turn (shedding the last card ends the game).
		play.Enabled, play.WhyNot = m.probeFirst(raw, playerID, s.Hands[playerID])
	}
	play.Source = &module.Selector{
		Zone: module.FromHand, OwnerID: playerID,
		Cards: playable, MinCards: 1, MaxCards: 1,
	}
	play.Target = &module.Selector{Zone: module.FromDiscardPile}
	// The wild's suit choice. Declared only when a wild is actually among the
	// playable cards, so a client never renders a suit picker it cannot use.
	if containsWild(playable) {
		play.Params = []module.ParamSpec{suitParam()}
	}
	offers = append(offers, play)

	// --- draw ----------------------------------------------------------------
	draw := module.ActionOffer{ID: OfferDraw, Verb: VerbDraw}
	draw.Enabled, draw.WhyNot = probe(m, raw, playerID, module.Action{Verb: VerbDraw})
	draw.Source = &module.Selector{Zone: module.FromDeck}
	draw.Target = &module.Selector{Zone: module.FromHand, OwnerID: playerID}
	offers = append(offers, draw)

	// --- take a skip ---------------------------------------------------------
	pass := module.ActionOffer{ID: OfferPass, Verb: VerbPass}
	pass.Enabled, pass.WhyNot = probe(m, raw, playerID, module.Action{Verb: VerbPass})
	offers = append(offers, pass)

	return offers, nil
}

// playableCards lists which cards in hand the engine would actually accept.
//
// Probed one card at a time rather than reasoning about suits and ranks: the
// obligations (an unanswered 7, an unanswered Ace, a named suit) interact, and
// a second implementation of that interaction is exactly the drift this
// avoids. A wild is probed with a concrete suit so a missing-suit refusal is
// not mistaken for the card being unplayable.
func (m *Module) playableCards(raw module.State, s *GameState, playerID string) []string {
	seen := map[string]bool{}
	var out []string
	for _, card := range s.Hands[playerID] {
		if seen[card] {
			continue
		}
		seen[card] = true
		a := module.Action{Verb: VerbPlay, Cards: []string{card}}
		if rankOf(card) == rankWild {
			a.Params = map[string]string{"suit": suits[0]}
		}
		if ok, _ := probe(m, raw, playerID, a); ok {
			out = append(out, card)
		}
	}
	sort.Strings(out)
	return out
}

// probeFirst reports the engine's reason for the first card in hand, so a
// disabled offer still says something specific.
func (m *Module) probeFirst(raw module.State, playerID string, hand []string) (bool, string) {
	if len(hand) == 0 {
		return probe(m, raw, playerID, module.Action{Verb: VerbPlay})
	}
	return probe(m, raw, playerID, module.Action{Verb: VerbPlay, Cards: []string{hand[0]}})
}

// probe runs an action through the real engine and reports whether it would be
// accepted, plus the engine's own reason if not.
//
// No cloning needed, unlike Žolíky's equivalent: state here is a JSON blob that
// Apply decodes into a fresh value every call, so a dry run physically cannot
// touch the caller's copy. That is a quiet benefit of making state opaque — the
// aliasing hazard that needed a regression test in the rummy engine simply does
// not exist.
func probe(m *Module, raw module.State, playerID string, a module.Action) (bool, string) {
	if _, _, err := m.Apply(raw, playerID, a); err != nil {
		return false, module.CodeOf(err)
	}
	return true, ""
}

func containsWild(cards []string) bool {
	for _, c := range cards {
		if rankOf(c) == rankWild {
			return true
		}
	}
	return false
}

// suitParam declares the choice a wild demands: which suit follows it.
//
// This is the field Prší added to the module protocol. Rummy has no action
// whose input is anything but cards, so nothing in Žolíky would ever have
// asked for it.
func suitParam() module.ParamSpec {
	choices := make([]module.ParamChoice, 0, len(suits))
	for _, s := range suits {
		choices = append(choices, module.ParamChoice{Value: s, LabelKey: "suit." + s})
	}
	return module.ParamSpec{Name: "suit", LabelKey: "prompt.chooseSuit", Choices: choices}
}
