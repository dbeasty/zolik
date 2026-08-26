package canasta

import (
	"zolik/server/internal/module"
)

// Offer IDs. Stable and content-addressable so a client can diff them across
// pushes.
const (
	OfferDraw    = "draw:deck"
	OfferDiscard = "discard"
	// OfferTakePile is the id of the *disabled* placeholder when no capture is
	// available; a live capture is "take_pile:naturals", "take_pile:wild" or
	// "take_pile:meld:<meldId>".
	OfferTakePile = "take_pile"
	// OfferLayMeld is likewise the placeholder; live candidates are
	// "lay_meld:<rank>".
	OfferLayMeld = "lay_meld"
)

// LegalActions answers "what may this player do right now?".
//
// Every enabled/disabled decision comes from probing the real engine against
// the state and reading back its error code, exactly as the other two modules
// do. Nothing here restates a rule, so the offer list cannot drift from
// `Apply` — it is `Apply`'s own answer, asked in advance.
//
// What is different here, and deliberately so: the card lists are *concrete*.
// A Canasta meld is n of a single rank, so there are at most thirteen
// candidates and the server can hand a client the exact cards rather than a
// shape to solve. Rummy runs cannot do this — that is the offer-explosion
// limit in extensibility-plan.md §1.1 — which is why Žolíky's offers describe
// shapes and this module's do not, and why a driver that reads nothing but
// offers can play this game to the end.
func (m *Module) LegalActions(raw module.State, playerID string) ([]module.ActionOffer, error) {
	s, err := decode(raw)
	if err != nil {
		return nil, err
	}

	// Cost control, the same bargain the rummy engine strikes: only the player
	// on turn gets per-card enumeration. Everyone else gets the shape of the
	// offer set with no lists to build, which is what keeps this cheap enough
	// to run on every broadcast.
	if s.Status != "active" || s.Current != playerID {
		why := ErrNotYourTurn
		if s.Status != "active" {
			why = ErrGameNotActive
		}
		return []module.ActionOffer{
			{ID: OfferDraw, Verb: VerbDraw, WhyNot: why},
			{ID: OfferTakePile, Verb: VerbTakePile, WhyNot: why},
			{ID: OfferLayMeld, Verb: VerbLayMeld, WhyNot: why},
			{ID: OfferDiscard, Verb: VerbDiscard, WhyNot: why},
		}, nil
	}

	t := s.team(playerID)
	hand := s.Hands[playerID]
	offers := make([]module.ActionOffer, 0, 8)

	// --- draw ----------------------------------------------------------------
	draw := module.ActionOffer{ID: OfferDraw, Verb: VerbDraw}
	draw.Enabled, draw.WhyNot = probe(m, raw, playerID, module.Action{Verb: VerbDraw})
	draw.Source = &module.Selector{Zone: module.FromDeck}
	draw.Target = &module.Selector{Zone: module.FromHand, OwnerID: playerID}
	offers = append(offers, draw)

	// --- discard ---------------------------------------------------------------
	// Built here, right behind the draw, rather than after every capture and
	// meld control below: the two ends of an ordinary turn — take a card, get
	// rid of one — belong next to each other on screen, not separated by a run
	// of controls most turns never touch.
	discard := module.ActionOffer{ID: OfferDiscard, Verb: VerbDiscard}
	discardable := discardableCards(m, raw, s, playerID)
	if len(discardable) > 0 {
		discard.Enabled = true
	} else {
		var first []string
		if len(hand) > 0 {
			first = hand[:1]
		}
		discard.Enabled, discard.WhyNot = probe(m, raw, playerID, module.Action{
			Verb: VerbDiscard, Cards: first,
		})
	}
	discard.Source = &module.Selector{
		Zone: module.FromHand, OwnerID: playerID,
		Cards: discardable, MinCards: 1, MaxCards: 1,
	}
	discard.Target = &module.Selector{Zone: module.FromDiscardPile, ZoneID: discardZoneID}
	offers = append(offers, discard)

	// --- take the pile -------------------------------------------------------
	//
	// One offer per legal capture, each carrying the exact cards that make it
	// legal, because "you may take the pile" without saying how is a rule the
	// client would then have to own.
	took := 0
	for _, opt := range pileTakeOptions(s, playerID) {
		a := module.Action{Verb: VerbTakePile, Cards: opt.Cards, Target: opt.MeldID}
		ok, why := probe(m, raw, playerID, a)
		if !ok {
			continue // the engine disagrees; it is the authority, not this list
		}
		o := module.ActionOffer{ID: pileOfferID(opt), Verb: VerbTakePile, Enabled: ok, WhyNot: why}
		// Several captures can be legal at once and they are different moves;
		// labelled only by the verb they would be a row of identical buttons.
		if opt.MeldID != "" {
			o.LabelKey = "verb.takePileOntoMeld"
		} else {
			o.LabelKey = "verb.takePileFromHand"
		}
		if opt.MeldID != "" {
			o.Source = &module.Selector{Zone: module.FromDiscardPile, ZoneID: discardZoneID}
			o.Target = &module.Selector{Zone: module.ToMeld, MeldID: opt.MeldID, ZoneID: meldsZoneID(t.ID)}
		} else {
			o.Source = &module.Selector{
				Zone: module.FromHand, OwnerID: playerID, ZoneID: handZoneID(playerID),
				Cards: opt.Cards, MinCards: len(opt.Cards), MaxCards: len(opt.Cards),
			}
			o.Target = &module.Selector{Zone: module.FromDiscardPile, ZoneID: discardZoneID}
		}
		offers = append(offers, o)
		took++
	}
	if took == 0 {
		// Say why, using the engine's own refusal for the capture a player
		// would most plausibly attempt.
		o := module.ActionOffer{ID: OfferTakePile, Verb: VerbTakePile}
		o.Enabled, o.WhyNot = probe(m, raw, playerID, module.Action{
			Verb: VerbTakePile, Cards: plausibleCapture(s, playerID),
		})
		o.Source = &module.Selector{Zone: module.FromDiscardPile, ZoneID: discardZoneID}
		offers = append(offers, o)
	}

	// --- lay a new meld ------------------------------------------------------
	laid := 0
	for _, c := range newMeldCandidates(hand, t) {
		a := module.Action{Verb: VerbLayMeld, Cards: c.Cards}
		ok, _ := probe(m, raw, playerID, a)
		if !ok {
			continue
		}
		offers = append(offers, module.ActionOffer{
			ID: OfferLayMeld + ":" + c.Rank, Verb: VerbLayMeld, Enabled: true,
			// One of these per meldable rank, so the rank is what tells them
			// apart on screen.
			Facts: []module.Fact{{LabelKey: "canasta.offer.rank", Value: c.Rank}},
			Source: &module.Selector{
				Zone: module.FromHand, OwnerID: playerID, ZoneID: handZoneID(playerID),
				Cards: c.Cards, MinCards: len(c.Cards), MaxCards: len(c.Cards),
			},
			Target: &module.Selector{Zone: module.ToTable, ZoneID: meldsZoneID(t.ID)},
		})
		laid++
	}
	// Going out on a set of black threes is a real, if rare, move, and it has
	// its own candidate because nothing else would ever produce it.
	if bt := blackThreeCandidate(hand); bt != nil {
		if ok, _ := probe(m, raw, playerID, module.Action{Verb: VerbLayMeld, Cards: bt}); ok {
			offers = append(offers, module.ActionOffer{
				ID: OfferLayMeld + ":" + rankThree, Verb: VerbLayMeld, Enabled: true,
				Facts: []module.Fact{{LabelKey: "canasta.offer.rank", Value: rankThree}},
				Source: &module.Selector{
					Zone: module.FromHand, OwnerID: playerID, ZoneID: handZoneID(playerID),
					Cards: bt, MinCards: len(bt), MaxCards: len(bt),
				},
				Target: &module.Selector{Zone: module.ToTable, ZoneID: meldsZoneID(t.ID)},
			})
			laid++
		}
	}
	if laid == 0 {
		o := module.ActionOffer{ID: OfferLayMeld, Verb: VerbLayMeld}
		o.Enabled, o.WhyNot = probe(m, raw, playerID, module.Action{
			Verb: VerbLayMeld, Cards: plausibleMeld(hand, t),
		})
		o.Source = &module.Selector{
			Zone: module.FromHand, OwnerID: playerID, ZoneID: handZoneID(playerID),
			MinCards: minMeldSize, MaxCards: canastaSize,
		}
		o.Target = &module.Selector{Zone: module.ToTable, ZoneID: meldsZoneID(t.ID)}
		offers = append(offers, o)
	}

	// --- lay off onto the partnership's melds --------------------------------
	//
	// The partnership's, not the player's: either partner may extend any of
	// them, which is the fact that made Canasta a module rather than a profile.
	for i := range t.Melds {
		mm := t.Melds[i]
		eligible := layOffCards(hand, &mm)
		// One per meld the partnership has down, told apart by the rank each
		// one is built on.
		o := module.ActionOffer{
			ID: "lay_off:" + mm.ID, Verb: VerbLayOff,
			Facts: []module.Fact{{LabelKey: "canasta.offer.rank", Value: mm.Rank}},
		}
		var probeCards []string
		if len(eligible) > 0 {
			probeCards = eligible[:1]
		}
		o.Enabled, o.WhyNot = probe(m, raw, playerID, module.Action{
			Verb: VerbLayOff, Cards: probeCards, Target: mm.ID,
		})
		if !o.Enabled {
			eligible = nil
		}
		o.Source = &module.Selector{
			Zone: module.FromHand, OwnerID: playerID, ZoneID: handZoneID(playerID),
			Cards: eligible, MinCards: 1, MaxCards: canastaSize - len(mm.Cards),
		}
		o.Target = &module.Selector{Zone: module.ToMeld, MeldID: mm.ID, ZoneID: meldsZoneID(t.ID)}
		offers = append(offers, o)
	}

	return offers, nil
}

