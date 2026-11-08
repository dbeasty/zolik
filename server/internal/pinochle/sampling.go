package pinochle

import (
	"math/rand"
	"sort"
	"strings"

	"zolik/server/internal/tricks"
	"zolik/server/internal/tricks/search"
)

// The hard bot's card play: deal the cards it cannot see in a way consistent
// with everything the table has shown, play each candidate out in every such
// deal, and keep the candidate that settled best on average.
//
// What it may read is exactly what its seat may: its own hand, the melds laid
// (those cards are still in their owners' hands until played), the cards
// played, what it passed or buried itself, the widow everybody saw. Inference
// from the play is the follow rules run backwards: a player who did not follow
// is void, one who did not trump is out of trumps, one who followed without
// beating holds nothing higher.

// knowledge is what the table has shown about the cards a seat cannot see.
type knowledge struct {
	pool   []string         // unknown cards, to be dealt
	known  map[int][]string // cards known to be in a seat's hand
	need   map[int]int      // unknown cards each seat still holds
	void   map[int]map[byte]bool
	cap    map[int]map[byte]byte // highest rank a seat may still hold in a suit
	widow  map[string]int        // cards that are the bidder's or buried
	bidder int
	buried int // unknown buried slots (another seat's bury)
}

// buriedSeat is the pseudo-seat the sampler deals the unseen bury to.
const buriedSeat = -1

func (s *GameState) playedBy() map[int]counts {
	out := map[int]counts{}
	for i := range s.Players {
		out[i] = counts{}
	}
	for _, tr := range s.History {
		for _, pl := range tr {
			out[pl.Seat][pl.Card]++
		}
	}
	for _, pl := range s.Trick {
		out[pl.Seat][pl.Card]++
	}
	return out
}

func (s *GameState) infer(me string) knowledge {
	t := s.table()
	mi := s.seat(me)
	k := knowledge{known: map[int][]string{}, need: map[int]int{}, void: map[int]map[byte]bool{}, cap: map[int]map[byte]byte{}, widow: map[string]int{}, bidder: -2}
	if s.Bidder != "" {
		k.bidder = s.seat(s.Bidder)
	}
	unseen := countOf(t.buildDeck())
	sub := func(c counts, cards []string) {
		for _, x := range cards {
			if c[x] > 0 {
				c[x]--
			}
		}
	}
	sub(unseen, s.Hands[me])
	for _, tr := range s.History {
		for _, pl := range tr {
			sub(unseen, []string{pl.Card})
		}
	}
	for _, pl := range s.Trick {
		sub(unseen, []string{pl.Card})
	}
	if me == s.Bidder {
		sub(unseen, s.Buried)
	} else if len(s.Buried) > 0 {
		k.buried = len(s.Buried)
	}

	// Cards each other seat is known to hold: its meld still unplayed, and
	// anything this seat passed it.
	played := s.playedBy()
	for i, p := range s.Players {
		if i == mi {
			continue
		}
		knownC := counts{}
		if m, ok := s.Melds[p]; ok {
			for _, c := range m.MeldCards() {
				knownC[c]++
			}
		}
		if u := s.Used[p]; u != nil {
			for c, n := range countOf(u.shown()) {
				knownC[c] = max(knownC[c], n)
			}
		}
		if s.partner(me) == p {
			for c, n := range countOf(s.Given[me]) {
				knownC[c] = max(knownC[c], n)
			}
		}
		var list []string
		for _, c := range sortedKeys(knownC) {
			n := knownC[c] - played[i][c]
			n = min(n, unseen[c])
			for j := 0; j < n; j++ {
				list = append(list, c)
			}
			if n > 0 {
				unseen[c] -= n
			}
		}
		if len(list) > len(s.Hands[p]) {
			list = list[:len(s.Hands[p])]
		}
		k.known[i] = list
		k.need[i] = len(s.Hands[p]) - len(list)
	}
	if me != s.Bidder && len(s.Widow) > 0 {
		for c, n := range countOf(s.Widow) {
			k.widow[c] = n
		}
	}
	for _, c := range sortedKeys(unseen) {
		for j := 0; j < unseen[c]; j++ {
			k.pool = append(k.pool, c)
		}
	}

	// The play, run backwards.
	pol, _ := s.policy()
	o := t.order()
	trump := s.trump()
	tr := append(append([][]tricks.Play(nil), s.History...), s.Trick)
	for ti, plays := range tr {
		if s.Variation == VarTwoHanded && ti < len(s.History) && !s.lateTrick(ti) {
			continue
		}
		if s.Variation == VarTwoHanded && ti == len(s.History) && !s.Late {
			continue
		}
		if pol != tricks.FollowBeatTrump {
			continue
		}
		for j := 1; j < len(plays); j++ {
			pl := plays[j]
			if pl.Seat == mi {
				continue
			}
			before := tricks.Trick{Plays: plays[:j]}
			led := before.Led()
			best, _ := before.Winning(trump, o)
			su, rk := tricks.Suit(pl.Card), tricks.Rank(pl.Card)
			mark := func(su byte) {
				if k.void[pl.Seat] == nil {
					k.void[pl.Seat] = map[byte]bool{}
				}
				k.void[pl.Seat][su] = true
			}
			capAt := func(su, r byte) {
				if k.cap[pl.Seat] == nil {
					k.cap[pl.Seat] = map[byte]byte{}
				}
				if old, ok := k.cap[pl.Seat][su]; !ok || o.Beats(old, r) {
					k.cap[pl.Seat][su] = r
				}
			}
			switch {
			case su != led:
				mark(led)
				if su != trump && trump != 0 {
					mark(trump)
				} else if su == trump && tricks.Suit(best.Card) == trump && !o.Beats(rk, tricks.Rank(best.Card)) {
					capAt(trump, tricks.Rank(best.Card))
				}
			case tricks.Suit(best.Card) == led && !o.Beats(rk, tricks.Rank(best.Card)):
				capAt(led, tricks.Rank(best.Card))
			}
		}
	}
	return k
}

