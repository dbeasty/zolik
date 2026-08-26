package holdem

import (
	"strconv"

	"zolik/server/internal/module"
)

// Offer IDs. Four, and none of them takes a card — which is itself the finding:
// every selector in this module's offers is empty, and the protocol did not
// mind.
const (
	OfferFold  = "fold"
	OfferCheck = "check"
	OfferCall  = "call"
	OfferRaise = "raise"
)

// LegalActions answers "what may this player do right now?".
//
// Built the same way every other module builds it: each enabled/disabled
// decision comes from probing the real engine and reading back its error code,
// so the offer list is `Apply`'s own answer asked in advance and cannot drift
// from it.
//
// The raise offer is where this module differs from the card games. A no-limit
// raise cannot be enumerated — there are as many legal raises as there are
// chips — so instead of a list of concrete submissions it ships a *range*: the
// minimum legal raise, the all-in ceiling, and a default. The engine still
// validates whatever comes back, exactly as it validates a meld shape.
func (m *Module) LegalActions(raw module.State, playerID string) ([]module.ActionOffer, error) {
	s, err := decode(raw)
	if err != nil {
		return nil, err
	}

	// Between hands the only thing anybody may do is agree to go on.
	if s.Break.Open {
		return s.Break.Offers(order(s), playerID), nil
	}

	if s.Status != "active" || s.Current < 0 || s.Seats[s.Current].PlayerID != playerID {
		why := ErrNotYourTurn
		if s.Status != "active" {
			why = ErrGameNotActive
		}
		return []module.ActionOffer{
			{ID: OfferFold, Verb: VerbFold, WhyNot: why},
			{ID: OfferCheck, Verb: VerbCheck, WhyNot: why},
			{ID: OfferCall, Verb: VerbCall, WhyNot: why},
			{ID: OfferRaise, Verb: VerbRaise, WhyNot: why},
		}, nil
	}

	seat := &s.Seats[s.Current]
	offers := make([]module.ActionOffer, 0, 4)

	fold := module.ActionOffer{ID: OfferFold, Verb: VerbFold}
	fold.Enabled, fold.WhyNot = probe(m, raw, playerID, module.Action{Verb: VerbFold})
	offers = append(offers, fold)

	check := module.ActionOffer{ID: OfferCheck, Verb: VerbCheck}
	check.Enabled, check.WhyNot = probe(m, raw, playerID, module.Action{Verb: VerbCheck})
	offers = append(offers, check)

	call := module.ActionOffer{ID: OfferCall, Verb: VerbCall}
	call.Enabled, call.WhyNot = probe(m, raw, playerID, module.Action{Verb: VerbCall})
	if owed := s.toCall(seat); owed > 0 {
		// What the button costs, pushed rather than derived. A client working
		// this out from the highest bet minus its own would be re-implementing
		// a rule.
		call.Facts = []module.Fact{{
			LabelKey: "holdem.cost.call", Value: strconv.Itoa(owed),
			Params: map[string]any{"amount": owed, "allIn": owed == seat.Stack},
		}}
	}
	offers = append(offers, call)

	// --- raise ---------------------------------------------------------------
	raise := module.ActionOffer{ID: OfferRaise, Verb: VerbRaise}
	minTo, maxTo := raiseRange(s, seat)
	raise.Enabled, raise.WhyNot = probe(m, raw, playerID, module.Action{
		Verb: VerbRaise, Params: map[string]string{ParamAmount: strconv.Itoa(minTo)},
	})
	if raise.Enabled {
		raise.Params = []module.ParamSpec{{
			Name:     ParamAmount,
			Kind:     module.ParamKindInt,
			LabelKey: "holdem.prompt.raiseTo",
			Min:      minTo,
			Max:      maxTo,
			Step:     1,
			Default:  minTo,
		}}
		raise.Facts = []module.Fact{{
			LabelKey: "holdem.cost.pot", Value: strconv.Itoa(s.potIfCalled()),
		}}
	}
	offers = append(offers, raise)

	return offers, nil
}

// raiseRange is the smallest and largest total this seat may raise to.
//
// The minimum is the current bet plus the size of the last raise — not plus
// the big blind, which is the version of this rule most implementations ship
// by accident and which quietly allows an illegal re-raise after a big one.
// Both ends are capped at the stack, since a player may always move all in.
func raiseRange(s *GameState, seat *Seat) (int, int) {
	maxTo := seat.Bet + seat.Stack
	minTo := s.CurrentBet + s.MinRaise
	if minTo > maxTo {
		minTo = maxTo
	}
	if minTo < 1 {
		minTo = 1
	}
	return minTo, maxTo
}

// potIfCalled is what the pot would be if the current bet were called all
// round — the number a player actually reasons about.
func (s *GameState) potIfCalled() int {
	total := s.Pot
	for i := range s.Seats {
		st := &s.Seats[i]
		if !st.inHand() {
			total += st.Bet
			continue
		}
		total += min(s.CurrentBet, st.Bet+st.Stack)
	}
	return total
}

// probe runs an action through the real engine and reports whether it would be
// accepted, plus the engine's own reason if not.
//
// No cloning needed: state is a JSON blob that `Apply` decodes into a fresh
// value every call, so a dry run physically cannot touch the caller's copy.
func probe(m *Module, raw module.State, playerID string, a module.Action) (bool, string) {
	if _, _, err := m.Apply(raw, playerID, a); err != nil {
		return false, module.CodeOf(err)
	}
	return true, ""
}
