package zolikmod

import (
	"zolik/server/internal/module"
	"zolik/server/internal/rules"
)

// What to do *instead* — the third thing a refused player needs, after the
// reason and the rule.
//
// A remedy is only worth sending when it names something the player can
// actually do right now, so most of it is a lookup against the offer list
// that was just built: if the move that fixes this is on offer, the interface
// can put a working control under the sentence rather than an instruction the
// player has to go and find. Where nothing fixes it — it is not your turn,
// the table is paused — there is no remedy and the field stays empty.
//
// Like ExplainRefusal, this decides nothing about legality. It reads the
// refusal that already happened and the offers that were already built.
func annotate(gs rules.GameState, playerID string, offers []module.ActionOffer) {
	cfg := rules.ResolveConfig(gs.Rules)

	enabled := map[string]bool{}
	for _, o := range offers {
		if o.Enabled {
			enabled[o.ID] = true
		}
	}
	// The first of these that is actually on offer. Order matters: undoing
	// the pickup is a smaller step than undoing the whole turn, and a player
	// offered the smallest way out takes back the least.
	firstEnabled := func(ids ...string) string {
		for _, id := range ids {
			if enabled[id] {
				return id
			}
		}
		return ""
	}

	for i := range offers {
		o := &offers[i]
		if o.Enabled || o.WhyNot == "" {
			continue
		}
		o.RuleIDs = explainRefusal(cfg, o.WhyNot)

		switch rules.RulesErrorCode(o.WhyNot) {

		case rules.ErrDiscardCardNotMelded:
			// The one refusal this whole feature was reported for. The card
			// is named because "the card you picked up" is not identification
			// in a thirteen-card hand.
			o.Remedy = &module.Fact{
				LabelKey: "zolik.remedy.meldThePickup",
				Params:   map[string]any{"card": gs.DiscardDrawnCardPendingMeld},
			}
			o.RemedyOfferID = firstEnabled(rules.OfferUndoDrawDiscard, rules.OfferUndoTurn)

		case rules.ErrDiscardTakenCard:
			o.Remedy = &module.Fact{
				LabelKey: "zolik.remedy.discardSomethingElse",
				Params:   map[string]any{"card": gs.DiscardTakenCard},
			}

		case rules.ErrIncompleteInitialMeld:
			o.Remedy = &module.Fact{LabelKey: "zolik.remedy.finishOrUndoLayDown"}
			o.RemedyOfferID = firstEnabled(rules.OfferUndoLayMeld, rules.OfferUndoTurn)

		case rules.ErrMeldBelowMinimum:
			// The gap, not the floor. A player who has laid 30 against a
			// floor of 35 is five points away; telling them the floor is 35
			// makes them do the arithmetic the server already did.
			short := cfg.InitialMeldMinimum - rules.PlayerInitialMeldNaturalValue(gs, playerID)
			if short < 1 {
				short = 1
			}
			o.Remedy = &module.Fact{
				LabelKey: "zolik.remedy.needMorePoints",
				Params:   map[string]any{"n": short},
			}

		case rules.ErrNeedCleanRun:
			o.Remedy = &module.Fact{LabelKey: "zolik.remedy.layACleanRun"}

		case rules.ErrRoundReqNotMet:
			o.Remedy = &module.Fact{LabelKey: "zolik.remedy.goDownFirst"}
			o.RemedyOfferID = firstEnabled(rules.OfferLayMeld)

		case rules.ErrMustDrawFirst, rules.ErrWrongPhase:
			if gs.Phase == rules.PhaseDraw {
				o.Remedy = &module.Fact{LabelKey: "zolik.remedy.drawFirst"}
				o.RemedyOfferID = firstEnabled(rules.OfferDrawDeck, rules.OfferDrawDiscard)
			}

		case rules.ErrDiscardLocked:
			o.Remedy = &module.Fact{
				LabelKey: "zolik.remedy.drawFromStock",
				Params:   map[string]any{"n": cfg.DiscardDrawMinRound},
			}
			o.RemedyOfferID = firstEnabled(rules.OfferDrawDeck)

		case rules.ErrDiscardPileEmpty:
			o.Remedy = &module.Fact{LabelKey: "zolik.remedy.drawFromStockEmpty"}
			o.RemedyOfferID = firstEnabled(rules.OfferDrawDeck)

		case rules.ErrJokerDiscard:
			o.Remedy = &module.Fact{LabelKey: "zolik.remedy.discardNotAJoker"}
		}
	}
}
