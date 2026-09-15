package holdem

import "zolik/server/internal/module"

// What to do *instead* — the third thing a refused player needs, after the
// reason and the rule.
//
// Poker's refusals are all about a number, and the number is exactly what the
// code leaves out. "You can't check", "a raise has to be at least the last
// one", "you don't have that many chips" each become actionable the moment the
// figure is in the sentence — what is owed, what the smallest legal raise is,
// what the stack actually holds. The engine has all three already.
//
// It decides nothing about legality: it reads the refusal that already
// happened and the offers that were already built.
func (m *Module) annotate(cfg module.MatchConfig, s *GameState, seat *Seat, offers []module.ActionOffer) {
	stated := module.StatedRuleIDs(m, cfg)

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
		if seat == nil {
			continue
		}
		minTo, maxTo := raiseRange(s, seat)

		switch o.WhyNot {

		case ErrCannotCheck:
			o.Remedy = &module.Fact{
				LabelKey: "holdem.remedy.callOrFold",
				Params:   map[string]any{"n": s.toCall(seat)},
			}
			o.RemedyOfferID = enabled(OfferCall, OfferRaise, OfferFold)

		case ErrNothingToCall:
			o.Remedy = &module.Fact{LabelKey: "holdem.remedy.checkOrRaise"}
			o.RemedyOfferID = enabled(OfferCheck, OfferRaise)

		case ErrCannotRaise:
			// Everything this seat has still does not get above the bet, so
			// the raise control is not coming back this street. Calling all
			// in is the move, and saying so beats "you can't raise here".
			o.Remedy = &module.Fact{
				LabelKey: "holdem.remedy.callAllInOrFold",
				Params:   map[string]any{"n": s.toCall(seat)},
			}
			o.RemedyOfferID = enabled(OfferCall, OfferFold)

		case ErrRaiseTooSmall:
			o.Remedy = &module.Fact{
				LabelKey: "holdem.remedy.raiseAtLeast",
				Params:   map[string]any{"n": minTo},
			}
			o.RemedyOfferID = enabled(OfferRaise)

		case ErrNotEnoughChips:
			o.Remedy = &module.Fact{
				LabelKey: "holdem.remedy.raiseAtMost",
				Params:   map[string]any{"n": maxTo},
			}
			o.RemedyOfferID = enabled(OfferRaise)

		case ErrAmountRequired:
			o.Remedy = &module.Fact{
				LabelKey: "holdem.remedy.nameAnAmount",
				Params:   map[string]any{"min": minTo, "max": maxTo},
			}
			o.RemedyOfferID = enabled(OfferRaise)

		case ErrSeatNotInHand:
			o.Remedy = &module.Fact{LabelKey: "holdem.remedy.waitForNextHand"}
		}
	}
}

// configOf is the table a hand is actually being played at, rebuilt from the
// state's own resolved numbers rather than read back from the lobby — a match
// keeps the rules it was dealt under, so the rules a refusal points at have to
// be those same ones.
func configOf(s *GameState) module.MatchConfig {
	return module.MatchConfig{
		Variation: s.Variation,
		Options: module.Options{
			OptStartingStack: s.StartingStack,
			OptBigBlind:      s.BigBlind,
			OptHandLimit:     s.HandLimit,
		},
	}
}