func pileOfferID(opt pileOption) string {
	if opt.MeldID != "" {
		return OfferTakePile + ":meld:" + opt.MeldID
	}
	for _, c := range opt.Cards {
		if isWild(c) {
			return OfferTakePile + ":wild"
		}
	}
	return OfferTakePile + ":naturals"
}

// discardableCards lists which cards the engine would actually accept as this
// turn's discard.
//
// Probed one card at a time rather than reasoned about: whether a discard is
// legal depends on red threes, on an unfinished initial meld, and on whether
// shedding the last card would be going out without the canastas for it. A
// second implementation of that interaction is exactly the drift this avoids.
func discardableCards(m *Module, raw module.State, s *GameState, playerID string) []string {
	seen := map[string]bool{}
	var out []string
	for _, card := range s.Hands[playerID] {
		if seen[card] {
			continue
		}
		seen[card] = true
		if ok, _ := probe(m, raw, playerID, module.Action{Verb: VerbDiscard, Cards: []string{card}}); ok {
			out = append(out, card)
		}
	}
	return sortedUnique(out)
}

// plausibleCapture is the attempt whose refusal best explains why the pile
// cannot be taken: the two cards a player would reach for.
func plausibleCapture(s *GameState, playerID string) []string {
	top := s.top()
	if top == "" {
		return nil
	}
	var naturals []string
	for _, c := range s.Hands[playerID] {
		if !isWild(c) && rankOf(c) == rankOf(top) {
			naturals = append(naturals, c)
		}
	}
	if len(naturals) >= 2 {
		return naturals[:2]
	}
	return naturals
}

