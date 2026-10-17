package rules

import "sync"

// cloneState returns a deep copy of game state for dry-run validation.
func cloneState(s GameState) GameState {
	out := s
	out.TurnOrder = append([]string(nil), s.TurnOrder...)
	out.DrawPile = append([]string(nil), s.DrawPile...)
	out.DiscardPile = append([]string(nil), s.DiscardPile...)
	out.Hands = map[string][]string{}
	for k, v := range s.Hands {
		out.Hands[k] = append([]string(nil), v...)
	}
	out.Melds = map[string][][]string{}
	for k, melds := range s.Melds {
		for _, m := range melds {
			out.Melds[k] = append(out.Melds[k], append([]string(nil), m...))
		}
	}
	out.MeldMeta = map[string][]MeldInfo{}
	for k, metas := range s.MeldMeta {
		for _, mi := range metas {
			out.MeldMeta[k] = append(out.MeldMeta[k], mi)
		}
	}
	out.RoundReqMet = map[string]bool{}
	for k, v := range s.RoundReqMet {
		out.RoundReqMet[k] = v
	}
	out.GameScores = map[string][]int{}
	for k, v := range s.GameScores {
		out.GameScores[k] = append([]int(nil), v...)
	}
	out.TotalScores = map[string]int{}
	for k, v := range s.TotalScores {
		out.TotalScores[k] = v
	}
	// The maps are never written once recorded, but the slice is appended to,
	// and a clone sharing its backing array would append into the original's.
	out.DealWinners = append([]string(nil), s.DealWinners...)
	out.DealHands = append([]map[string]HandTally(nil), s.DealHands...)
	return out
}

// probeScratch is the throwaway state the offer probes run against: one copy
// of the state per offer scan, put back the way it was before each probe
// instead of being made afresh.
//
// An offer list is built by putting every candidate move through the real
// engine (see probe), and every one of those used to start with a full deep
// copy of the state — two, in fact, since ApplyAction copies what it is
// handed. A player's offers are thirty-odd probes, so a single broadcast was
// a few hundred copies of every hand, pile and meld on the table, and a
// profile of a loaded server had them as the largest single cost in it, with
// the garbage collector's share on top.
//
// The copy is laid out once, in a few flat arenas, with a pristine image of
// each kept beside it; putting it back is a handful of block copies and
// refilling six small maps. What a probe sees is exactly what cloneState
// would have given it: the same lengths, the same nil-ness, maps holding the
// same keys. Every slice is cut from its arena with its capacity equal to its
// length, so an append in a validator moves the slice somewhere new, as it
// would from a fresh copy, rather than writing into whichever region happens
// to lie next to it.
type probeScratch struct {
	// base is the state with every slice and map field already pointing at
	// the arenas and maps below.
	base GameState

	strs, strsImage     []string
	melds, meldsImage   [][]string
	metas, metasImage   []MeldInfo
	ints, intsImage     []int
	tallys, tallysImage []map[string]HandTally

	hands   []keyed[[]string]
	meldMap []keyed[[][]string]
	metaMap []keyed[[]MeldInfo]
	reqMet  []keyed[bool]
	scores  []keyed[[]int]
	totals  []keyed[int]

	handsM  map[string][]string
	meldsM  map[string][][]string
	metasM  map[string][]MeldInfo
	reqMetM map[string]bool
	scoresM map[string][]int
	totalsM map[string]int
}

type keyed[V any] struct {
	k string
	v V
}

// probeScratches keeps scratch copies between offer scans. A scan's copy is
// a few kilobytes and a loaded server makes thousands of scans a second, so
// reusing them is most of what keeps the probes off the garbage collector.
var probeScratches = sync.Pool{New: func() any { return &probeScratch{} }}

// newProbeScratch lays out the copy of s every probe will start from, in a
// scratch from the pool. Give it back with release.
func newProbeScratch(s GameState) *probeScratch {
	p := probeScratches.Get().(*probeScratch)
	p.load(s)
	return p
}

// release returns the scratch to the pool. Nothing it handed out may be used
// afterwards; the probes keep nothing of it but their verdicts.
func (p *probeScratch) release() {
	// The base state's shared fields would otherwise keep a finished match's
	// ruleset and snapshots reachable from the pool.
	p.base = GameState{}
	clear(p.tallys)
	clear(p.tallysImage)
	probeScratches.Put(p)
}

// arena returns buf emptied, with room for n without growing.
func arena[T any](buf []T, n int) []T {
	if cap(buf) < n {
		return make([]T, 0, n)
	}
	return buf[:0]
}

