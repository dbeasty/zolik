package canasta

import "sort"

// The shape of a legal meld that no variation argues about. The wild limits used
// to live here too and now come from the ruleset, because Samba counts them
// differently (docs/samba-plan.md §2).
const (
	minMeldSize = 3
	canastaSize = 7
)

// meldRank is the rank a group of cards melds as: the single natural rank
// present. Reported separately from validity because "which rank is this"
// and "is this legal" are different questions and callers need both.
func meldRank(cards []string) (string, bool) {
	rank := ""
	for _, c := range cards {
		if isWild(c) {
			continue
		}
		r := rankOf(c)
		if rank == "" {
			rank = r
			continue
		}
		if r != rank {
			return "", false
		}
	}
	return rank, rank != ""
}

// validateMeld is the one implementation of "is this a legal set of cards".
//
// Called by every path that puts cards on the table — a new meld, a lay-off's
// resulting meld, the meld a pile capture creates — so none of them can hold a
// different opinion about wild limits.
// It dispatches on what the cards *are* rather than on a flag the caller passes,
// because a submission is a list of cards and nothing else: one natural rank is
// a group, and where the variation has sequences, anything else is tried as one.
// Where it does not, a mixed list is a group with mixed ranks, which is exactly
// the refusal Canasta gave before sequences existed.
func validateMeld(r ruleset, cards []string) error {
	if meldKindOf(r, cards) == meldRun {
		return validateRun(r, cards)
	}
	return validateGroup(r, cards)
}

// meldKindOf is what a list of cards is asking to be.
func meldKindOf(r ruleset, cards []string) string {
	if !r.Sequences {
		return meldSet
	}
	if _, ok := meldRank(cards); ok {
		return meldSet
	}
	return meldRun
}

// validateRun is the rules for a sequence: three to seven natural cards of one
// suit, consecutive, ace high.
//
// No wilds, ever — which is not an arbitrary restriction but the reason Samba's
// runs can be offered as concrete cards at all (docs/samba-plan.md §3.4). A
// sequence with a joker in it would be a shape a player composes; without one it
// is a fact about the hand.
func validateRun(r ruleset, cards []string) error {
	if len(cards) < minMeldSize {
		return errCode(ErrMeldTooSmall)
	}
	// Seven is a samba and a samba is complete, so an eighth card is not a
	// longer sequence — it is not a sequence.
	if len(cards) > canastaSize {
		return errCode(ErrMeldTooLarge)
	}

	suit := ""
	idx := make([]int, 0, len(cards))
	for _, c := range cards {
		if isWild(c) {
			return errCode(ErrSequenceNoWilds)
		}
		if rankOf(c) == rankThree {
			return errCode(ErrCannotMeldThree)
		}
		i, ok := runIndexOf(c)
		if !ok {
			return errCode(ErrRunNotConsecutive)
		}
		if suit == "" {
			suit = suitOf(c)
		} else if suitOf(c) != suit {
			return errCode(ErrSequenceNeedsOneSuit)
		}
		idx = append(idx, i)
	}

	sort.Ints(idx)
	for i := 1; i < len(idx); i++ {
		// Equal catches the duplicate that three decks make easy to hold: two
		// nines of hearts are two cards, not two steps.
		if idx[i] != idx[i-1]+1 {
			return errCode(ErrRunNotConsecutive)
		}
	}
	return nil
}

// validateGroup is the rules for n cards of one rank.
func validateGroup(r ruleset, cards []string) error {
	if len(cards) < minMeldSize {
		return errCode(ErrMeldTooSmall)
	}
	// A group stops at seven only where a canasta closes. Where it does not —
	// Samba — the wild limits below are what bound it.
	if r.GroupCanastaCloses && len(cards) > canastaSize {
		return errCode(ErrMeldTooLarge)
	}
	rank, ok := meldRank(cards)
	if !ok {
		return errCode(ErrMeldMixedRanks)
	}
	// Threes are never an ordinary meld. Red ones never reach the table from
	// a hand at all, and black ones only leave on the way out — which is a
	// going-out rule, handled where going out is decided.
	if rank == rankThree {
		return errCode(ErrCannotMeldThree)
	}
	wilds := 0
	for _, c := range cards {
		if isWild(c) {
			wilds++
		}
	}
	naturals := len(cards) - wilds
	if wilds > r.MaxWilds {
		return errCode(ErrTooManyWilds)
	}
	if naturals < r.MinNaturals {
		return errCode(ErrNotEnoughNaturals)
	}
	// Samba's ratio: twice as many naturals as wilds, which makes a two-wild
	// group need four naturals rather than the two an absolute floor asks for.
	if r.NaturalsPerWild > 0 && naturals < wilds*r.NaturalsPerWild {
		return errCode(ErrNotEnoughNaturals)
	}
	return nil
}

