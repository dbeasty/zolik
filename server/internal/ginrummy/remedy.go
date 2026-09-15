package ginrummy

import "zolik/server/internal/module"

// What to do *instead* — the third thing a refused player needs, after the
// reason and the rule.
//
// Gin Rummy's turn is four states deep (take the upcard, pass it, draw,
// discard) and every one of them refuses the other three with the same
// WRONG_PHASE. A code that means "take the upcard or pass" at the start of a
// hand and "you have already drawn" thirty seconds later is not an
// explanation; the phase is, which is what this reads.
//
// Like the rule index, it decides nothing about legality: it reads the refusal
// that already happened and the offers that were already built, and where
// nothing on the list fixes the refusal it says nothing.
func (m *Module) annotate(s *GameState, offers []module.ActionOffer) {
	stated := module.StatedRuleIDs(m, configOf(s))

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
		o.RuleIDs = explainWith(stated, o.WhyNot)

		switch o.WhyNot {

		case ErrWrongPhase:
			switch s.Phase {
			case phaseUpcardNonDealer, phaseUpcardDealer:
				o.Remedy = &module.Fact{LabelKey: "ginrummy.remedy.takeOrPassUpcard"}
				o.RemedyOfferID = enabled(OfferDrawDiscard, OfferPassUpcard)
			case phaseDraw:
				o.Remedy = &module.Fact{LabelKey: "ginrummy.remedy.drawFirst"}
				o.RemedyOfferID = enabled(OfferDrawStock, OfferDrawDiscard)
			case phaseDiscard:
				o.Remedy = &module.Fact{LabelKey: "ginrummy.remedy.discardToEndTurn"}
				o.RemedyOfferID = enabled(OfferDiscard)
			case phaseLayoff:
				o.Remedy = &module.Fact{LabelKey: "ginrummy.remedy.finishLayoff"}
				o.RemedyOfferID = enabled(OfferFinishLayoff)
			}

		case ErrUpcardDeclined:
			// Both players passed this card, so the pile is shut for this
			// draw only. Saying which draw is left beats "you already passed
			// on that card", which leaves a player looking for a move.
			o.Remedy = &module.Fact{LabelKey: "ginrummy.remedy.stockDrawForced"}
			o.RemedyOfferID = enabled(OfferDrawStock)

		case ErrNothingToDraw:
			o.Remedy = &module.Fact{LabelKey: "ginrummy.remedy.drawElsewhere"}
			o.RemedyOfferID = enabled(OfferDrawStock, OfferDrawDiscard)

		case ErrDeadwoodTooHigh:
			// The limit, which is a number the player cannot see anywhere
			// else at an Oklahoma table — there it is set by the upcard and
			// changes every hand.
			o.Remedy = &module.Fact{
				LabelKey: "ginrummy.remedy.getDeadwoodDown",
				Params:   map[string]any{"n": s.KnockLimit},
			}

		case ErrCardDoesNotExtendMeld:
			o.Remedy = &module.Fact{LabelKey: "ginrummy.remedy.finishLayoff"}
			o.RemedyOfferID = enabled(OfferFinishLayoff)
		}
	}
}

// configOf is the table a hand is actually being played at, rebuilt from the
// state's own resolved options rather than read back from the lobby.
//
// A match keeps the rules it was dealt under — the lobby's options may have
// moved since — so the rules a refusal points at have to be these ones. The
// raw knock-limit option is used rather than this hand's effective limit,
// because Oklahoma's limit changes with the upcard and it is the *rule* that
// is being addressed, not the number.
func configOf(s *GameState) module.MatchConfig {
	return module.MatchConfig{
		Variation: s.Variation,
		Options: module.Options{
			OptTargetScore: s.TargetScore,
			OptKnockLimit:  s.KnockLimitOpt,
			OptBigGin:      module.BoolOpt(s.BigGin),
			OptLineBonuses: module.BoolOpt(s.LineBonuses),
		},
	}
}