// plausibleMeld is the same idea for melding: the largest same-rank group in
// hand, so a refusal says "not enough of them" or "you have not opened" rather
// than a generic no.
func plausibleMeld(hand []string, t *Team) []string {
	best := []string(nil)
	for rank, cards := range countByRank(hand) {
		if rank == rankThree || isWild(cards[0]) {
			continue
		}
		if t != nil && t.meld(rank) != nil {
			continue
		}
		if len(cards) > len(best) {
			best = cards
		}
	}
	if len(best) > canastaSize {
		best = best[:canastaSize]
	}
	return best
}

// blackThreeCandidate is the going-out meld of black threes, or nil.
func blackThreeCandidate(hand []string) []string {
	var out []string
	for _, c := range hand {
		if isBlackThree(c) {
			out = append(out, c)
		}
	}
	if len(out) < minMeldSize {
		return nil
	}
	if len(out) > 4 {
		out = out[:4]
	}
	return sortedUnique(out)
}

// probe runs an action through the real engine and reports whether it would be
// accepted, plus the engine's own reason if not.
//
// No cloning needed: state is a JSON blob that `Apply` decodes into a fresh
// value every call, so a dry run physically cannot touch the caller's copy.
func probe(m *Module, raw module.State, playerID string, a module.Action) (bool, string) {
	if _, _, err := m.Apply(raw, playerID, a); err != nil {
		return false, module.CodeOf(err)
	}
	return true, ""
}
