package canasta

import (
	"slices"
	"strconv"
	"strings"

	"zolik/server/internal/module"
)

// The CanastaX house rules (docs/wild-samba-plan.md): dirty sequences, the meld
// of 2s, rearranging a side's own melds, and poaching wilds off anybody's.
//
// Kept in one file because they share one idea the rest of the module never
// needed: a meld on the table is no longer only ever grown. A move shrinks one
// meld to grow another, and a poach swaps a card inside a meld, so each needs
// to know where a card sits in a sequence — which a wild, having no rank of its
// own, can only say through the run's stored start (Meld.Low).

// --- sequences with wilds in them ------------------------------------------------

// noLow is "no position asked for" to arrangeRun: spare wilds go on top, and
// below only once the top is full. lowFront and lowEnd ask for every spare
// wild below, or above, and are refused where that end has no room.
const (
	noLow    = -1
	lowFront = -2
	lowEnd   = -3
)

// span is the first and last position a sequence covers, wilds included.
func (m Meld) span() (low, high int) {
	low = m.runLow()
	return low, low + len(m.Cards) - 1
}

// runLow is where a sequence starts. A clean run says so with its lowest
// natural; a dirty one stores it, because its first card may be a wild.
func (m Meld) runLow() int {
	if m.Low != nil {
		return *m.Low
	}
	low, _ := runSpan(m.Cards)
	return low
}

// setRun stores a sequence's cards, already in position order, and where they
// start. Low is kept only while there is a wild for it to place.
func (m *Meld) setRun(cards []string, low int) {
	m.Cards = cards
	m.Low = nil
	if containsWild(cards) {
		l := low
		m.Low = &l
	}
}

// standsFor is the natural card a position in this sequence stands for.
func (m Meld) standsFor(i int) string {
	return runRanks[m.runLow()+i] + m.Suit
}

// arrangeRun places cards into a sequence and checks the result.
//
// fixed is a run already on the table, in position order from fixedLow; its
// cards keep their positions, wilds included. add are cards joining it: a
// natural goes to its own position, wilds fill the gaps between, and any wild
// left over extends the run — upward first, because a sequence is worth more at
// the top, unless wantLow says where the run should start instead.
//
// Every sequence in the module is built here, clean or dirty, so a lay, a
// lay-off, a move and the top card taken onto a run cannot disagree about what
// a run is.
func arrangeRun(r ruleset, fixed []string, fixedLow int, add []string, wantLow int) ([]string, int, error) {
	slots := map[int]string{}
	suit := ""
	setSuit := func(c string) error {
		if suit == "" {
			suit = suitOf(c)
		} else if suitOf(c) != suit {
			return errCode(ErrSequenceNeedsOneSuit)
		}
		return nil
	}
	for i, c := range fixed {
		pos := fixedLow + i
		if !isWild(c) {
			if j, ok := runIndexOf(c); !ok || j != pos {
				return nil, 0, errCode(ErrRunNotConsecutive)
			}
			if err := setSuit(c); err != nil {
				return nil, 0, err
			}
		}
		slots[pos] = c
	}
	var wilds []string
	for _, c := range add {
		if isWild(c) {
			if !r.DirtySequences {
				return nil, 0, errCode(ErrSequenceNoWilds)
			}
			wilds = append(wilds, c)
			continue
		}
		if rankOf(c) == rankThree {
			return nil, 0, errCode(ErrCannotMeldThree)
		}
		i, ok := runIndexOf(c)
		if !ok {
			return nil, 0, errCode(ErrRunNotConsecutive)
		}
		if err := setSuit(c); err != nil {
			return nil, 0, err
		}
		// Equal catches the duplicate three decks make easy to hold: two nines
		// of hearts are two cards, not two steps.
		if _, taken := slots[i]; taken {
			return nil, 0, errCode(ErrRunNotConsecutive)
		}
		slots[i] = c
	}
	if suit == "" {
		return nil, 0, errCode(ErrNotEnoughNaturals)
	}

	lo, hi := len(runRanks), -1
	for i := range slots {
		lo, hi = min(lo, i), max(hi, i)
	}
	for i := lo; i <= hi; i++ {
		if _, ok := slots[i]; ok {
			continue
		}
		if len(wilds) == 0 {
			return nil, 0, errCode(ErrRunNotConsecutive)
		}
		slots[i], wilds = wilds[0], wilds[1:]
	}
	extra := len(wilds)
	top := len(runRanks) - 1
	below := 0
	switch {
	case wantLow == lowFront:
		below = extra
	case wantLow == lowEnd:
		below = 0
	case wantLow != noLow:
		below = lo - wantLow
		if below < 0 || below > extra {
			return nil, 0, errCode(ErrRunNotConsecutive)
		}
	default:
		if above := min(extra, top-hi); above < extra {
			below = extra - above
		}
	}
	above := extra - below
	if lo-below < 0 || hi+above > top {
		return nil, 0, errCode(ErrRunNotConsecutive)
	}
	for i := 1; i <= above; i++ {
		slots[hi+i], wilds = wilds[0], wilds[1:]
	}
	for i := 1; i <= below; i++ {
		slots[lo-i], wilds = wilds[0], wilds[1:]
	}
	lo -= below

	out := make([]string, 0, len(slots))
	for i := lo; i < lo+len(slots); i++ {
		out = append(out, slots[i])
	}
	if err := checkRunShape(r, out); err != nil {
		return nil, 0, err
	}
	return out, lo, nil
}

