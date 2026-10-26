package sietemedio

import "zolik/server/internal/module"

// What to do *instead* — the third thing a refused player needs, after the
// reason and the rule.
//
// It decides nothing about legality: it reads the refusal that already happened
// and the offers that were already built, and where nothing on the list fixes
// the refusal it says nothing.
//
// The commonest refusal at this table is WRONG_PHASE, and the code alone is the
// same sentence whether the answer is "put a stake up", "play the hand in front
// of you" or "wait, the bank is drawing" — which is the whole reason this file
// exists rather than a sentence per code in a locale bundle.
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
			case phaseStakes:
				o.Remedy = &module.Fact{
					LabelKey: "sietemedio.remedy.putAStakeUp",
					Params:   map[string]any{"n": s.MinStake},
				}
				o.RemedyOfferID = enabled(OfferStake)
			case phasePlay, phaseBank:
				o.Remedy = &module.Fact{LabelKey: "sietemedio.remedy.playThisHand"}
				o.RemedyOfferID = enabled(OfferHit, OfferStand)
			}

		case ErrNotYourTurn:
			o.Remedy = &module.Fact{LabelKey: "sietemedio.remedy.waitYourTurn"}

		case ErrAlreadyStaked:
			o.Remedy = &module.Fact{LabelKey: "sietemedio.remedy.stakeIsUp"}

		case ErrStakeTooSmall:
			o.Remedy = &module.Fact{
				LabelKey: "sietemedio.remedy.stakeAtLeast",
				Params:   map[string]any{"n": s.MinStake},
			}
			o.RemedyOfferID = enabled(OfferStake)

		case ErrNotEnoughChips:
			if seat != nil {
				o.Remedy = &module.Fact{
					LabelKey: "sietemedio.remedy.stakeAtMost",
					Params:   map[string]any{"n": seat.Stack},
				}
				o.RemedyOfferID = enabled(OfferStake)
			}

		// The bank's limit is somebody else's stack, so the way out is a
		// smaller stake and the figure is one the player could not have worked
		// out for themselves.
		case ErrBankCannotCover:
			if seat != nil {
				top := maxStakeFor(s, s.seatIndex(seat.PlayerID))
				o.Remedy = &module.Fact{
					LabelKey: "sietemedio.remedy.bankCoversAtMost",
					Params:   map[string]any{"n": top},
				}
				o.RemedyOfferID = enabled(OfferStake)
			}

		case ErrAmountRequired:
			o.Remedy = &module.Fact{
				LabelKey: "sietemedio.remedy.sayHowMuch",
				Params:   map[string]any{"n": s.MinStake},
			}
			o.RemedyOfferID = enabled(OfferStake)

		case ErrBankerDoesNotStake:
			o.Remedy = &module.Fact{LabelKey: "sietemedio.remedy.youHoldTheBank"}

		case ErrNotInRound:
			o.Remedy = &module.Fact{LabelKey: "sietemedio.remedy.waitForNextRound"}
		}
	}
}
