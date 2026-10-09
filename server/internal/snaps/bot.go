package snaps

import (
	"math/rand"
	"sort"

	"zolik/server/internal/module"
	"zolik/server/internal/tricks"
)

// How a seat nobody is sitting at plays Šnaps.
//
// Easy plays a legal card at random, exchanges when it can and never
// closes. Medium plays the textbook rules of thumb: take an ace or ten led
// to you, keep trumps and marriages, lead low. Hard plays like medium while
// the talon is open, and solves the rest exactly once it is closed or used
// up (solve.go) — and decides whether to close by solving the closed deal
// over samples of the other hand.
//
// It reads only what its seat may know: its own hand, the cards played, the
// turned-up trump and the cards the table saw go into the other hand.
// Legality is never its business: it chooses among the cards the offer list
// already says are legal.
type bot struct {
	// skill, if set, overrides the seat's (the bench's styles).
	skill module.Skill
}

var _ module.Bot = bot{}

// Sampling bounds for the hard seat.
const (
	samples       = 24
	nodesPerSolve = 200_000
	// closeWinRate is how sure a closed deal must look before the hard seat
	// closes: the share of sampled deals it wins.
	closeWinRate = 0.8
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
	r := rand.New(rand.NewSource(seat.Seed + int64(s.Deal)*1009 + int64(len(s.History))*31 + int64(len(s.Trick))))
	enabled := func(id string) bool {
		for _, o := range offers {
			if o.ID == id && o.Enabled {
				return true
			}
		}
		return false
	}
	var legal []string
	for _, o := range offers {
		if o.ID == OfferPlay && o.Enabled && o.Source != nil {
			legal = o.Source.Cards
		}
	}
	if enabled(OfferExchange) {
		return module.Action{OfferID: OfferExchange, Verb: VerbExchange}, true
	}
	if len(legal) == 0 {
		return module.ChooseAction(offers, nil)
	}
	play := func(c string) (module.Action, bool) {
		return module.Action{OfferID: OfferPlay, Verb: VerbPlay, Cards: []string{c}}, true
	}

	if skill == module.SkillEasy {
		return play(legal[r.Intn(len(legal))])
	}
	if skill == module.SkillHard {
		if enabled(OfferClose) && s.shouldClose(me, r) {
			return module.Action{OfferID: OfferClose, Verb: VerbClose}, true
		}
		if s.strict() && len(legal) > 1 {
			if c, ok := s.solvedCard(me, legal, r); ok {
				return play(c)
			}
		}
	} else if enabled(OfferClose) && s.mediumCloses(me) {
		return module.Action{OfferID: OfferClose, Verb: VerbClose}, true
	}
	return play(s.ruleOfThumb(me, legal))
}

// isTrump reports whether c is a trump.
func (s *GameState) isTrump(c string) bool { return string(tricks.Suit(c)) == s.Trump }

// keep is how much a card is worth holding on to: trumps above everything,
// then marriage halves, then card points.
func (s *GameState) keep(me, c string) int {
	v := cardPoints(c)
	if s.isTrump(c) {
		v += 30
	}
	if o := partner(c); o != "" && hasCard(s.Hands[me], o) && s.marriagesAllowed() {
		v += 15
	}
	return v
}

// cheapest is the card from cards least worth keeping.
func (s *GameState) cheapest(me string, cards []string) string {
	best := cards[0]
	for _, c := range cards[1:] {
		if s.keep(me, c) < s.keep(me, best) {
			best = c
		}
	}
	return best
}

// beats reports whether c, played second, takes the trick from led.
func (s *GameState) beats(c, led string) bool {
	t := tricks.Trick{Plays: []tricks.Play{{Seat: 0, Card: led}, {Seat: 1, Card: c}}}
	w, _ := t.Winning(s.Trump[0], order)
	return w.Seat == 1
}

