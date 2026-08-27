package rules

import (
	"sort"
	"strconv"
)

// This file answers "what may this player do right now?" as data, so no
// client ever has to work it out from raw state.
//
// The whole point is anti-drift. Every enabled/disabled decision below is
// produced by *probing the real validator* — the same function ApplyAction
// calls — against a throwaway clone of the state, and reading back the
// RulesError it returns. LegalActions therefore cannot disagree with
// ApplyAction, because it is not a second opinion: it is ApplyAction's own
// answer, asked in advance. Nothing here restates a rule.
//
// Authority is unchanged: an offer is a rendering input, never a permission
// grant. Every concrete submission is still validated on arrival.

// Zone names an area of the board an offer reads from or writes to. These
// are deliberately generic — a game with no discard pile simply never
// mentions ZoneDiscardPile.
type Zone string

const (
	ZoneHand        Zone = "hand"
	ZoneDeck        Zone = "deck"
	ZoneDiscardPile Zone = "discard_pile"
	ZoneMeld        Zone = "meld"
	ZoneTable       Zone = "table"
)

// OfferVerb is the kind of move an offer describes. It mirrors ActionType
// for this module, but is a separate vocabulary on purpose: a verb is what
// the *interface* offers ("undo"), while an ActionType is what the engine
// applies ("undo_lay_off").
type OfferVerb string

const (
	VerbDraw      OfferVerb = "draw"
	VerbLayMeld   OfferVerb = "lay_meld"
	VerbLayOff    OfferVerb = "lay_off"
	VerbSwapJoker OfferVerb = "swap_joker"
	VerbDiscard   OfferVerb = "discard"
	VerbUndo      OfferVerb = "undo"
)

// Stable offer IDs. A client matches on these and echoes nothing back (the
// action vocabulary on the wire is unchanged in this phase), so they are
// free to be human-readable. Lay-off and joker-swap offers are per-meld and
// suffix the meld ID — see LayOffOfferID / SwapJokerOfferID.
const (
	OfferDrawDeck        = "draw:deck"
	OfferDrawDiscard     = "draw:discard"
	OfferLayMeld         = "lay_meld"
	OfferDiscard         = "discard"
	OfferUndoDrawDiscard = "undo:draw_discard"
	OfferUndoLayOff      = "undo:lay_off"
	OfferUndoLayMeld     = "undo:lay_meld"
	OfferUndoTurn        = "undo:turn"
	offerLayOffPrefix    = "lay_off:"
	offerSwapJokerPrefix = "swap_joker:"
)

// LayOffOfferID is the offer ID for extending the meld with this ID.
func LayOffOfferID(meldID string) string { return offerLayOffPrefix + meldID }

// SwapJokerOfferID is the offer ID for swapping this meld's joker out.
func SwapJokerOfferID(meldID string) string { return offerSwapJokerPrefix + meldID }

// Placement is one card the offer would accept, plus — for a run — which
// end(s) of it that card may extend. Sets have no ends, so Positions is
// empty for them.
//
// A placement means "legal on its own". Multi-card submissions are not
// enumerated (that is combinatorial — see the offer-explosion risk in the
// architecture blueprint) and are validated on arrival instead.
type Placement struct {
	Card      string   `json:"card"`
	Positions []string `json:"positions,omitempty"` // "front" and/or "end"
}

// Selector describes where an offer's cards come from, or where they go.
type Selector struct {
	Zone    Zone   `json:"zone"`
	OwnerID string `json:"ownerId,omitempty"`
	MeldID  string `json:"meldId,omitempty"`

	// Cards are the individually-eligible cards in Zone. Always the card
	// values of Placements when Placements is set, so the two can never
	// disagree — simple clients read Cards, drag-and-drop clients read
	// Placements for the run-end hints.
	Cards      []string    `json:"cards,omitempty"`
	Placements []Placement `json:"placements,omitempty"`

	// MinCards/MaxCards bound a *shape* offer (lay_meld), where listing
	// every legal card combination is not feasible.
	MinCards int `json:"minCards,omitempty"`
	MaxCards int `json:"maxCards,omitempty"`
}

