// Package cardinfer estimates, from public information only, which unseen
// cards each opponent at a rummy table probably holds — and which cards each
// of them would be glad to be handed.
//
// It is game-agnostic: a game describes its deck and meld shapes in a Spec,
// and each seat's public record (what it took off the pile, what it threw,
// what it declined, what it has on the table) in a Seat. The Canasta and
// Žolíky adapters build those from their own state; nothing here knows either
// game's rules beyond the Spec.
//
// # The model
//
// Exact counting first. Every card has a known number of copies in play
// (Spec.Copies, Spec.Jokers). Copies the observer can see — its own hand, the
// pile, every meld — are gone. Copies a seat was seen to take and has not put
// down since are that seat's, exactly (Seat.Held). What is left (the unseen
// pool) is in the stock or in the part of somebody's hand nobody has seen.
//
// Then a likelihood weight per seat and card for that unseen part, a product
// of heuristic factors read off the seat's public behaviour (weights below):
//
//	discarded     a seat that threw a rank is less likely to be keeping it,
//	              and — where runs exist — its suit neighbours. Its last few
//	              discards say more than old ones.
//	passed        a seat that drew from the stock rather than take the top
//	              discard is less likely to hold cards that card would have
//	              made something with. "Strong" passes (the adapter knows the
//	              take would have been legal had the seat held a pair) count
//	              for more.
//	took          cards a seat took off the pile are cards it is building
//	              around: the same rank and the suit neighbours are more
//	              likely held, because partials are what a player keeps.
//	melds         a seat that may lay off is less likely to be sitting on a
//	              card that goes straight onto a meld it could add to.
//
// The unseen copies are shared out between the seats and the stock by
// iterative proportional fitting: each seat's expected unseen cards sum to
// its hand size minus what it is known to hold, each card's expected copies
// across seats and stock sum to its unseen count, and the weights set the
// proportions within those two constraints. The stock is a row with weight
// one everywhere — it is a random draw from the pool.
//
// Hold is then the known cards plus that expectation, so the expected cards
// per opponent equal its hand size exactly whenever the counts are consistent
// (they are, for any real position). Want is computed from Hold: the chance
// a card lays straight onto something the seat may add to, or completes (or
// nearly completes) a set or a run window with what the seat probably holds,
// a wild filling one gap.
//
// Everything is deterministic and allocation-free apart from the caller's
// Observation; a call costs a few microseconds at a four-seat table.
package cardinfer

import "math"

// A card slot: rank (2..A, rules.RankOrder) × suit (H C D S), then one slot
// for every joker. The same layout as the Žolíky encoder's cardSlot.
const (
	NumRanks  = 13
	NumSuits  = 4
	JokerSlot = NumRanks * NumSuits
	NumSlots  = JokerSlot + 1
	// MaxSeats is the most opponents an Estimate describes (Samba's six
	// seats, less the observer).
	MaxSeats = 5
)

const (
	rankOrder = "23456789TJQKA"
	suitOrder = "HCDS"
	rankTwo   = 0
	rankAce   = NumRanks - 1
)

// Slot is a card's slot, or -1 for something that is not a card.
func Slot(card string) int {
	if len(card) >= 5 && card[:5] == "JOKER" {
		return JokerSlot
	}
	if len(card) != 2 {
		return -1
	}
	r, s := -1, -1
	for i := 0; i < len(rankOrder); i++ {
		if rankOrder[i] == card[0] {
			r = i
		}
	}
	for i := 0; i < len(suitOrder); i++ {
		if suitOrder[i] == card[1] {
			s = i
		}
	}
	if r < 0 || s < 0 {
		return -1
	}
	return r*NumSuits + s
}

// SlotCard is the card a slot names; the joker slot names JOKER1.
func SlotCard(slot int) string {
	if slot == JokerSlot {
		return "JOKER1"
	}
	if slot < 0 || slot >= JokerSlot {
		return ""
	}
	return string(rankOrder[slot/NumSuits]) + string(suitOrder[slot%NumSuits])
}

// RankOf and SuitOf are a natural slot's rank and suit indices.
func RankOf(slot int) int { return slot / NumSuits }
func SuitOf(slot int) int { return slot % NumSuits }

func slotOf(rank, suit int) int { return rank*NumSuits + suit }