// ruleOfThumb is medium's card.
func (s *GameState) ruleOfThumb(me string, legal []string) string {
	if len(s.Trick) == 0 {
		return s.leadOfThumb(me, legal)
	}
	led := s.Trick[0].Card
	var winners []string
	for _, c := range legal {
		if s.beats(c, led) {
			winners = append(winners, c)
		}
	}
	if len(winners) == 0 {
		return s.cheapest(me, legal)
	}
	sort.Slice(winners, func(i, j int) bool { return s.keep(me, winners[i]) < s.keep(me, winners[j]) })
	cheapWin := winners[0]
	// An ace or ten led is worth taking; so is any trick that puts us out,
	// and in the strict phase every trick counts.
	if cardPoints(led) >= 10 || s.strict() || s.total(me)+cardPoints(led)+cardPoints(cheapWin) >= goal {
		return cheapWin
	}
	// A low card led: take it with an ace or ten of its suit, which banks
	// the points before a trump can catch them.
	for _, c := range winners {
		if !s.isTrump(c) && cardPoints(c) >= 10 {
			return c
		}
	}
	return s.cheapest(me, legal)
}

func (s *GameState) leadOfThumb(me string, legal []string) string {
	hand := s.Hands[me]
	// A marriage, trumps first: lead its svršek and bank the points.
	if s.marriagesAllowed() {
		var best string
		for _, c := range legal {
			if tricks.Rank(c) == 'Q' && hasCard(hand, partner(c)) {
				if best == "" || s.isTrump(c) {
					best = c
				}
			}
		}
		if best != "" {
			return best
		}
	}
	if s.strict() {
		// Cash the top cards: an ace, then a ten whose ace is gone.
		for _, c := range legal {
			if tricks.Rank(c) == 'A' {
				return c
			}
		}
		for _, c := range legal {
			if tricks.Rank(c) == 'T' && s.played("A"+string(tricks.Suit(c))) {
				return c
			}
		}
	}
	return s.cheapest(me, legal)
}

// played reports whether card has been played this deal.
func (s *GameState) played(card string) bool {
	for _, t := range s.History {
		for _, pl := range t {
			if pl.Card == card {
				return true
			}
		}
	}
	for _, pl := range s.Trick {
		if pl.Card == card {
			return true
		}
	}
	return false
}

// mediumCloses: close with most of the way to 66 banked and the top trumps
// in hand.
func (s *GameState) mediumCloses(me string) bool {
	trumpPoints := 0
	for _, c := range s.Hands[me] {
		if s.isTrump(c) {
			trumpPoints += cardPoints(c)
		}
	}
	return s.total(me) >= 45 && s.total(me)+trumpPoints >= goal+10 && hasCard(s.Hands[me], "A"+s.Trump)
}

// --- the hard seat ----------------------------------------------------------

// unseen is every card me cannot place: not in its hand, not played, not
// the turned-up trump, not known to be in the other hand.
func (s *GameState) unseen(me string) []string {
	opp := s.other(me)
	gone := map[string]bool{}
	for _, c := range s.Hands[me] {
		gone[c] = true
	}
	for _, t := range s.History {
		for _, pl := range t {
			gone[pl.Card] = true
		}
	}
	for _, pl := range s.Trick {
		gone[pl.Card] = true
	}
	if up := s.turnedUp(); up != "" {
		gone[up] = true
	}
	for _, c := range s.Known[opp] {
		gone[c] = true
	}
	var out []string
	for _, c := range buildDeck(s.rules().ranks) {
		if !gone[c] {
			out = append(out, c)
		}
	}
	return out
}

// forbidden is the cards the other player has shown they cannot hold: in
// the strict phase, a suit not followed, a trump not played when void, a
// higher card not played when one had to beat.
func (s *GameState) forbidden(me string) map[string]bool {
	opp := s.seat(s.other(me))
	out := map[string]bool{}
	if s.StrictFrom < 0 {
		return out
	}
	deck := buildDeck(s.rules().ranks)
	suit := func(x byte, f func(string) bool) {
		for _, c := range deck {
			if tricks.Suit(c) == x && f(c) {
				out[c] = true
			}
		}
	}
	all := func(string) bool { return true }
	check := func(t []tricks.Play) {
		if len(t) < 2 || t[1].Seat != opp {
			return
		}
		led, c := t[0].Card, t[1].Card
		switch {
		case tricks.Suit(c) != tricks.Suit(led):
			suit(tricks.Suit(led), all)
			if !s.isTrump(c) {
				suit(s.Trump[0], all)
			}
		case !order.Beats(tricks.Rank(c), tricks.Rank(led)):
			suit(tricks.Suit(led), func(x string) bool { return order.Beats(tricks.Rank(x), tricks.Rank(led)) })
		}
	}
	for i, t := range s.History {
		if i >= s.StrictFrom {
			check(t)
		}
	}
	if s.strict() {
		check(s.Trick)
	}
	return out
}

