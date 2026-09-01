package rummytiles

import (
	"strconv"
	"strings"

	"zolik/server/internal/module"
)

// Offer IDs.
const (
	OfferPlace     = "place"
	OfferResetTurn = "reset_turn"
	OfferCommit    = "commit"
	OfferDraw      = "draw"
)

// LegalActions answers "what may this player do right now?", built entirely
// by probing the real engine.
//
// Most of these offers are Composite: place, add and take describe a move's
// *shape* — a hand tile, or a set — but never the exact combination a player
// means, the same offer-explosion limit extensibility-plan.md §1.1 already
// names. commit, draw, reset_turn and — usually — swap_joker are the
// exceptions: each takes no cards, or names the one card that could possibly
// work, so SubmissionFor can build them without a person.
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
	return m.turnOffers(raw, s, playerID), nil
}

func placeholderOffers(why string) []module.ActionOffer {
	return []module.ActionOffer{
		{ID: OfferPlace, Verb: VerbPlace, LabelKey: "rummytiles.offer.place", WhyNot: why},
		{ID: OfferResetTurn, Verb: VerbResetTurn, LabelKey: "rummytiles.offer.resetTurn", WhyNot: why},
		{ID: OfferCommit, Verb: VerbCommit, LabelKey: "rummytiles.offer.commit", WhyNot: why},
		{ID: OfferDraw, Verb: VerbDraw, LabelKey: "rummytiles.offer.draw", WhyNot: why},
	}
}

func (m *Module) probe(raw module.State, playerID string, a module.Action) (bool, string) {
	if _, _, err := m.Apply(raw, playerID, a); err != nil {
		return false, module.CodeOf(err)
	}
	return true, ""
}

func setFact(set Set) module.Fact {
	return module.Fact{LabelKey: "rummytiles.fact.setCards", Value: strings.Join(set.Cards, ",")}
}

func (m *Module) turnOffers(raw module.State, s *GameState, playerID string) []module.ActionOffer {
	hand := s.Hands[playerID]
	ws := s.Workspace
	offers := make([]module.ActionOffer, 0, 8+4*len(ws.Sets))

	place := module.ActionOffer{
		ID: OfferPlace, Verb: VerbPlace, LabelKey: "rummytiles.offer.place", Composite: true,
		Source: &module.Selector{Zone: module.FromHand, OwnerID: playerID, ZoneID: handZoneID(playerID),
			Cards: sortedUnique(hand), MinCards: 1},
	}
	if len(hand) > 0 {
		place.Enabled, place.WhyNot = m.probe(raw, playerID, module.Action{OfferID: place.ID, Verb: VerbPlace, Cards: hand[:1]})
	} else {
		place.WhyNot = ErrTileNotInHand
	}
	offers = append(offers, place)

	for _, set := range ws.Sets {
		offers = append(offers, m.addOffer(raw, s, playerID, set, module.FromHand, sortedUnique(hand))...)
		offers = append(offers, m.addOffer(raw, s, playerID, set, module.FromTable, sortedUnique(ws.Tray))...)
		offers = append(offers, m.takeOffer(raw, s, playerID, set))
		offers = append(offers, m.splitOffer(raw, s, playerID, set))
		if o, ok := m.swapJokerOffer(raw, s, playerID, set, hand); ok {
			offers = append(offers, o)
		}
	}

	reset := module.ActionOffer{ID: OfferResetTurn, Verb: VerbResetTurn, LabelKey: "rummytiles.offer.resetTurn"}
	reset.Enabled, reset.WhyNot = m.probe(raw, playerID, module.Action{OfferID: reset.ID, Verb: VerbResetTurn})
	offers = append(offers, reset)

	commit := module.ActionOffer{ID: OfferCommit, Verb: VerbCommit, LabelKey: "rummytiles.offer.commit"}
	commit.Enabled, commit.WhyNot = m.probe(raw, playerID, module.Action{OfferID: commit.ID, Verb: VerbCommit})
	offers = append(offers, commit)

	draw := module.ActionOffer{ID: OfferDraw, Verb: VerbDraw, LabelKey: "rummytiles.offer.draw"}
	draw.Enabled, draw.WhyNot = m.probe(raw, playerID, module.Action{OfferID: draw.ID, Verb: VerbDraw})
	offers = append(offers, draw)

	return offers
}

