package holdem

import (
	"math/rand"
	"sort"

	"zolik/server/internal/module"
)

// How a bot plays Five-Card Draw.
//
// Two decisions, and the second is the one Hold'em's bot already knows how to
// make. Which cards to throw is a textbook: keep anything already made, draw
// to a four-card flush or an open-ended straight, otherwise keep the pair or
// the two best cards. What to bet is the arithmetic of postflop(), against an
// equity measured by dealing the draw out: this seat's own draw as it would
// make it, and each opponent's from a hand dealt out of the cards this seat
// cannot see.
//
// The same two disciplines as bot.go hold. It reads its own five cards, its own
// discards and the table's public record, never another seat's hand or the
// deck's order — the rollout deals from a fresh deck minus what this seat has
// seen. And every move it makes is one of the offers it was handed.
//
// What the skill ladder changes is the second question again: how much the
// table's draws and bets are worth. Hard reads them — a seat that stood pat is
// dealt, in the rollout, mostly hands that would stand pat, one that drew a
// single card mostly hands that would draw one, and one that bets after the
// draw mostly hands that would bet — so a pat hand and a bet are respected,
// and a three-card draw is not. Medium and Easy deal every opponent anything.
// Easy also calls light and keeps a kicker to its pair, the beginner's draw.

// drawAct is one decision at a Five-Card Draw table.
func drawAct(s *GameState, seat *Seat, offers []module.ActionOffer, p profile, rnd *rand.Rand) (module.Action, bool) {
	if s.Street == streetDraw {
		discard := drawChoice(seat.Hole, p.skill == module.SkillEasy)
		for _, o := range offers {
			if !o.Enabled {
				continue
			}
			if len(discard) == 0 && o.Verb == VerbStand {
				return module.Action{OfferID: o.ID, Verb: VerbStand}, true
			}
			if len(discard) > 0 && o.Verb == VerbDiscard {
				return module.Action{OfferID: o.ID, Verb: VerbDiscard, Cards: discard}, true
			}
		}
		return module.ChooseAction(offers, []string{VerbStand})
	}
	mn := menuOf(offers)
	if len(mn.byVerb) == 0 {
		return module.ChooseAction(offers, nil)
	}
	return mn.action(drawBet(s, seat, mn, p, rnd))
}

// drawBet decides a betting round, before or after the draw.
func drawBet(s *GameState, seat *Seat, mn menu, p profile, rnd *rand.Rand) choice {
	opponents := opponentsOf(s)
	if opponents < 1 {
		return choice{verb: VerbCheck}
	}
	eq := drawEquity(s, seat, p, drawTrials(p), rnd)
	return betOnEquity(s, seat, mn, p, rnd, eq, opponents, s.Street == streetPostdraw)
}

// betOnEquity is a betting decision from a rolled-out equity: the arithmetic
// postflop() does for Hold'em, shared by the games whose hands it cannot read
// (Draw and Omaha). bluffSpot is the street where a heads-up bluff tells a
// story — after the draw, or on the turn and river.
func betOnEquity(s *GameState, seat *Seat, mn menu, p profile, rnd *rand.Rand, eq float64, opponents int, bluffSpot bool) choice {
	// Equity against the field, as a multiple of a fair share of it: 1 is an
	// average hand at this table, whatever its size. Thresholds on this rather
	// than on raw equity are what let one rule play heads-up and six-handed.
	edge := eq * float64(opponents+1)
	owed := s.toCall(seat)

	if owed == 0 {
		switch {
		case edge >= 1.5 && mn.can(VerbRaise):
			return choice{verb: VerbRaise, to: raiseTarget(s, mn, betOf(s, seat, 0.70))}
		case edge >= 1.25 && mn.can(VerbRaise) && rnd.Float64() < 0.5:
			return choice{verb: VerbRaise, to: raiseTarget(s, mn, betOf(s, seat, 0.50))}
		case bluffSpot && p.bluff > 0 && opponents == 1 && edge < 0.6 &&
			mn.can(VerbRaise) && rnd.Float64() < p.bluff:
			// A bluff, heads-up, rarely, where a story can be told with a bet
			// — "I drew one and made it", or the card that just came.
			return choice{verb: VerbRaise, to: raiseTarget(s, mn, betOf(s, seat, 0.60))}
		}
		return choice{verb: VerbCheck}
	}

	required := float64(owed) / float64(potNow(s)+owed)
	allIn := owed >= seat.Stack
	// Easy's leak is the station's: it pays to see one more card, and then to
	// see the showdown, on hands that are not worth the price.
	slack := 0.0
	if p.skill == module.SkillEasy {
		slack = 0.1
	}
	switch {
	case eq >= required+0.25 && edge >= 1.6 && mn.can(VerbRaise) && !allIn:
		return choice{verb: VerbRaise, to: raiseTarget(s, mn, betOf(s, seat, 0.75))}
	case allIn && eq >= required+0.05-slack:
		return choice{verb: VerbCall}
	case !allIn && eq >= required-slack:
		return choice{verb: VerbCall}
	}
	return choice{verb: VerbFold}
}

