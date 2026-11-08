package pinochle

import (
	"math/rand"
	"sort"
	"strconv"

	"zolik/server/internal/module"
	"zolik/server/internal/tricks"
)

// How a seat nobody is sitting at plays Pinochle.
//
// Easy bids on its meld alone, names its longest suit and plays a legal card
// at random half the time. Medium counts: it bids meld plus a share of the
// tricks its hand should take, cashes its boss cards, pulls trumps when its
// side has them, smears counters on a partner's sure trick and keeps them
// off an opponent's. Hard plays every candidate card out across sampled
// deals consistent with everything the table has shown — with medium in
// every seat, or exactly, with the solver, once few enough cards are left —
// and plays the card that settles best.
//
// It reads only what its seat may know: its own hand, the auction, the melds
// laid, the cards played, what it passed or buried itself. TestBotDoesNotPeek
// holds it to that. Legality is never its business: it chooses among what
// the offers allow.
type bot struct {
	// skill, if set, overrides the seat's (the bench's styles).
	skill module.Skill
}

var _ module.Bot = bot{}

// Hard's budget: deals sampled per candidate, and the solver's node limit
// per sampled deal.
const (
	hardSamples   = 10
	hardNodeLimit = 250_000
)

func (b bot) Act(raw module.State, seat module.BotSeat, offers []module.ActionOffer) (module.Action, bool) {
	s, err := decode(raw)
	if err != nil || s.Status != "active" || s.Intermission.Open || s.Current != seat.PlayerID {
		return module.ChooseAction(offers, []string{module.VerbContinue})
	}
	me := seat.PlayerID
	skill := seat.Skill
	if b.skill != "" {
		skill = b.skill
	}
	if skill == "" {
		skill = module.SkillMedium
	}
	r := rand.New(rand.NewSource(seat.Seed + int64(s.Deal)*1009 + int64(len(s.History))*31 + int64(len(s.Trick))*7 + int64(len(s.Phase))))

	enabled := map[string]module.ActionOffer{}
	for _, o := range offers {
		if o.Enabled {
			enabled[o.ID] = o
		}
	}
	act := func(id string, a module.Action) (module.Action, bool) {
		o, ok := enabled[id]
		if !ok {
			return module.ChooseAction(offers, nil)
		}
		a.OfferID, a.Verb = o.ID, o.Verb
		return a, true
	}

	switch s.Phase {
	case phaseAuction:
		if n, ok := s.botBid(me, skill, r); ok {
			return act(OfferBid, module.Action{Params: map[string]string{ParamAmount: strconv.Itoa(n)}})
		}
		return act(OfferPass, module.Action{})
	case phaseTrump:
		return act(OfferTrump, module.Action{Params: map[string]string{ParamSuit: s.botTrump(me, skill)}})
	case phaseBury:
		return act(OfferBury, module.Action{Cards: s.botBury(me)})
	case phasePassIn, phasePassBack:
		return act(OfferGive, module.Action{Cards: s.botGive(me)})
	case phaseMeld:
		if id := s.botMeld(me, skill, r); id != "" {
			return act(id, module.Action{})
		}
		return act(OfferDraw, module.Action{})
	case phasePlay:
		if _, ok := enabled[OfferThrowIn]; ok && skill != module.SkillEasy && s.hopeless() {
			return act(OfferThrowIn, module.Action{})
		}
		legal := s.legal(me)
		if len(legal) == 0 {
			return module.ChooseAction(offers, nil)
		}
		card := legal[0]
		if len(legal) > 1 {
			switch skill {
			case module.SkillEasy:
				if r.Intn(2) == 0 {
					card = legal[r.Intn(len(legal))]
				} else {
					card = mediumCard(s.viewFor(me), legal)
				}
			case module.SkillHard:
				card = s.hardCard(me, legal, r)
			default:
				card = mediumCard(s.viewFor(me), legal)
			}
		}
		return act(OfferPlay, module.Action{Cards: []string{card}})
	}
	return module.ChooseAction(offers, nil)
}

// --- bidding -------------------------------------------------------------------

