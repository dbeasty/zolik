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
// A placement means "may take part". Most may go on their own; Requires names
// the ones that may not, and what they need for company.
//
// The old rule here was "legal on its own", with multi-card submissions left
// unenumerated as combinatorial. That was half right: enumerating every
// *combination* would explode, but enumerating each card's own prerequisite
// does not, and leaving the gap unstated meant a client reading this list as
// the whole truth refused a move the validator accepts — the 5 dropped with
// the 6 onto a run of 7-8-9-10.
type Placement struct {
	Card      string   `json:"card"`
	Positions []string `json:"positions,omitempty"` // "front" and/or "end"

	// Requires names the other cards in hand that must be laid off in the
	// same action for this one to be legal. Empty means "may go on its
	// own", which is what every placement meant before this field existed.
	//
	// The set is transitive and closed — everything the card needs, not
	// just its immediate neighbour — so a client checks membership rather
	// than walking a chain. And it is proven rather than derived: the
	// server ran ValidateMeld over the meld plus these cards plus this one.
	Requires []string `json:"requires,omitempty"`
}

// Selector describes where an offer's cards come from, or where they go.
type Selector struct {
	Zone    Zone   `json:"zone"`
	OwnerID string `json:"ownerId,omitempty"`
	MeldID  string `json:"meldId,omitempty"`

	// Cards are the cards in Zone that may be sent on their own — the
	// placements with no Requires.
	//
	// Deliberately narrower than Placements, which also lists cards that
	// are legal only in company. A client that reads Cards is one that
	// sends a single card: a one-tap control (see the RN client's
	// isOneTap) or the terminal client, which submits Cards[:MinCards]
	// sight unseen. Neither may be handed a card that needs a companion.
	// Drag-and-drop clients read Placements, for the run-end hints and for
	// what a multi-card selection is allowed to contain.
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
		Cards: standaloneCardsOf(placements), Placements: placements,
	}
	if len(placements) == 0 {
		// The verb is available in principle but nothing in hand fits this
		// particular meld — a real, showable reason.
		o.Enabled = false
		o.WhyNot = ErrInvalidMeld
	}
	return o
}

// layOffPlacements finds every card in hand that can take part in a lay-off
// onto this meld, and for a run, which end(s) it may extend. Pure: uses the
// same state-free helpers the AI does, so no state clone per card.
//
// Two passes, and they may not be merged.
//
// The first is the original question — which cards extend the meld exactly as
// it lies. Those are the anchors, and they are what Source.Cards ships, what a
// one-tap control sends and what the terminal client submits blind.
//
// The second grows a working copy of the meld and asks again, so a card that
// only bridges a gap is found too: the 5 that is illegal against 7-8-9-10 and
// legal the moment the 6 goes with it. ValidateLayOff has always taken that
// submission — see TestValidateLayOff_MultiCardInOneAction — so this pass
// enumerates a fact the engine already had rather than adding a rule.
//
// Merging the passes would lose anchors. Two jokers against a run of four both
// extend it on their own, but not together (adjacent wilds), so a single
// accumulating loop would drop the second and grey out a move the engine
// accepts.
func layOffPlacements(state GameState, cfg RulesConfig, playerID string, m tableMeld) []Placement {
	hand := state.Hands[playerID]

	// Hoisted out of the scan because it never mentioned the card: on a
	// non-final deal, a player down to one card cannot lay it off at all.
	// ValidateLayOff applies the same rule to the hand it leaves behind.
	if !cfg.IsFinalDeal(state.GameNumber) && len(hand) == 1 {
		return nil
	}

	// Pass one: the anchors, measured against the meld as it lies.
	seen := map[string]bool{}
	var out []Placement
	for _, c := range hand {
		if seen[c] {
			continue
		}
		extended := append(append([]string(nil), m.Cards...), c)
		mv, err := ValidateMeld(extended, cfg)
		if err != nil {
			continue
		}
		seen[c] = true
		p := Placement{Card: c}
		if mv.Type == MeldRun {
			p.Positions = droppableEnds(m.Cards, extended, cfg)
		}
		out = append(out, p)
	}

	// Pass two is skipped outright in the common cases, which is what keeps
	// this off the hot path (see TestLegalActions_ObserverIsCheaperThanActivePlayer).
	//
	// No anchor means no chain: a bridge has to start from a card that
	// touches the meld as it lies, and that card is an anchor by
	// definition. A set cannot chain either — one card per suit, four
	// total, so a set on the table has room for at most one more and there
	// is nothing to bridge. And if every card in hand is already listed,
	// there is nothing left to find.
	if len(out) == 0 || !chainable(m, cfg) || len(out) == distinctCards(hand) {
		return out
	}

	// The closure. Anchors join the working meld without being listed
	// again — they are the bridge every deeper card hangs off.
	//
	// `remaining` shrinks as cards are taken, and `working` grows inside the
	// pass rather than between passes, so a chain running the same way the
	// hand is ordered is found in a single sweep. The outer loop is only
	// there for one running the other way.
	listed := map[string]bool{}
	for _, p := range out {
		listed[p.Card] = true
	}
	// Only a card of the run's own suit, or a wild, can ever extend it.
	// Checking that here rather than letting ValidateMeld say so is worth
	// it because this loop asks the question once per card per pass, and a
	// 13-card hand is mostly other suits (see the allocation tripwire in
	// TestLegalActions_ObserverIsCheaperThanActivePlayer).
	suit := runSuit(m.Cards)
	remaining := make([]string, 0, len(hand))
	for _, c := range hand {
		if suit == "" || IsWild(c) || CardSuit(c) == suit {
			remaining = append(remaining, c)
		}
	}
	working := append([]string(nil), m.Cards...)
	var accepted []string
	room := len(hand) - 1 // leave a card to discard
	if cfg.IsFinalDeal(state.GameNumber) {
		room = len(hand)
	}
	for len(accepted) < room {
		grew := false
		tried := make(map[string]bool, len(remaining))
		for i := 0; i < len(remaining) && len(accepted) < room; i++ {
			c := remaining[i]
			if tried[c] {
				continue
			}
			tried[c] = true
			cand := append(append([]string(nil), working...), c)
			mv, err := ValidateMeld(cand, cfg)
			if err != nil {
				continue
			}
			if !seen[c] && !listed[c] {
				need := minimalPrereq(m.Cards, accepted, c, cfg)
				pl := Placement{Card: c, Requires: need}
				if mv.Type == MeldRun {
					// The hint describes the submission this card is part
					// of, not the card alone — alone it has no submission.
					whole := append(append(append([]string(nil), m.Cards...), need...), c)
					pl.Positions = droppableEnds(m.Cards, whole, cfg)
				}
				out = append(out, pl)
				listed[c] = true
			}
			working = cand
			accepted = append(accepted, c)
			remaining = append(remaining[:i], remaining[i+1:]...)
			i--
			grew = true
		}
		if !grew {
			break
		}
	}
	return out
}

