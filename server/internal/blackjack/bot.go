package blackjack

import (
	"math/rand"
	"strconv"

	"zolik/server/internal/module"
)

// bot is Blackjack's own player.
//
// module.OfferBot cannot play this game at all. Hit and stand are offered on
// every hand, so an offer-preference bot either hits until it goes bust or
// stands on four — and unlike a shedding game, where playing legally is most
// of playing well, blackjack's whole content is *which* of two always-legal
// moves to make. So this one reads the hand.
//
// What it plays is basic strategy: the published, exact answer to "what does
// this hand do against that up card", worked out from the composition of the
// deck rather than from taste. That makes the skill dial unusually honest
// here — the strong setting is not a better guesser, it is the correct play,
// and the weaker ones are named mistakes real players make:
//
//	easy    mimics the dealer (draw to seventeen, stand on it), never
//	        doubles or splits, and takes insurance — the four commonest
//	        beginner errors, and between them worth several percent
//	medium  the hit/stand chart, ten and eleven doubled, aces and eights
//	        split; no soft doubling, no surrender, no insurance
//	hard    the whole chart: soft doubles, every pair, late surrender where
//	        the table offers it, and insurance declined on principle
//
// It counts nothing. A card counter would be a different and much larger
// program, and — since it would have to be told the shoe's composition, which
// the offer list correctly does not carry — a much less interesting test of
// this protocol.
type bot struct{}

var _ module.Bot = bot{}

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
	case phaseBets:
		return chooseBet(s, seat, skill, offers)
	case phaseInsurance:
		// Insurance is a bet on the hole card at 2:1 when it comes in at worse
		// than that, whatever the hand holding it looks like. Only the beginner
		// takes it.
		if skill == module.SkillEasy {
			if a, ok := take(offers, OfferInsure); ok {
				return a, true
			}
		}
		return firstOf(offers, OfferDecline, OfferInsure)
	case phasePlay:
		st := s.seat(seat.PlayerID)
		hand := current(s)
		if st == nil || hand == nil {
			return module.ChooseAction(offers, nil)
		}
		return firstOf(offers, append(plan(s, hand, skill), OfferStand, OfferHit)...)
	}
	return module.ChooseAction(offers, nil)
}

// chooseBet sizes the stake.
//
// A unit rather than the table minimum, so a match of ten rounds actually
// moves a stack; flat, because betting more or less on a hand you have not
// seen changes nothing at all about who wins. Easy varies it anyway — chasing
// a loss is the fifth beginner error — off the seat's own seed, so the same
// match deals the same mistakes twice.
func chooseBet(s *GameState, seat module.BotSeat, skill module.Skill, offers []module.ActionOffer) (module.Action, bool) {
	o := module.FindOffer(offers, OfferBet)
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
		OfferID: OfferBet, Verb: VerbBet,
		Params: map[string]string{ParamAmount: strconv.Itoa(unit)},
	}, true
}

// plan is what this hand wants to do, most-preferred first, so that a move
// the table does not allow falls through to the next-best one rather than
// being submitted and refused.
//
// The fallbacks are part of the strategy, not error handling: "double, else
// hit" and "double, else stand" are different answers, and a chart that
// collapsed them would misplay every soft eighteen where doubling is off.
func plan(s *GameState, hand *Hand, skill module.Skill) []string {
	up := upValue(s.upCard())
	total, soft := Total(hand.Cards)

	if skill == module.SkillEasy {
		// Mimic the dealer: draw to seventeen, stand on it, and never mind
		// what the dealer is showing.
		if total < 17 {
			return []string{OfferHit}
		}
		return []string{OfferStand}
	}

	if len(hand.Cards) == 2 && samePairValue(hand.Cards[0], hand.Cards[1]) {
		if p := pairPlan(s, hand, up, skill); p != nil {
			return p
		}
	}
	// Only where the table actually offers it: a plan naming a move this
	// table does not have would fall through to the same hit anyway, but it
	// would be describing somebody else's chart.
	if skill == module.SkillHard && s.AllowSurrender {
		if sur := surrenderPlan(total, soft, up); sur != nil {
			return sur
		}
	}
	if soft {
		return softPlan(total, up, skill)
	}
	return hardPlan(total, up, skill)
}

