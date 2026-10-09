package sedma

import (
	"math/rand"
	"sort"

	"zolik/server/internal/module"
	"zolik/server/internal/tricks"
)

// How a seat nobody is sitting at plays Sedma.
//
// Easy plays a legal card at random and lets a trick go more often than not.
// Medium plays the rules of thumb: take a trick with points in it, throw
// points on a trick its side is taking, keep the sevens, lead a rank it holds
// twice. Hard samples the cards it cannot see and plays each choice out to
// the end of the hand with medium in every seat, then takes the choice that
// did best — its own side's stakes first, then its points.
//
// It reads only what its seat may know: its own hand and the cards played.
// Legality is never its business: it chooses among what the offers allow.
type bot struct {
	// skill, if set, overrides the seat's (the bench's styles).
	skill module.Skill
}

var _ module.Bot = bot{}

// Sampling bounds for the hard seat.
const rollouts = 24

// choice is one thing a seat may do: play a card, or let the trick go.
type choice struct {
	card string
	stop bool
}

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

	var choices []choice
	for _, o := range offers {
		if !o.Enabled {
			continue
		}
		switch o.ID {
		case OfferPlay:
			if o.Source != nil {
				for _, c := range o.Source.Cards {
					choices = append(choices, choice{card: c})
				}
			}
		case OfferStop:
			choices = append(choices, choice{stop: true})
		}
	}
	if len(choices) == 0 {
		return module.ChooseAction(offers, nil)
	}

	var pick choice
	switch {
	case len(choices) == 1:
		pick = choices[0]
	case skill == module.SkillEasy:
		pick = easyChoice(s, choices, r)
	case skill == module.SkillHard:
		pick = s.searched(me, choices, r)
	default:
		pick = s.ruleOfThumb(me)
	}
	if pick.stop {
		return module.Action{OfferID: OfferStop, Verb: VerbStop}, true
	}
	id := OfferPlay
	return module.Action{OfferID: id, Verb: VerbPlay, Cards: []string{pick.card}}, true
}

func easyChoice(s *GameState, choices []choice, r *rand.Rand) choice {
	if s.Phase == phaseDecide && r.Intn(3) > 0 {
		return choice{stop: true}
	}
	return choices[r.Intn(len(choices))]
}

// worth is how much a card is worth keeping: a seven most, then aces and
// tens, then a card whose rank the hand holds again.
func worth(hand []string, c string) int {
	v := 0
	switch {
	case tricks.Rank(c) == '7':
		v += 30
	case cardPoints(c) > 0:
		v += 20
	}
	for _, o := range hand {
		if o != c && tricks.Rank(o) == tricks.Rank(c) {
			v += 4
		}
	}
	return v
}

// cheapest is the card from cards least worth keeping.
func cheapest(hand, cards []string) string {
	best := cards[0]
	for _, c := range cards[1:] {
		if worth(hand, c) < worth(hand, best) {
			best = c
		}
	}
	return best
}

// ruleOfThumb is medium's choice for the seat on turn.
func (s *GameState) ruleOfThumb(me string) choice {
	hand := s.Hands[me]
	if s.Phase == phaseDecide {
		w := s.Players[winning(s.Trick)]
		conts := s.continuers()
		if s.side(w) == s.side(me) || len(conts) == 0 || s.trickPoints() == 0 {
			return choice{stop: true}
		}
		return choice{card: cheapestCapture(hand, conts)}
	}
	if len(s.Trick) == 0 {
		return choice{card: s.lead(me)}
	}
	led := s.led()
	n := len(s.Players)
	last := len(s.Trick)%n == n-1
	w := s.Players[winning(s.Trick)]
	if s.side(w) == s.side(me) {
		// Ours: put points on it when nobody after us can take it back.
		if last {
			var sharp []string
			for _, c := range hand {
				if cardPoints(c) > 0 && tricks.Rank(c) != '7' {
					sharp = append(sharp, c)
				}
			}
			if len(sharp) > 0 {
				return choice{card: sharp[0]}
			}
		}
		return choice{card: junk(hand)}
	}
	var caps []string
	for _, c := range hand {
		if captures(c, led) {
			caps = append(caps, c)
		}
	}
	if len(caps) > 0 && (s.trickPoints() > 0 || last) {
		return choice{card: cheapestCapture(hand, caps)}
	}
	return choice{card: junk(hand)}
}