// drawReadTrust is how much of an opponent's range Hard deals consistent
// with what it has shown — its draw count, and a bet after the draw — and the
// rest is any hand at all. Half, because each is a claim like any bet: a seat
// that stands pat on nothing is telling exactly the story a reader who
// believed every count would fold to.
//
// Measured heads-up over 150 seeds with both seatings (300 matches of 15
// hands, fifty big blinds deep). The draw count alone was worth nothing
// against Medium at any trust, and believed outright it cost two big blinds a
// match against a calling station that stands pat on every hand. Reading a
// bet after the draw as at least two pair is what made Hard the stronger
// player: +5.6 big blinds a match against Medium, against +0.7 without it,
// and no worse against the station, which never bets.
const drawReadTrust = 0.5

// drawClaimFloor is what a bet after the draw is read as claiming. Two pair
// measured better than one: a pair is what most hands that drew are holding,
// so it filtered almost nothing.
const drawClaimFloor = twoPair

// drawTrials is how many deals a rollout averages over. Each trial scores at
// most seven five-card hands, so these are cheap next to Hold'em's.
func drawTrials(p profile) int {
	switch p.skill {
	case module.SkillHard:
		return 600
	case module.SkillEasy:
		return 150
	default:
		return 300
	}
}

// drawEquity is how often this seat's hand wins the pot, dealt out.
//
// Before the draw both this seat's draw and every opponent's are played by
// drawChoice; after it this seat's hand is what it is, and each opponent's is
// a hand dealt from the unseen cards and drawn the way drawChoice draws. Hard
// keeps an opponent's hand only if drawChoice would have drawn as many cards
// as that seat did — the count is public — which is the whole of reading a
// draw.
func drawEquity(s *GameState, seat *Seat, p profile, trials int, rnd *rand.Rand) float64 {
	var seen uint64
	for _, cs := range [][]string{seat.Hole, seat.Discarded} {
		for _, c := range cs {
			if r, su, ok := cardIndex(c); ok {
				seen |= 1 << (r*4 + su)
			}
		}
	}
	var poolBuf [52]code
	pool := poolBuf[:0]
	for _, c := range deckCodes {
		if seen&(1<<c) == 0 {
			pool = append(pool, c)
		}
	}

	type opp struct {
		drew  int
		reads bool
		// claims is a bet this street in front of this seat: the seat's
		// hand, dealt out, has to be one that would make it.
		claims bool
	}
	var opps []opp
	for i := range s.Seats {
		o := &s.Seats[i]
		if o == seat || !o.inHand() {
			continue
		}
		opps = append(opps, opp{
			drew:   o.Drew,
			reads:  p.skill == module.SkillHard && o.Drawn,
			claims: p.skill == module.SkillHard && s.Street == streetPostdraw && o.Bet > 0 && o.Bet == s.CurrentBet && o.Acted,
		})
	}
	if len(opps) == 0 {
		return 1
	}

	predraw := s.Street == streetPredraw
	mine := codesOf(seat.Hole)
	won := 0.0
	for t := 0; t < trials; t++ {
		d := dealer{pool: pool, rnd: rnd}
		var myHand [5]code
		copy(myHand[:], mine)
		if predraw {
			d.drawTo(&myHand, discardIndices(myHand, false))
		}
		best := score5(myHand)

		ahead, split := true, 1
		for _, o := range opps {
			var theirs [5]code
			read := o.reads && rnd.Float64() < drawReadTrust
			claim := o.claims && rnd.Float64() < drawReadTrust
			for tries := 0; ; tries++ {
				d.deal(theirs[:])
				used := d.used
				// A hand that has drawn — or, before the draw, will — draws
				// the way the textbook says. After the draw, a seat with no
				// read on it is still assumed to have drawn sensibly; that it
				// drew at all is all Medium knows.
				at := discardIndices(theirs, false)
				fits := !read || len(at) == o.drew
				d.drawTo(&theirs, at)
				if fits && claim && scoreCategory(score5(theirs)) < drawClaimFloor {
					fits = false
				}
				if fits || tries >= 24 {
					break
				}
				d.undeal(5 + d.used - used)
			}
			sc := score5(theirs)
			switch {
			case sc > best:
				ahead = false
			case sc == best:
				split++
			}
			if !ahead {
				break
			}
		}
		if ahead {
			won += 1 / float64(split)
		}
	}
	return won / float64(trials)
}