// ActionOffer is one affordance the interface may present. The full set is
// always returned, disabled entries included: "greyed out, and here is why"
// is a UI requirement, and an omitted offer is indistinguishable from a
// client bug.
type ActionOffer struct {
	ID      string    `json:"id"`
	Verb    OfferVerb `json:"verb"`
	Enabled bool      `json:"enabled"`
	// WhyNot is the engine's own error code for why this is unavailable —
	// empty when Enabled. It is a stable key, not a sentence, so clients
	// own the wording and a locale bundle can be added without touching
	// the server (see Phase 2 of the extensibility plan).
	WhyNot RulesErrorCode `json:"whyNot,omitempty"`

	// LabelKey names the control when the verb cannot tell two offers apart.
	// This engine offers "draw" twice — from the deck and from the discard
	// pile — and labelled by verb alone they are two buttons reading the same
	// word and doing different things. A key, never a sentence.
	LabelKey string `json:"labelKey,omitempty"`

	// Facts are printed on the control itself. Here they say *which* meld an
	// offer acts on: there is one lay-off and one joker swap per meld on the
	// table, and without saying which they are a row of identical buttons.
	Facts []OfferFact `json:"facts,omitempty"`

	Source *Selector `json:"source,omitempty"`
	Target *Selector `json:"target,omitempty"`
}

// OfferFact is a labelled value shown on a control. A key and a value, never
// a sentence, like every other label this engine emits.
type OfferFact struct {
	LabelKey string `json:"labelKey"`
	Value    string `json:"value,omitempty"`
}

// LegalActions returns every offer this module can present to playerID in
// this state, in a stable order.
//
// Pure and allocation-bounded: coarse per-verb gating costs one state clone
// per offer (roughly a dozen), and the per-card eligibility lists are only
// computed for the player whose turn it actually is — every other viewer
// gets the same offer set, disabled, with no card lists. That keeps the
// cost of calling this on every broadcast independent of table size.
func LegalActions(state GameState, playerID string) []ActionOffer {
	cfg := effectiveRules(state)
	active := state.CurrentTurn == playerID &&
		state.Status == StatusActive &&
		state.Phase != PhaseSuspended
	hand := state.Hands[playerID]

	offers := make([]ActionOffer, 0, 10+2*countMelds(state))

	// --- draw -------------------------------------------------------------
	deckOffer := ActionOffer{ID: OfferDrawDeck, Verb: VerbDraw, LabelKey: "verb.drawFromDeck"}
	deckOffer.Enabled, deckOffer.WhyNot = probe(state, playerID, Action{
		Type: ActionDrawCard, DrawFrom: DrawFromDeck,
	})
	deckOffer.Source = &Selector{Zone: ZoneDeck}
	deckOffer.Target = &Selector{Zone: ZoneHand, OwnerID: playerID}
	offers = append(offers, deckOffer)

	// --- discard ------------------------------------------------------------
	// Built here, right behind the deck draw, rather than after every meld and
	// lay-off control below: the two ends of an ordinary turn — take a card,
	// get rid of one — belong next to each other on screen, not separated by
	// a run of controls most turns never touch.
	discard := ActionOffer{ID: OfferDiscard, Verb: VerbDiscard}
	// Probed with a real card so the joker restriction and the
	// incomplete-initial-meld/pending-pickup obligations are all exercised;
	// falls back to a phase-only probe for an empty hand.
	discard.Enabled, discard.WhyNot = probeDiscard(state, playerID, hand)
	dsrc := &Selector{Zone: ZoneHand, OwnerID: playerID, MinCards: 1, MaxCards: 1}
	if active {
		dsrc.Cards = discardableCards(state, playerID, hand)
	}
	discard.Source = dsrc
	discard.Target = &Selector{Zone: ZoneDiscardPile}
	offers = append(offers, discard)

	discardDraw := ActionOffer{ID: OfferDrawDiscard, Verb: VerbDraw, LabelKey: "verb.takeFromDiscard"}
	discardDraw.Enabled, discardDraw.WhyNot = probe(state, playerID, Action{
		Type: ActionDrawCard, DrawFrom: DrawFromDiscard,
	})
	src := &Selector{Zone: ZoneDiscardPile}
	if discardDraw.Enabled {
		src.Cards = drawablePileCards(state, cfg)
	}
	discardDraw.Source = src
	discardDraw.Target = &Selector{Zone: ZoneHand, OwnerID: playerID}
	offers = append(offers, discardDraw)

	// --- lay_meld ---------------------------------------------------------
	// A shape offer: which concrete card combinations are valid is
	// combinatorial, so the offer bounds the shape and the server validates
	// the submission. Probed with an empty card list, which fails the phase
	// and turn checks first — exactly the gating a client needs — and only
	// reaches "no cards" once those pass.
	layMeld := ActionOffer{ID: OfferLayMeld, Verb: VerbLayMeld}
	layMeld.Enabled, layMeld.WhyNot = probeMeldPhase(state, playerID)
	minMeld := cfg.MinSetSize
	if cfg.MinRunSize < minMeld {
		minMeld = cfg.MinRunSize
	}
	layMeld.Source = &Selector{
		Zone: ZoneHand, OwnerID: playerID,
		MinCards: minMeld, MaxCards: maxMeldSize(hand, cfg, state.GameNumber),
	}
	layMeld.Target = &Selector{Zone: ZoneTable}
	offers = append(offers, layMeld)

	// --- lay_off / swap_joker, one offer per table meld --------------------
	for _, m := range tableMelds(state) {
		offers = append(offers, layOffOffer(state, cfg, playerID, m, active))
		offers = append(offers, swapJokerOffer(state, playerID, m, active))
	}

	// --- undo -------------------------------------------------------------
	//
	// Four of them, each undoing a different thing, so each says which — as
	// one row of buttons all reading "Undo" they are a guess rather than a
	// choice, whether or not they happen to be available.
	for _, u := range []struct {
		id    string
		at    ActionType
		label string
	}{
		{OfferUndoDrawDiscard, ActionUndoDrawDiscard, "verb.undoDraw"},
		{OfferUndoLayOff, ActionUndoLayOff, "verb.undoLayOff"},
		{OfferUndoLayMeld, ActionUndoLayMeld, "verb.undoMeld"},
		{OfferUndoTurn, ActionUndoTurn, "verb.undoTurn"},
	} {
		o := ActionOffer{ID: u.id, Verb: VerbUndo, LabelKey: u.label}
		o.Enabled, o.WhyNot = probe(state, playerID, Action{Type: u.at})
		offers = append(offers, o)
	}

	return offers
}