// sampleHand deals the other hand from the unseen cards, consistent with
// what the table has shown; ok is false if no consistent deal was found.
func (s *GameState) sampleHand(me string, r *rand.Rand) ([]string, bool) {
	opp := s.other(me)
	need := len(s.Hands[opp]) - len(s.Known[opp])
	pool := s.unseen(me)
	forbid := s.forbidden(me)
	var allowed []string
	for _, c := range pool {
		if !forbid[c] {
			allowed = append(allowed, c)
		}
	}
	if need > len(allowed) {
		allowed = pool // the inference was wrong somewhere: drop it
	}
	if need > len(allowed) || need < 0 {
		return nil, false
	}
	r.Shuffle(len(allowed), func(i, j int) { allowed[i], allowed[j] = allowed[j], allowed[i] })
	return append(append([]string(nil), s.Known[opp]...), allowed[:need]...), true
}

// positionFor is the strict deal from here with the other hand as sampled.
func (s *GameState) positionFor(me string, oppHand []string, closeNow bool) *position {
	p := &position{
		r: s.rules(), trump: s.Trump[0], t: s.tally(), close: s.closing(),
		closed: s.ClosedBy != "" || closeNow, limit: nodesPerSolve,
	}
	mi := s.seat(me)
	p.hands[mi] = append([]string(nil), s.Hands[me]...)
	p.hands[1-mi] = oppHand
	if closeNow {
		o := s.other(me)
		p.close = closing{by: mi, oppTricks: s.TricksWon[o], oppPoints: s.total(o)}
	}
	p.marry = !p.r.strictMarriages
	if len(s.Trick) > 0 {
		p.leader, p.led = s.Trick[0].Seat, s.Trick[0].Card
	} else {
		p.leader = s.seat(s.Current)
	}
	return p
}

// solvedCard is the legal card with the best average over sampled deals.
func (s *GameState) solvedCard(me string, legal []string, r *rand.Rand) (string, bool) {
	n := samples
	if len(s.Talon) == 0 && s.ClosedBy == "" {
		n = 1 // used up: every unseen card is in the other hand
	}
	mi := s.seat(me)
	totals := map[string]int{}
	counted := 0
	for i := 0; i < n; i++ {
		opp, ok := s.sampleHand(me, r)
		if !ok {
			break
		}
		solvedAll := true
		vals := map[string]int{}
		for _, c := range legal {
			p := s.positionFor(me, opp, false)
			v := p.after(mi, c, -4, 4)
			if p.cutoff {
				solvedAll = false
				break
			}
			if mi == 1 {
				v = -v
			}
			vals[c] = v
		}
		if !solvedAll {
			continue
		}
		for c, v := range vals {
			totals[c] += v
		}
		counted++
	}
	if counted == 0 {
		return "", false
	}
	best, bestV := "", 0
	for _, c := range legal {
		v := totals[c]
		if best == "" || v > bestV || (v == bestV && s.keep(me, c) < s.keep(me, best)) {
			best, bestV = c, v
		}
	}
	return best, true
}

// shouldClose solves the deal as if closed now over sampled other hands,
// and closes if it wins often enough.
func (s *GameState) shouldClose(me string, r *rand.Rand) bool {
	mi := s.seat(me)
	won, tried := 0, 0
	for i := 0; i < samples; i++ {
		opp, ok := s.sampleHand(me, r)
		if !ok {
			break
		}
		p := s.positionFor(me, opp, true)
		_, v, ok := p.solve(mi)
		if !ok {
			continue
		}
		tried++
		if v > 0 {
			won++
		}
	}
	return tried > 0 && float64(won) >= closeWinRate*float64(tried)
}