// validateBlackThreeMeld is the one exception: on the way out a player may put
// down three or four black threes, and never with a wild among them.
func validateBlackThreeMeld(cards []string) error {
	if len(cards) < minMeldSize || len(cards) > 4 {
		return errCode(ErrMeldTooSmall)
	}
	for _, c := range cards {
		if !isBlackThree(c) {
			return errCode(ErrMeldMixedRanks)
		}
	}
	return nil
}

// candidate is one concrete meld a player could lay right now.
//
// Concrete, not a shape. Canasta melds are "n of one rank", so the whole
// candidate set is at most one per rank — unlike a rummy run, whose shapes
// explode (see extensibility-plan.md §1.1's offer-explosion note). That is why
// this module can ship exact card lists in its offers, and why a driver that
// reads nothing but offers can play it to the end.
type candidate struct {
	// Kind is "set" or "run"; empty means set, so the group candidates below
	// did not have to grow a field to say what they always were.
	Kind  string
	Rank  string
	Suit  string
	Cards []string
	Value int
}

// offerKey is what tells one candidate from another on screen: a rank for a
// group, and a suit and low card for a sequence.
func (c candidate) offerKey() string {
	if c.Kind == meldRun {
		return "run:" + c.Suit + c.Cards[0][:len(c.Cards[0])-1]
	}
	return c.Rank
}

// meldableNaturals lists, for each rank the hand could still open a new meld
// on, that rank's natural (non-wild) cards — the one fact both meld-candidate
// functions below need before they diverge on how to spend the hand's wilds.
func meldableNaturals(r ruleset, hand []string, t *Team) map[string][]string {
	byRank := countByRank(hand)
	out := map[string][]string{}
	for rank, cards := range byRank {
		if rank == rankThree || isWild(cards[0]) {
			continue
		}
		if t != nil && t.rankIsFull(r, rank) {
			continue // already melded: that is a lay-off, not a new meld
		}
		naturals := make([]string, 0, len(cards))
		for _, c := range cards {
			if !isWild(c) {
				naturals = append(naturals, c)
			}
		}
		if len(naturals) < r.MinNaturals {
			continue
		}
		out[rank] = naturals
	}
	return out
}

func handWilds(hand []string) []string {
	var wilds []string
	for _, c := range hand {
		if isWild(c) {
			wilds = append(wilds, c)
		}
	}
	sort.Strings(wilds)
	return wilds
}

func sortCandidates(out []candidate) {
	sort.Slice(out, func(i, j int) bool {
		if out[i].Value != out[j].Value {
			return out[i].Value > out[j].Value
		}
		// offerKey rather than Rank: a sequence has no rank, so two of them
		// would tie here and sort unstably against each other.
		return out[i].offerKey() < out[j].offerKey()
	})
}

// newMeldCandidates enumerates the new melds a hand could lay, best first, as
// a *turn's* worth of melding: every rank draws from one shared pool of
// wilds, spent greedily on the most valuable rank that needs one. That is
// what makes the result achievable rather than merely plausible, which is
// exactly the property reachableValue needs — the one caller of this
// function — to stay a safe lower bound. It is the wrong function for asking
// "what single meld could I lay right now": see allMeldCandidates for that.
func newMeldCandidates(r ruleset, hand []string, t *Team) []candidate {
	naturalsByRank := meldableNaturals(r, hand, t)
	wilds := handWilds(hand)

	// Most valuable first, so a scarce wild is spent where it buys the most.
	type opt struct {
		rank     string
		naturals []string
	}
	opts := make([]opt, 0, len(naturalsByRank))
	for rank, naturals := range naturalsByRank {
		opts = append(opts, opt{rank: rank, naturals: naturals})
	}
	sort.Slice(opts, func(i, j int) bool {
		vi, vj := cardValue(opts[i].naturals[0]), cardValue(opts[j].naturals[0])
		if vi != vj {
			return vi > vj
		}
		return opts[i].rank < opts[j].rank
	})

	spent := 0
	var out []candidate
	for _, o := range opts {
		cards := append([]string(nil), o.naturals...)
		if len(cards) > canastaSize {
			cards = cards[:canastaSize]
		}
		// Pad with wilds only when the meld would otherwise be too small.
		for len(cards) < minMeldSize {
			if spent >= len(wilds) {
				break
			}
			cards = append(cards, wilds[spent])
			spent++
		}
		if validateMeld(r, cards) != nil {
			continue
		}
		out = append(out, candidate{Rank: o.rank, Cards: cards, Value: handValue(cards)})
	}
	sortCandidates(out)
	return out
}