// checkRunShape is the size and wild limits on a sequence whose positions are
// already settled. The wild limits are a group's: at most MaxWilds, and the
// naturals-per-wild ratio where the variation has one.
func checkRunShape(r ruleset, cards []string) error {
	if len(cards) < minMeldSize {
		return errCode(ErrMeldTooSmall)
	}
	// Seven is a samba and a samba is complete, so an eighth card is not a
	// longer sequence — it is not a sequence.
	if len(cards) > canastaSize {
		return errCode(ErrMeldTooLarge)
	}
	wilds := 0
	for _, c := range cards {
		if isWild(c) {
			wilds++
		}
	}
	if wilds == 0 {
		return nil
	}
	return checkWildLimits(r, len(cards)-wilds, wilds)
}

// checkWildLimits is the one statement of how many wilds a meld may carry,
// shared by groups, dirty sequences and the meld of 2s.
func checkWildLimits(r ruleset, naturals, wilds int) error {
	if wilds > r.MaxWilds {
		return errCode(ErrTooManyWilds)
	}
	if naturals < r.MinNaturals {
		return errCode(ErrNotEnoughNaturals)
	}
	if r.NaturalsPerWild > 0 && naturals < wilds*r.NaturalsPerWild {
		return errCode(ErrNotEnoughNaturals)
	}
	return nil
}

// --- the meld of 2s --------------------------------------------------------------

// isWildMeld is whether a list of cards asks to be the meld of 2s: wild cards
// only, and at least one of them a 2.
func isWildMeld(r ruleset, cards []string) bool {
	if !r.WildMeld || len(cards) == 0 {
		return false
	}
	twos := 0
	for _, c := range cards {
		if !isWild(c) {
			return false
		}
		if rankOf(c) == rankTwo {
			twos++
		}
	}
	return twos > 0
}

// validateWildMeld is the meld of 2s: the 2s are its naturals and the jokers
// its wilds, under the variation's group limits. It closes at seven where a
// group does.
func validateWildMeld(r ruleset, cards []string) error {
	if len(cards) < minMeldSize {
		return errCode(ErrMeldTooSmall)
	}
	if r.GroupCanastaCloses && len(cards) > canastaSize {
		return errCode(ErrMeldTooLarge)
	}
	twos := 0
	for _, c := range cards {
		switch {
		case rankOf(c) == rankTwo:
			twos++
		case rankOf(c) != rankJoker:
			return errCode(ErrMeldMixedRanks)
		}
	}
	return checkWildLimits(r, twos, len(cards)-twos)
}

