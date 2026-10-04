package search

// The solver as it was before its inner loop stopped allocating, kept as the
// reference the rewrite must agree with node for node.

import (
	"math/bits"
	"math/rand"
	"reflect"
	"sort"
	"testing"

	"zolik/server/internal/tricks"
)

// Solve values every legal move of the seat to move.
func refSolve(p Problem) Result {
	s := refNewSolver(p)
	seat := (p.Leader + len(p.Trick)) % len(p.Hands)
	res := Result{Values: map[string]int{}}
	// Every legal card gets a value — an equivalent one the value of the
	// card it is equivalent to, without searching it again.
	all := s.sorted(s.legal(seat))
	for i, c := range all {
		if i > 0 && s.equivalent(all[i-1], c) {
			res.Values[s.cards[c]] = res.Values[s.cards[all[i-1]]]
			continue
		}
		res.Values[s.cards[c]] = s.play(seat, c, -inf, inf)
		if s.exhausted {
			break
		}
	}
	res.Nodes, res.Exhausted = s.nodes, s.exhausted
	return res
}

// --- the solver, on bitmasks -------------------------------------------------

type refTTEntry struct {
	lower, upper int
}

type refTTKey struct {
	hands  [4]uint64
	leader int
}

type refSolver struct {
	p     Problem
	n     int
	cards []string       // index → code
	index map[string]int // code → index
	suit  []byte
	// rank strength: higher beats lower within a suit.
	strength []int
	suitMask map[byte]uint64
	// beats[c] is the cards of c's suit that beat c.
	beats []uint64

	class []int

	hands    []uint64
	trick    []tricks.Play
	trickIdx []int
	leader   int
	reserves []refReserveMask
	pairs    []refPairMask

	tt        map[refTTKey]refTTEntry
	nodes     int
	exhausted bool
}

type refPairMask struct {
	first, second int
	value         int
}

type refReserveMask struct {
	seat      int
	card      int
	untilLeft int
}

func refNewSolver(p Problem) *refSolver {
	s := &refSolver{p: p, n: len(p.Hands), index: map[string]int{}, suitMask: map[byte]uint64{}, tt: map[refTTKey]refTTEntry{}}
	add := func(c string) {
		if _, ok := s.index[c]; !ok {
			s.index[c] = len(s.cards)
			s.cards = append(s.cards, c)
		}
	}
	for _, h := range p.Hands {
		for _, c := range h {
			add(c)
		}
	}
	for _, pl := range p.Trick {
		add(pl.Card)
	}
	for _, c := range s.cards {
		su := tricks.Suit(c)
		s.suit = append(s.suit, su)
		s.strength = append(s.strength, len(p.Order)-refRankIndex(p.Order, tricks.Rank(c)))
	}
	for i := range s.cards {
		s.suitMask[s.suit[i]] |= 1 << i
	}
	s.beats = make([]uint64, len(s.cards))
	for i := range s.cards {
		for j := range s.cards {
			if s.suit[i] == s.suit[j] && s.strength[j] > s.strength[i] {
				s.beats[i] |= 1 << j
			}
		}
	}
	s.class = make([]int, len(s.cards))
	for i, c := range s.cards {
		if p.CardClass != nil {
			s.class[i] = p.CardClass(c)
		} else {
			s.class[i] = i // every card its own class: no pruning
		}
	}
	s.hands = make([]uint64, s.n)
	for seat, h := range p.Hands {
		for _, c := range h {
			s.hands[seat] |= 1 << s.index[c]
		}
	}
	s.trick = append([]tricks.Play(nil), p.Trick...)
	for _, pl := range p.Trick {
		s.trickIdx = append(s.trickIdx, s.index[pl.Card])
	}
	s.leader = p.Leader
	for _, pr := range p.Pairs {
		f, okF := s.index[pr.First]
		sec, okS := s.index[pr.Second]
		if okF && okS {
			s.pairs = append(s.pairs, refPairMask{first: f, second: sec, value: pr.Value})
		}
	}
	for _, r := range p.Reserves {
		if i, ok := s.index[r.Card]; ok {
			s.reserves = append(s.reserves, refReserveMask{seat: r.Seat, card: i, untilLeft: r.UntilLeft})
		}
	}
	return s
}

func refRankIndex(o tricks.Order, r byte) int {
	for i := 0; i < len(o); i++ {
		if o[i] == r {
			return i
		}
	}
	return len(o)
}

// winning is the index into the trick of the play currently taking it.
func (s *refSolver) winning() int {
	best := 0
	for i := 1; i < len(s.trickIdx); i++ {
		c, b := s.trickIdx[i], s.trickIdx[best]
		switch {
		case s.suit[c] == s.suit[b]:
			if s.strength[c] > s.strength[b] {
				best = i
			}
		case s.p.Trump != tricks.NoTrump && s.suit[c] == s.p.Trump:
			best = i
		}
	}
	return best
}

