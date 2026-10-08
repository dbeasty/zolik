package lora

import (
	"math/rand"
	"sort"

	"zolik/server/internal/module"
	"zolik/server/internal/tricks"
)

// How a seat nobody is sitting at plays Lóra.
//
// Easy plays a legal card at random, and in desítky stops laying at random.
// Medium plays the rules of thumb: duck under the card taking the trick with
// the highest card that still loses, throw the dangerous cards when it cannot
// follow, lead low from short suits; in kvarty start the quart that lays the
// most of its own cards and keeps the lead; in desítky lay the cards that
// free its own and hold back the ones that would free somebody else's. A
// defender in a maturita feeds the maturant penalty cards.
//
// Hard re-deals the cards it cannot see — honouring every void the play has
// shown, the pool sorted first so the real deal cannot leak in — and plays
// each choice out to the end of the deal with medium in every seat, then
// takes the choice that did best for it. Both medium and hard choose their
// maturita game by playing each candidate out over sampled deals.
//
// It reads only what its seat may know: its own hand, the cards played and
// laid, and the voids shown. Legality is never its business: it chooses among
// what the offers allow.
type bot struct {
	// skill, if set, overrides the seat's (the bench's styles).
	skill module.Skill
}

var _ module.Bot = bot{}

// Sampling bounds.
const (
	hardSamples     = 24
	mediumMaturita  = 8
	hardMaturita    = 24
	rolloutStepsMax = 400
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
	r := rand.New(rand.NewSource(seat.Seed + int64(s.Deal)*1009 + int64(s.Tricks)*31 + int64(len(s.Trick)+len(s.Laid))*7))

	choices := choicesFrom(offers)
	if len(choices) == 0 {
		return module.ChooseAction(offers, nil)
	}
	switch {
	case len(choices) == 1:
		return choices[0], true
	case s.Phase == phaseChoose && skill == module.SkillEasy:
		return choices[r.Intn(len(choices))], true
	case s.Phase == phaseChoose && skill == module.SkillHard:
		return s.chooseGame(me, hardMaturita, r), true
	case s.Phase == phaseChoose:
		return s.chooseGame(me, mediumMaturita, r), true
	case skill == module.SkillEasy:
		return easyChoice(choices, r), true
	case skill == module.SkillHard:
		return s.searched(me, choices, r), true
	}
	return s.medium(me), true
}

// choicesFrom is every action the offers allow: each card of the play
// offer, each game of the choose offer, and the bare verbs.
func choicesFrom(offers []module.ActionOffer) []module.Action {
	var out []module.Action
	for _, o := range offers {
		if !o.Enabled {
			continue
		}
		switch {
		case o.ID == OfferPlay && o.Source != nil:
			for _, c := range o.Source.Cards {
				out = append(out, module.Action{OfferID: o.ID, Verb: o.Verb, Cards: []string{c}})
			}
		case o.ID == OfferChoose:
			for _, p := range o.Params {
				for _, c := range p.Choices {
					out = append(out, module.Action{OfferID: o.ID, Verb: o.Verb, Params: map[string]string{p.Name: c.Value}})
				}
			}
		case o.ID == OfferDone || o.ID == OfferTuk:
			out = append(out, module.Action{OfferID: o.ID, Verb: o.Verb})
		}
	}
	return out
}

// easyChoice is a legal card at random, and in desítky an even chance of
// stopping once something is laid.
func easyChoice(choices []module.Action, r *rand.Rand) module.Action {
	for _, c := range choices {
		if c.Verb == VerbDone && r.Intn(2) == 0 {
			return c
		}
	}
	var cards []module.Action
	for _, c := range choices {
		if c.Verb == VerbPlay {
			cards = append(cards, c)
		}
	}
	if len(cards) == 0 {
		return choices[0]
	}
	return cards[r.Intn(len(cards))]
}

func playCard(c string) module.Action {
	return module.Action{OfferID: OfferPlay, Verb: VerbPlay, Cards: []string{c}}
}

// medium is the rules of thumb for the seat on turn, outside the choose
// phase.
func (s *GameState) medium(p string) module.Action {
	switch s.Phase {
	case phaseQuart:
		return playCard(s.quartCard(p))
	case phaseTens:
		return s.tensMove(p)
	}
	return playCard(s.trickCard(p))
}

// --- the trick games --------------------------------------------------------

// penaltyOf is what card costs whoever takes it in this game, by itself.
func (s *GameState) penaltyOf(c string) int {
	switch s.Contract {
	case gameCervene:
		if tricks.Suit(c) == red {
			return 1
		}
	case gameFilky:
		if tricks.Rank(c) == 'Q' {
			return 2
		}
	case gameKral:
		if c == redKing {
			return 8
		}
	}
	return 0
}