// FindOffer returns the offer with this ID, or nil.
func FindOffer(offers []ActionOffer, id string) *ActionOffer {
	for i := range offers {
		if offers[i].ID == id {
			return &offers[i]
		}
	}
	return nil
}

// probe runs the action through the real engine against a throwaway clone
// and reports whether it would be accepted, plus the engine's own reason if
// not. This is the anti-drift mechanism: no rule is restated here.
func probe(state GameState, playerID string, action Action) (bool, RulesErrorCode) {
	_, err := ApplyAction(cloneState(state), playerID, action)
	if err == nil {
		return true, ""
	}
	return false, codeOf(err)
}

// probeMeldPhase asks whether a lay_meld could happen at all, ignoring which
// cards it would use. A lay_meld with no cards fails ValidateMeld's minimum
// size *after* the status/phase/turn checks, so any code other than the
// generic INVALID_MELD is a real gate the client should surface; the
// INVALID_MELD case means "you may meld, you just have not chosen cards".
func probeMeldPhase(state GameState, playerID string) (bool, RulesErrorCode) {
	ok, code := probe(state, playerID, Action{Type: ActionLayMeld})
	if ok {
		// Cannot happen (an empty meld is never valid), but if the engine
		// ever accepted it, believe the engine.
		return true, ""
	}
	if code == ErrInvalidMeld {
		return true, ""
	}
	return false, code
}

// probeDiscard gates the discard verb using a card actually in hand, so the
// joker restriction and the two initial-meld obligations are all exercised.
// A card-specific rejection (CARD_NOT_IN_HAND, JOKER_DISCARD_FORBIDDEN) is
// about *that* card, not about the verb — the verb stays available as long
// as some card in hand is discardable, which discardableCards then lists.
func probeDiscard(state GameState, playerID string, hand []string) (bool, RulesErrorCode) {
	if len(hand) == 0 {
		return probe(state, playerID, Action{Type: ActionDiscard})
	}
	var cardSpecific RulesErrorCode
	for _, c := range hand {
		ok, code := probe(state, playerID, Action{Type: ActionDiscard, Card: c})
		if ok {
			return true, ""
		}
		if !isCardSpecific(code) {
			// A gate that has nothing to do with which card was tried —
			// every other card would hit it too, so report it and stop
			// (this also keeps a non-current viewer to a single probe).
			return false, code
		}
		if cardSpecific == "" {
			cardSpecific = code
		}
	}
	return false, cardSpecific
}

func isCardSpecific(code RulesErrorCode) bool {
	return code == ErrJokerDiscard || code == ErrCardNotInHand
}

// discardableCards lists which cards in hand would actually be accepted as
// this turn's discard — again by probing, so the joker rule and its
// end-of-hand exception need no second implementation.
func discardableCards(state GameState, playerID string, hand []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(hand))
	for _, c := range hand {
		if seen[c] {
			continue
		}
		seen[c] = true
		if ok, _ := probe(state, playerID, Action{Type: ActionDiscard, Card: c}); ok {
			out = append(out, c)
		}
	}
	return out
}