// handValue is what a hand expects to score with su as trumps: its meld and
// its share of the trick points, the share leaning on aces and trump length.
func handValue(hand []string, su byte, t table, scoring int) int {
	v := BestMeld(hand, su, t).Total
	total := t.handTotal(scoring)
	share := total / t.seats
	if t.partners {
		share = total / 4
	}
	trumps, aces := 0, 0
	for _, c := range hand {
		if tricks.Suit(c) == su {
			trumps++
		}
		if tricks.Rank(c) == 'A' {
			aces++
		}
	}
	per := total / 50 // five on a single pack
	avgTrumps := t.handSize / 4
	share += per * 3 * (aces - t.copies)
	share += per * 4 * (trumps - avgTrumps)
	return v + max(0, share)
}

// estimate is the most this seat thinks its side can make as bidder.
func (s *GameState) estimate(me string) int {
	t := s.table()
	hand := s.Hands[me]
	best := 0
	for _, su := range []byte(suits) {
		best = max(best, handValue(hand, su, t, s.Scoring))
	}
	switch {
	case t.partners:
		// The partner's meld and tricks, unseen: a typical hand's.
		best += t.handTotal(s.Scoring) / 4
		if p := s.partner(me); s.LastBid[p] > 0 {
			best += t.handTotal(s.Scoring) / 10 // a partner who bid has meld
		}
	case t.widow > 0:
		best += t.handTotal(s.Scoring) / 10 // the widow and what is buried
	}
	return best
}

// botBid is the bid to make, or false to pass. A bot raises by the least the
// auction allows, and does not bid against its own partner without a hand
// well over the call.
func (s *GameState) botBid(me string, skill module.Skill, r *rand.Rand) (int, bool) {
	next := s.minNext()
	est := s.estimate(me)
	switch skill {
	case module.SkillEasy:
		est = BestMeld(s.Hands[me], s.botTrump(me, skill)[0], s.table()).Total + s.table().handTotal(s.Scoring)/3 + r.Intn(81) - 40
	case module.SkillHard:
		// A partner bidding is not competition: leave it to them unless
		// this hand is clearly the stronger.
		est -= s.table().handTotal(s.Scoring) / 25
	}
	if s.Bidder != "" && s.Bidder == s.partner(me) && est < next+s.table().handTotal(s.Scoring)/5 {
		return 0, false
	}
	// The last seat in with no bid made is stuck with it anyway at the
	// minimum: it may as well bid it.
	if s.Bid == 0 && s.AllPass == AllPassDealerStuck && s.othersPassed(me) {
		return next, true
	}
	if est < next {
		return 0, false
	}
	return next, true
}

func (s *GameState) othersPassed(me string) bool {
	for _, p := range s.Players {
		if p != me && !s.Passed[p] {
			return false
		}
	}
	return true
}

// botTrump is the suit to name: the most meld and length, aces and tens in
// it counting extra.
func (s *GameState) botTrump(me string, skill module.Skill) string {
	hand := s.Hands[me]
	if skill == module.SkillEasy {
		return bestTrumpFor(hand, s.table())
	}
	best, bestV := "H", -1
	for _, su := range []byte(suits) {
		v := BestMeld(hand, su, s.table()).Total
		for _, c := range hand {
			if tricks.Suit(c) != su {
				continue
			}
			v += 20
			if r := tricks.Rank(c); r == 'A' || r == 'T' {
				v += 10
			}
		}
		if v > bestV {
			best, bestV = string(su), v
		}
	}
	return best
}

// hopeless is a bidder looking at the meld on the table and a bid the tricks
// cannot make up: throwing in loses the bid and no more.
func (s *GameState) hopeless() bool {
	need := s.Bid - s.sideMeld(s.side(s.Bidder))
	return need > s.table().handTotal(s.Scoring)*17/20
}

// --- the widow and the pass -------------------------------------------------------

// keepScore ranks a card for keeping: trumps and aces highest, cards in the
// hand's meld next, then by points. Lower is first to go.
func (s *GameState) keepScore(hand []string, card string) int {
	su, rk := tricks.Suit(card), tricks.Rank(card)
	inSuit := 0
	for _, c := range hand {
		if tricks.Suit(c) == su {
			inSuit++
		}
	}
	v := 0
	switch {
	case su == s.trump():
		v += 1000
	case rk == 'A':
		v += 500
	}
	v += inSuit * 10
	v += strength(s.table().order(), rk)
	return v
}