// allMeldCandidates enumerates every meld a hand could lay as its own,
// independent move — the question offers.go actually asks. Unlike
// newMeldCandidates, it never lets one rank's use of a wild count against
// another's: two ranks can each want the same physical joker, and both are
// still real melds a player could choose to lay this turn, even though
// choosing one spends the wild the other wanted. Hiding the second because
// the first claimed it first would take a legal move off the board.
func allMeldCandidates(r ruleset, hand []string, t *Team) []candidate {
	naturalsByRank := meldableNaturals(r, hand, t)
	wilds := handWilds(hand)

	var out []candidate
	for rank, naturals := range naturalsByRank {
		cards := append([]string(nil), naturals...)
		if len(cards) > canastaSize {
			cards = cards[:canastaSize]
		}
		for i := 0; len(cards) < minMeldSize && i < len(wilds); i++ {
			cards = append(cards, wilds[i])
		}
		if validateMeld(r, cards) != nil {
			continue
		}
		out = append(out, candidate{Rank: rank, Cards: cards, Value: handValue(cards)})
	}
	sortCandidates(out)
	return out
}

// layOffCards lists which cards in a hand this meld would accept.
//
// A natural of the meld's rank always fits; a wild fits only while the meld
// still has room for one. A closed canasta accepts nothing.
func layOffCards(r ruleset, hand []string, m *Meld) []string {
	if m == nil || m.closed(r) {
		return nil
	}
	if m.kind() == meldRun {
		return runExtensions(hand, m, m.room(r))
	}
	room := m.room(r)
	var out []string
	for _, c := range hand {
		if isWild(c) {
			if m.wilds() < r.MaxWilds && room > 0 {
				out = append(out, c)
			}
			continue
		}
		if rankOf(c) == m.Rank {
			out = append(out, c)
		}
	}
	return sortedUnique(out)
}

// runExtensions is which cards in a hand this sequence would accept.
//
// Both ends, and as far as the hand actually reaches: holding the eight and the
// nine of hearts against a run ending at the seven is a two-card lay-off, and
// listing only the eight would make the player do it in two turns. Bounded by
// room, so a run three cards from a samba never offers four.
func runExtensions(hand []string, m *Meld, room int) []string {
	if room <= 0 {
		return nil
	}
	// One card per position: the second nine of hearts in a three-deck hand is
	// not a second step, so it is not a second option either.
	byIndex := map[int]string{}
	for _, c := range hand {
		if suitOf(c) != m.Suit {
			continue
		}
		i, ok := runIndexOf(c)
		if !ok {
			continue
		}
		if _, taken := byIndex[i]; !taken {
			byIndex[i] = c
		}
	}

	low, high := runSpan(m.Cards)
	var out []string
	// Upward first, then downward, because a sequence is worth more at the top
	// and a scarce slot should be spent there.
	for i := high + 1; len(out) < room; i++ {
		c, ok := byIndex[i]
		if !ok {
			break
		}
		out = append(out, c)
	}
	for i := low - 1; len(out) < room && i >= 0; i-- {
		c, ok := byIndex[i]
		if !ok {
			break
		}
		out = append(out, c)
	}
	return sortedUnique(out)
}