// upValue is the dealer's up card as the charts index it: an ace is eleven,
// because that is what it will be if the dealer's hand allows it.
func upValue(card string) int {
	if rankOf(card) == "A" {
		return 11
	}
	return cardValue(card)
}

// pairPlan is the pair chart. Nil means "this pair is not split here — play
// it as the total it is", which is how fives and tens are handled: a pair of
// fives is a ten and a pair of tens is a twenty, and splitting either is a way
// of turning a good hand into two worse ones.
func pairPlan(s *GameState, hand *Hand, up int, skill module.Skill) []string {
	v := cardValue(hand.Cards[0])
	splitThen := func(fallback string) []string { return []string{OfferSplit, fallback} }

	if skill == module.SkillMedium {
		// Aces and eights, the two every chart agrees on and the two worth
		// most: splitting aces turns a soft twelve into two hands drawing to
		// eleven, and splitting eights breaks up the worst total in the game.
		if v == 1 || v == 8 {
			return splitThen(OfferHit)
		}
		return nil
	}

	switch v {
	case 1: // aces
		return splitThen(OfferHit)
	case 10:
		return []string{OfferStand}
	case 9:
		if up == 7 || up >= 10 {
			return []string{OfferStand}
		}
		return splitThen(OfferStand)
	case 8:
		return splitThen(OfferHit)
	case 7, 3, 2:
		if up <= 7 {
			return splitThen(OfferHit)
		}
		return nil
	case 6:
		if up <= 6 {
			return splitThen(OfferHit)
		}
		return nil
	case 4:
		// Only worth splitting into a dealer bust card, and only where the
		// split hands may then be doubled.
		if s.DoubleAfterSplit && (up == 5 || up == 6) {
			return splitThen(OfferHit)
		}
		return nil
	default: // fives play as a hard ten
		return nil
	}
}

// surrenderPlan gives up the two hands that lose more than half the time
// however they are played. Late surrender only — the dealer has already
// peeked by the time this is offered.
func surrenderPlan(total int, soft bool, up int) []string {
	if soft {
		return nil
	}
	switch {
	case total == 16 && up >= 9:
		return []string{OfferSurrender, OfferHit}
	case total == 15 && up == 10:
		return []string{OfferSurrender, OfferHit}
	}
	return nil
}

// softPlan is the ace-counted-as-eleven chart, where a hand cannot bust and
// doubling is therefore free of the risk that makes it a decision at all.
func softPlan(total, up int, skill module.Skill) []string {
	if skill == module.SkillMedium {
		// No soft doubling: stand on eighteen and up, draw below it.
		if total >= 18 {
			return []string{OfferStand}
		}
		return []string{OfferHit}
	}

	switch {
	case total >= 19:
		return []string{OfferStand}
	case total == 18:
		if up >= 3 && up <= 6 {
			return []string{OfferDouble, OfferStand}
		}
		if up == 2 || up == 7 || up == 8 {
			return []string{OfferStand}
		}
		return []string{OfferHit}
	case total == 17:
		if up >= 3 && up <= 6 {
			return []string{OfferDouble, OfferHit}
		}
		return []string{OfferHit}
	case total >= 15: // A,4 and A,5
		if up >= 4 && up <= 6 {
			return []string{OfferDouble, OfferHit}
		}
		return []string{OfferHit}
	default: // A,2 and A,3
		if up == 5 || up == 6 {
			return []string{OfferDouble, OfferHit}
		}
		return []string{OfferHit}
	}
}

// hardPlan is the chart everything else falls back to: stand where the dealer
// is likely to bust, draw where they are not.
func hardPlan(total, up int, skill module.Skill) []string {
	switch {
	case total >= 17:
		return []string{OfferStand}
	case total >= 13:
		if up <= 6 {
			return []string{OfferStand}
		}
		return []string{OfferHit}
	case total == 12:
		if up >= 4 && up <= 6 {
			return []string{OfferStand}
		}
		return []string{OfferHit}
	case total == 11:
		return []string{OfferDouble, OfferHit}
	case total == 10:
		if up <= 9 {
			return []string{OfferDouble, OfferHit}
		}
		return []string{OfferHit}
	case total == 9:
		if skill != module.SkillMedium && up >= 3 && up <= 6 {
			return []string{OfferDouble, OfferHit}
		}
		return []string{OfferHit}
	default:
		return []string{OfferHit}
	}
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
