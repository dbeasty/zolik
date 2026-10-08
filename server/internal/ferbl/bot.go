package ferbl

import (
	"math/rand"
	"sort"

	"zolik/server/internal/module"
)

// How a seat nobody is sitting at plays Ferbl.
//
// Every skill reads the same thing: its chance of holding the best hand at
// the showdown, sampled from the cards it cannot see — the other live
// hands and the cards still to come. Easy calls almost anything and never
// raises. Medium bets and raises on a strong chance and folds when the
// price is more than the pot is worth to it. Hard samples more, weighs the
// pot odds more finely, and now and then bets a weak hand it has checked
// into on four cards.
//
// It reads only its own cards: the pool it samples from is every card it
// cannot see, sorted before it is shuffled. Legality is never its business.
type bot struct {
	// skill, if set, overrides the seat's (the bench's styles).
	skill module.Skill
}

var _ module.Bot = bot{}

func (b bot) Act(raw module.State, seat module.BotSeat, offers []module.ActionOffer) (module.Action, bool) {
	s, err := decode(raw)
	if err != nil || s.Status != "active" || s.Intermission.Open || s.Current != seat.PlayerID {
		return module.ChooseAction(offers, []string{module.VerbContinue})
	}
	skill := seat.Skill
	if b.skill != "" {
		skill = b.skill
	}
	if skill == "" {
		skill = module.SkillMedium
	}
	st := s.seat(seat.PlayerID)
	r := rand.New(rand.NewSource(seat.Seed + int64(s.Hand)*1009 + int64(len(st.Hand))*31 + int64(st.Total)))
	enabled := func(verb string) bool {
		for _, o := range offers {
			if o.Verb == verb && o.Enabled {
				return true
			}
		}
		return false
	}
	do := func(verb string) (module.Action, bool) {
		return module.Action{OfferID: verb, Verb: verb}, true
	}

	trials := 150
	if skill == module.SkillHard {
		trials = 500
	}
	eq := s.equity(st, trials, skill == module.SkillHard, r)
	owe := s.toCall(st)
	pot := s.pot()

	if owe == 0 {
		switch {
		case skill == module.SkillEasy:
			if eq > 0.7 && enabled(VerbBet) {
				return do(VerbBet)
			}
		case eq > 0.55 && enabled(VerbBet):
			return do(VerbBet)
		}
		return do(VerbCheck)
	}

	// Facing a bet: what share of the pot after calling is the call?
	price := float64(owe) / float64(pot+owe)
	switch skill {
	case module.SkillEasy:
		if eq < 0.12 && r.Float64() < 0.5 {
			return do(VerbFold)
		}
		return do(VerbCall)
	case module.SkillHard:
		if eq > 0.72 && enabled(VerbRaise) {
			return do(VerbRaise)
		}
		if eq >= price {
			return do(VerbCall)
		}
		return do(VerbFold)
	}
	if eq > 0.72 && enabled(VerbRaise) {
		return do(VerbRaise)
	}
	if eq >= price {
		return do(VerbCall)
	}
	return do(VerbFold)
}

// equity is this seat's chance of the best hand at the showdown against the
// others still in: the unseen cards dealt out at random, trials times, ties
// counted as half.
//
// A reading seat (hard) also weighs what the betting said: a player who has
// bet or raised this hand is dealt, as far as the sampling can manage, a
// hand that would have bet — less on two cards than on four.
func (s *GameState) equity(me *Seat, trials int, read bool, r *rand.Rand) float64 {
	seen := map[string]bool{}
	for _, c := range me.Hand {
		seen[c] = true
	}
	var pool []string
	for _, c := range buildDeck() {
		if !seen[c] {
			pool = append(pool, c)
		}
	}
	sort.Strings(pool)
	var others []*Seat
	for i := range s.Seats {
		if st := &s.Seats[i]; st != me && st.live() {
			others = append(others, st)
		}
	}
	if len(others) == 0 {
		return 1
	}
	// On two cards a bet says less than on four: a fair two of a suit, then
	// two aces; on four, two aces, then three of a suit.
	floor := func(o *Seat) int {
		switch {
		case !read || o.Raised == 0:
			return 0
		case s.Phase == phaseFirst && o.Raised == 1:
			return catTwoSuited*100 + 17
		case s.Phase == phaseFirst, o.Raised == 1:
			return catTwoAces * 100
		}
		return catThreeSuited * 100
	}
	need := handSize - len(me.Hand)
	won := 0.0
	for t := 0; t < trials; t++ {
		deck := append([]string(nil), pool...)
		r.Shuffle(len(deck), func(i, j int) { deck[i], deck[j] = deck[j], deck[i] })
		mine := append(append([]string(nil), me.Hand...), deck[:need]...)
		deck = deck[need:]
		my := score(mine)
		best, ties := true, 0
		for _, o := range others {
			theirs := score(deck[:handSize])
			// Look a little further into the deck for a hand that fits how
			// this player has bet; settle for what comes if none does.
			if f := floor(o); f > 0 && theirs < f {
				for k := handSize; k+handSize <= len(deck) && k < 6*handSize; k += handSize {
					if sc := score(deck[k : k+handSize]); sc >= f {
						deck[0], deck[1], deck[2], deck[3], deck[k], deck[k+1], deck[k+2], deck[k+3] =
							deck[k], deck[k+1], deck[k+2], deck[k+3], deck[0], deck[1], deck[2], deck[3]
						theirs = sc
						break
					}
				}
			}
			deck = deck[handSize:]
			switch {
			case theirs > my:
				best = false
			case theirs == my:
				ties++
			}
		}
		switch {
		case !best:
		case ties > 0:
			won += 1.0 / float64(ties+1)
		default:
			won++
		}
	}
	return won / float64(trials)
}

// liveOthers is how many other seats are still in the hand.
func (s *GameState) liveOthers(me *Seat) int {
	n := 0
	for i := range s.Seats {
		if st := &s.Seats[i]; st != me && st.live() {
			n++
		}
	}
	return n
}
