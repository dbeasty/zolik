package zolikmod

import (
	"sort"

	"zolik/server/internal/ai"
	"zolik/server/internal/rules"
)

// What a seat can work out about going down and about the other seats, for
// the encoder (the distance and opponent blocks of the state) and for the
// candidates (how a take or a discard moves the distance, whether a discard
// feeds somebody). Everything here reads the seat's own hand and the public
// view (ai.VisibleState, the ledger's pickups, the unseen counts) and nothing
// else, which TestLearnEncodeDoesNotPeek pins along with the rest of Encode.
//
// Distances are counted in cards: how many more natural cards the hand needs
// to hold before a contract part can be laid. They are bounded, greedy counts
// over run windows and set ranks — cheap enough to ask once per candidate —
// not a search, and never a statement about legality (the engine alone says
// what may be laid).

// distance is how far a hand is from each part of going down.
type distance struct {
	// clean is the fewest natural cards missing from any clean run window
	// (minRun naturals of one suit in sequence) whose missing cards are still
	// unseen somewhere; cleanLive is false when no window is completable at
	// all, and clean is then minRun. cleanOuts is the unseen copies of the
	// best window's missing cards.
	clean     int
	cleanLive bool
	cleanOuts int
	// runs and sets are the cards missing for the contract's run and set
	// counts, jokers filling gaps (at most one joker per natural). Zero when
	// the contract asks for none.
	runs, sets int
	// value is the natural value of the best disjoint melds the hand holds
	// now, and short what the point floor still wants beyond it.
	value, short int
}

// total is every card the contract still wants, the floor counted at ten
// points a card.
func (d distance) total(req rules.ContractRequirement) int {
	n := d.runs + d.sets + (d.short+9)/10
	if req.RequireCleanRun {
		n += d.clean
	}
	return n
}

// runSlots is a hand's naturals as run positions per suit: 1..14, the ace at
// both ends, copies counted.
type runSlots map[string]*[15]int

func slotsOf(hand []string) (runSlots, int) {
	out := runSlots{}
	jokers := 0
	for _, c := range hand {
		if rules.IsJoker(c) {
			jokers++
			continue
		}
		s := rules.CardSuit(c)
		if out[s] == nil {
			out[s] = &[15]int{}
		}
		if rules.IsAce(c) {
			out[s][1]++
			out[s][14]++
		} else if r := rules.CardRank(c); r >= 0 {
			out[s][r+2]++
		}
	}
	return out, jokers
}

func runSlotValue(r int) int {
	switch {
	case r == 1:
		return rules.AceRunLowValue
	case r == 14:
		return rules.AceMeldValue
	case r >= 10:
		return 10
	}
	return r
}

func minSizes(cfg rules.RulesConfig) (int, int) {
	minSet, minRun := cfg.MinSetSize, cfg.MinRunSize
	if minSet == 0 {
		minSet = 3
	}
	if minRun == 0 {
		minRun = 4
	}
	return minSet, minRun
}

// distanceOf measures a hand against the contract. unseen is the copies of
// each card this seat has not seen (unseenCounts), which is what says whether
// a missing card can still come.
func distanceOf(hand []string, cfg rules.RulesConfig, req rules.ContractRequirement, unseen *[numCardSlot]int) distance {
	minSet, minRun := minSizes(cfg)
	slots, jokers := slotsOf(hand)
	d := distance{clean: minRun}
	for _, s := range learnSuits {
		have := slots[s]
		for lo := 1; lo+minRun-1 <= 14; lo++ {
			if lo == 1 && lo+minRun-1 == 14 {
				continue
			}
			missing, outs, live := 0, 0, true
			for r := lo; r < lo+minRun; r++ {
				if have != nil && have[r] > 0 {
					continue
				}
				missing++
				n := unseen[cardSlot(runRankCard(r, s))]
				outs += n
				if n == 0 {
					live = false
				}
			}
			if !live {
				continue
			}
			if !d.cleanLive || missing < d.clean || (missing == d.clean && outs > d.cleanOuts) {
				d.clean, d.cleanOuts, d.cleanLive = missing, outs, true
			}
		}
	}
	if req.Runs > 0 {
		d.runs = runsMissing(slots, jokers, req.Runs, minRun)
	}
	if req.Sets > 0 {
		d.sets = setsMissing(hand, jokers, req.Sets, minSet)
	}
	d.value = layValue(hand, cfg)
	if floor := cfg.InitialMeldMinimum; floor > d.value {
		d.short = floor - d.value
	}
	return d
}

