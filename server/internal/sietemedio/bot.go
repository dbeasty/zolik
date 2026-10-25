package sietemedio

import (
	"math/rand"
	"strconv"

	"zolik/server/internal/module"
)

// bot is Siete y Medio's own player.
//
// module.OfferBot cannot play this game at all: ask-for-a-card and stand are
// offered on every hand, so an offer-preference bot either draws itself past
// 7½ every round or stands on a half. Unlike a shedding game, where playing
// legally is most of playing well, this game's whole content is *which* of two
// always-legal moves to make. So this one reads the hand.
//
// What it reads is only what a player in that seat can see. Its own hand in
// full; everybody else's through the same filter the board uses, which means
// the card each of them was dealt first is as unknown to the bot as it is to a
// person — see `visible`. A bot that peeked would be a different and much
// stronger player, and not one anybody would want to sit down against.
//
// The skill dial, in halves (7½ is fifteen):
//
//	easy    draws to 7 and stands there, so it goes over more often than not,
//	        banks without looking at what it has to beat, and chases its
//	        losses with the stake
//	medium  stands on 6½, and banks to 6 regardless of the table
//	hard    stands on 6 — about where the arithmetic turns, since a hand on
//	        6½ survives a draw two times in five — and, holding the bank,
//	        draws to what the standing hands look like rather than to a
//	        number it picked in advance
type bot struct{}

var _ module.Bot = bot{}

// Standing thresholds, in halves: draw while the total is under this.
const (
	standEasy   = 14 // 7 — over-drawing, which is the commonest way to lose this game
	standMedium = 13 // 6½
	standHard   = 12 // 6
)

// bankFloor is the total the bank draws up to however weak the table looks.
// Standing on five and a half with the whole table still to be paid is giving
// the round away.
const bankFloor = 12

// hiddenCardEstimate is what the bot assumes an unseen card is worth, in
// halves: the pack's own mean, which is 236 halves across 40 cards.
const hiddenCardEstimate = 6

func (b bot) Act(raw module.State, seat module.BotSeat, offers []module.ActionOffer) (module.Action, bool) {
	s, err := decode(raw)
	if err != nil {
		return module.ChooseAction(offers, nil)
	}
	skill := seat.Skill
	if skill == "" {
		// An empty skill is a seat taken before skills existed, not the
		// weakest setting — see module.BotSeat.
		skill = module.SkillMedium
	}

	if s.Break.Open {
		return module.ChooseAction(offers, nil)
	}

	switch s.Phase {
	case phaseStakes:
		return chooseStake(s, seat, skill, offers)
	case phasePlay, phaseBank:
		st := s.seat(seat.PlayerID)
		if st == nil {
			return module.ChooseAction(offers, nil)
		}
		if want := plan(s, st, seat.PlayerID, skill); want != "" {
			if a, ok := take(offers, want); ok {
				return a, true
			}
		}
		return firstOf(offers, OfferStand, OfferHit)
	}
	return module.ChooseAction(offers, nil)
}

// chooseStake sizes the stake.
//
// A unit rather than the table minimum, so a match of twelve rounds actually
// moves a stack; flat, because staking more or less on a hand nobody has seen
// changes nothing about who wins it. Easy varies it anyway — chasing a loss is
// the other beginner mistake — off the seat's own seed, so the same match deals
// the same mistakes twice.
func chooseStake(s *GameState, seat module.BotSeat, skill module.Skill, offers []module.ActionOffer) (module.Action, bool) {
	o := module.FindOffer(offers, OfferStake)
	if o == nil || !o.Enabled || len(o.Params) == 0 {
		return module.ChooseAction(offers, nil)
	}
	spec := o.Params[0]

	unit := s.StartingStack / 25
	if unit < spec.Min {
		unit = spec.Min
	}
	if skill == module.SkillEasy {
		r := rand.New(rand.NewSource(seat.Seed + int64(s.RoundNumber)))
		unit *= 1 + r.Intn(3)
	}
	if unit > spec.Max {
		unit = spec.Max
	}
	if unit < spec.Min {
		unit = spec.Min
	}

	return module.Action{
		OfferID: OfferStake, Verb: VerbStake,
		Params: map[string]string{ParamAmount: strconv.Itoa(unit)},
	}, true
}

// plan is what this hand wants to do: the offer id it would rather play, or ""
// to fall through to whatever the list will take.
func plan(s *GameState, st *Seat, ownID string, skill module.Skill) string {
	total := Total(st.Cards)
	if total >= Target {
		// Cannot happen — a hand on 7½ is stood by the engine rather than
		// offered a draw — but a bot that would draw on it is a bot one state
		// change away from drawing itself out of a won round.
		return OfferStand
	}

	want := standHard
	switch skill {
	case module.SkillEasy:
		want = standEasy
	case module.SkillMedium:
		want = standMedium
	}

	// Holding the bank is a different problem: the bank plays last, so what it
	// has to beat is on the table in front of it rather than a guess about the
	// pack. Only the strong setting uses that; the other two draw to a number
	// they picked before they sat down, which is exactly the mistake.
	if s.isBanker(s.seatIndex(st.PlayerID)) && skill == module.SkillHard {
		if need := bankNeed(s, ownID); need > want {
			want = need
		}
		if want > Target {
			want = Target
		}
		if want < bankFloor {
			want = bankFloor
		}
	}

	if total < want {
		return OfferHit
	}
	return OfferStand
}

// bankNeed is how high the bank reckons it has to get, in halves.
//
// It is the best total still standing against it, estimated: the cards a seat
// has turned up, plus the pack's mean for the one it has not. A bank that
// could see the face-down cards would not need to estimate anything, which is
// precisely why it is not allowed to — `visible` is the same filter the board
// uses, and this reads the bank's own view of the table through it.
func bankNeed(s *GameState, ownID string) int {
	need := 0
	for i := range s.Seats {
		if s.isBanker(i) {
			continue
		}
		st := &s.Seats[i]
		if !st.inRound() {
			continue
		}
		shown := visible(s, st, ownID, false)
		// A hand already past 7½ has lost and is nothing to beat. It is also
		// face up by then, so reading it is reading the table rather than
		// peeking.
		if IsBust(shown) {
			continue
		}
		est := Total(shown) + (len(st.Cards)-len(shown))*hiddenCardEstimate
		if est > Target {
			est = Target
		}
		if est > need {
			need = est
		}
	}
	return need
}

// firstOf submits the first of these offers that is actually on, falling back
// to whatever the offer list will accept — the module's own bot never gets to
// hang a seat.
func firstOf(offers []module.ActionOffer, ids ...string) (module.Action, bool) {
	for _, id := range ids {
		if a, ok := take(offers, id); ok {
			return a, true
		}
	}
	return module.ChooseAction(offers, nil)
}

func take(offers []module.ActionOffer, id string) (module.Action, bool) {
	o := module.FindOffer(offers, id)
	if o == nil || !o.Enabled {
		return module.Action{}, false
	}
	return module.SubmissionFor(*o)
}