func (m *Module) addOffer(raw module.State, s *GameState, playerID string, set Set, zone module.SelectorZone, cards []string) []module.ActionOffer {
	if zone == module.FromTable && len(cards) == 0 {
		return nil
	}
	var o module.ActionOffer
	if zone == module.FromTable {
		o = module.ActionOffer{
			ID: "add:" + set.ID + ":tray", Verb: VerbAdd, LabelKey: "rummytiles.offer.addFromTray", Composite: true,
			Target: &module.Selector{MeldID: set.ID},
			Facts:  []module.Fact{setFact(set)},
		}
	} else {
		o = module.ActionOffer{
			ID: "add:" + set.ID + ":hand", Verb: VerbAdd, LabelKey: "rummytiles.offer.addFromHand", Composite: true,
			Target: &module.Selector{MeldID: set.ID},
			Facts:  []module.Fact{setFact(set)},
		}
	}
	if len(cards) == 0 {
		o.WhyNot = ErrTileNotInHand
		return []module.ActionOffer{o}
	}
	o.Source = &module.Selector{Zone: zone, OwnerID: playerID, ZoneID: handZoneID(playerID), Cards: cards, MinCards: 1}
	if zone == module.FromTable {
		o.Source.ZoneID = trayZoneID
	}
	o.Enabled, o.WhyNot = m.probe(raw, playerID, module.Action{OfferID: o.ID, Verb: VerbAdd, Target: set.ID, Cards: cards[:1]})
	return []module.ActionOffer{o}
}

func (m *Module) takeOffer(raw module.State, s *GameState, playerID string, set Set) module.ActionOffer {
	cards := sortedUnique(set.Cards)
	// Target names the set this move concerns, per the convention every
	// verb here shares — SubmissionFor only ever reads Target.MeldID into
	// Action.Target, never Source.MeldID, so a take's addressed set has to
	// travel there too even though the tiles' source, not their
	// destination, is what Target usually means elsewhere.
	o := module.ActionOffer{
		ID: "take:" + set.ID, Verb: VerbTake, LabelKey: "rummytiles.offer.take", Composite: true,
		Source: &module.Selector{Zone: module.FromMeld, MeldID: set.ID, Cards: cards, MinCards: 1},
		Target: &module.Selector{MeldID: set.ID},
		Facts:  []module.Fact{setFact(set)},
	}
	if len(cards) == 0 {
		o.WhyNot = ErrTileDoesNotFit
		return o
	}
	o.Enabled, o.WhyNot = m.probe(raw, playerID, module.Action{OfferID: o.ID, Verb: VerbTake, Target: set.ID, Cards: cards[:1]})
	return o
}

func (m *Module) splitOffer(raw module.State, s *GameState, playerID string, set Set) module.ActionOffer {
	o := module.ActionOffer{
		ID: "split:" + set.ID, Verb: VerbSplit, LabelKey: "rummytiles.offer.split",
		Target: &module.Selector{MeldID: set.ID},
		Facts:  []module.Fact{setFact(set)},
	}
	kind, canonical, ok := validateSet(set.Cards)
	if !ok || kind != "run" || len(canonical) < 6 {
		o.WhyNot = ErrNotARun
		return o
	}
	o.Params = []module.ParamSpec{{
		Name: "position", Kind: module.ParamKindInt,
		LabelKey: "rummytiles.param.position",
		Min:      3, Max: len(canonical) - 3, Default: 3,
	}}
	o.Enabled, o.WhyNot = m.probe(raw, playerID, module.Action{
		OfferID: o.ID, Verb: VerbSplit, Target: set.ID, Params: map[string]string{"position": strconv.Itoa(3)},
	})
	return o
}

// swapJokerOffer is enumerable in full: every hand tile that would actually
// match a joker in this set is a distinct, individually-legal submission, so
// unlike place/add/take this needs no Composite escape hatch.
func (m *Module) swapJokerOffer(raw module.State, s *GameState, playerID string, set Set, hand []string) (module.ActionOffer, bool) {
	kind, canonical, ok := validateSet(set.Cards)
	if !ok {
		return module.ActionOffer{}, false
	}
	hasJoker := false
	for _, c := range canonical {
		if isJoker(c) {
			hasJoker = true
		}
	}
	if !hasJoker {
		return module.ActionOffer{}, false
	}

	var matching []string
	seen := map[string]bool{}
	for _, tile := range hand {
		if seen[tile] || isJoker(tile) {
			continue
		}
		seen[tile] = true
		if _, errCode := jokerMatching(kind, canonical, tile); errCode == "" {
			matching = append(matching, tile)
		}
	}

	o := module.ActionOffer{
		ID: "swap_joker:" + set.ID, Verb: VerbSwapJoker, LabelKey: "rummytiles.offer.swapJoker",
		Target: &module.Selector{MeldID: set.ID},
		Facts:  []module.Fact{setFact(set)},
	}
	if len(matching) == 0 {
		o.WhyNot = ErrJokerSwapMismatch
		return o, true
	}
	o.Source = &module.Selector{Zone: module.FromHand, OwnerID: playerID, ZoneID: handZoneID(playerID),
		Cards: matching, MinCards: 1}
	o.Enabled, o.WhyNot = m.probe(raw, playerID, module.Action{
		OfferID: o.ID, Verb: VerbSwapJoker, Target: set.ID, Cards: matching[:1],
	})
	return o, true
}
