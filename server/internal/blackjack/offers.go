package blackjack

import (
	"strconv"

	"zolik/server/internal/module"
)

// Offer IDs. One per decision a player ever makes, and — like poker's — not
// one of them takes a card: everything in this game is dealt to you, so the
// only input any offer needs is a number, and only the bet needs that.
const (
	OfferBet       = "bet"
	OfferInsure    = "insure"
	OfferDecline   = "decline_insurance"
	OfferHit       = "hit"
	OfferStand     = "stand"
	OfferDouble    = "double"
	OfferSplit     = "split"
	OfferSurrender = "surrender"
)

// LegalActions answers "what may this player do right now?".
//
// Every enabled/disabled decision comes from probing the real engine and
// reading back its code, so the list is Apply's own answer asked in advance
// and cannot drift from it. What it adds on top is the arithmetic a player
// would otherwise have to do by eye: what a double costs, what insurance
// costs, what a surrender hands back.
func (m *Module) LegalActions(raw module.State, playerID string) ([]module.ActionOffer, error) {
	s, err := decode(raw)
	if err != nil {
		return nil, err
	}

	if s.Break.Open {
		return s.Break.Offers(order(s), playerID), nil
	}
	if s.Status != "active" {
		return placeholders(s, ErrGameNotActive), nil
	}

	seat := s.seat(playerID)
	if seat == nil {
		return placeholders(s, module.ErrNotSeated), nil
	}

	switch s.Phase {
	case phaseBets:
		return m.bettingOffers(raw, s, seat), nil
	case phaseInsurance:
		return m.insuranceOffers(raw, s, seat), nil
	case phasePlay:
		if s.Current < 0 || s.Seats[s.Current].PlayerID != playerID {
			return placeholders(s, ErrNotYourTurn), nil
		}
		return m.playOffers(raw, s, seat), nil
	}
	return placeholders(s, ErrGameNotActive), nil
}

// placeholders is the whole control set, off, with the one reason that
// applies. A disabled offer stays on screen with its reason — an omitted one
// is indistinguishable from a client bug.
//
// Surrender is the exception: a table that does not offer it has no such
// control at all, the same way a game with no melds emits no meld zone. That
// is a rule of this table rather than a state of this turn, and the rules
// screen is where a player reads it.
func placeholders(s *GameState, why string) []module.ActionOffer {
	out := []module.ActionOffer{
		{ID: OfferBet, Verb: VerbBet, LabelKey: "blackjack.offer.bet", WhyNot: why},
		{ID: OfferHit, Verb: VerbHit, LabelKey: "blackjack.offer.hit", WhyNot: why},
		{ID: OfferStand, Verb: VerbStand, LabelKey: "blackjack.offer.stand", WhyNot: why},
		{ID: OfferDouble, Verb: VerbDouble, LabelKey: "blackjack.offer.double", WhyNot: why},
		{ID: OfferSplit, Verb: VerbSplit, LabelKey: "blackjack.offer.split", WhyNot: why},
	}
	if s.AllowSurrender {
		out = append(out, module.ActionOffer{
			ID: OfferSurrender, Verb: VerbSurrender, LabelKey: "blackjack.offer.surrender", WhyNot: why,
		})
	}
	return out
}

func (m *Module) probe(raw module.State, playerID string, a module.Action) (bool, string) {
	if _, _, err := m.Apply(raw, playerID, a); err != nil {
		return false, module.CodeOf(err)
	}
	return true, ""
}