// runsMissing is the cards k runs still want: the best window first, its
// cards and jokers spent, then the next.
func runsMissing(slots runSlots, jokers, k, minRun int) int {
	used := map[string]*[15]int{}
	for s, v := range slots {
		c := *v
		used[s] = &c
	}
	total := 0
	for i := 0; i < k; i++ {
		best, bestSuit, bestLo, bestJ := minRun+1, "", 0, 0
		for _, s := range learnSuits {
			have := used[s]
			for lo := 1; lo+minRun-1 <= 14; lo++ {
				if lo == 1 && lo+minRun-1 == 14 {
					continue
				}
				gaps := 0
				for r := lo; r < lo+minRun; r++ {
					if have == nil || have[r] == 0 {
						gaps++
					}
				}
				naturals := minRun - gaps
				// Jokers fill gaps up to one per natural; a gap left is a
				// card to find.
				j := min(jokers, gaps, naturals)
				missing := gaps - j
				if missing < best || (missing == best && j < bestJ) {
					best, bestSuit, bestLo, bestJ = missing, s, lo, j
				}
			}
		}
		total += best
		jokers -= bestJ
		if have := used[bestSuit]; have != nil {
			for r := bestLo; r < bestLo+minRun; r++ {
				if have[r] > 0 {
					have[r]--
					// An ace is one card at both ends.
					if r == 1 {
						have[14]--
					} else if r == 14 {
						have[1]--
					}
				}
			}
		}
	}
	return total
}

// setsMissing is the cards k sets still want, each rank's distinct suits
// counted, jokers filling up to half.
func setsMissing(hand []string, jokers, k, minSet int) int {
	suits := map[byte]map[string]bool{}
	for _, c := range hand {
		if rules.IsJoker(c) {
			continue
		}
		if suits[c[0]] == nil {
			suits[c[0]] = map[string]bool{}
		}
		suits[c[0]][rules.CardSuit(c)] = true
	}
	var have []int
	for _, ss := range suits {
		have = append(have, len(ss))
	}
	sort.Sort(sort.Reverse(sort.IntSlice(have)))
	total := 0
	for i := 0; i < k; i++ {
		n := 0
		if i < len(have) {
			n = have[i]
		}
		gaps := max(0, minSet-n)
		j := min(jokers, gaps, n)
		jokers -= j
		total += gaps - j
	}
	return total
}

// layValue is the natural value of the melds a hand holds now, greedily:
// the dearest group first — a set of three or four suits (or two and a
// joker), a run of naturals at least minRun long — and no card used twice.
// An estimate of what the floor would count, not the engine's reckoning.
func layValue(hand []string, cfg rules.RulesConfig) int {
	minSet, minRun := minSizes(cfg)
	slots, jokers := slotsOf(hand)
	type group struct {
		value, jokers int
		cards         []string
	}
	var groups []group
	byRank := map[byte][]string{}
	for _, c := range hand {
		if !rules.IsJoker(c) {
			byRank[c[0]] = append(byRank[c[0]], c)
		}
	}
	for i := 0; i < len(rules.RankOrder); i++ {
		cards := byRank[rules.RankOrder[i]]
		if len(cards) == 0 {
			continue
		}
		seen := map[string]bool{}
		var distinct []string
		for _, c := range cards {
			if s := rules.CardSuit(c); !seen[s] {
				seen[s] = true
				distinct = append(distinct, c)
			}
		}
		v := rules.NaturalSetCardValue(distinct[0])
		switch {
		case len(distinct) >= minSet:
			groups = append(groups, group{value: v * len(distinct), cards: distinct})
		case len(distinct) == minSet-1 && len(distinct) > 0:
			groups = append(groups, group{value: v * minSet, jokers: 1, cards: distinct})
		}
	}
	for _, s := range learnSuits {
		have := slots[s]
		if have == nil {
			continue
		}
		for lo := 1; lo <= 14; {
			if have[lo] == 0 {
				lo++
				continue
			}
			hi := lo
			for hi+1 <= 14 && have[hi+1] > 0 && !(lo == 1 && hi+1 == 14) {
				hi++
			}
			if hi-lo+1 >= minRun {
				g := group{}
				for r := lo; r <= hi; r++ {
					g.value += runSlotValue(r)
					g.cards = append(g.cards, runRankCard(r, s))
				}
				groups = append(groups, g)
			}
			lo = hi + 1
		}
	}
	// Stable over a fixed build order (ranks, then suits), so equal values
	// always resolve alike: the encoding has to be a function of the hand.
	sort.SliceStable(groups, func(i, j int) bool { return groups[i].value > groups[j].value })
	left := map[string]int{}
	for _, c := range hand {
		left[c]++
	}
	total := 0
	for _, g := range groups {
		if g.jokers > jokers {
			continue
		}
		ok := true
		for _, c := range g.cards {
			ok = ok && left[c] > 0
		}
		if !ok {
			continue
		}
		for _, c := range g.cards {
			left[c]--
		}
		jokers -= g.jokers
		total += g.value
	}
	return total
}

