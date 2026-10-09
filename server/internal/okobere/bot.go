package okobere

import (
	"math/rand"
	"strconv"

	"zolik/server/internal/module"
)

// How a seat nobody is sitting at plays Oko bere.
//
// Easy stakes the minimum, draws to 14 and banks the same way. Medium stakes
// more on a strong first card (an eso or a ten), stands from 15, and banks to
// 16. Hard counts what it has seen — its own cards and every bust or oko
// turned up — and draws while the chance of going over is small enough for
// the total it holds; it sizes its stake by a quick simulation of the hand
// against a bank that draws to 16.
//
// It reads only what its seat may know: its own cards, and the cards turned
// up for everyone. Legality is never its business.
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
	r := rand.New(rand.NewSource(seat.Seed + int64(s.Round)*1009 + int64(len(st.Hand))*31))

	if s.Phase == phasePunters && st.Bet == 0 {
		return module.Action{OfferID: OfferBet, Verb: VerbBet, Params: map[string]string{
			ParamAmount: strconv.Itoa(s.stake(st, skill, r)),
		}}, true
	}
	if s.drawMore(st, skill) {
		return module.Action{OfferID: OfferCard, Verb: VerbCard}, true
	}
	return module.Action{OfferID: OfferStand, Verb: VerbStand}, true
}

// stake is what a punter puts down on its first card.
func (s *GameState) stake(st *Seat, skill module.Skill, r *rand.Rand) int {
	top := s.maxBet(st)
	clamp := func(n int) int { return max(s.MinBet, min(top, n)) }
	first := value(st.Hand[0])
	switch skill {
	case module.SkillEasy:
		return s.MinBet
	case module.SkillHard:
		edge := s.edge(st, r)
		if edge <= 0 {
			return s.MinBet
		}
		// A share of the stack in proportion to the edge, never more than a
		// quarter of it.
		return clamp(int(float64(st.Stack) * min(0.25, edge)))
	}
	switch {
	case first >= 10:
		return clamp(s.MinBet * 4)
	case first >= 8:
		return clamp(s.MinBet * 2)
	}
	return s.MinBet
}

// seen is every card this seat can place: its own, and every hand turned up.
func (s *GameState) seen(me *Seat) map[string]bool {
	out := map[string]bool{}
	for _, c := range me.Hand {
		out[c] = true
	}
	b := s.bankerIndex()
	for i, st := range s.Seats {
		if st.Result == resultBust || st.Result == resultOko || (i == b && s.Phase == phaseBank) {
			for _, c := range st.Hand {
				out[c] = true
			}
		}
	}
	return out
}

// unseen is the cards this seat cannot place: the deck and every hidden card,
// as far as it knows.
func (s *GameState) unseen(me *Seat) []string {
	seen := s.seen(me)
	var out []string
	for _, c := range buildDeck() {
		if !seen[c] {
			out = append(out, c)
		}
	}
	return out
}

// bustChance is the chance one more card takes the hand over 21, from the
// cards this seat cannot place.
func (s *GameState) bustChance(st *Seat) float64 {
	pool := s.unseen(st)
	if len(pool) == 0 {
		return 1
	}
	over := 0
	t := total(st.Hand)
	for _, c := range pool {
		hand := append(append([]string(nil), st.Hand...), c)
		if t+value(c) > goal && !oko(hand) {
			over++
		}
	}
	return float64(over) / float64(len(pool))
}

// drawMore is "ještě" or "stačí".
func (s *GameState) drawMore(st *Seat, skill module.Skill) bool {
	t := total(st.Hand)
	banker := st == s.banker()
	switch skill {
	case module.SkillEasy:
		return t <= 14
	case module.SkillHard:
		risk := s.bustChance(st)
		switch {
		case t <= 11:
			return true
		case t <= 14:
			return risk < 0.6
		case t <= 16:
			if banker {
				return risk < 0.5
			}
			return risk < 0.35
		case t == 17:
			return risk < 0.15
		}
		return false
	}
	if banker {
		return t < 16
	}
	return t < 15
}

// edge is a quick estimate of what a stake on this first card returns per
// chip: the rest of the hand dealt from the unseen cards, played as hard
// plays it, against a bank drawing to 16.
func (s *GameState) edge(st *Seat, r *rand.Rand) float64 {
	pool := s.unseen(st)
	const trials = 200
	sum := 0.0
	for i := 0; i < trials; i++ {
		deck := append([]string(nil), pool...)
		r.Shuffle(len(deck), func(a, b int) { deck[a], deck[b] = deck[b], deck[a] })
		hand := append([]string(nil), st.Hand...)
		for total(hand) <= 14 && len(deck) > 0 {
			hand, deck = append(hand, deck[0]), deck[1:]
		}
		if oko(hand) {
			sum += float64(s.OkoPays)
			continue
		}
		if total(hand) > goal {
			sum--
			continue
		}
		bank := []string{}
		for total(bank) < 16 && len(deck) > 0 {
			bank, deck = append(bank, deck[0]), deck[1:]
		}
		switch {
		case oko(bank):
			sum--
		case total(bank) > goal || total(hand) > total(bank):
			sum++
		default:
			sum--
		}
	}
	return sum / trials
}