// danger is how much a card wants to be got rid of: its penalty first, then
// its height.
func (s *GameState) danger(c string) int { return s.penaltyOf(c)*20 + rankIndex(c) }

func suitCount(hand []string, suit byte) int {
	n := 0
	for _, c := range hand {
		if tricks.Suit(c) == suit {
			n++
		}
	}
	return n
}

// trickCard is medium's card in a trick game.
func (s *GameState) trickCard(p string) string {
	legal := s.legal(p)
	if len(legal) == 1 {
		return legal[0]
	}
	hand := s.Hands[p]
	m := s.maturant()
	defending := m != "" && m != p

	if len(s.Trick) == 0 {
		// Lead low, from a short suit, nothing dangerous.
		best, bestV := legal[0], 1<<30
		for _, c := range legal {
			v := rankIndex(c)*4 + suitCount(hand, tricks.Suit(c)) + s.penaltyOf(c)*40
			if v < bestV {
				best, bestV = c, v
			}
		}
		return best
	}

	win, _ := tricks.Trick{Plays: s.Trick}.Winning(tricks.NoTrump, order)
	led := tricks.Suit(s.Trick[0].Card)
	maturantWins := defending && s.Players[win.Seat] == m

	if suitCount(hand, led) == 0 {
		// Void: throw what is most dangerous — or, feeding a maturant who
		// is taking the trick, the costliest card.
		return maxBy(legal, func(c string) int {
			if defending && !maturantWins && s.penaltyOf(c) > 0 {
				return rankIndex(c) - 100 // keep it for the maturant
			}
			return s.danger(c)
		})
	}
	var under []string
	for _, c := range legal {
		if order.Beats(tricks.Rank(win.Card), tricks.Rank(c)) {
			under = append(under, c)
		}
	}
	if len(under) > 0 {
		// The highest card that still loses, a penalty card before a blank.
		return maxBy(under, s.danger)
	}
	if len(s.Trick) == len(s.Players)-1 {
		return maxBy(legal, rankIndex) // taking it anyway: shed the top
	}
	return maxBy(legal, func(c string) int { return -rankIndex(c) })
}

func maxBy(cards []string, f func(string) int) string {
	best, bestV := cards[0], f(cards[0])
	for _, c := range cards[1:] {
		if v := f(c); v > bestV {
			best, bestV = c, v
		}
	}
	return best
}

// --- kvarty -----------------------------------------------------------------

// quartCard is medium's start for a quart: the one that lays the most of its
// own cards and leaves it on lead. It reads only its hand and what is laid:
// a card in neither is in somebody else's hand.
func (s *GameState) quartCard(p string) string {
	hand := s.Hands[p]
	laid := map[string]bool{}
	for _, c := range s.Laid {
		laid[c] = true
	}
	return maxBy(hand, func(c string) int {
		suit, from := tricks.Suit(c), rankIndex(c)
		top := min(from+3, len(ascending)-1)
		mine, lastMine := 0, false
		for k := from; k <= top; k++ {
			x := cardAt(k, suit)
			if laid[x] {
				break
			}
			lastMine = hasCard(hand, x)
			if lastMine {
				mine++
			}
		}
		v := mine * 10
		if lastMine {
			v += 6
		}
		if mine == len(hand) {
			v += 1000
		}
		return v - from // the lower start, all else equal
	})
}

// --- desítky ----------------------------------------------------------------

// tensMove is medium's move in desítky: lay the card that frees its own next
// card, or ends a row; once something is laid, stop rather than free a card
// for somebody else.
func (s *GameState) tensMove(p string) module.Action {
	legal := s.legal(p)
	if len(legal) == 0 {
		if s.Turned {
			return module.Action{OfferID: OfferDone, Verb: VerbDone}
		}
		return module.Action{OfferID: OfferTuk, Verb: VerbTuk}
	}
	hand := s.Hands[p]
	worth := func(c string) int {
		suit, i := tricks.Suit(c), rankIndex(c)
		if len(hand) == 1 {
			return 1000
		}
		if tricks.Rank(c) == 'T' {
			if _, _, open := s.row(suit); !open {
				return suitCount(hand, suit)*2 - 3
			}
		}
		lo, hi, _ := s.row(suit)
		next := -1
		switch {
		case i == lo-1:
			next = i - 1
		case i == hi+1:
			next = i + 1
		}
		switch {
		case next < 0 || next >= len(ascending):
			return 3 // the end of the row frees nothing
		case hasCard(hand, cardAt(next, suit)):
			return 4
		}
		return -1
	}
	best := maxBy(legal, worth)
	if s.Turned && worth(best) <= 0 {
		return module.Action{OfferID: OfferDone, Verb: VerbDone}
	}
	return playCard(best)
}

// --- sampling ---------------------------------------------------------------

