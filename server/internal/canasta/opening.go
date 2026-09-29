package canasta

import (
	"sort"
	"strconv"
	"strings"

	"zolik/server/internal/module"
)

// How the bot opens its side's account without walking into a dead turn.
//
// The initial-meld minimum is a property of a whole turn, so the engine lets a
// lay fall short of it and then refuses the discard that would end the turn
// (applyDiscard's ErrInitialMeldNotMet) until the minimum is met. If it can no
// longer be met, the turn has nothing left in it but undos. The engine's own
// guard, checkInitialMeld, asks meld.go's reachableValue whether the floor is
// still in reach, and that bound does not know that a side which cannot go out
// has to end its lays holding two cards, one to discard and one to keep
// (checkLeavesPlayable): it will count three kings towards the floor when
// laying them would empty the hand. So a side on 100 of 120 holding exactly
// KKK passes every check on the way in and is stranded at the end.
//
// This bot used to guard the opening with its own arithmetic — a greedy walk of
// at most two melds over the lay_meld offers — and that walk never saw the
// other two ways an unopened side puts value on the table: the pile capture,
// which draw() made at any size, and the lay-offs onto the captured meld,
// which build() made before it asked about an opening at all. Three of
// thirty-two Hard-vs-Hard matches stalled exactly there.
//
// So the question is now put to the engine instead of estimated. Before an
// unopened side captures the pile, lays a meld or lays off, the bot looks for a
// whole sequence of lays, applied through Apply one at a time, that ends with
// the account open and a discard the engine accepts (or with the side legally
// out). It makes the first move of such a sequence and nothing else. Every
// later move re-asks, and the rest of the sequence it found is still there to
// be found, so a turn that starts an opening always has a way to finish it.

// openingSearchNodes bounds how many positions one question may visit. A plan
// that exists is usually found in a handful — the search walks the richest
// lays first, and valueBound cuts off any branch that cannot reach the floor —
// so the bound is for the rare hand wide enough to wander in. Running out
// answers no, which before an opening costs a turn of waiting, the price a
// person short of the minimum pays anyway.
//
// Once something is down it costs the turn, so an opening already under way
// is asked again with committedSearchNodes before the bot gives up on it.
const (
	openingSearchNodes   = 400
	committedSearchNodes = 20000
)

// openingSearch is one question: can this side, from here, finish an opening?
type openingSearch struct {
	m        *Module
	playerID string
	// wilds is whether the plan may spend a wild card. The first attempt says
	// no, because an opening paid for out of naturals costs nothing but cards
	// and the same opening paid with a joker costs the seventh card of some
	// canasta later; see worthAWild. The second says yes, because not opening
	// is far more expensive than any wild.
	wilds bool
	limit int
	nodes int
	dead  map[string]bool
}

func newOpeningSearch(playerID string, wilds bool, limit int) *openingSearch {
	return &openingSearch{m: New(), playerID: playerID, wilds: wilds, limit: limit, dead: map[string]bool{}}
}

// openingMove is the move to start or continue an opening with, chosen from the
// offers actually on the table, or false when no offered move leads to one.
func openingMove(raw module.State, s *GameState, playerID string, mn menu) (module.Action, bool) {
	limits := []int{openingSearchNodes}
	if s.LaidThisTurn > 0 {
		limits = append(limits, committedSearchNodes)
	}
	for _, limit := range limits {
		for _, wilds := range []bool{false, true} {
			os := newOpeningSearch(playerID, wilds, limit)
			for _, a := range os.offeredLays(mn) {
				if os.after(raw, a) {
					return a, true
				}
			}
		}
	}
	return module.Action{}, false
}

// captureOpens reports that taking the pile this way leaves the side a whole
// opening to finish. A capture that already clears the floor on its own passes
// as long as the turn can still end.
func captureOpens(raw module.State, playerID string, capture module.Action) bool {
	for _, wilds := range []bool{false, true} {
		if newOpeningSearch(playerID, wilds, openingSearchNodes).after(raw, capture) {
			return true
		}
	}
	return false
}

// offeredLays is every lay_meld and lay_off the menu enables, as submissions,
// in the order the search tries them.
func (os *openingSearch) offeredLays(mn menu) []module.Action {
	var out []module.Action
	for _, o := range mn.byVerb(VerbLayMeld) {
		if a, ok := module.SubmissionFor(o); ok && (os.wilds || !containsWild(a.Cards)) {
			out = append(out, a)
		}
	}
	for _, o := range mn.byVerb(VerbLayOff) {
		if o.Source == nil || o.Target == nil {
			continue
		}
		seen := map[string]bool{}
		for _, c := range o.Source.Cards {
			if seen[c] || (!os.wilds && isWild(c)) {
				continue
			}
			seen[c] = true
			out = append(out, module.Action{OfferID: o.ID, Verb: VerbLayOff, Target: o.Target.MeldID, Cards: []string{c}})
		}
	}
	orderLays(out)
	return out
}