// dealer deals from the unseen cards without replacement, one trial at a
// time. It works on its own copy so trials never see each other's deals.
type dealer struct {
	pool []code
	used int
	rnd  *rand.Rand
	buf  [52]code
	init bool
}

func (d *dealer) next() code {
	if !d.init {
		n := copy(d.buf[:], d.pool)
		d.pool = d.buf[:n]
		d.init = true
	}
	k := d.used + d.rnd.Intn(len(d.pool)-d.used)
	d.pool[d.used], d.pool[k] = d.pool[k], d.pool[d.used]
	c := d.pool[d.used]
	d.used++
	return c
}

func (d *dealer) deal(into []code) {
	for i := range into {
		into[i] = d.next()
	}
}

// undeal puts the last n cards back. They stay in the undealt part of the
// pool, and the next deal picks among all of it at random, so a rejected hand
// leaves no trace on the next one.
func (d *dealer) undeal(n int) { d.used -= n }

// drawTo replaces the cards at these positions with new ones.
func (d *dealer) drawTo(h *[5]code, at []int) {
	for _, i := range at {
		h[i] = d.next()
	}
}

func codesOf(cards []string) []code {
	out := make([]code, 0, len(cards))
	for _, c := range cards {
		out = append(out, codeOf(c))
	}
	return out
}

func score5(h [5]code) uint32 {
	var x hand
	for _, c := range h {
		x.add(c)
	}
	return x.score()
}

// drawChoice is which cards to throw away, by the textbook.
//
// beginner keeps a kicker beside a pair — draws two rather than three — which
// is the commonest draw a new player makes and a measurably worse one.
func drawChoice(hole []string, beginner bool) []string {
	if len(hole) != 5 {
		return nil
	}
	var h [5]code
	for i, c := range hole {
		h[i] = codeOf(c)
	}
	var out []string
	for _, i := range discardIndices(h, beginner) {
		out = append(out, hole[i])
	}
	return out
}

// discardIndices is drawChoice on codes, for the rollout. At most three.
func discardIndices(h [5]code, beginner bool) []int {
	rank := func(i int) int { return int(h[i] >> 2) }
	suit := func(i int) int { return int(h[i] & 3) }

	var count [13]int
	for i := range h {
		count[rank(i)]++
	}
	pairs, trips, quads := 0, 0, 0
	for _, n := range count {
		switch n {
		case 2:
			pairs++
		case 3:
			trips++
		case 4:
			quads++
		}
	}
	keepMade := func(min int) []int {
		var out []int
		for i := range h {
			if count[rank(i)] < min {
				out = append(out, i)
			}
		}
		return out
	}

	switch cat := scoreCategory(score5(h)); {
	case cat >= straight:
		return nil
	case quads == 1:
		// Four of a kind is a made hand; the fifth card does not matter.
		return nil
	case trips == 1:
		return keepMade(3)
	case pairs == 2:
		return keepMade(2)
	}

	// Four to a flush: throw the odd one.
	var suits [4][]int
	for i := range h {
		suits[suit(i)] = append(suits[suit(i)], i)
	}
	for _, idx := range suits {
		if len(idx) == 4 && pairs == 0 {
			for i := range h {
				if suit(i) != suit(idx[0]) {
					return []int{i}
				}
			}
		}
	}

	if pairs == 1 {
		out := keepMade(2)
		if beginner {
			// Keep the highest kicker too.
			sort.Slice(out, func(a, b int) bool { return rank(out[a]) < rank(out[b]) })
			out = out[:len(out)-1]
		}
		return out
	}

	// Four in a row, open at both ends: throw the fifth.
	if at, ok := openEnded(h); ok {
		return []int{at}
	}

	// Nothing: keep the two highest, draw three.
	order := []int{0, 1, 2, 3, 4}
	sort.Slice(order, func(a, b int) bool { return rank(order[a]) < rank(order[b]) })
	return order[:3]
}

// openEnded finds four cards of consecutive rank that two different cards
// would complete, and returns the index of the fifth. J-Q-K-A and A-2-3-4 are
// one-ended and do not qualify; 2-3-4-5, which an ace or a six makes, does.
func openEnded(h [5]code) (int, bool) {
	for skip := 0; skip < 5; skip++ {
		var ranks []int
		for i := range h {
			if i != skip {
				ranks = append(ranks, int(h[i]>>2))
			}
		}
		sort.Ints(ranks)
		run := true
		for j := 1; j < 4; j++ {
			if ranks[j] != ranks[j-1]+1 {
				run = false
				break
			}
		}
		// Ranks are 0 (deuce) to 12 (ace). 2-3-4-5 is open — an ace below,
		// a six above — so only the top end can close a run: J-Q-K-A has
		// nothing above it.
		if run && ranks[3] <= 11 {
			return skip, true
		}
	}
	return 0, false
}