// runSuit is the suit every natural card of a run shares, or "" when the
// cards do not agree on one (a meld of nothing but wilds, or not a run at
// all) and no card can therefore be ruled out cheaply.
func runSuit(cards []string) string {
	suit := ""
	for _, c := range cards {
		if IsWild(c) {
			continue
		}
		s := CardSuit(c)
		if suit == "" {
			suit = s
			continue
		}
		if s != suit {
			return ""
		}
	}
	return suit
}

// chainable reports whether this meld is one a card could bridge a gap in.
// Only a run has gaps; a set is capped at four cards, one per suit, so it can
// take at most one more card and that card is always legal on its own.
func chainable(m tableMeld, cfg RulesConfig) bool {
	if mv, err := ValidateMeld(m.Cards, cfg); err == nil {
		return mv.Type == MeldRun
	}
	// An unvalidatable meld is not one this can reason about; the recorded
	// type is the only thing left to go on.
	return m.Meta.Type == MeldRun
}

// distinctCards counts the distinct card values in a hand.
func distinctCards(hand []string) int {
	seen := make(map[string]struct{}, len(hand))
	for _, c := range hand {
		seen[c] = struct{}{}
	}
	return len(seen)
}

// minimalPrereq is the smallest set of already-chained cards this one
// genuinely cannot do without: drop each in turn and keep the drop when the
// meld still validates. Validator-driven, so it stays right for a rule this
// function has never been told about.
//
// Newest first, and that order is load-bearing. A later card can depend on an
// earlier one, so removing an early card while a late one still sits in the
// trial set fails for the late card's sake and the early card is wrongly kept.
// Run 7-8-9-10 holding 5C 6C JC QC: shrinking forwards leaves the queen
// dangling when the jack is dropped, and reports the 5 as needing the jack.
func minimalPrereq(base, accepted []string, c string, cfg RulesConfig) []string {
	need := append([]string(nil), accepted...)
	for i := len(need) - 1; i >= 0; i-- {
		if i >= len(need) {
			continue
		}
		trial := append(append([]string(nil), need[:i]...), need[i+1:]...)
		cand := append(append(append([]string(nil), base...), trial...), c)
		if _, err := ValidateMeld(cand, cfg); err == nil {
			need = trial
		}
	}
	if len(need) == 0 {
		return nil
	}
	return need
}

// standaloneCardsOf is the subset of placements a client may send one at a
// time — everything with no Requires. See Selector.Cards.
func standaloneCardsOf(ps []Placement) []string {
	out := make([]string, 0, len(ps))
	for _, p := range ps {
		if len(p.Requires) == 0 {
			out = append(out, p.Card)
		}
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