func (p *probeScratch) load(s GameState) {
	nStr, nMeld, nMeta, nInt := len(s.TurnOrder)+len(s.DrawPile)+len(s.DiscardPile)+len(s.DealWinners), 0, 0, 0
	for _, v := range s.Hands {
		nStr += len(v)
	}
	for _, melds := range s.Melds {
		nMeld += len(melds)
		for _, m := range melds {
			nStr += len(m)
		}
	}
	for _, metas := range s.MeldMeta {
		nMeta += len(metas)
	}
	for _, v := range s.GameScores {
		nInt += len(v)
	}
	// Room for everything up front, so no append below moves an arena out
	// from under the regions already cut from it.
	p.strs = arena(p.strs, nStr)
	p.melds = arena(p.melds, nMeld)
	p.metas = arena(p.metas, nMeta)
	p.ints = arena(p.ints, nInt)
	p.hands, p.meldMap, p.metaMap = p.hands[:0], p.meldMap[:0], p.metaMap[:0]
	p.reqMet, p.scores, p.totals = p.reqMet[:0], p.scores[:0], p.totals[:0]

	// cut appends src to the arena and returns its own region of it, or nil
	// for nothing, which is what append([]string(nil), src...) returns.
	cut := func(src []string) []string {
		if len(src) == 0 {
			return nil
		}
		at := len(p.strs)
		p.strs = append(p.strs, src...)
		return p.strs[at:len(p.strs):len(p.strs)]
	}

	out := s
	out.TurnOrder = cut(s.TurnOrder)
	out.DrawPile = cut(s.DrawPile)
	out.DiscardPile = cut(s.DiscardPile)
	out.DealWinners = cut(s.DealWinners)
	for k, v := range s.Hands {
		p.hands = append(p.hands, keyed[[]string]{k, cut(v)})
	}
	// A key whose meld list is empty is left out, as cloneState leaves it.
	for k, ms := range s.Melds {
		if len(ms) == 0 {
			continue
		}
		at := len(p.melds)
		for _, m := range ms {
			p.melds = append(p.melds, cut(m))
		}
		p.meldMap = append(p.meldMap, keyed[[][]string]{k, p.melds[at:len(p.melds):len(p.melds)]})
	}
	for k, ms := range s.MeldMeta {
		if len(ms) == 0 {
			continue
		}
		at := len(p.metas)
		p.metas = append(p.metas, ms...)
		p.metaMap = append(p.metaMap, keyed[[]MeldInfo]{k, p.metas[at:len(p.metas):len(p.metas)]})
	}
	for k, v := range s.RoundReqMet {
		p.reqMet = append(p.reqMet, keyed[bool]{k, v})
	}
	for k, v := range s.GameScores {
		var region []int
		if len(v) > 0 {
			at := len(p.ints)
			p.ints = append(p.ints, v...)
			region = p.ints[at:len(p.ints):len(p.ints)]
		}
		p.scores = append(p.scores, keyed[[]int]{k, region})
	}
	for k, v := range s.TotalScores {
		p.totals = append(p.totals, keyed[int]{k, v})
	}
	p.tallys = append(p.tallys[:0], s.DealHands...)
	if len(s.DealHands) == 0 {
		out.DealHands = nil
	} else {
		out.DealHands = p.tallys[:len(p.tallys):len(p.tallys)]
	}

	p.strsImage = append(p.strsImage[:0], p.strs...)
	p.meldsImage = append(p.meldsImage[:0], p.melds...)
	p.metasImage = append(p.metasImage[:0], p.metas...)
	p.intsImage = append(p.intsImage[:0], p.ints...)
	p.tallysImage = append(p.tallysImage[:0], p.tallys...)

	if p.handsM == nil {
		p.handsM = map[string][]string{}
		p.meldsM = map[string][][]string{}
		p.metasM = map[string][]MeldInfo{}
		p.reqMetM = map[string]bool{}
		p.scoresM = map[string][]int{}
		p.totalsM = map[string]int{}
	}
	out.Hands, out.Melds, out.MeldMeta = p.handsM, p.meldsM, p.metasM
	out.RoundReqMet, out.GameScores, out.TotalScores = p.reqMetM, p.scoresM, p.totalsM
	p.base = out
}

// reset puts the copy back as load laid it out, whatever the
// previous probe did to it, and returns it for applyOwned to write to.
func (p *probeScratch) reset() GameState {
	copy(p.strs, p.strsImage)
	copy(p.melds, p.meldsImage)
	copy(p.metas, p.metasImage)
	copy(p.ints, p.intsImage)
	copy(p.tallys, p.tallysImage)
	refill(p.handsM, p.hands)
	refill(p.meldsM, p.meldMap)
	refill(p.metasM, p.metaMap)
	refill(p.reqMetM, p.reqMet)
	refill(p.scoresM, p.scores)
	refill(p.totalsM, p.totals)
	return p.base
}

func refill[V any](m map[string]V, from []keyed[V]) {
	clear(m)
	for _, e := range from {
		m[e.k] = e.v
	}
}