// wildMeld is the side's meld of 2s, if it has one.
func (t *Team) wildMeld() *Meld {
	for i := range t.Melds {
		if t.Melds[i].Kind == meldWild {
			return &t.Melds[i]
		}
	}
	return nil
}

// --- growing and shrinking a meld ---------------------------------------------

// grownMeld is m with cards added, checked, without touching m: the one
// implementation of "these cards go onto that meld", for a lay-off, a move, and
// the top card taken onto a sequence.
func grownMeld(r ruleset, m Meld, cards []string, wantLow int) (Meld, error) {
	if m.closed(r) {
		return m, errCode(ErrMeldClosed)
	}
	switch m.kind() {
	case meldRun:
		out, low, err := arrangeRun(r, m.Cards, m.runLow(), cards, wantLow)
		if err != nil {
			return m, err
		}
		m.setRun(out, low)
		return m, nil
	case meldWild:
		for _, c := range cards {
			if !isWild(c) {
				return m, errCode(ErrWrongRank)
			}
		}
		grown := append(append([]string(nil), m.Cards...), cards...)
		if err := validateWildMeld(r, grown); err != nil {
			return m, err
		}
		m.Cards = grown
		return m, nil
	}
	for _, c := range cards {
		if !isWild(c) && rankOf(c) != m.Rank {
			return m, errCode(ErrWrongRank)
		}
	}
	grown := append(append([]string(nil), m.Cards...), cards...)
	if err := validateGroup(r, grown); err != nil {
		return m, err
	}
	m.Cards = grown
	return m, nil
}

// shrunkMeld is m with cards taken out of it, checked, without touching m.
// Empty is allowed — the meld is gone — and reported by the second return.
//
// A sequence gives up cards only from its ends: a card out of the middle leaves
// two runs where there was one, and neither is the meld it was.
func shrunkMeld(r ruleset, m Meld, cards []string) (Meld, bool, error) {
	if !hasCards(m.Cards, cards) {
		return m, false, errCode(ErrCardNotInHand)
	}
	if len(cards) == len(m.Cards) {
		return m, true, nil
	}
	if m.kind() != meldRun {
		rest, _ := removeCards(m.Cards, cards)
		var err error
		if m.kind() == meldWild {
			err = validateWildMeld(r, rest)
		} else {
			err = validateGroup(r, rest)
		}
		if err != nil {
			return m, false, err
		}
		m.Cards = rest
		return m, false, nil
	}

	slots := append([]string(nil), m.Cards...)
	low := m.runLow()
	want := append([]string(nil), cards...)
	for len(want) > 0 {
		if i := slices.Index(want, slots[0]); i >= 0 {
			want = slices.Delete(want, i, i+1)
			slots = slots[1:]
			low++
			continue
		}
		if i := slices.Index(want, slots[len(slots)-1]); i >= 0 {
			want = slices.Delete(want, i, i+1)
			slots = slots[:len(slots)-1]
			continue
		}
		return m, false, errCode(ErrRunGap)
	}
	if err := checkRunShape(r, slots); err != nil {
		return m, false, err
	}
	// A run whose naturals are all gone is no sequence at all — it has no suit
	// left to be one in.
	if !slices.ContainsFunc(slots, func(c string) bool { return !isWild(c) }) {
		return m, false, errCode(ErrNotEnoughNaturals)
	}
	m.setRun(slots, low)
	return m, false, nil
}

// wantLowOf reads the position a player asked a sequence to start at — a rank,
// "4" to "A" — or noLow when they did not say.
func wantLowOf(a module.Action) int {
	if a.Params == nil {
		return noLow
	}
	// The generic drop hint: which end of the run the card was let go on.
	switch a.Params[module.PositionParam] {
	case positionFront:
		return lowFront
	case positionEnd:
		return lowEnd
	}
	if a.Params["low"] == "" {
		return noLow
	}
	if i, ok := runRankIndex[a.Params["low"]]; ok {
		return i
	}
	if i, err := strconv.Atoi(a.Params["low"]); err == nil {
		return i
	}
	return noLow
}