func strength(o tricks.Order, r byte) int {
	for i := 0; i < len(o); i++ {
		if o[i] == r {
			return len(o) - i
		}
	}
	return 0
}

// botBury buries three: never a meld card, a trump or an ace while anything
// else will do, and counters from short suits first — buried counters still
// score for the bidder.
func (s *GameState) botBury(me string) []string {
	hand := s.Hands[me]
	n := s.table().widow
	melded := countOf(BestMeld(hand, s.trump(), s.table()).MeldCards())
	type cand struct {
		card  string
		score int
	}
	var cs []cand
	for _, c := range hand {
		sc := s.keepScore(hand, c) - cardPoints(c, s.Scoring)*3
		if melded[c] > 0 {
			melded[c]--
			sc += 2000
		}
		cs = append(cs, cand{c, sc})
	}
	sort.SliceStable(cs, func(i, j int) bool {
		if cs[i].score != cs[j].score {
			return cs[i].score < cs[j].score
		}
		return cs[i].card < cs[j].card
	})
	var out []string
	for i := 0; i < n && i < len(cs); i++ {
		out = append(out, cs[i].card)
	}
	return out
}

// botGive is the cards to pass: the partner sends its best trumps and aces
// to the bidder; the bidder sends back what it can spare outside its meld.
func (s *GameState) botGive(me string) []string {
	hand := s.Hands[me]
	n := s.PassCount
	type cand struct {
		card  string
		score int
	}
	var cs []cand
	if me != s.Bidder {
		for _, c := range hand {
			cs = append(cs, cand{c, -s.keepScore(hand, c)})
		}
	} else {
		melded := countOf(BestMeld(hand, s.trump(), s.table()).MeldCards())
		for _, c := range hand {
			sc := s.keepScore(hand, c)
			if melded[c] > 0 {
				melded[c]--
				sc += 2000
			}
			cs = append(cs, cand{c, sc})
		}
	}
	sort.SliceStable(cs, func(i, j int) bool {
		if cs[i].score != cs[j].score {
			return cs[i].score < cs[j].score
		}
		return cs[i].card < cs[j].card
	})
	var out []string
	for i := 0; i < n && i < len(cs); i++ {
		out = append(out, cs[i].card)
	}
	return out
}

// botMeld is the two-handed meld to lay: the one worth most, or none.
func (s *GameState) botMeld(me string, skill module.Skill, r *rand.Rand) string {
	best, bestV := "", 0
	if s.canExchange(me) {
		best, bestV = OfferExchange, pointsDix
	}
	if s.canDeclareDix(me) && bestV < pointsDix {
		best, bestV = OfferDix, pointsDix
	}
	for _, o := range twoHandOptions(s.Hands[me], s.Used[me], s.trump()) {
		if o.Points > bestV {
			best, bestV = o.ID, o.Points
		}
	}
	if skill == module.SkillEasy && r.Intn(3) == 0 {
		return ""
	}
	return best
}

// --- card play -------------------------------------------------------------------

// seatView is one seat's knowledge at the moment it plays: its hand, the
// trick, and every card it cannot place.
type seatView struct {
	t        table
	trump    byte
	scoring  int
	n        int
	me       int
	hand     []string
	trick    []tricks.Play
	unseen   counts
	partners bool
	bidSide  string
}

func (v seatView) side(seat int) string { return sideOf(seat, v.partners) }

// higherUnseen is how many cards of card's suit outrank it among the unseen.
func (v seatView) higherUnseen(card string) int {
	o := v.t.order()
	n := 0
	for c, k := range v.unseen {
		if tricks.Suit(c) == tricks.Suit(card) && o.Beats(tricks.Rank(c), tricks.Rank(card)) {
			n += k
		}
	}
	return n
}

// unseenOfSuit is how many cards of a suit are still out.
func (v seatView) unseenOfSuit(su byte) int {
	n := 0
	for c, k := range v.unseen {
		if tricks.Suit(c) == su {
			n += k
		}
	}
	return n
}

func (v seatView) winnerWith(card string) int {
	plays := append(append([]tricks.Play(nil), v.trick...), tricks.Play{Seat: v.me, Card: card})
	best, _ := tricks.Trick{Plays: plays}.Winning(v.trump, v.t.order())
	return best.Seat
}

