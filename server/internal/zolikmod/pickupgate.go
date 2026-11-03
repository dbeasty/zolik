package zolikmod

import (
	"slices"

	"zolik/server/internal/ai"
	"zolik/server/internal/rules"
)

// gatePickup withdraws the discard-pile pickups that could only strand the
// player who took them.
//
// The engine lets a player who is not down take a card off the pile, but then
// refuses every discard until they are down (rules.ErrDiscardCardNotMelded).
// A pickup after which no opening exists is a dead end that only an undo gets
// out of, so it is not offered: the control stays dark rather than inviting a
// move the turn cannot finish.
//
// The question is the one the engine enforces — can the player go down this
// turn holding what the take brings — not whether the taken card is in the
// opening. The written rule asks for that too, but the engine accepts any
// opening, and withholding a move the engine would take is the worse error.
//
// This is the one offer this adapter decides rather than translates, because
// the question is a search, and the search lives with the agents.
// It errs towards offering: a search that cannot prove the opening impossible
// leaves the pickup enabled. The search is bounded per offer build, not per
// card, because this runs on every broadcast and under any-from-pile one
// build can ask about every card in the pile.
func gatePickup(gs rules.GameState, playerID string, offers []rules.ActionOffer) {
	gatePickupWithin(gs, playerID, offers, pickupGateSteps)
}

// errPickupUnplayable is why a pickup is withheld. No validator returns it —
// the draw itself is legal — so it is this module's own code rather than the
// engine's: the take could only end in rules.ErrDiscardCardNotMelded and
// an undo.
const errPickupUnplayable rules.RulesErrorCode = "DISCARD_PICKUP_UNPLAYABLE"

// pickupGateSteps bounds one gatePickup: each step is one candidate meld
// validated. An ordinary hand settles in a few hundred.
const pickupGateSteps = 20000

func gatePickupWithin(gs rules.GameState, playerID string, offers []rules.ActionOffer, steps int) {
	if gs.CurrentTurn != playerID || gs.Phase != rules.PhaseDraw || gs.RoundReqMet[playerID] {
		return
	}
	for i := range offers {
		o := &offers[i]
		if o.ID != rules.OfferDrawDiscard || !o.Enabled || o.Source == nil {
			continue
		}
		st := gs
		st.Rules = rules.ResolveConfig(gs.Rules)
		hand := gs.Hands[playerID]
		pile := gs.DiscardPile
		var keep []string
		var refused []rules.CardRefusal
		// From the top of the pile down: a shallow take is the common one and
		// the cheap one to search, so it is settled before a deep take that
		// brings half the pile into hand can spend the budget.
		for _, c := range slices.Backward(o.Source.Cards) {
			// Where the take starts, as rules.ValidateDraw finds it: the top
			// card, or under any-from-pile the named card and all above it.
			at := len(pile) - 1
			if st.Rules.DiscardPickupMode == rules.DiscardPickupAnyFromPile {
				if j := slices.Index(pile, c); j >= 0 {
					at = j
				}
			}
			combined := append(append([]string(nil), hand...), pile[at:]...)
			if ai.CanGoDown(st, playerID, combined, &steps) {
				keep = append(keep, c)
			} else {
				refused = append(refused, rules.CardRefusal{Card: c, WhyNot: errPickupUnplayable})
			}
		}
		if len(keep) == 0 {
			o.Enabled = false
			o.WhyNot = errPickupUnplayable
			o.Source.Cards = nil
			continue
		}
		slices.Reverse(keep)
		slices.Reverse(refused)
		o.Source.Cards = keep
		o.Source.Refused = append(o.Source.Refused, refused...)
	}
}