// legal is tricks.Legal on bitmasks, plus the reserves.
func (s *refSolver) legal(seat int) uint64 {
	hand := s.hands[seat]
	var out uint64
	switch {
	case len(s.trickIdx) == 0:
		out = hand
	default:
		led := s.suit[s.trickIdx[0]]
		best := s.trickIdx[s.winning()]
		if follow := hand & s.suitMask[led]; follow != 0 {
			out = follow
			if s.p.Policy == tricks.FollowBeatTrump && s.suit[best] == led {
				if higher := follow & s.beats[best]; higher != 0 {
					out = higher
				}
			}
			break
		}
		if s.p.Policy == tricks.FollowOnly || s.p.Trump == tricks.NoTrump {
			out = hand
			break
		}
		trumps := hand & s.suitMask[s.p.Trump]
		if trumps == 0 {
			out = hand
			break
		}
		out = trumps
		if s.suit[best] == s.p.Trump {
			if higher := trumps & s.beats[best]; higher != 0 {
				out = higher
			}
		}
	}
	held := bits.OnesCount64(hand)
	for _, r := range s.reserves {
		if r.seat == seat && held > r.untilLeft && out&(1<<r.card) != 0 && bits.OnesCount64(out) > 1 {
			out &^= 1 << r.card
		}
	}
	return out
}

// ordered lists a move mask strongest card first — winners early make
// alpha-beta cut sooner — and drops every card equivalent to one already
// listed: the same suit and class, with nothing still in play ranked
// between them.
func (s *refSolver) ordered(moves uint64, _ int) []int {
	all := s.sorted(moves)
	out := all[:0:0]
	for i, c := range all {
		if i > 0 && s.equivalent(all[i-1], c) {
			continue
		}
		out = append(out, c)
	}
	return out
}

// sorted lists a move mask strongest card first.
func (s *refSolver) sorted(moves uint64) []int {
	var all []int
	for m := moves; m != 0; m &= m - 1 {
		all = append(all, bits.TrailingZeros64(m))
	}
	sort.Slice(all, func(i, j int) bool { return s.strength[all[i]] > s.strength[all[j]] })
	return all
}

// equivalent is whether c, the next card down from prev in a sorted move
// list, would play exactly as prev does: same suit and class, and nothing
// still in play ranked between them.
func (s *refSolver) equivalent(prev, c int) bool {
	if s.suit[c] != s.suit[prev] || s.class[c] != s.class[prev] {
		return false
	}
	var inPlay uint64
	for _, h := range s.hands {
		inPlay |= h
	}
	for _, t := range s.trickIdx {
		inPlay |= 1 << t
	}
	between := s.beats[c] &^ s.beats[prev] &^ (1 << prev)
	return between&inPlay == 0
}

// play puts card c down for seat, searches on, and takes it back.
func (s *refSolver) play(seat, c, alpha, beta int) int {
	// A pair scores as the card goes down, for the side that plays it.
	bonus := 0
	for _, pr := range s.pairs {
		if pr.first == c && s.hands[seat]&(1<<pr.second) != 0 {
			if s.p.Maximizer[seat] {
				bonus += pr.value
			} else {
				bonus -= pr.value
			}
		}
	}
	if bonus != 0 {
		return bonus + s.playOn(seat, c, alpha-bonus, beta-bonus)
	}
	return s.playOn(seat, c, alpha, beta)
}

func (s *refSolver) playOn(seat, c, alpha, beta int) int {
	s.hands[seat] &^= 1 << c
	s.trick = append(s.trick, tricks.Play{Seat: seat, Card: s.cards[c]})
	s.trickIdx = append(s.trickIdx, c)
	defer func() {
		s.trick = s.trick[:len(s.trick)-1]
		s.trickIdx = s.trickIdx[:len(s.trickIdx)-1]
		s.hands[seat] |= 1 << c
	}()

	if len(s.trick) < s.n {
		return s.search((seat+1)%s.n, alpha, beta)
	}

	// The trick is complete: score it and go on from its winner.
	winner := s.trick[s.winning()].Seat
	left := bits.OnesCount64(s.hands[0])
	value, stop := s.p.TrickValue(s.trick, winner, left)
	if stop || left == 0 {
		return value
	}
	saved, savedIdx, savedLeader := s.trick, s.trickIdx, s.leader
	s.trick, s.trickIdx, s.leader = nil, nil, winner
	rest := s.search(winner, alpha-value, beta-value)
	s.trick, s.trickIdx, s.leader = saved, savedIdx, savedLeader
	return value + rest
}