// drawablePileCards lists which discard-pile cards this ruleset allows the
// draw to target. Under top_only that is the top card; under any_from_pile
// it is every card in the pile (taking one also takes everything above it).
func drawablePileCards(state GameState, cfg RulesConfig) []string {
	if len(state.DiscardPile) == 0 {
		return nil
	}
	if cfg.DiscardPickupMode == DiscardPickupAnyFromPile {
		return append([]string(nil), state.DiscardPile...)
	}
	return []string{state.DiscardPile[len(state.DiscardPile)-1]}
}

// maxMeldSize is the largest meld a player could lay: their whole hand,
// except that on a non-final deal they must keep a card back to discard.
func maxMeldSize(hand []string, cfg RulesConfig, gameNumber int) int {
	n := len(hand)
	if !cfg.IsFinalDeal(gameNumber) && n > 0 {
		n--
	}
	return n
}

type tableMeld struct {
	OwnerID string
	MeldID  string
	Index   int
	Cards   []string
	Meta    MeldInfo
}

// tableMelds lists every meld on the table in a stable order (owner, then
// index) — Go map iteration order is not stable, and an offer list that
// reshuffles itself between pushes makes clients flicker and tests flake.
func tableMelds(state GameState) []tableMeld {
	owners := make([]string, 0, len(state.Melds))
	for owner := range state.Melds {
		owners = append(owners, owner)
	}
	sort.Strings(owners)

	var out []tableMeld
	for _, owner := range owners {
		metas := state.MeldMeta[owner]
		for i, cards := range state.Melds[owner] {
			meta := MeldInfo{}
			if i < len(metas) {
				meta = metas[i]
			}
			out = append(out, tableMeld{
				OwnerID: owner, MeldID: meta.MeldID, Index: i,
				Cards: cards, Meta: meta,
			})
		}
	}
	return out
}

func countMelds(state GameState) int {
	n := 0
	for _, ms := range state.Melds {
		n += len(ms)
	}
	return n
}

func layOffOffer(state GameState, cfg RulesConfig, playerID string, m tableMeld, active bool) ActionOffer {
	o := ActionOffer{
		ID: LayOffOfferID(m.MeldID), Verb: VerbLayOff,
		Facts: []OfferFact{{LabelKey: "zolik.offer.meld", Value: strconv.Itoa(m.Index + 1)}},
	}
	o.Target = &Selector{Zone: ZoneMeld, OwnerID: m.OwnerID, MeldID: m.MeldID}

	// Gate the verb by probing with a card that is in hand, so the probe
	// reaches the real lay-off checks rather than tripping over
	// CARD_NOT_IN_HAND first. Which specific cards fit is then answered by
	// the placement scan below.
	hand := state.Hands[playerID]
	var code RulesErrorCode
	if len(hand) == 0 {
		_, code = probe(state, playerID, Action{Type: ActionLayOff, MeldID: m.MeldID})
	} else {
		_, code = probe(state, playerID, Action{
			Type: ActionLayOff, MeldID: m.MeldID, Card: hand[0],
		})
	}
	// A rejection about the specific probe card (it does not fit this meld,
	// or the meld would be emptied) says nothing about the verb: the offer
	// stays available and its placement list is what is empty.
	switch code {
	// ErrMeldBelowMinimum is deliberately not in this list: it is now raised
	// by the round-requirement gate (see notDownError), where it means the
	// player may not lay off at all — a verb-level refusal, not a complaint
	// about the probe card.
	case "", ErrInvalidMeld, ErrTooManyWilds, ErrSetTooLarge, ErrRunTooLong,
		ErrAdjacentWilds, ErrAceBridge, ErrWrongRunEnd:
		o.Enabled = true
	default:
		o.Enabled = false
		o.WhyNot = code
	}

	if !o.Enabled || !active {
		o.Source = &Selector{Zone: ZoneHand, OwnerID: playerID, MinCards: 1}
		return o
	}

	placements := layOffPlacements(state, cfg, playerID, m)
	o.Source = &Selector{
		Zone: ZoneHand, OwnerID: playerID, MinCards: 1, MaxCards: len(hand),
		Cards: cardsOf(placements), Placements: placements,
	}
	if len(placements) == 0 {
		// The verb is available in principle but nothing in hand fits this
		// particular meld — a real, showable reason.
		o.Enabled = false
		o.WhyNot = ErrInvalidMeld
	}
	return o
}