// Slots converts cards to slots, dropping anything that is not a card.
func Slots(cards []string) []int {
	out := make([]int, 0, len(cards))
	for _, c := range cards {
		if k := Slot(c); k >= 0 {
			out = append(out, k)
		}
	}
	return out
}

// Spec is what the model needs to know about a game.
type Spec struct {
	// Copies of each natural card in play, and jokers in play.
	Copies, Jokers int
	// MinSet is the smallest set; MinRun the smallest run, zero where the
	// game has no runs.
	MinSet, MinRun int
	// AceLow lets the ace sit below the two in a run as well as above the
	// king (Žolíky). RunFrom is the lowest rank index a run may use otherwise
	// (Samba's sequences start at the four: 2).
	AceLow  bool
	RunFrom int
	// DistinctSuitSets is a set that takes one card of each suit (Žolíky);
	// otherwise any natural of the rank joins it (Canasta).
	DistinctSuitSets bool
	// WildDeuces makes the twos wild (Canasta), alongside the jokers.
	WildDeuces bool
	// Unmeldable ranks are never wanted for a meld (Canasta's threes).
	Unmeldable [NumRanks]bool
}

// IsWild reports a slot that stands in for any card.
func (s Spec) IsWild(slot int) bool {
	return slot == JokerSlot || (s.WildDeuces && slot >= 0 && slot < JokerSlot && RankOf(slot) == rankTwo)
}

func (s Spec) total(slot int) int {
	if slot == JokerSlot {
		return s.Jokers
	}
	return s.Copies
}

// Pass is a top discard a seat declined. Strong is the adapter saying the
// seat would certainly have taken it had it held what the card goes with
// (Canasta's frozen pile: a natural pair of the rank).
type Pass struct {
	Slot   int
	Strong bool
}

// Seat is one opponent's public record this deal.
type Seat struct {
	HandSize int
	// Held is the cards the seat was seen to take and has not shown since.
	Held []int
	// Discards is the seat's discards this deal, oldest first.
	Discards []int
	// Passes is the top discards it declined, oldest first.
	Passes []Pass
	// Extends marks the cards that would go straight onto a meld this seat
	// may add to right now: they are wanted with certainty.
	Extends [NumSlots]bool
	// Down says the seat may lay off now, so it is unlikely to be sitting on
	// a card that Extends marks.
	Down bool
}

// Observation is one seat's view of the table.
type Observation struct {
	Spec Spec
	// Mine is the observer's own hand; Seen every other card face up (the
	// pile, every meld, anything else on the table).
	Mine, Seen []int
	// Seats are the opponents, in the order they play after the observer.
	Seats []Seat
}

// Estimate is what Infer works out. Rows beyond N are zero.
type Estimate struct {
	N int
	// Unseen is the copies of each card the observer cannot account for
	// (the stock plus the unknown parts of the hands).
	Unseen [NumSlots]float64
	// Hold is each seat's expected copies of each card; a row sums to the
	// seat's hand size.
	Hold [MaxSeats][NumSlots]float64
	// Known is the part of Hold that is certain: cards the seat was seen to
	// take and still holds.
	Known [MaxSeats][NumSlots]float64
	// Want is, per seat and card, how likely the card is to help that seat:
	// it lays off, or completes a set or a run window with what the seat
	// probably holds. 0..1.
	Want [MaxSeats][NumSlots]float64
	// Base is, per seat, the Want of a card drawn at random from the unseen
	// pool: what any card is worth to that seat before the evidence about
	// this one. A bot pricing a discard cares about the excess over it (see
	// Excess) — every card helps a random hand a little.
	Base [MaxSeats]float64
}

// The evidence weights. Multipliers on the prior (the unseen pool), per seat
// and card. Chosen by hand to be modest — no single observation moves a card
// by more than a factor of four — and documented here so a change is one
// line.
const (
	// The last freshDiscards discards of a seat count fully; older ones
	// count at the "old" weights, because a hand changes.
	freshDiscards       = 3
	wDiscardRank        = 0.45
	wDiscardRankOld     = 0.75
	wDiscardNeighbour   = 0.7
	wDiscardNeighbourOd = 0.9
	wPassRank           = 0.65
	wPassRankStrong     = 0.25
	wPassNeighbour      = 0.85
	wHeldRank           = 1.6
	wHeldNeighbour1     = 1.5
	wHeldNeighbour2     = 1.2
	// wLayable is a card a seat that may lay off could have put down.
	wLayable = 0.5
	// rho is how much a card that only builds (a single partner, no
	// completion) counts toward Want.
	rho = 0.2
	// ipfRounds bounds the fitting; it converges in far fewer.
	ipfRounds = 24
)