// viewFor is a seat's own knowledge of the real table.
func (s *GameState) viewFor(me string) seatView {
	t := s.table()
	v := seatView{
		t: t, trump: s.trump(), scoring: s.Scoring, n: len(s.Players), me: s.seat(me),
		hand: s.Hands[me], trick: s.Trick, partners: t.partners,
	}
	if s.Bidder != "" {
		v.bidSide = s.side(s.Bidder)
	}
	v.unseen = countOf(t.buildDeck())
	sub := func(cards []string) {
		for _, c := range cards {
			if v.unseen[c] > 0 {
				v.unseen[c]--
			}
		}
	}
	sub(s.Hands[me])
	for _, tr := range s.History {
		for _, pl := range tr {
			sub([]string{pl.Card})
		}
	}
	for _, pl := range s.Trick {
		sub([]string{pl.Card})
	}
	if me == s.Bidder {
		sub(s.Buried)
	}
	return v
}

// mediumCard is the rules of thumb.
func mediumCard(v seatView, legal []string) string {
	if len(legal) == 1 {
		return legal[0]
	}
	o := v.t.order()
	pts := func(c string) int { return cardPoints(c, v.scoring) }
	rank := func(c string) int { return strength(o, tricks.Rank(c)) }
	pick := func(cards []string, better func(a, b string) bool) string {
		best := cards[0]
		for _, c := range cards[1:] {
			if better(c, best) {
				best = c
			}
		}
		return best
	}
	cheapest := func(a, b string) bool {
		if pts(a) != pts(b) {
			return pts(a) < pts(b)
		}
		ta, tb := tricks.Suit(a) == v.trump, tricks.Suit(b) == v.trump
		if ta != tb {
			return !ta
		}
		return rank(a) < rank(b)
	}

	if len(v.trick) == 0 {
		mine := v.side(v.me)
		var bossOff, trumps []string
		for _, c := range legal {
			switch {
			case tricks.Suit(c) == v.trump:
				trumps = append(trumps, c)
			case v.higherUnseen(c) == 0 && v.unseenOfSuit(tricks.Suit(c)) > 0:
				bossOff = append(bossOff, c)
			}
		}
		if len(bossOff) > 0 {
			return pick(bossOff, func(a, b string) bool { return pts(a) > pts(b) || pts(a) == pts(b) && rank(a) > rank(b) })
		}
		if mine == v.bidSide && len(trumps)*3 >= len(v.hand) && v.unseenOfSuit(v.trump) > 0 {
			return pick(trumps, func(a, b string) bool { return rank(a) > rank(b) })
		}
		var off []string
		for _, c := range legal {
			if tricks.Suit(c) != v.trump {
				off = append(off, c)
			}
		}
		if len(off) == 0 {
			off = legal
		}
		return pick(off, cheapest)
	}

	best, _ := tricks.Trick{Plays: v.trick}.Winning(v.trump, o)
	last := len(v.trick) == v.n-1
	partnerWinning := v.partners && v.side(best.Seat) == v.side(v.me)
	if partnerWinning {
		safe := last || v.higherUnseen(best.Card) == 0 && (tricks.Suit(best.Card) == v.trump || v.unseenOfSuit(tricks.Suit(best.Card)) >= v.n-1-len(v.trick))
		if safe {
			return pick(legal, func(a, b string) bool {
				if pts(a) != pts(b) {
					return pts(a) > pts(b)
				}
				return rank(a) < rank(b)
			})
		}
		return pick(legal, cheapest)
	}
	var winners []string
	for _, c := range legal {
		if v.winnerWith(c) == v.me {
			winners = append(winners, c)
		}
	}
	if len(winners) > 0 {
		if last {
			return pick(winners, func(a, b string) bool {
				if pts(a) != pts(b) {
					return pts(a) > pts(b)
				}
				return cheapest(a, b)
			})
		}
		var boss []string
		for _, c := range winners {
			if v.higherUnseen(c) == 0 {
				boss = append(boss, c)
			}
		}
		if len(boss) > 0 {
			return pick(boss, cheapest)
		}
		return pick(winners, cheapest)
	}
	return pick(legal, cheapest)
}