// The two ends of a sequence, as a drop names them (module.PositionParam):
// "front" is below the lowest card, "end" above the highest — the words the
// rummy engine already sends, so a client draws both games' runs the same way.
const (
	positionFront = "front"
	positionEnd   = "end"
)

// runEndPlacements says, for each wild in cards that could join the run at
// either end, both ends and where each lands on screen; the rest go as they
// are. Nil when no wild has a choice, which leaves an offer as it was. try
// is the engine's own answer for one card at one end.
func runEndPlacements(r ruleset, target Meld, cards []string, try func(card, position string) bool) []module.Placement {
	if !r.DirtySequences || target.kind() != meldRun {
		return nil
	}
	var out []module.Placement
	choice := false
	asked := map[string]bool{}
	for _, c := range cards {
		p := module.Placement{Card: c}
		if isWild(c) {
			both, seen := asked[c]
			if !seen {
				both = try(c, positionFront) && try(c, positionEnd)
				asked[c] = both
			}
			if both {
				p.Positions = []string{positionFront, positionEnd}
				p.Slots = []int{0, len(target.Cards)}
				choice = true
			}
		}
		out = append(out, p)
	}
	if !choice {
		return nil
	}
	return out
}

// --- reshapes: their bookkeeping ------------------------------------------------

// nextSeq numbers the turn's next undoable entry: one past the newest still
// standing. Derived rather than counted, so unwinding a turn leaves no trace
// of it in the state.
func (s *GameState) nextSeq() int { return s.latestSeq() + 1 }

// latestSeq is the newest entry on any of the turn's undo stacks.
func (s *GameState) latestSeq() int {
	n := 0
	for _, lo := range s.LaidOff {
		n = max(n, lo.Seq)
	}
	for _, ml := range s.MeldsLaid {
		n = max(n, ml.Seq)
	}
	for _, rs := range s.Reshapes {
		n = max(n, rs.Seq)
	}
	return n
}

// laterReshape reports a reshape newer than seq still standing: a lay under it
// cannot come back off first, because the reshape snapshotted the meld the lay
// built.
func (s *GameState) laterReshape(seq int) bool {
	for _, rs := range s.Reshapes {
		if rs.Seq > seq {
			return true
		}
	}
	return false
}

func cloneMeld(m Meld) Meld {
	m.Cards = append([]string(nil), m.Cards...)
	if m.Low != nil {
		l := *m.Low
		m.Low = &l
	}
	return m
}

func sameMeld(a, b Meld) bool {
	if a.ID != b.ID || !slices.Equal(a.Cards, b.Cards) {
		return false
	}
	if (a.Low == nil) != (b.Low == nil) {
		return false
	}
	return a.Low == nil || *a.Low == *b.Low
}

// --- moving cards between a side's own melds ------------------------------------

// moveSource is the meld a move takes its cards from: Params["from"] where the
// caller said, else the offer it pressed ("move:<from>:<to>"), else the one
// meld of the side's holding every card named — so a driver that only echoes
// an offer's id and its cards is understood.
func moveSource(t *Team, a module.Action) string {
	if a.Params != nil && a.Params["from"] != "" {
		return a.Params["from"]
	}
	if rest, ok := strings.CutPrefix(a.OfferID, "move:"); ok {
		if from, _, ok := strings.Cut(rest, ":"); ok {
			return from
		}
	}
	found := ""
	for _, m := range t.Melds {
		if m.ID != a.Target && hasCards(m.Cards, a.Cards) {
			if found != "" {
				return ""
			}
			found = m.ID
		}
	}
	return found
}