// bettingOffers is the one place this game asks for a number.
//
// The range is declared rather than enumerated, exactly as poker's raise is:
// a stake is any figure from the table minimum to this seat's whole stack, and
// a thousand offers for a thousand chips is the offer-explosion problem in its
// purest form. The default is the table minimum — a real move, and not a
// reckless one, so a client that presses the control unchanged is not shoving.
func (m *Module) bettingOffers(raw module.State, s *GameState, seat *Seat) []module.ActionOffer {
	bet := module.ActionOffer{ID: OfferBet, Verb: VerbBet, LabelKey: "blackjack.offer.bet"}
	bet.Enabled, bet.WhyNot = m.probe(raw, seat.PlayerID, module.Action{
		OfferID: OfferBet, Verb: VerbBet,
		Params: map[string]string{ParamAmount: strconv.Itoa(s.MinBet)},
	})
	if bet.Enabled {
		bet.Params = []module.ParamSpec{{
			Name:     ParamAmount,
			Kind:     module.ParamKindInt,
			LabelKey: "blackjack.prompt.betAmount",
			Min:      s.MinBet,
			Max:      seat.Stack,
			Step:     1,
			Default:  s.MinBet,
			Choices:  betQuickChoices(s.MinBet, seat.Stack),
		}}
		bet.Facts = []module.Fact{{
			LabelKey: "blackjack.fact.tableMinimum", Value: strconv.Itoa(s.MinBet),
			Params: map[string]any{"n": s.MinBet},
		}}
	}
	bet.RuleIDs = ruleIDsFor(s, bet.WhyNot)

	offers := []module.ActionOffer{bet}
	for _, o := range placeholders(s, ErrWrongPhase) {
		if o.ID != OfferBet {
			offers = append(offers, o)
		}
	}
	return offers
}

// betQuickChoices names a couple of stakes a player reasons about without
// doing the multiplication themselves — twice the table minimum, and the
// whole stack — alongside the bare range `ParamKindInt` already carries.
//
// Built the same way holdem's raiseQuickChoices is: every LabelKey a literal
// string in its own module.ParamChoice{...}, since dump-keys finds label
// keys by reading the source rather than by running it, and "all in" always
// present — inserted first, so a double-the-minimum stake that happens to
// equal the whole stack (a short one) is dropped in its favour rather than
// showing two buttons for the same figure.
func betQuickChoices(minBet, stack int) []module.ParamChoice {
	clamp := func(n int) int {
		if n < minBet {
			return minBet
		}
		if n > stack {
			return stack
		}
		return n
	}

	allIn := stack
	doubleMin := clamp(minBet * 2)

	choices := []module.ParamChoice{{Value: strconv.Itoa(allIn), LabelKey: "blackjack.quick.allIn"}}
	if doubleMin != allIn {
		choices = append([]module.ParamChoice{{Value: strconv.Itoa(doubleMin), LabelKey: "blackjack.quick.doubleMin"}}, choices...)
	}
	return choices
}

// insuranceOffers is a decision with two answers, both of which are offered,
// because "decline" is a move a player makes rather than a control they fail
// to press. Without it a seat that wants no insurance would have nothing to
// do, and the table would wait on a silence.
func (m *Module) insuranceOffers(raw module.State, s *GameState, seat *Seat) []module.ActionOffer {
	insure := module.ActionOffer{ID: OfferInsure, Verb: VerbInsure, LabelKey: "blackjack.offer.insure"}
	insure.Enabled, insure.WhyNot = m.probe(raw, seat.PlayerID, module.Action{OfferID: OfferInsure, Verb: VerbInsure})
	if cost := insuranceCost(seat); cost > 0 {
		insure.Facts = []module.Fact{{
			LabelKey: "blackjack.fact.insuranceCost", Value: strconv.Itoa(cost),
			Params: map[string]any{"n": cost},
		}}
	}
	insure.RuleIDs = ruleIDsFor(s, insure.WhyNot)

	decline := module.ActionOffer{ID: OfferDecline, Verb: VerbDecline, LabelKey: "blackjack.offer.declineInsurance"}
	decline.Enabled, decline.WhyNot = m.probe(raw, seat.PlayerID, module.Action{OfferID: OfferDecline, Verb: VerbDecline})
	decline.RuleIDs = ruleIDsFor(s, decline.WhyNot)

	offers := []module.ActionOffer{insure, decline}
	for _, o := range placeholders(s, ErrWrongPhase) {
		offers = append(offers, o)
	}
	return offers
}