// --- the other seats ---------------------------------------------------------------

// rival is what one other seat has shown it is collecting: the cards it was
// seen to take off the pile and still holds (the ledger), the cards it has on
// the table, and from both, the cards it would want.
type rival struct {
	takenSuit [numSuits]int
	takenRank [numRanks]int
	meldSuit  [numSuits]int
	meldRank  [numRanks]int
	// wants is, per card, how clearly this seat wants it: 1 where it would
	// lay off onto the seat's own melds or complete a pair of cards it took
	// (two of a rank, or two of a suit a rank or two apart); 0.5 where it
	// sits next to, or shares a rank with, a single card it took.
	wants [numCardSlot]float32
	taken []string
}

func suitIndex(c string) int {
	s := rules.CardSuit(c)
	for i, x := range learnSuits {
		if x == s {
			return i
		}
	}
	return -1
}

// rivalOf reads one seat off the public view.
func rivalOf(vis ai.VisibleState, id string, cfg rules.RulesConfig) *rival {
	r := &rival{}
	want := func(c string, v float32) {
		if k := cardSlot(c); k >= 0 && k != jokerSlot && r.wants[k] < v {
			r.wants[k] = v
		}
	}
	for _, c := range vis.KnownHeld[id] {
		if rules.IsJoker(c) {
			continue
		}
		r.taken = append(r.taken, c)
		if i := suitIndex(c); i >= 0 {
			r.takenSuit[i]++
		}
		if k := rules.CardRank(c); k >= 0 {
			r.takenRank[k]++
		}
	}
	for _, m := range vis.Melds[id] {
		for _, c := range m {
			if rules.IsJoker(c) {
				continue
			}
			if i := suitIndex(c); i >= 0 {
				r.meldSuit[i]++
			}
			if k := rules.CardRank(c); k >= 0 {
				r.meldRank[k]++
			}
		}
	}
	if len(vis.Melds[id]) > 0 {
		own := extenders(map[string][][]string{id: vis.Melds[id]}, cfg)
		for k, ok := range own {
			if ok && k != jokerSlot {
				r.wants[k] = 1
			}
		}
	}
	for i, c := range r.taken {
		rank, suit := rules.CardRank(c), rules.CardSuit(c)
		for _, s := range learnSuits {
			if s != suit {
				want(string(rules.RankOrder[rank])+s, 0.5)
			}
		}
		for _, adj := range neighbourRanks(rank, 2) {
			want(string(rules.RankOrder[adj])+suit, 0.5)
		}
		for _, o := range r.taken[i+1:] {
			or := rules.CardRank(o)
			switch {
			case or == rank && rules.CardSuit(o) != suit:
				for _, s := range learnSuits {
					if s != suit && s != rules.CardSuit(o) {
						want(string(rules.RankOrder[rank])+s, 1)
					}
				}
			case rules.CardSuit(o) == suit && or != rank && abs(or-rank) <= 2:
				lo, hi := min(rank, or), max(rank, or)
				for x := lo - 1; x <= hi+1; x++ {
					if x >= 0 && x < numRanks && x != rank && x != or {
						want(string(rules.RankOrder[x])+suit, 1)
					}
				}
			}
		}
	}
	return r
}

// neighbourRanks is the ranks within d run positions of this one, the ace
// both below the two and above the king (never both at once: no wrapping).
func neighbourRanks(rank, d int) []int {
	pos := []int{rank + 2}
	if rank == numRanks-1 {
		pos = append(pos, 1)
	}
	seen := map[int]bool{rank: true}
	var out []int
	for _, p := range pos {
		for x := p - d; x <= p+d; x++ {
			if x < 1 || x > 14 {
				continue
			}
			y := x - 2
			if x == 1 || x == 14 {
				y = numRanks - 1
			}
			if !seen[y] {
				seen[y] = true
				out = append(out, y)
			}
		}
	}
	return out
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// nearTaken counts the cards this seat took of the card's suit within two
// ranks, and of its rank.
func (r *rival) nearTaken(card string) (suitNear, sameRank int) {
	if rules.IsJoker(card) {
		return 0, 0
	}
	rank, suit := rules.CardRank(card), rules.CardSuit(card)
	for _, c := range r.taken {
		cr := rules.CardRank(c)
		if cr == rank {
			sameRank++
		} else if rules.CardSuit(c) == suit && abs(cr-rank) <= 2 {
			suitNear++
		}
	}
	return suitNear, sameRank
}