func applyMoveCards(s *GameState, playerID string, a module.Action) ([]module.Event, error) {
	r := s.rules()
	if !r.Rearrange {
		return nil, module.Error{Code: ErrUnknownAction, Message: a.Verb}
	}
	if s.Phase != phaseMeld {
		return nil, errCode(ErrWrongPhase)
	}
	t := s.team(playerID)
	if !t.HasMelded {
		return nil, errCode(ErrMustMeldFirst)
	}
	if len(a.Cards) == 0 {
		return nil, errCode(ErrNothingFitsHere)
	}
	fromID := moveSource(t, a)
	if fromID == a.Target {
		return nil, errCode(ErrSameMeld)
	}
	fromOwner, from := s.findMeld(fromID)
	toOwner, to := s.findMeld(a.Target)
	if from == nil || to == nil {
		return nil, errCode(ErrNoSuchMeld)
	}
	if fromOwner.ID != t.ID || toOwner.ID != t.ID {
		return nil, errCode(ErrNotYourMeld)
	}

	shrunk, gone, err := shrunkMeld(r, *from, a.Cards)
	if err != nil {
		return nil, err
	}
	grown, err := grownMeld(r, *to, a.Cards, wantLowOf(a))
	if err != nil {
		return nil, err
	}

	rs := Reshape{
		Kind:   reshapeMove,
		Before: []Meld{cloneMeld(*from), cloneMeld(*to)},
		Cards:  append([]string(nil), a.Cards...),
	}
	fromIdx := slices.IndexFunc(t.Melds, func(m Meld) bool { return m.ID == fromID })
	toIdx := slices.IndexFunc(t.Melds, func(m Meld) bool { return m.ID == a.Target })
	rs.BeforeIndex = []int{fromIdx, toIdx}

	t.Melds[toIdx] = grown
	if gone {
		t.Melds = slices.Delete(t.Melds, fromIdx, fromIdx+1)
		rs.After = []Meld{{ID: fromID}, cloneMeld(grown)}
	} else {
		t.Melds[fromIdx] = shrunk
		rs.After = []Meld{cloneMeld(shrunk), cloneMeld(grown)}
	}
	// A move can take apart the canasta this side needed to go out with one
	// card in hand: asked exactly, as a lay is.
	if err := checkTurnFinishes(s, playerID); err != nil {
		return nil, err
	}
	rs.Seq = s.nextSeq()
	s.Reshapes = append(s.Reshapes, rs)

	return []module.Event{{Type: "cards_moved", Data: map[string]any{
		"playerId": playerID, "from": fromID, "meldId": a.Target, "cards": a.Cards,
	}}}, nil
}

// --- poaching a wild --------------------------------------------------------------

// poachSlot is which card in m the natural c would replace, or -1. Params
// "wild" picks among several wilds in a group; otherwise the most valuable is
// taken.
func poachSlot(m Meld, c, want string) int {
	if isWild(c) && !(m.Kind == meldWild && rankOf(c) == rankTwo) {
		return -1
	}
	switch m.kind() {
	case meldRun:
		i, ok := runIndexOf(c)
		if !ok || suitOf(c) != m.Suit {
			return -1
		}
		low, high := m.span()
		if i < low || i > high || !isWild(m.Cards[i-low]) {
			return -1
		}
		return i - low
	case meldWild:
		if rankOf(c) != rankTwo {
			return -1
		}
		// A 2 is the natural card of this meld; only a joker can be bought.
		for i, w := range m.Cards {
			if rankOf(w) == rankJoker && (want == "" || w == want) {
				return i
			}
		}
		return -1
	}
	if rankOf(c) != m.Rank {
		return -1
	}
	best := -1
	for i, w := range m.Cards {
		if !isWild(w) || (want != "" && w != want) {
			continue
		}
		if best < 0 || cardValue(w) > cardValue(m.Cards[best]) {
			best = i
		}
	}
	return best
}

