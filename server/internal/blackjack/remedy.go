package blackjack

import "zolik/server/internal/module"

// What to do *instead* — the third thing a refused player needs, after the
// reason and the rule.
//
// This module had the rule half from the start (ruleIDsFor) and not this one,
// which left the commonest refusal at the table saying nothing useful: every
// control that is not the one this phase wants comes back WRONG_PHASE, and
// "not available right now" is the same sentence whether the answer is "put a
// stake up", "say yes or no to insurance" or "wait, the dealer is drawing".
//
// It decides nothing about legality: it reads the refusal that already
// happened and the offers that were already built, and where nothing on the
// list fixes the refusal it says nothing.
func (m *Module) annotate(s *GameState, seat *Seat, offers []module.ActionOffer) {
	enabled := func(ids ...string) string {
		for _, want := range ids {
			for _, o := range offers {
				if o.Enabled && o.ID == want {
					return o.ID
				}
			}
		}
		return ""
	}

	for i := range offers {
		o := &offers[i]
		if o.Enabled || o.WhyNot == "" {
			continue
		}
		// Set here as well as at each build site, so an offer added later
		// cannot ship a bare code by forgetting the one line.
		o.RuleIDs = ruleIDsFor(s, o.WhyNot)

		switch o.WhyNot {

		case ErrWrongPhase:
			switch s.Phase {
			case phaseBets:
				o.Remedy = &module.Fact{
					LabelKey: "blackjack.remedy.putAStakeUp",
					Params:   map[string]any{"n": s.MinBet},
				}
				o.RemedyOfferID = enabled(OfferBet)
			case phaseInsurance:
				o.Remedy = &module.Fact{LabelKey: "blackjack.remedy.answerInsurance"}
				o.RemedyOfferID = enabled(OfferInsure, OfferDecline)
			case phasePlay:
				o.Remedy = &module.Fact{LabelKey: "blackjack.remedy.playThisHand"}
				o.RemedyOfferID = enabled(OfferHit, OfferStand)
			}

		case ErrAlreadyBet:
			o.Remedy = &module.Fact{LabelKey: "blackjack.remedy.stakeIsUp"}

		case ErrBetTooSmall:
			o.Remedy = &module.Fact{
				LabelKey: "blackjack.remedy.stakeAtLeast",
				Params:   map[string]any{"n": s.MinBet},
			}
			o.RemedyOfferID = enabled(OfferBet)

		case ErrNotEnoughChips:
			if seat != nil {
				o.Remedy = &module.Fact{
					LabelKey: "blackjack.remedy.stakeAtMost",
					Params:   map[string]any{"n": seat.Stack},
				}
				o.RemedyOfferID = enabled(OfferBet)
			}

		case ErrAmountRequired:
			o.Remedy = &module.Fact{
				LabelKey: "blackjack.remedy.sayHowMuch",
				Params:   map[string]any{"n": s.MinBet},
			}
			o.RemedyOfferID = enabled(OfferBet)

		// The three optional moves. Each is refused for one of two quite
		// different reasons — this table never offers it, or this hand is
		// past the point where it is allowed — and the rule the sheet shows
		// alongside already distinguishes them, so the remedy says the part
		// that is the same either way: what is left to do with the hand.
		case ErrCannotDouble, ErrCannotSplit, ErrCannotSurrender:
			o.Remedy = &module.Fact{LabelKey: "blackjack.remedy.hitOrStand"}
			o.RemedyOfferID = enabled(OfferHit, OfferStand)

		case ErrInsuranceClosed:
			o.Remedy = &module.Fact{LabelKey: "blackjack.remedy.playThisHand"}
			o.RemedyOfferID = enabled(OfferHit, OfferStand)

		case ErrNotInRound:
			o.Remedy = &module.Fact{LabelKey: "blackjack.remedy.waitForNextRound"}
		}
	}
}