// Infer fills e from o. Seats beyond MaxSeats are ignored.
func Infer(o *Observation, e *Estimate) {
	*e = Estimate{}
	sp := o.Spec
	if sp.Copies <= 0 {
		sp.Copies = 2
	}
	n := len(o.Seats)
	if n > MaxSeats {
		n = MaxSeats
	}
	e.N = n

	// Exact counting.
	var left [NumSlots]float64
	for k := 0; k < NumSlots; k++ {
		left[k] = float64(sp.total(k))
	}
	take := func(k int) bool {
		if k >= 0 && k < NumSlots && left[k] >= 1 {
			left[k]--
			return true
		}
		return false
	}
	for _, k := range o.Mine {
		take(k)
	}
	for _, k := range o.Seen {
		take(k)
	}
	var known [MaxSeats][NumSlots]float64
	var unknown [MaxSeats + 1]float64
	for i := 0; i < n; i++ {
		s := &o.Seats[i]
		held := 0
		for _, k := range s.Held {
			if held < s.HandSize && take(k) {
				known[i][k]++
				held++
			}
		}
		unknown[i] = float64(max(0, s.HandSize-held))
	}
	e.Unseen = left
	pool := 0.0
	for _, v := range left {
		pool += v
	}

	// Likelihood weights for the unknown part of each hand.
	var w [MaxSeats + 1][NumSlots]float64
	for i := 0; i < n; i++ {
		seatWeights(&sp, &o.Seats[i], &w[i])
	}
	for k := range w[n] {
		w[n][k] = 1 // the stock
	}
	hands := 0.0
	for i := 0; i < n; i++ {
		hands += unknown[i]
	}
	unknown[n] = math.Max(0, pool-hands)
	if hands > pool && hands > 0 {
		// Inconsistent counts (a caller that saw a card twice): share out
		// what there is in proportion rather than invent cards.
		for i := 0; i < n; i++ {
			unknown[i] *= pool / hands
		}
	}

	// Iterative proportional fitting: rows are the seats and the stock, each
	// summing to its unknown cards; columns are the cards, each summing to its
	// unseen copies.
	rows := n + 1
	var m [MaxSeats + 1][NumSlots]float64
	for r := 0; r < rows; r++ {
		for k := 0; k < NumSlots; k++ {
			m[r][k] = w[r][k] * left[k]
		}
	}
	for round := 0; round < ipfRounds; round++ {
		worst := 0.0
		for k := 0; k < NumSlots; k++ {
			col := 0.0
			for r := 0; r < rows; r++ {
				col += m[r][k]
			}
			if col > 0 {
				f := left[k] / col
				worst = max(worst, math.Abs(f-1))
				for r := 0; r < rows; r++ {
					m[r][k] *= f
				}
			}
		}
		for r := 0; r < rows; r++ {
			row := 0.0
			for k := 0; k < NumSlots; k++ {
				row += m[r][k]
			}
			if row > 0 {
				f := unknown[r] / row
				for k := 0; k < NumSlots; k++ {
					m[r][k] *= f
				}
			}
		}
		if worst < 1e-4 {
			break
		}
	}
	for i := 0; i < n; i++ {
		for k := 0; k < NumSlots; k++ {
			e.Hold[i][k] = known[i][k] + m[i][k]
			e.Known[i][k] = known[i][k]
		}
	}
	for i := 0; i < n; i++ {
		wants(&sp, &o.Seats[i], &e.Hold[i], &e.Want[i])
		if pool > 0 {
			b := 0.0
			for k := 0; k < NumSlots; k++ {
				b += left[k] * e.Want[i][k]
			}
			e.Base[i] = b / pool
		}
	}
}

// Excess is how much more seat i wants card k than a random unseen card,
// rescaled to 0..1: zero for a card no better than the pool, one for a card
// it certainly wants.
func (e *Estimate) Excess(i, k int) float64 {
	if i < 0 || i >= e.N || k < 0 || k >= NumSlots {
		return 0
	}
	b := e.Base[i]
	if b >= 1 {
		return 0
	}
	return max(0, e.Want[i][k]-b) / (1 - b)
}