func applyPoach(s *GameState, playerID string, a module.Action) ([]module.Event, error) {
	r := s.rules()
	if !r.Poach {
		return nil, module.Error{Code: ErrUnknownAction, Message: a.Verb}
	}
	if s.Phase != phaseMeld {
		return nil, errCode(ErrWrongPhase)
	}
	t := s.team(playerID)
	if !t.HasMelded {
		return nil, errCode(ErrMustMeldFirst)
	}
	if len(a.Cards) != 1 {
		return nil, errCode(ErrNothingToPoach)
	}
	natural := a.Cards[0]
	if !hasCards(s.Hands[playerID], a.Cards) {
		return nil, errCode(ErrCardNotInHand)
	}
	owner, m := s.findMeld(a.Target)
	if m == nil {
		return nil, errCode(ErrNoSuchMeld)
	}
	want := ""
	if a.Params != nil {
		want = a.Params["wild"]
	}
	slot := poachSlot(*m, natural, want)
	if slot < 0 {
		return nil, errCode(ErrNothingToPoach)
	}

	before := cloneMeld(*m)
	idx := slices.IndexFunc(owner.Melds, func(x Meld) bool { return x.ID == m.ID })
	wild := m.Cards[slot]
	m.Cards = append([]string(nil), m.Cards...)
	m.Cards[slot] = natural
	if m.kind() == meldRun {
		m.setRun(m.Cards, before.runLow())
	}
	priorHand := append([]string(nil), s.Hands[playerID]...)
	hand, _ := removeCards(s.Hands[playerID], a.Cards)
	s.Hands[playerID] = append(hand, wild)

	if err := checkTurnFinishes(s, playerID); err != nil {
		return nil, err
	}
	s.Reshapes = append(s.Reshapes, Reshape{
		Kind: reshapePoach, Seq: s.nextSeq(),
		Before: []Meld{before}, After: []Meld{cloneMeld(*m)}, BeforeIndex: []int{idx},
		Cards: []string{natural}, Wild: wild, PriorHand: priorHand,
	})

	return []module.Event{{Type: "wild_poached", Data: map[string]any{
		"playerId": playerID, "meldId": m.ID, "teamId": owner.ID,
		"card": natural, "wild": wild,
	}}}, nil
}

// --- taking a reshape back -----------------------------------------------------

func applyUndoReshape(s *GameState, playerID string) ([]module.Event, error) {
	if len(s.Reshapes) == 0 {
		return nil, errCode(ErrNothingToUndo)
	}
	rs := s.Reshapes[len(s.Reshapes)-1]
	if s.latestSeq() != rs.Seq {
		return nil, errCode(ErrUndoLatestFirst)
	}
	// Every meld it touched must still be exactly what it left, or putting the
	// snapshot back would throw away a card somebody laid since.
	for i, after := range rs.After {
		owner, m := s.findMeld(after.ID)
		if after.Cards == nil {
			if m != nil {
				return nil, errCode(ErrNothingToUndo)
			}
			continue
		}
		if m == nil || !sameMeld(*m, after) || owner.ID != rs.Before[i].TeamID {
			return nil, errCode(ErrNothingToUndo)
		}
	}
	if rs.Kind == reshapePoach && !hasCards(s.Hands[playerID], []string{rs.Wild}) {
		return nil, errCode(ErrNothingToUndo)
	}

	for i, before := range rs.Before {
		owner := &s.Teams[0]
		for j := range s.Teams {
			if s.Teams[j].ID == before.TeamID {
				owner = &s.Teams[j]
			}
		}
		if m := owner.meldByID(before.ID); m != nil {
			*m = cloneMeld(before)
			continue
		}
		at := min(max(rs.BeforeIndex[i], 0), len(owner.Melds))
		owner.Melds = slices.Insert(owner.Melds, at, cloneMeld(before))
	}
	if rs.Kind == reshapePoach {
		s.Hands[playerID] = append([]string(nil), rs.PriorHand...)
	}
	s.Reshapes = s.Reshapes[:len(s.Reshapes)-1]

	return []module.Event{{Type: "reshape_undone", Data: map[string]any{
		"playerId": playerID, "kind": rs.Kind, "cards": rs.Cards,
	}}}, nil
}

// --- candidates: what a hand could lay under the house rules --------------------