// layOffPlacements finds every single card in hand that legally extends this
// meld, and for a run, which end(s) it may extend. Pure: uses the same
// state-free helpers the AI does, so no state clone per card.
func layOffPlacements(state GameState, cfg RulesConfig, playerID string, m tableMeld) []Placement {
	seen := map[string]bool{}
	var out []Placement
	for _, c := range state.Hands[playerID] {
		if seen[c] {
			continue
		}
		seen[c] = true

		extended := append(append([]string(nil), m.Cards...), c)
		mv, err := ValidateMeld(extended, cfg)
		if err != nil {
			continue
		}
		// Laying this off must not empty the hand on a non-final deal —
		// the same rule ValidateLayOff applies at the end.
		if !cfg.IsFinalDeal(state.GameNumber) && len(state.Hands[playerID]) == 1 {
			continue
		}
		p := Placement{Card: c}
		if mv.Type == MeldRun {
			p.Positions = droppableEnds(m.Cards, extended, cfg)
		}
		out = append(out, p)
	}
	return out
}

// droppableEnds lists which ends of an existing run a submission may be
// dropped on.
//
// It asks the question the way ValidateLayOff answers it, once per end,
// rather than resolving the run once and reading off the result. Two reasons,
// both found by TestLegalActions_RunEndHintsMatchTheValidator:
//
//   - The validator re-resolves the run *preferring the end dropped on*, so a
//     wild card is legal at the front and at the end even though any single
//     resolution names only one.
//   - When the ends cannot be resolved at all the validator imposes no
//     constraint and accepts either end, so "unresolvable" means droppable,
//     not undroppable.
//
// An empty result means "legal, but send no position hint" — the submission
// grows both ends at once, which is exactly what naming either one would get
// rejected for.
func droppableEnds(prevCards, submission []string, cfg RulesConfig) []string {
	minRun := cfg.MinRunSize
	if minRun == 0 {
		minRun = 4
	}
	var sides []string
	for _, position := range []string{"front", "end"} {
		mv, err := validateRun(submission, minRun, position == "end")
		if err != nil {
			continue
		}
		resolved, known := runGrowthSides(prevCards, mv)
		if known && !containsString(resolved, position) {
			continue // the validator would reject this end
		}
		sides = append(sides, position)
	}
	// Growing both ends at once is rejected whichever end is named, so offer
	// no hint at all rather than one the server will refuse.
	if len(sides) == 2 {
		if mv, err := validateRun(submission, minRun); err == nil {
			if resolved, known := runGrowthSides(prevCards, mv); known && len(resolved) == 0 {
				return nil
			}
		}
	}
	return sides
}

func swapJokerOffer(state GameState, playerID string, m tableMeld, active bool) ActionOffer {
	o := ActionOffer{
		ID: SwapJokerOfferID(m.MeldID), Verb: VerbSwapJoker,
		Facts: []OfferFact{{LabelKey: "zolik.offer.meld", Value: strconv.Itoa(m.Index + 1)}},
	}
	o.Target = &Selector{Zone: ZoneMeld, OwnerID: m.OwnerID, MeldID: m.MeldID}
	o.Source = &Selector{Zone: ZoneHand, OwnerID: playerID, MinCards: 1, MaxCards: 1}

	hand := state.Hands[playerID]
	// Probe with a natural card from hand so the probe reaches the real
	// checks; a joker or an empty hand would trip earlier guards that are
	// about the probe, not about the offer.
	probeCard := ""
	for _, c := range hand {
		if !IsJoker(c) {
			probeCard = c
			break
		}
	}
	_, code := probe(state, playerID, Action{
		Type: ActionSwapJoker, MeldID: m.MeldID, Card: probeCard,
	})
	switch code {
	case "", ErrJokerSwapMismatch:
		// Mismatch is about the probe card, not the offer.
		o.Enabled = true
	default:
		o.Enabled = false
		o.WhyNot = code
	}
	if !o.Enabled || !active {
		return o
	}

	var cards []string
	seen := map[string]bool{}
	for _, c := range hand {
		if seen[c] || IsJoker(c) {
			continue
		}
		seen[c] = true
		if ok, _ := probe(state, playerID, Action{
			Type: ActionSwapJoker, MeldID: m.MeldID, Card: c,
		}); ok {
			cards = append(cards, c)
		}
	}
	o.Source.Cards = cards
	if len(cards) == 0 {
		o.Enabled = false
		o.WhyNot = ErrNoJokerInMeld
	}
	return o
}

func cardsOf(ps []Placement) []string {
	out := make([]string, 0, len(ps))
	for _, p := range ps {
		out = append(out, p.Card)
	}
	return out
}

func codeOf(err error) RulesErrorCode {
	if re, ok := err.(RulesError); ok {
		return re.Code
	}
	return ErrInvalidMeld
}