// cheapestCapture prefers the led rank to a seven, and a point card of the
// led rank to a blank one: it banks the points it plays.
func cheapestCapture(hand, caps []string) string {
	best := caps[0]
	score := func(c string) int {
		v := 0
		if tricks.Rank(c) == '7' {
			v += 50
		}
		if cardPoints(c) > 0 {
			v -= 5
		}
		return v + worth(hand, c)
	}
	for _, c := range caps[1:] {
		if score(c) < score(best) {
			best = c
		}
	}
	return best
}

// junk is the card to throw on a trick we are not taking: no points, no
// seven, if there is one.
func junk(hand []string) string { return cheapest(hand, hand) }

// lead is the card to open a trick with: a blank rank held twice, so it can
// be taken back, and never a seven or a point card while anything else is
// left.
func (s *GameState) lead(me string) string {
	hand := s.Hands[me]
	best, bestV := "", 0
	for _, c := range hand {
		v := 0
		switch {
		case tricks.Rank(c) == '7':
			v -= 40
		case cardPoints(c) > 0:
			v -= 20
		}
		for _, o := range hand {
			if o != c && tricks.Rank(o) == tricks.Rank(c) {
				v += 10
			}
			if o != c && tricks.Rank(o) == '7' {
				v += 3
			}
		}
		if best == "" || v > bestV {
			best, bestV = c, v
		}
	}
	return best
}

// --- the hard seat ----------------------------------------------------------

// clone is a deep enough copy to play a hand out on.
func (s *GameState) clone() *GameState {
	c := *s
	c.Hands = make(map[string][]string, len(s.Hands))
	for k, v := range s.Hands {
		c.Hands[k] = append([]string(nil), v...)
	}
	c.Stock = append([]string(nil), s.Stock...)
	c.Trick = append([]tricks.Play(nil), s.Trick...)
	c.LastTrick, c.History = nil, nil
	c.Points, c.Cards, c.Scores = copyMap(s.Points), copyMap(s.Cards), copyMap(s.Scores)
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

// world is s with the cards me cannot see dealt again: the other hands and
// the stock, shuffled together and dealt back out to the same sizes.
func (s *GameState) world(me string, r *rand.Rand) *GameState {
	w := s.clone()
	var pool []string
	for _, p := range s.Players {
		if p != me {
			pool = append(pool, w.Hands[p]...)
		}
	}
	pool = append(pool, w.Stock...)
	// Sorted first: the pool's order is where the hidden cards really are,
	// and a shuffle seeded the same must not start from that.
	sort.Strings(pool)
	r.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
	for _, p := range s.Players {
		if p != me {
			n := len(w.Hands[p])
			w.Hands[p], pool = pool[:n], pool[n:]
		}
	}
	w.Stock = pool
	return w
}

// apply plays one choice on a rollout state.
func (s *GameState) apply(c choice) {
	if c.stop {
		s.stop()
		return
	}
	s.play(s.Current, c.card)
}

// playOut finishes the hand with medium in every seat and returns its worth
// to me's side: stakes won or lost first, card points after.
func (s *GameState) playOut(me string) int {
	for steps := 0; len(s.Rounds) == 0 && steps < 400; steps++ {
		s.apply(s.ruleOfThumb(s.Current))
	}
	if len(s.Rounds) == 0 {
		return 0
	}
	h := s.Rounds[0]
	mine := s.side(me)
	v := 0
	for _, w := range h.Winners {
		if s.side(w) == mine {
			v = 100 * h.Stakes
			break
		}
	}
	if v == 0 && len(h.Winners) > 0 {
		v = -100 * h.Stakes
	}
	best := 0
	for _, side := range s.sides() {
		if side != mine && s.Points[side] > best {
			best = s.Points[side]
		}
	}
	return v + s.Points[mine] - best
}

// searched is the hard seat's choice: each one played out over the same
// sampled worlds.
func (s *GameState) searched(me string, choices []choice, r *rand.Rand) choice {
	totals := make([]int, len(choices))
	for i := 0; i < rollouts; i++ {
		w := s.world(me, r)
		for k, c := range choices {
			x := w.clone()
			x.apply(c)
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