// dirtyRunCandidates are the sequences a hand could lay with wilds in them: per
// suit, each stretch from one natural to another that the hand's wilds can
// bridge within the limits, as long as it will go — plus two neighbours made a
// sequence by one wild. Clean stretches are runCandidates' and are left to it.
//
// Still concrete, as Samba's offers have always been: the wilds only fill
// gaps between naturals the hand holds, so there are a handful per suit, not
// the explosion of every place a joker could stand.
func dirtyRunCandidates(r ruleset, hand []string, t *Team) []candidate {
	if !r.DirtySequences {
		return nil
	}
	wilds := handWilds(hand)
	if len(wilds) == 0 {
		return nil
	}
	bySuit := map[string]map[int]string{}
	for _, c := range hand {
		i, ok := runIndexOf(c)
		if !ok {
			continue
		}
		s := suitOf(c)
		if bySuit[s] == nil {
			bySuit[s] = map[int]string{}
		}
		if _, taken := bySuit[s][i]; !taken {
			bySuit[s][i] = c
		}
	}
	var out []candidate
	seen := map[string]bool{}
	for _, s := range sortedKeys(bySuit) {
		byIndex := bySuit[s]
		var positions []int
		for i := range byIndex {
			positions = append(positions, i)
		}
		slices.Sort(positions)
		for a := range positions {
			var best []string
			for b := a + 1; b < len(positions); b++ {
				lo, hi := positions[a], positions[b]
				length := hi - lo + 1
				if length > canastaSize {
					break
				}
				var naturals []string
				for _, p := range positions[a : b+1] {
					naturals = append(naturals, byIndex[p])
				}
				need := length - len(naturals)
				if length < minMeldSize {
					need += minMeldSize - length
				}
				if need == 0 || need > len(wilds) {
					continue
				}
				cards := append(append([]string(nil), naturals...), wilds[:need]...)
				if _, _, err := arrangeRun(r, nil, 0, cards, noLow); err != nil {
					continue
				}
				best = cards
			}
			if best == nil {
				continue
			}
			arranged, low, _ := arrangeRun(r, nil, 0, best, noLow)
			key := s + runRanks[low] + strconv.Itoa(len(arranged))
			if seen[key] || runOverlapsTable(t, s, best) {
				continue
			}
			seen[key] = true
			// No pool wider than the submission: a pool holding every wild in
			// the hand made a selection of wilds alone look like the start of
			// this run, and three 2s picked for the meld of 2s became a
			// question about which meld they were for.
			out = append(out, candidate{
				Kind: meldRun, Suit: s, Dirty: true,
				Cards: arranged, Value: handValue(best),
			})
		}
	}
	sortCandidates(out)
	return out
}

