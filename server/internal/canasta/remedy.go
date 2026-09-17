package canasta

import (
	"strings"

	"zolik/server/internal/module"
)

// What to do *instead* — the third thing a refused player needs, after the
// reason and the rule.
//
// The reason says what happened and the rule says why it is so; neither says
// what to press. "WRONG_PHASE" is the case this was written for: it is the
// refusal a player meets most often in this game and the one a code explains
// least, because it means "you have not drawn yet" and "you have already
// drawn" at the same table ten seconds apart. A remedy says which.
//
// A remedy is only worth sending when it names something the player can do
// right now, so most of it is a lookup against the offer list that was just
// built — if the move that fixes this is on offer, the interface can put a
// working control under the sentence rather than an instruction the player
// has to go and find. Where nothing fixes it (it is not your turn, the table
// is between deals) there is no remedy and the field stays empty.
//
// Like the rule index, this decides nothing about legality. It reads the
// refusal that already happened and the offers that were already built.
func (m *Module) annotate(s *GameState, playerID string, offers []module.ActionOffer) {
	v := s.rules()
	t := s.team(playerID)
	stated := m.statedRules(s)

	// The first offer on this list that is both enabled and the move named —
	// by id, or by id prefix, because one move can be several offers. A
	// capture is "take_pile:naturals" or "take_pile:meld:<id>" depending on
	// how it is paid for, and a remedy that said "take the pile" would
	// otherwise have no control to point at on the one turn it matters.
	firstEnabled := func(ids ...string) string {
		for _, want := range ids {
			for _, o := range offers {
				if !o.Enabled {
					continue
				}
				if o.ID == want || strings.HasPrefix(o.ID, want+":") {
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
		o.RuleIDs = explainWith(stated, v, o.WhyNot)

		switch o.WhyNot {

		case ErrWrongPhase:
			// The one refusal that means opposite things at opposite ends of
			// the same turn, which is why it is answered from the phase
			// rather than from the code.
			if s.Phase == phaseDraw {
				o.Remedy = &module.Fact{LabelKey: "canasta.remedy.drawOrTakePile"}
				o.RemedyOfferID = firstEnabled(OfferDraw, OfferTakePile, OfferTakeTop)
			} else {
				o.Remedy = &module.Fact{LabelKey: "canasta.remedy.meldOrDiscard"}
				o.RemedyOfferID = firstEnabled(OfferDiscard, OfferLayMeld)
			}

		case ErrNothingToDraw:
			// The stock is out. The pile is the only way into a hand now, and
			// saying so beats "there is nothing left to draw" — which reads
			// like the deal is over when it is not.
			o.Remedy = &module.Fact{LabelKey: "canasta.remedy.takePileInstead"}
			o.RemedyOfferID = firstEnabled(OfferTakePile, OfferTakeTop)

		case ErrPileEmpty:
			o.Remedy = &module.Fact{LabelKey: "canasta.remedy.drawFromStock"}
			o.RemedyOfferID = firstEnabled(OfferDraw)

		case ErrPileBlocked:
			o.Remedy = &module.Fact{LabelKey: "canasta.remedy.pileBlocked"}
			o.RemedyOfferID = firstEnabled(OfferDraw)

		case ErrPileFrozen:
			// The top card itself rather than its rank: a client renders a
			// card code as the card, so "two natural cards matching K of
			// spades" is a sentence a player can check against the pile in
			// front of them without first translating a letter.
			o.Remedy = &module.Fact{
				LabelKey: "canasta.remedy.pileFrozen",
				Params:   map[string]any{"card": s.top()},
			}
			o.RemedyOfferID = firstEnabled(OfferDraw)

		case ErrTopCardUnusable:
			o.Remedy = &module.Fact{
				LabelKey: "canasta.remedy.topCardUnusable",
				Params:   map[string]any{"card": s.top()},
			}
			o.RemedyOfferID = firstEnabled(OfferDraw)

		case ErrCaptureNeedsTwo:
			// The commonest refusal in the game, and the one a player is most
			// likely to read as "the software is wrong": the pile is right
			// there, and nothing on screen said what it costs.
			o.Remedy = &module.Fact{
				LabelKey: "canasta.remedy.needTwoMatching",
				Params:   map[string]any{"card": s.top()},
			}
			o.RemedyOfferID = firstEnabled(OfferDraw)

		case ErrMeldCaptureNotAllowed:
			o.Remedy = &module.Fact{
				LabelKey: "canasta.remedy.captureFromHand",
				Params:   map[string]any{"card": s.top()},
			}
			o.RemedyOfferID = firstEnabled(OfferTakePile, OfferDraw)

		case ErrInitialMeldNotMet:
			// The gap first, because it is what the player acts on: a side that
			// has laid 30 against a floor of 50 is twenty points away, and
			// telling them only the floor makes them do arithmetic the server
			// already did. The floor as well, because the gap on its own reads
			// like one — "your first meld is 40 points short" and a board
			// saying the minimum is 150 were mistaken for a rule that said 40.
			if t != nil {
				floor := v.meldFloor(t.Score)
				short := floor - s.LaidThisTurn
				if short < 1 {
					short = 1
				}
				o.Remedy = &module.Fact{
					LabelKey: "canasta.remedy.needMorePoints",
					Params:   map[string]any{"n": short, "floor": floor},
				}
			}
			// Forward first, then back. Melding on is what a side short of the
			// floor normally does; taking the melds off is for the turn that
			// cannot — which, until there was an undo, was a turn with no move
			// at all.
			o.RemedyOfferID = firstEnabled(OfferLayMeld, OfferUndoLayMeld)

		case ErrMustMeldFirst:
			o.Remedy = &module.Fact{LabelKey: "canasta.remedy.openFirst"}
			o.RemedyOfferID = firstEnabled(OfferLayMeld)

		case ErrCannotGoOutYet:
			if t != nil {
				short := s.CanastasToGoOut - t.canastas()
				if short < 1 {
					short = 1
				}
				o.Remedy = &module.Fact{
					LabelKey: "canasta.remedy.needCanastas",
					Params:   map[string]any{"n": short, "size": canastaSize},
				}
			}

		case ErrMustKeepACard:
			o.Remedy = &module.Fact{LabelKey: "canasta.remedy.keepACard"}

		case ErrRankAlreadyMelded:
			// Named by the meld it would go onto rather than by the rank
			// alone, so the control under the sentence is the lay-off that
			// does it — the move the player wanted in the first place.
			o.Remedy = &module.Fact{LabelKey: "canasta.remedy.layOffInstead"}
			o.RemedyOfferID = firstEnabled("lay_off")

		case ErrMeldClosed:
			o.Remedy = &module.Fact{
				LabelKey: "canasta.remedy.meldClosed",
				Params:   map[string]any{"n": canastaSize},
			}
			o.RemedyOfferID = firstEnabled(OfferLayMeld, "lay_off")

		case ErrCannotDiscardThree:
			o.Remedy = &module.Fact{LabelKey: "canasta.remedy.discardNotARedThree"}
			o.RemedyOfferID = firstEnabled(OfferDiscard)

		case ErrBlackThreeGoOutOnly:
			o.Remedy = &module.Fact{LabelKey: "canasta.remedy.blackThreesOnTheWayOut"}

		case ErrNotYourMeld:
			o.Remedy = &module.Fact{LabelKey: "canasta.remedy.ownMeldsOnly"}
			o.RemedyOfferID = firstEnabled("lay_off")
		}
	}
}