// playOffers is the hand in front of this seat: hit, stand, and whichever of
// the three optional moves this table and this hand allow.
func (m *Module) playOffers(raw module.State, s *GameState, seat *Seat) []module.ActionOffer {
	hand := current(s)

	offers := make([]module.ActionOffer, 0, 5)
	for _, spec := range []struct {
		id, verb, label string
	}{
		{OfferHit, VerbHit, "blackjack.offer.hit"},
		{OfferStand, VerbStand, "blackjack.offer.stand"},
		{OfferDouble, VerbDouble, "blackjack.offer.double"},
		{OfferSplit, VerbSplit, "blackjack.offer.split"},
	} {
		o := module.ActionOffer{ID: spec.id, Verb: spec.verb, LabelKey: spec.label}
		o.Enabled, o.WhyNot = m.probe(raw, seat.PlayerID, module.Action{OfferID: spec.id, Verb: spec.verb})
		o.RuleIDs = ruleIDsFor(s, o.WhyNot)
		offers = append(offers, o)
	}

	if s.AllowSurrender {
		o := module.ActionOffer{ID: OfferSurrender, Verb: VerbSurrender, LabelKey: "blackjack.offer.surrender"}
		o.Enabled, o.WhyNot = m.probe(raw, seat.PlayerID, module.Action{OfferID: OfferSurrender, Verb: VerbSurrender})
		o.RuleIDs = ruleIDsFor(s, o.WhyNot)
		offers = append(offers, o)
	}

	if hand == nil {
		return offers
	}

	// What each move costs, pushed rather than left to be worked out. A client
	// deriving "a double costs another stake" or "surrender hands half back,
	// odd chip included" would be re-implementing a rule, which is the one
	// thing this protocol exists to stop.
	for i := range offers {
		switch offers[i].ID {
		case OfferDouble, OfferSplit:
			offers[i].Facts = []module.Fact{{
				LabelKey: "blackjack.fact.extraStake", Value: strconv.Itoa(hand.Bet),
				Params: map[string]any{"n": hand.Bet},
			}}
		case OfferSurrender:
			back := hand.Bet - hand.Bet/2
			offers[i].Facts = []module.Fact{{
				LabelKey: "blackjack.fact.surrenderReturn", Value: strconv.Itoa(back),
				Params: map[string]any{"n": back},
			}}
		}
	}
	return offers
}

// ruleIDsFor points a refusal at the written rules that justify it at this
// table.
//
// It states no rule of its own: every id here is the label key of a sentence
// Rules() builds for the same table, under the same condition, so a refusal
// gets a config-accurate explanation in whatever locale the reader has. The
// conditions are duplicated between the two — which is why a test walks every
// option combination and checks each id resolves.
func ruleIDsFor(s *GameState, code string) []string {
	switch code {
	case ErrBetTooSmall, ErrAlreadyBet:
		return []string{"blackjack.rules.minBet"}
	case ErrCannotDouble:
		if s.DoubleAfterSplit {
			return []string{"blackjack.rules.double", "blackjack.rules.doubleAfterSplit"}
		}
		return []string{"blackjack.rules.double", "blackjack.rules.noDoubleAfterSplit"}
	case ErrCannotSplit:
		if s.MaxSplits > 0 {
			return []string{"blackjack.rules.split"}
		}
		return []string{"blackjack.rules.noSplit"}
	case ErrCannotSurrender:
		if s.AllowSurrender {
			return []string{"blackjack.rules.surrender"}
		}
		return nil
	case ErrInsuranceClosed:
		if s.AllowInsurance {
			return []string{"blackjack.rules.insurance"}
		}
		return nil
	default:
		return nil
	}
}