// search is alpha-beta over the seat to move. Positions at the start of a
// trick are remembered by the cards left and who leads, since the value of
// the rest of the play does not depend on how it was reached.
func (s *refSolver) search(seat, alpha, beta int) int {
	s.nodes++
	if s.p.NodeLimit > 0 && s.nodes > s.p.NodeLimit {
		s.exhausted = true
		return 0
	}

	var key refTTKey
	atStart := len(s.trickIdx) == 0
	if atStart {
		copy(key.hands[:], s.hands)
		key.leader = seat
		if e, ok := s.tt[key]; ok {
			if e.lower >= beta {
				return e.lower
			}
			if e.upper <= alpha {
				return e.upper
			}
			// A stored bound only ever cuts; it never narrows the window.
			// Narrowing it lets a fail-soft result fall below a known lower
			// bound and be stored as an upper one, contradicting it.
		}
	}
	origAlpha, origBeta := alpha, beta

	max_ := s.p.Maximizer[seat]
	best := inf
	if max_ {
		best = -inf
	}
	for _, c := range s.ordered(s.legal(seat), seat) {
		v := s.play(seat, c, alpha, beta)
		if s.exhausted {
			return 0
		}
		if max_ {
			best = max(best, v)
			alpha = max(alpha, v)
		} else {
			best = min(best, v)
			beta = min(beta, v)
		}
		if alpha >= beta {
			break
		}
	}

	if atStart {
		e, ok := s.tt[key]
		if !ok {
			e = refTTEntry{lower: -inf, upper: inf}
		}
		switch {
		case best <= origAlpha:
			e.upper = min(e.upper, best)
		case best >= origBeta:
			e.lower = max(e.lower, best)
		default:
			e.lower, e.upper = best, best
		}
		s.tt[key] = e
	}
	return best
}

// The rewrite searches exactly what the reference did: the same value for
// every card, the same node count, the same point of exhaustion — from the
// start of a trick and from part-way through one, with pairs, reserves and a
// node limit.
func TestSolverMatchesTheReferenceNodeForNode(t *testing.T) {
	r := rand.New(rand.NewSource(17))
	for i := 0; i < 3000; i++ {
		tricksLeft := 1 + r.Intn(10)
		p := randomProblem(r, tricksLeft)
		if r.Intn(2) == 0 {
			for _, su := range "HDCS" {
				p.Pairs = append(p.Pairs, Pair{First: "Q" + string(su), Second: "K" + string(su), Value: 20 + 20*r.Intn(2)})
			}
		}
		if r.Intn(3) == 0 && p.Trump != tricks.NoTrump {
			p.Reserves = []Reserve{{Seat: r.Intn(3), Card: p.Hands[r.Intn(3)][0], UntilLeft: 1 + r.Intn(2)}}
		}
		if r.Intn(4) == 0 {
			p.CardClass = nil
		}
		if r.Intn(3) == 0 {
			p.NodeLimit = 1 + r.Intn(5000)
		}
		if tricksLeft > 8 {
			p.NodeLimit = 1 + r.Intn(200_000)
		}
		// Part-way through a trick: the leader and maybe the next seat have
		// already played a legal card.
		if k := r.Intn(3); k > 0 && tricksLeft > 1 {
			for j := 0; j < k; j++ {
				seat := (p.Leader + j) % 3
				legal := tricks.Legal(p.Hands[seat], tricks.Trick{Plays: p.Trick}, p.Trump, p.Order, p.Policy)
				c := legal[r.Intn(len(legal))]
				p.Trick = append(p.Trick, tricks.Play{Seat: seat, Card: c})
				p.Hands[seat] = without(p.Hands[seat], c)
			}
		}
		got, want := Solve(p), refSolve(p)
		if got.Nodes != want.Nodes || got.Exhausted != want.Exhausted || !reflect.DeepEqual(got.Values, want.Values) {
			t.Fatalf("problem %d (%v, trick %v, trump %c, limit %d):\n got  %v nodes %d exhausted %v\n want %v nodes %d exhausted %v",
				i, p.Hands, p.Trick, p.Trump, p.NodeLimit, got.Values, got.Nodes, got.Exhausted, want.Values, want.Nodes, want.Exhausted)
		}
	}
}

func benchProblems() []Problem {
	r := rand.New(rand.NewSource(5))
	out := make([]Problem, 8)
	for i := range out {
		p := randomProblem(r, 7)
		p.Trump = 'H'
		for _, su := range "HDCS" {
			p.Pairs = append(p.Pairs, Pair{First: "Q" + string(su), Second: "K" + string(su), Value: 20})
		}
		p.NodeLimit = 400_000
		out[i] = p
	}
	return out
}

func benchSolve(b *testing.B, solve func(Problem) Result) {
	ps := benchProblems()
	nodes := 0
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		nodes += solve(ps[i%len(ps)]).Nodes
	}
	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(nodes), "ns/node")
}

func BenchmarkSolve(b *testing.B)          { benchSolve(b, Solve) }
func BenchmarkSolveReference(b *testing.B) { benchSolve(b, refSolve) }
