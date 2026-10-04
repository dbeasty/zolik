package canasta

import (
	"sort"

	"zolik/server/internal/module"
)

// Whether a turn can still be finished.
//
// A turn ends with a discard, or with the side out. Every move before that
// changes what the hand holds, and a move that leaves no way to either end is
// a dead end: the seat on turn has nothing to do but take moves back, and with
// the stock empty, or a bot in the seat, not even that leads anywhere. The
// engine used to guard the moves that cause it one rule at a time —
// checkLeavesPlayable for the last two cards, reachableValue for an opening —
// and each rule was a bound that missed a case. A capture onto a side's own
// meld that left one card it could not shed; an opening that counted the two
// cards the side has to keep. So the question is now asked exactly, of the
// table the move leaves, before the move is accepted.

// finishSearchNodes bounds one question. The search walks the richest lays
// first and turnValueBound cuts off any branch that cannot open, so a turn that
// can be finished is found in a handful of positions; the bound is for a wide
// hand that cannot. Running out answers no, which refuses the move: a move
// refused costs a player a choice, a move accepted into a dead end costs the
// table.
const finishSearchNodes = 20000

// turnCanFinish reports that the player on turn, in the melding phase, has a
// sequence of lays after which the turn ends: a discard the engine accepts, or
// a lay that takes the side out.
//
// Read-only on s: every lay is tried on a copy.
func turnCanFinish(s *GameState, playerID string) bool {
	f := finishSearch{playerID: playerID, dead: map[string]bool{}}
	return f.from(s)
}

// checkTurnFinishes is turnCanFinish as a refusal, for the lays: the only side
// a lay can strand is one still opening, since an open side's lays are held to
// two cards by checkLeavesPlayable, and a stranded opening is the minimum out
// of reach.
func checkTurnFinishes(s *GameState, playerID string) error {
	if turnCanFinish(s, playerID) {
		return nil
	}
	if !s.team(playerID).HasMelded {
		return errCode(ErrInitialMeldNotMet)
	}
	return errCode(ErrMustKeepACard)
}

type finishSearch struct {
	playerID string
	nodes    int
	dead     map[string]bool
}

func (f *finishSearch) from(s *GameState) bool {
	if discardEndsTurn(s, f.playerID) {
		return true
	}
	if f.nodes >= finishSearchNodes {
		return false
	}
	f.nodes++
	t := s.team(f.playerID)
	if t == nil {
		return false
	}
	if !t.HasMelded && turnValueBound(s, t, f.playerID, true) < s.rules().meldFloor(t.Score) {
		return false
	}
	key := positionKey(s, f.playerID)
	if f.dead[key] {
		return false
	}
	for _, a := range turnLays(s, f.playerID, true) {
		c := s.layCopy(f.playerID)
		var rest []string
		var err error
		if a.Verb == VerbLayMeld {
			_, rest, err = layMeld(c, f.playerID, a)
		} else {
			_, rest, err = layOff(c, f.playerID, a)
		}
		if err != nil {
			continue
		}
		// Laid out: the lay functions only allow that to a side that may go
		// out, and the turn is over.
		if len(rest) == 0 || f.from(c) {
			return true
		}
	}
	f.dead[key] = true
	return false
}

// discardEndsTurn reports that applyDiscard would accept some card in this
// player's hand. The same checks, in the same order, without the move.
func discardEndsTurn(s *GameState, playerID string) bool {
	if s.Status != "active" || s.Break.Open || s.Current != playerID || s.Phase != phaseMeld {
		return false
	}
	t := s.team(playerID)
	if t == nil || (!t.HasMelded && s.LaidThisTurn > 0) {
		return false
	}
	hand := s.Hands[playerID]
	if len(hand) == 1 && !canGoOut(s, t) {
		return false
	}
	for _, c := range hand {
		if !isRedThree(c) {
			return true
		}
	}
	return false
}

// turnValueBound is the most this turn could have laid by the end of it: what
// is down already, plus every card in hand that may be spent, less the cheapest
// of them when the side cannot go out — it has to finish holding a discard and
// a card to keep (checkLeavesPlayable), and a wild that is not being spent is
// already one of the two. An overestimate by construction, which is what lets
// it prune: a branch below it cannot open whatever it lays.
func turnValueBound(s *GameState, t *Team, playerID string, wilds bool) int {
	var values []int
	kept := 0
	for _, c := range s.Hands[playerID] {
		if isRedThree(c) || (!wilds && isWild(c)) {
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

// turnLays is every meld and lay-off worth trying from a position, built the
// way the offer list builds them and left to the lay functions to accept or
// refuse.
func turnLays(s *GameState, playerID string, wilds bool) []module.Action {
	r := s.rules()
	t := s.team(playerID)
	hand := s.Hands[playerID]
	var out []module.Action
	add := func(a module.Action) {
		if wilds || !containsWild(a.Cards) {
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

// layCopy is a copy of the table deep enough for layMeld and layOff to be
// played on by this player: the hands map and this player's hand, and this
// player's side's melds. Everything else is shared and must not be written —
// the lay functions do not, short of the endDeal that turnCanFinish never
// reaches. The undo stacks start empty, since nothing here will undo.
func (s *GameState) layCopy(playerID string) *GameState {
	c := *s
	c.Hands = make(map[string][]string, len(s.Hands))
	for id, h := range s.Hands {
		c.Hands[id] = h
	}
	c.Hands[playerID] = append([]string(nil), s.Hands[playerID]...)
	c.Teams = append([]Team(nil), s.Teams...)
	if t := c.team(playerID); t != nil {
		t.Melds = append([]Meld(nil), t.Melds...)
		for j := range t.Melds {
			t.Melds[j].Cards = append([]string(nil), t.Melds[j].Cards...)
		}
	}
	c.LaidOff = nil
	c.MeldsLaid = nil
	return &c
}