// clone is a deep enough copy to play a deal out on.
func (s *GameState) clone() *GameState {
	c := *s
	c.Hands = make(map[string][]string, len(s.Hands))
	for k, v := range s.Hands {
		c.Hands[k] = append([]string(nil), v...)
	}
	c.Trick = append([]tricks.Play(nil), s.Trick...)
	c.LastTrick = nil
	c.Laid = append([]string(nil), s.Laid...)
	c.Quart = nil
	c.Won, c.Tuks, c.Points, c.Scores = copyMap(s.Won), copyMap(s.Tuks), copyMap(s.Points), copyMap(s.Scores)
	c.Voids = make(map[string]string, len(s.Voids))
	for k, v := range s.Voids {
		c.Voids[k] = v
	}
	c.Rounds = nil
	c.Pause = true
	return &c
}

func copyMap(m map[string]int) map[string]int {
	out := make(map[string]int, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// world is s with the cards me cannot see dealt again to the same hand
// sizes, each to a player not known to be void in its suit.
func (s *GameState) world(me string, r *rand.Rand) *GameState {
	w := s.clone()
	var pool []string
	var others []string
	for _, p := range s.Players {
		if p != me {
			pool = append(pool, w.Hands[p]...)
			others = append(others, p)
		}
	}
	// Sorted first: the pool's order is where the hidden cards really are,
	// and a shuffle seeded the same must not start from that.
	sort.Strings(pool)
	for try := 0; try < 30; try++ {
		r.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
		if hands, ok := dealRespecting(pool, others, s, try < 29); ok {
			for p, h := range hands {
				w.Hands[p] = h
			}
			return w
		}
	}
	return w // unreachable: the last try ignores voids
}

// dealRespecting deals pool out to the others' hand sizes, each card to a
// player who may hold its suit, most constrained card first.
func dealRespecting(pool, others []string, s *GameState, voids bool) (map[string][]string, bool) {
	need := map[string]int{}
	for _, p := range others {
		need[p] = len(s.Hands[p])
	}
	out := map[string][]string{}
	for _, c := range pool {
		placed := false
		for _, p := range others {
			if need[p] == 0 || (voids && indexOf(s.Voids[p], tricks.Suit(c)) >= 0) {
				continue
			}
			out[p] = append(out[p], c)
			need[p]--
			placed = true
			break
		}
		if !placed {
			return nil, false
		}
	}
	return out, true
}

// playOut finishes the deal with medium in every seat, and returns its
// worth to me: fewer points than the others' average, or in a maturita
// whether it held — the maturant's eight against the rest.
func (s *GameState) playOut(me string) float64 {
	for steps := 0; len(s.Rounds) == 0 && s.Status == "active" && steps < rolloutStepsMax; steps++ {
		if _, err := s.act(s.Current, s.medium(s.Current)); err != nil {
			return 0
		}
	}
	if len(s.Rounds) == 0 {
		return 0
	}
	d := s.Rounds[0]
	if d.Maturant != "" {
		switch {
		case d.Passed && me == d.Maturant:
			return 0
		case me == d.Maturant:
			return -maturitaPenalty
		case d.Passed:
			return 0
		}
		return maturitaPenalty / 3.0
	}
	others := 0
	for _, p := range s.Players {
		if p != me {
			others += d.Deltas[p]
		}
	}
	return float64(others)/3 - float64(d.Deltas[me])
}

// searched is the hard seat's choice: each one played out over the same
// sampled worlds.
func (s *GameState) searched(me string, choices []module.Action, r *rand.Rand) module.Action {
	totals := make([]float64, len(choices))
	for i := 0; i < hardSamples; i++ {
		w := s.world(me, r)
		for k, c := range choices {
			x := w.clone()
			if _, err := x.act(me, c); err != nil {
				totals[k] -= 1000
				continue
			}
			totals[k] += x.playOut(me)
		}
	}
	best := 0
	for k := range choices {
		if totals[k] > totals[best] {
			best = k
		}
	}
	return choices[best]
}

// chooseGame is the maturant's choice: the game it comes through clean most
// often over n sampled deals of the others' hands.
func (s *GameState) chooseGame(me string, n int, r *rand.Rand) module.Action {
	var games []string
	for _, g := range sequence {
		if s.choosable(g) {
			games = append(games, g)
		}
	}
	held := make([]int, len(games))
	for i := 0; i < n; i++ {
		w := s.world(me, r)
		for k, g := range games {
			x := w.clone()
			x.begin(g)
			if x.playOut(me) == 0 {
				held[k]++
			}
		}
	}
	best := 0
	for k := range games {
		if held[k] > held[best] {
			best = k
		}
	}
	return module.Action{OfferID: OfferChoose, Verb: VerbChoose, Params: map[string]string{ParamGame: games[best]}}
}
