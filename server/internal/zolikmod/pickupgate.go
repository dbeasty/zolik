package zolikmod

import (
	"slices"

	"zolik/server/internal/ai"
	"zolik/server/internal/rules"
)

// gatePickup withdraws the discard-pile pickups that would bring the player a
// card they cannot use.
//
// A card is only worth taking off the pile if it goes somewhere this turn:
//
//   - for a player not yet down, into the opening that takes them down. The
//     engine refuses every discard after the take until they are down
//     (rules.ErrDiscardCardNotMelded), so a take with no opening is a dead end
//     only an undo gets out of — and one whose opening leaves the taken card
//     in hand is the stock's card bought at the price of telling the table.
//     The opening has to hold the taken card, as the written rule says;
//   - for a player already down, onto a meld on the table or into a new meld
//     with cards in hand. A card that fits nowhere is just the card the
//     previous player threw away, now held for at least a turn
//     (rules.ErrDiscardTakenCard) instead of the stock's unknown one.
//
// The engine still accepts any of these takes; this decides only what is
// offered, so the control stays dark rather than inviting a move with no
// purpose.
//
// This is the one offer this adapter decides rather than translates, because
// the question is a search, and the search lives with the agents.
// It errs towards offering: a search that cannot prove the card useless
// leaves the pickup enabled. The search is bounded per offer build, not per
// card, because this runs on every broadcast and under any-from-pile one
// build can ask about every card in the pile.
func gatePickup(gs rules.GameState, playerID string, offers []rules.ActionOffer) {
	gatePickupWithin(gs, playerID, offers, pickupGateSteps)
}

// errPickupUnplayable is why a pickup is withheld. No validator returns it —
// the draw itself is legal — so it is this module's own code rather than the
// engine's: the card taken would have nowhere to go this turn.
const errPickupUnplayable rules.RulesErrorCode = "DISCARD_PICKUP_UNPLAYABLE"

// pickupGateSteps bounds one gatePickup: each step is one candidate meld
// validated. An ordinary hand settles in a few hundred.
const pickupGateSteps = 20000

func gatePickupWithin(gs rules.GameState, playerID string, offers []rules.ActionOffer, steps int) {
	if gs.CurrentTurn != playerID || gs.Phase != rules.PhaseDraw {
		return
	}
	down := gs.RoundReqMet[playerID]
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
			usable := false
			if down {
				usable = ai.PickupFits(st, combined, c, &steps)
			} else {
				usable = ai.CanGoDownUsing(st, playerID, combined, c, &steps)
			}
			if usable {
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