// seatWeights is one seat's likelihood weights over the card slots.
func seatWeights(sp *Spec, s *Seat, w *[NumSlots]float64) {
	for k := range w {
		w[k] = 1
	}
	runs := sp.MinRun > 0
	rank := func(r int, f float64) {
		for su := 0; su < NumSuits; su++ {
			w[slotOf(r, su)] *= f
		}
	}
	near := func(k, d int, f float64) {
		r, su := RankOf(k), SuitOf(k)
		for x := 0; x < NumRanks; x++ {
			if isNeighbour(sp, r, x, d) {
				w[slotOf(x, su)] *= f
			}
		}
	}
	natural := func(k int) bool { return k >= 0 && k < JokerSlot && !sp.IsWild(k) }

	for i, k := range s.Discards {
		if !natural(k) {
			continue
		}
		fresh := i >= len(s.Discards)-freshDiscards
		if fresh {
			rank(RankOf(k), wDiscardRank)
		} else {
			rank(RankOf(k), wDiscardRankOld)
		}
		if runs {
			if fresh {
				near(k, 1, wDiscardNeighbour)
			} else {
				near(k, 1, wDiscardNeighbourOd)
			}
		}
	}
	for _, p := range s.Passes {
		if !natural(p.Slot) {
			continue
		}
		if p.Strong {
			rank(RankOf(p.Slot), wPassRankStrong)
		} else {
			rank(RankOf(p.Slot), wPassRank)
		}
		if runs {
			near(p.Slot, 1, wPassNeighbour)
		}
	}
	for _, k := range s.Held {
		if !natural(k) {
			continue
		}
		r, su := RankOf(k), SuitOf(k)
		for x := 0; x < NumSuits; x++ {
			if x != su || !sp.DistinctSuitSets {
				w[slotOf(r, x)] *= wHeldRank
			}
		}
		if runs {
			near(k, 1, wHeldNeighbour1)
			for x := 0; x < NumRanks; x++ {
				if isNeighbour(sp, r, x, 2) && !isNeighbour(sp, r, x, 1) {
					w[slotOf(x, su)] *= wHeldNeighbour2
				}
			}
		}
	}
	if s.Down {
		for k := 0; k < NumSlots; k++ {
			if s.Extends[k] && natural(k) {
				w[k] *= wLayable
			}
		}
	}
}

// isNeighbour reports that ranks a and b are within d run steps of each
// other (and not the same place), the ace at both ends where the game allows
// it. Never, without runs.
func isNeighbour(sp *Spec, a, b, d int) bool {
	if sp.MinRun <= 0 || a == b {
		return false
	}
	pa, na := positions(sp, a)
	pb, nb := positions(sp, b)
	for i := 0; i < na; i++ {
		for j := 0; j < nb; j++ {
			if diff := pa[i] - pb[j]; diff != 0 && diff >= -d && diff <= d {
				return true
			}
		}
	}
	return false
}

// positions is where a rank sits in a run: 1..13 for the two to the ace, and
// 0 for the ace below the two where the game allows it. n is zero for a rank
// no run may hold.
func positions(sp *Spec, r int) (p [2]int, n int) {
	if r < sp.RunFrom && r != rankAce {
		return p, 0
	}
	if sp.Unmeldable[r] || (sp.WildDeuces && r == rankTwo) {
		return p, 0
	}
	if r == rankAce && sp.AceLow {
		return [2]int{0, NumRanks}, 2
	}
	return [2]int{r + 1}, 1
}

// rankAt is the rank at a run position, or -1.
func rankAt(sp *Spec, p int) int {
	switch {
	case p == 0:
		if sp.AceLow {
			return rankAce
		}
		return -1
	case p >= 1 && p <= NumRanks:
		r := p - 1
		if r < sp.RunFrom && r != rankAce {
			return -1
		}
		if sp.Unmeldable[r] || (sp.WildDeuces && r == rankTwo) {
			return -1
		}
		return r
	}
	return -1
}