// wildMeldCandidate is the meld of 2s a hand could lay, or nil: every 2 it
// holds (to seven where a group closes there), padded with jokers only if the
// 2s alone are too few.
func wildMeldCandidate(r ruleset, hand []string, t *Team) *candidate {
	if !r.WildMeld || t == nil || !t.HasMelded || t.wildMeld() != nil {
		return nil
	}
	var twos, jokers []string
	for _, c := range hand {
		switch rankOf(c) {
		case rankTwo:
			twos = append(twos, c)
		case rankJoker:
			jokers = append(jokers, c)
		}
	}
	twos, jokers = sortedCards(twos), sortedCards(jokers)
	if len(twos) == 0 {
		return nil
	}
	cards := append([]string(nil), twos...)
	if r.GroupCanastaCloses && len(cards) > canastaSize {
		cards = cards[:canastaSize]
	}
	for i := 0; len(cards) < minMeldSize && i < len(jokers); i++ {
		cards = append(cards, jokers[i])
	}
	if validateWildMeld(r, cards) != nil {
		return nil
	}
	return &candidate{
		Kind: meldWild, Rank: rankTwo, Cards: cards,
		Pool: sortedCards(append(append([]string(nil), twos...), jokers...)), Value: handValue(cards),
	}
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

// --- offers -------------------------------------------------------------------------

// houseOffers are the CanastaX controls for the player on turn: a move per
// pair of the side's own melds that some card could travel between, a poach
// per meld on the table with a wild the hand can buy, and the undo for the
// last of either.
//
// Each card listed is one the engine accepted on its own; the pure checks
// (shrunkMeld, grownMeld, poachSlot) go first so that only a card with a
// chance is put to Apply.
func houseOffers(m *Module, raw module.State, s *GameState, playerID string) []module.ActionOffer {
	r := s.rules()
	t := s.team(playerID)
	if t == nil || s.Phase != phaseMeld || !t.HasMelded {
		return nil
	}
	var out []module.ActionOffer

	if r.Rearrange {
		for _, from := range t.Melds {
			for _, to := range t.Melds {
				if from.ID == to.ID {
					continue
				}
				var movable []string
				asked := map[string]bool{}
				for _, c := range from.Cards {
					if asked[c] {
						continue
					}
					asked[c] = true
					if _, _, err := shrunkMeld(r, from, []string{c}); err != nil {
						continue
					}
					if _, err := grownMeld(r, to, []string{c}, noLow); err != nil {
						continue
					}
					if ok, _ := probe(m, raw, playerID, module.Action{
						Verb: VerbMoveCards, Cards: []string{c}, Target: to.ID,
						Params: map[string]string{"from": from.ID},
					}); ok {
						movable = append(movable, c)
					}
				}
				if len(movable) == 0 {
					continue
				}
				placements := runEndPlacements(r, to, sortedCards(movable), func(c, pos string) bool {
					ok, _ := probe(m, raw, playerID, module.Action{
						Verb: VerbMoveCards, Cards: []string{c}, Target: to.ID,
						Params: map[string]string{"from": from.ID, module.PositionParam: pos},
					})
					return ok
				})
				out = append(out, module.ActionOffer{
					ID: "move:" + from.ID + ":" + to.ID, Verb: VerbMoveCards, Enabled: true,
					LabelKey: "verb.moveCards",
					Facts:    []module.Fact{meldOfferFact(from), meldOfferFact(to)},
					Source: &module.Selector{
						Zone: module.FromMeld, MeldID: from.ID, ZoneID: meldsZoneID(t.ID),
						Cards: sortedCards(movable), Placements: placements, MinCards: 1, MaxCards: len(from.Cards),
					},
					Target: &module.Selector{Zone: module.ToMeld, MeldID: to.ID, ZoneID: meldsZoneID(t.ID)},
				})
			}
		}
	}

	if r.Poach {
		hand := s.Hands[playerID]
		for _, owner := range s.Teams {
			for _, mm := range owner.Melds {
				if mm.wilds() == 0 {
					continue
				}
				var naturals []string
				asked := map[string]bool{}
				for _, c := range hand {
					if asked[c] || poachSlot(mm, c, "") < 0 {
						continue
					}
					asked[c] = true
					if ok, _ := probe(m, raw, playerID, module.Action{
						Verb: VerbPoach, Cards: []string{c}, Target: mm.ID,
					}); ok {
						naturals = append(naturals, c)
					}
				}
				if len(naturals) == 0 {
					continue
				}
				out = append(out, module.ActionOffer{
					ID: "poach:" + mm.ID, Verb: VerbPoach, Enabled: true,
					LabelKey: "verb.poach",
					Facts:    []module.Fact{meldOfferFact(mm)},
					Source: &module.Selector{
						Zone: module.FromHand, OwnerID: playerID, ZoneID: handZoneID(playerID),
						Cards: sortedCards(naturals), MinCards: 1, MaxCards: 1,
					},
					Target: &module.Selector{Zone: module.ToMeld, MeldID: mm.ID, ZoneID: meldsZoneID(owner.ID)},
				})
			}
		}
	}

	if n := len(s.Reshapes); n > 0 {
		last := s.Reshapes[n-1]
		key := "verb.undoMove"
		if last.Kind == reshapePoach {
			key = "verb.undoPoach"
		}
		o := module.ActionOffer{ID: OfferUndoReshape, Verb: VerbUndoReshape, LabelKey: key, Undo: true}
		o.Enabled, o.WhyNot = probe(m, raw, playerID, module.Action{Verb: VerbUndoReshape})
		out = append(out, o)
	}
	return out
}

// OfferUndoReshape takes back the turn's last move or poach.
const OfferUndoReshape = "undo_reshape"
