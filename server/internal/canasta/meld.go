package canasta

import "sort"

// The shape of a legal meld. Named rather than written inline so the rules
// read as rules and there is exactly one place to change any of them.
const (
	minMeldSize = 3
	canastaSize = 7
	maxWilds    = 3
	minNaturals = 2
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
func validateMeld(cards []string) error {
	if len(cards) < minMeldSize {
		return errCode(ErrMeldTooSmall)
	}
	if len(cards) > canastaSize {
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
	if wilds > maxWilds {
		return errCode(ErrTooManyWilds)
	}
	if len(cards)-wilds < minNaturals {
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
	Rank  string
	Cards []string
	Value int
}

// meldableNaturals lists, for each rank the hand could still open a new meld
// on, that rank's natural (non-wild) cards — the one fact both meld-candidate
// functions below need before they diverge on how to spend the hand's wilds.
func meldableNaturals(hand []string, t *Team) map[string][]string {
	byRank := countByRank(hand)
	out := map[string][]string{}
	for rank, cards := range byRank {
		if rank == rankThree || isWild(cards[0]) {
			continue
		}
		if t != nil && t.meld(rank) != nil {
			continue // already melded: that is a lay-off, not a new meld
		}
		naturals := make([]string, 0, len(cards))
		for _, c := range cards {
			if !isWild(c) {
				naturals = append(naturals, c)
			}
		}
		if len(naturals) < minNaturals {
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
		return out[i].Rank < out[j].Rank
	})
}

// newMeldCandidates enumerates the new melds a hand could lay, best first, as
// a *turn's* worth of melding: every rank draws from one shared pool of
// wilds, spent greedily on the most valuable rank that needs one. That is
// what makes the result achievable rather than merely plausible, which is
// exactly the property reachableValue needs — the one caller of this
// function — to stay a safe lower bound. It is the wrong function for asking
// "what single meld could I lay right now": see allMeldCandidates for that.
func newMeldCandidates(hand []string, t *Team) []candidate {
	naturalsByRank := meldableNaturals(hand, t)
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
		if validateMeld(cards) != nil {
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
func allMeldCandidates(hand []string, t *Team) []candidate {
	naturalsByRank := meldableNaturals(hand, t)
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
		if validateMeld(cards) != nil {
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
func layOffCards(hand []string, m *Meld) []string {
	if m == nil || m.closed() {
		return nil
	}
	room := canastaSize - len(m.Cards)
	var out []string
	for _, c := range hand {
		if isWild(c) {
			if m.wilds() < maxWilds && room > 0 {
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
func reachableValue(hand []string, t *Team) int {
	remaining := append([]string(nil), hand...)
	total := 0

	// Lay-offs onto melds the team already has, including any laid earlier
	// this turn.
	if t != nil {
		for i := range t.Melds {
			m := t.Melds[i]
			for _, c := range layOffCards(remaining, &m) {
				if m.closed() {
					break
				}
				if isWild(c) && m.wilds() >= maxWilds {
					continue
				}
				m.Cards = append(m.Cards, c)
				remaining, _ = removeCards(remaining, []string{c})
				total += cardValue(c)
			}
		}
	}
	for _, c := range newMeldCandidates(remaining, t) {
		total += c.Value
	}
	return total
}