// wants fills one seat's Want row from its Hold row.
func wants(sp *Spec, s *Seat, hold, want *[NumSlots]float64) {
	// present is the chance of holding at least one copy, read as the
	// expected copies capped at one.
	var present [NumSlots]float64
	for k, v := range hold {
		present[k] = min(1, v)
	}
	wild := hold[JokerSlot]
	if sp.WildDeuces {
		for su := 0; su < NumSuits; su++ {
			wild += hold[slotOf(rankTwo, su)]
		}
	}
	pw := 1 - math.Exp(-wild)
	needSet := sp.MinSet - 1
	if needSet < 1 {
		needSet = 2
	}
	for k := 0; k < NumSlots; k++ {
		if sp.IsWild(k) {
			want[k] = 1
			continue
		}
		r, su := RankOf(k), SuitOf(k)
		if sp.Unmeldable[r] {
			continue
		}
		miss := 1.0
		if s.Extends[k] {
			miss = 0
		}
		// A set: partners of the rank, a wild filling the last place.
		lambda := 0.0
		for x := 0; x < NumSuits; x++ {
			kk := slotOf(r, x)
			if sp.DistinctSuitSets {
				if x != su {
					lambda += present[kk]
				}
			} else {
				lambda += hold[kk]
			}
		}
		miss *= 1 - setChance(lambda, needSet, pw)
		// Runs: every window of MinRun positions holding this card.
		if sp.MinRun > 0 {
			ps, np := positions(sp, r)
			for _, p := range ps[:np] {
				for lo := p - sp.MinRun + 1; lo <= p; lo++ {
					f, ok := windowChance(sp, su, lo, p, &present, pw)
					if ok {
						miss *= 1 - f
					}
				}
				adj := 1.0
				for q := p - 1; q <= p+1; q += 2 {
					if x := rankAt(sp, q); x >= 0 {
						adj *= 1 - present[slotOf(x, su)]
					}
				}
				miss *= 1 - rho*(1-adj)
			}
		}
		want[k] = 1 - miss
	}
}

// setChance is the chance that a card joins a set: at least need partners
// among an expected lambda (Poisson), or one short with a wild to fill it, or
// — at rho — one short with nothing to fill it, which is still progress.
func setChance(lambda float64, need int, pw float64) float64 {
	e := math.Exp(-lambda)
	cdf, term := 0.0, e
	for j := 0; j < need-1; j++ {
		cdf += term
		term *= lambda / float64(j+1)
	}
	short := term // P(exactly need-1)
	atLeast := 1 - cdf - short
	return min(1, atLeast+short*pw+rho*short*(1-pw))
}

// windowChance is the chance the run window [lo, lo+MinRun) in a suit is
// complete once this card (at position self) joins it: every other place
// held, or all but one with a wild for the gap. ok is false for a window
// that runs off the board.
func windowChance(sp *Spec, su, lo, self int, present *[NumSlots]float64, pw float64) (float64, bool) {
	all, oneGap := 1.0, 0.0
	for p := lo; p < lo+sp.MinRun; p++ {
		if p == self {
			continue
		}
		x := rankAt(sp, p)
		if x < 0 {
			return 0, false
		}
		q := present[slotOf(x, su)]
		oneGap = oneGap*q + all*(1-q)
		all *= q
	}
	return min(1, all+oneGap*pw), true
}

// Feed is how much discarding slot k helps the seat after the observer (seat
// 0) and the opponent it helps most, as Excess: the part of the want that is
// about this card rather than about any card.
func (e *Estimate) Feed(k int) (next, anyone float64) {
	if k < 0 || k >= NumSlots || e.N == 0 {
		return 0, 0
	}
	next = e.Excess(0, k)
	for i := 0; i < e.N; i++ {
		anyone = math.Max(anyone, e.Excess(i, k))
	}
	return next, anyone
}

// TopWanted is the k cards a seat most wants among those still unseen, best
// first, ties broken by slot. Fills out and returns how many it found.
func (e *Estimate) TopWanted(seat int, out []int) int {
	n := 0
	for i := range out {
		best, bestV := -1, 0.0
		for k := 0; k < NumSlots; k++ {
			if e.Unseen[k] <= 0 {
				continue
			}
			v := e.Want[seat][k]
			if v <= bestV {
				continue
			}
			taken := false
			for _, o := range out[:i] {
				taken = taken || o == k
			}
			if !taken {
				best, bestV = k, v
			}
		}
		if best < 0 {
			break
		}
		out[i] = best
		n++
	}
	return n
}