// lateTrick reports whether two-handed trick i was played after the stock ran
// out: the first twelve are the stock's.
func (s *GameState) lateTrick(i int) bool {
	return i >= (len(s.table().buildDeck())-len(s.Players)*s.table().handSize)/2
}

func sortedKeys(c counts) []string {
	out := make([]string, 0, len(c))
	for k := range c {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// allows is whether seat may be dealt card under k.
func (k knowledge) allows(seat int, card string, o tricks.Order, strict bool) bool {
	if seat == buriedSeat {
		return true
	}
	if k.void[seat][tricks.Suit(card)] {
		return false
	}
	if strict {
		if r, ok := k.cap[seat][tricks.Suit(card)]; ok && o.Beats(tricks.Rank(card), r) {
			return false
		}
	}
	return true
}

// sample deals the unknown cards; ok is false if no consistent deal was found.
// The deal has one hand per seat and, after them, the buried cards: the real
// ones for the bidder who buried them, imagined ones for anybody else.
func (s *GameState) sample(me string, k knowledge, r *rand.Rand) ([][]string, bool) {
	o := s.table().order()
	mi := s.seat(me)
	for attempt := 0; attempt < 40; attempt++ {
		strict := attempt < 30
		hands := make([][]string, len(s.Players)+1)
		if me == s.Bidder {
			hands[len(s.Players)] = append([]string(nil), s.Buried...)
		}
		room := map[int]int{}
		for i := range s.Players {
			if i == mi {
				hands[i] = append([]string(nil), s.Hands[me]...)
				continue
			}
			hands[i] = append([]string(nil), k.known[i]...)
			room[i] = k.need[i]
		}
		if k.buried > 0 {
			room[buriedSeat] = k.buried
		}
		pool := append([]string(nil), k.pool...)
		r.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
		widow := map[string]int{}
		for c, n := range k.widow {
			widow[c] = n
		}
		seats := make([]int, 0, len(room))
		for i := range s.Players {
			if i != mi {
				seats = append(seats, i)
			}
		}
		if k.buried > 0 {
			seats = append(seats, buriedSeat)
		}
		eligible := func(c string) []int {
			var out []int
			isWidow := widow[c] > 0
			for _, st := range seats {
				if room[st] == 0 {
					continue
				}
				if isWidow && st != k.bidder && st != buriedSeat {
					continue
				}
				if k.allows(st, c, o, strict) {
					out = append(out, st)
				}
			}
			return out
		}
		// The most constrained cards first.
		sort.SliceStable(pool, func(i, j int) bool { return len(eligible(pool[i])) < len(eligible(pool[j])) })
		ok := true
		for _, c := range pool {
			el := eligible(c)
			if len(el) == 0 {
				ok = false
				break
			}
			st := el[r.Intn(len(el))]
			room[st]--
			if widow[c] > 0 {
				widow[c]--
			}
			if st == buriedSeat {
				hands[len(s.Players)] = append(hands[len(s.Players)], c)
			} else {
				hands[st] = append(hands[st], c)
			}
		}
		if ok {
			return hands, true
		}
	}
	return nil, false
}

// hardCard is the legal card with the best average over sampled deals.
func (s *GameState) hardCard(me string, legal []string, r *rand.Rand) string {
	if s.Variation == VarTwoHanded && !s.Late {
		return mediumCard(s.viewFor(me), legal)
	}
	k := s.infer(me)
	worlds := make([][][]string, 0, hardSamples)
	for i := 0; i < hardSamples; i++ {
		w, ok := s.sample(me, k, r)
		if !ok {
			break
		}
		worlds = append(worlds, w)
		if s.Variation == VarTwoHanded {
			break // the stock is gone: the other hand is exactly the unseen cards
		}
	}
	if len(worlds) == 0 {
		return mediumCard(s.viewFor(me), legal)
	}
	left := 0
	for _, h := range worlds[0][:len(s.Players)] {
		left += len(h)
	}
	if left <= endgameCards(s) {
		if c, ok := s.solveCard(me, legal, worlds); ok {
			return c
		}
	}
	scores := map[string]int{}
	for _, w := range worlds {
		for _, c := range legal {
			scores[c] += s.rollout(me, c, w)
		}
	}
	best, bestV := legal[0], -1<<31
	for _, c := range legal {
		if scores[c] > bestV {
			best, bestV = c, scores[c]
		}
	}
	return best
}

// endgameCards is how few cards must be left in hands for an exact solve.
func endgameCards(s *GameState) int {
	switch s.Variation {
	case VarTwoHanded:
		return 24
	case VarCutthroat:
		return 18
	}
	return 20
}

// rollout plays card c in world w, then the rest of the hand with medium in
// every seat, and returns how the hand settles for me: my side's change less
// the best other side's.
func (s *GameState) rollout(me, c string, w [][]string) int {
	t := s.table()
	n := len(s.Players)
	mi := s.seat(me)
	hands := make([][]string, n)
	for i := 0; i < n; i++ {
		hands[i] = append([]string(nil), w[i]...)
	}
	trick := append([]tricks.Play(nil), s.Trick...)
	played := counts{}
	for _, tr := range s.History {
		for _, pl := range tr {
			played[pl.Card]++
		}
	}
	for _, pl := range trick {
		played[pl.Card]++
	}
	taken, won, ctr := copyMap(s.Taken), copyMap(s.Tricks), copyMap(s.Counters)
	deck := countOf(t.buildDeck())
	trump := s.trump()
	bidSide := ""
	if s.Bidder != "" {
		bidSide = s.side(s.Bidder)
	}
	seat := mi
	card := c
	for {
		hands[seat] = removeCards(hands[seat], []string{card})
		trick = append(trick, tricks.Play{Seat: seat, Card: card})
		played[card]++
		if len(trick) == n {
			best, _ := tricks.Trick{Plays: trick}.Winning(trump, t.order())
			side := sideOf(best.Seat, t.partners)
			for _, pl := range trick {
				taken[side] += cardPoints(pl.Card, s.Scoring)
				if isCounter(pl.Card, s.Scoring) {
					ctr[side]++
				}
			}
			won[side]++
			trick = nil
			seat = best.Seat
			if len(hands[seat]) == 0 {
				taken[side] += t.lastTrick
				break
			}
		} else {
			seat = (seat + 1) % n
		}
		v := seatView{t: t, trump: trump, scoring: s.Scoring, n: n, me: seat, hand: hands[seat], trick: trick, partners: t.partners, bidSide: bidSide}
		v.unseen = counts{}
		mine := countOf(hands[seat])
		for x, k := range deck {
			if u := k - played[x] - mine[x]; u > 0 {
				v.unseen[x] = u
			}
		}
		legal := uniq(tricks.Legal(hands[seat], tricks.Trick{Plays: trick}, trump, t.order(), tricks.FollowBeatTrump))
		card = mediumCard(v, legal)
	}
	for _, b := range w[n] {
		taken[bidSide] += cardPoints(b, s.Scoring)
	}
	_, delta, _ := s.handDeltas(taken, won, ctr, false)
	return outcome(delta, sideOf(mi, t.partners))
}

// outcome is my side's change less the best of everyone else's.
func outcome(delta map[string]int, mine string) int {
	best := -1 << 31
	for side, d := range delta {
		if side != mine {
			best = max(best, d)
		}
	}
	return delta[mine] - best
}

func copyMap(m map[string]int) map[string]int {
	out := make(map[string]int, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// renameApart gives every copy of a card after the first a code of its own —
// "QS", then "QS'", "QS”" — so the solver, which knows a card by its code,
// sees two queens of spades as two cards. Rank and suit are still the first
// two characters, which is all the solver reads; c[:2] names a card back.
func renameApart(hands [][]string, trick []tricks.Play) ([][]string, []tricks.Play) {
	seen := counts{}
	rename := func(c string) string {
		k := seen[c]
		seen[c]++
		return c + strings.Repeat("'", k)
	}
	var outTrick []tricks.Play
	for _, pl := range trick {
		outTrick = append(outTrick, tricks.Play{Seat: pl.Seat, Card: rename(pl.Card)})
	}
	outHands := make([][]string, len(hands))
	for i, h := range hands {
		for _, c := range h {
			outHands[i] = append(outHands[i], rename(c))
		}
	}
	return outHands, outTrick
}

// solveCard averages exact solves of the sampled deals. Duplicate cards are
// renamed apart first — the solver knows a card by its code, and the two
// queens of spades are two cards — and named back afterwards.
func (s *GameState) solveCard(me string, legal []string, worlds [][][]string) (string, bool) {
	t := s.table()
	mi := s.seat(me)
	mine := sideOf(mi, t.partners)
	total := map[string]int{}
	solved := 0
	for _, w := range worlds {
		p := search.Problem{
			Trump: s.trump(), Order: t.order(), Policy: tricks.FollowBeatTrump,
			Leader: mi, NodeLimit: hardNodeLimit,
		}
		if len(s.Trick) > 0 {
			p.Leader = s.Trick[0].Seat
		}
		p.Hands, p.Trick = renameApart(w[:len(s.Players)], s.Trick)
		for i := range s.Players {
			p.Maximizer = append(p.Maximizer, sideOf(i, t.partners) == mine)
		}
		scoring, last := s.Scoring, t.lastTrick
		p.TrickValue = func(plays []tricks.Play, winner, left int) (int, bool) {
			if sideOf(winner, t.partners) != mine {
				return 0, false
			}
			v := 0
			for _, pl := range plays {
				v += cardPoints(pl.Card[:2], scoring)
			}
			if left == 0 {
				v += last
			}
			return v, false
		}
		p.CardClass = func(c string) int { return cardPoints(c[:2], scoring) }
		res := search.Solve(p)
		if res.Exhausted {
			continue
		}
		solved++
		got := map[string]int{}
		for c, v := range res.Values {
			base := c[:2]
			if old, ok := got[base]; !ok || v > old {
				got[base] = v
			}
		}
		for _, c := range legal {
			total[c] += got[c]
		}
	}
	if solved == 0 {
		return "", false
	}
	best, bestV := legal[0], -1<<31
	for _, c := range legal {
		if total[c] > bestV {
			best, bestV = c, total[c]
		}
	}
	return best, true
}