// after applies a and reports whether the position it leaves can finish the
// opening.
func (os *openingSearch) after(raw module.State, a module.Action) bool {
	next, events, err := os.m.Apply(raw, os.playerID, a)
	if err != nil {
		return false
	}
	for _, e := range events {
		// Laid out: the engine only allows that to a side that may go out,
		// and the turn is over with nothing stranded.
		if e.Type == "deal_ended" {
			return true
		}
	}
	return os.completes(next)
}

// completes is the search proper: is there a sequence of lays from here that
// opens the account and leaves a discard?
func (os *openingSearch) completes(raw module.State) bool {
	if os.nodes >= os.limit {
		return false
	}
	os.nodes++
	s, err := decode(raw)
	if err != nil {
		return false
	}
	t := s.team(os.playerID)
	if t == nil {
		return false
	}
	if t.HasMelded && os.canDiscard(raw, s) {
		return true
	}
	if !t.HasMelded && os.valueBound(s, t) < s.rules().meldFloor(t.Score) {
		return false
	}
	key := positionKey(s, os.playerID)
	if os.dead[key] {
		return false
	}
	for _, a := range os.lays(s, t) {
		if os.after(raw, a) {
			return true
		}
	}
	os.dead[key] = true
	return false
}

// valueBound is the most this turn could have laid by the end of it: what is
// down already, plus every card in hand the search may spend, less the
// cheapest of them when the side cannot go out — it has to finish holding a
// discard and a card to keep (checkLeavesPlayable), and a wild the search is
// not spending is already one of the two. An overestimate by construction,
// which is what lets it prune: a branch below it cannot open whatever it lays.
func (os *openingSearch) valueBound(s *GameState, t *Team) int {
	var values []int
	kept := 0
	for _, c := range s.Hands[os.playerID] {
		if isRedThree(c) || (!os.wilds && isWild(c)) {
			kept++
			continue
		}
		values = append(values, cardValue(c))
	}
	sort.Ints(values)
	if !canGoOut(s, t) && kept < 2 {
		values = values[min(2-kept, len(values)):]
	}
	total := s.LaidThisTurn
	for _, v := range values {
		total += v
	}
	return total
}

// canDiscard reports that the engine would accept some card as the discard
// that ends this turn.
func (os *openingSearch) canDiscard(raw module.State, s *GameState) bool {
	seen := map[string]bool{}
	for _, c := range s.Hands[os.playerID] {
		if seen[c] {
			continue
		}
		seen[c] = true
		if ok, _ := probe(os.m, raw, os.playerID, module.Action{Verb: VerbDiscard, Cards: []string{c}}); ok {
			return true
		}
	}
	return false
}

// lays is every meld and lay-off worth trying from a position, built the way
// the offer list builds them and left to Apply to accept or refuse.
func (os *openingSearch) lays(s *GameState, t *Team) []module.Action {
	r := s.rules()
	hand := s.Hands[os.playerID]
	var out []module.Action
	add := func(a module.Action) {
		if os.wilds || !containsWild(a.Cards) {
			out = append(out, a)
		}
	}
	cands := allMeldCandidates(r, hand, t)
	cands = append(cands, runCandidates(r, hand, t)...)
	for _, c := range cands {
		add(module.Action{Verb: VerbLayMeld, Cards: c.Cards})
	}
	if bt := blackThreeCandidate(r, hand); bt != nil {
		add(module.Action{Verb: VerbLayMeld, Cards: bt})
	}
	for i := range t.Melds {
		mm := &t.Melds[i]
		seen := map[string]bool{}
		for _, c := range layOffCards(r, hand, mm) {
			if seen[c] {
				continue
			}
			seen[c] = true
			add(module.Action{Verb: VerbLayOff, Target: mm.ID, Cards: []string{c}})
		}
	}
	orderLays(out)
	return out
}

// orderLays puts the lays a person would reach for first: the most value on
// the table for the fewest wilds, melds before lay-offs of the same value.
func orderLays(as []module.Action) {
	sort.SliceStable(as, func(i, j int) bool {
		wi, wj := containsWild(as[i].Cards), containsWild(as[j].Cards)
		if wi != wj {
			return !wi
		}
		vi, vj := handValue(as[i].Cards), handValue(as[j].Cards)
		if vi != vj {
			return vi > vj
		}
		return as[i].Verb == VerbLayMeld && as[j].Verb != VerbLayMeld
	})
}

// positionKey identifies a position for the search's purposes: the hand, what
// has been laid this turn, and the side's melds. Lay-offs made in a different
// order reach the same position, and the undo stacks that tell them apart are
// no part of whether it can be finished.
func positionKey(s *GameState, playerID string) string {
	var b strings.Builder
	b.WriteString(strings.Join(sortedCards(s.Hands[playerID]), ","))
	b.WriteString("|")
	b.WriteString(strconv.Itoa(s.LaidThisTurn))
	if t := s.team(playerID); t != nil {
		for _, m := range t.Melds {
			b.WriteString("|")
			b.WriteString(m.ID)
			b.WriteString(":")
			b.WriteString(strings.Join(sortedCards(m.Cards), ","))
		}
	}
	return b.String()
}