// runCandidates enumerates the sequences a hand could lay right now.
//
// This is the function the whole "Samba is still offer-driven" claim rests on
// (docs/samba-plan.md §3.4). A sequence takes no wilds, so a candidate is not a
// shape to be composed but a fact about the hand: the maximal blocks of
// consecutive ranks within one suit. There are at most four suits' worth, and in
// practice nought to four — nothing like the explosion a rummy run produces when
// jokers may stand in anywhere.
func runCandidates(r ruleset, hand []string, t *Team) []candidate {
	if !r.Sequences {
		return nil
	}

	// One card per suit-and-rank: a duplicate cannot extend a run, so it cannot
	// change which runs exist.
	bySuit := map[string]map[int]string{}
	for _, c := range hand {
		if isWild(c) {
			continue
		}
		i, ok := runIndexOf(c)
		if !ok {
			continue // threes, which are never part of a sequence
		}
		s := suitOf(c)
		if bySuit[s] == nil {
			bySuit[s] = map[int]string{}
		}
		if _, taken := bySuit[s][i]; !taken {
			bySuit[s][i] = c
		}
	}

	suitsInOrder := append([]string(nil), suits...)
	sort.Strings(suitsInOrder)

	var out []candidate
	for _, s := range suitsInOrder {
		byIndex := bySuit[s]
		if len(byIndex) < minMeldSize {
			continue
		}
		for i := 0; i < len(runRanks); {
			var block []string
			for j := i; j < len(runRanks); j++ {
				c, ok := byIndex[j]
				if !ok {
					break
				}
				block = append(block, c)
			}
			if len(block) < minMeldSize {
				i++
				continue
			}
			// The top of the block, not the bottom: a sequence is capped at
			// seven and the high cards are the valuable ones.
			if len(block) > canastaSize {
				block = block[len(block)-canastaSize:]
			}
			if validateRun(r, block) == nil && !runOverlapsTable(t, s, block) {
				out = append(out, candidate{
					Kind: meldRun, Suit: s,
					Cards: sortRun(block), Value: handValue(block),
				})
			}
			i += len(block)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Value != out[j].Value {
			return out[i].Value > out[j].Value
		}
		return out[i].Cards[0] < out[j].Cards[0]
	})
	return out
}

// runOverlapsTable keeps a candidate from proposing cards that belong on a
// sequence the side already has — that is a lay-off, and offering it as a new
// meld would put two controls on screen for one move.
func runOverlapsTable(t *Team, suit string, block []string) bool {
	if t == nil {
		return false
	}
	for i := range t.Melds {
		m := &t.Melds[i]
		if m.kind() != meldRun || m.Suit != suit {
			continue
		}
		low, high := runSpan(m.Cards)
		for _, c := range block {
			if j, ok := runIndexOf(c); ok && j >= low-1 && j <= high+1 {
				return true
			}
		}
	}
	return false
}

// reachableValue is the most a hand could still add to this turn's melded
// total, counting only melds it could actually lay.
//
// It exists to keep the initial meld recoverable. The minimum is a property of
// a whole turn, so a player may lay two melds to reach 50 — but that means a
// player can also lay 30 and then find they cannot reach it, with no way to
// pick the cards back up. Rather than forbid multi-meld initials (which real
// play relies on) or leave a dead end, a lay that leaves the minimum
// unreachable is refused *before* the cards leave the hand.
//
// Deliberately a lower bound: it counts a greedy, concretely achievable set of
// melds, never a hypothetical one. Erring high would put the dead end back.
func reachableValue(r ruleset, hand []string, t *Team) int {
	remaining := append([]string(nil), hand...)
	total := 0

	// Lay-offs onto melds the team already has, including any laid earlier
	// this turn.
	if t != nil {
		for i := range t.Melds {
			m := t.Melds[i]
			for _, c := range layOffCards(r, remaining, &m) {
				if m.closed(r) {
					break
				}
				if isWild(c) && m.wilds() >= r.MaxWilds {
					continue
				}
				m.Cards = append(m.Cards, c)
				remaining, _ = removeCards(remaining, []string{c})
				total += cardValue(c)
			}
		}
	}
	for _, c := range newMeldCandidates(r, remaining, t) {
		total += c.Value
		// Spent, not merely counted. The king of hearts can be the third king
		// of a group *and* the top of a heart sequence, and a bound that let it
		// be both would be an over-estimate — which is precisely the dead end
		// this function exists to prevent, reintroduced by the fix for it.
		remaining, _ = removeCards(remaining, c.Cards)
	}
	// Sequences count too, and they have to: a Samba side opening on the
	// 4-5-6-7-8 of hearts reaches its floor on a run alone, and a reachability
	// check blind to runs would refuse the lay that gets it there.
	for _, c := range runCandidates(r, remaining, t) {
		total += c.Value
		remaining, _ = removeCards(remaining, c.Cards)
	}
	return total
}
