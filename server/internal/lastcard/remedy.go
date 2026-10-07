package lastcard

import "zolik/server/internal/module"

// annotate says what to do *instead* of a refused move — the third thing a
// refused player needs, after the reason and the rule. It decides nothing
// about legality: it reads the refusal that already happened and the offers
// that were already built.
func (m *Module) annotate(cfg module.MatchConfig, s *GameState, offers []module.ActionOffer) {
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

		switch o.WhyNot {
		case ErrCardDoesNotMatch:
			// What would fit, named: the colour in play and the card on top.
			o.Remedy = &module.Fact{
				LabelKey: "lastcard.remedy.matchOrDraw",
				Params:   map[string]any{"colour": colourNameKey(s.colourInPlay()), "card": s.top()},
			}
			o.RemedyOfferID = enabled(OfferDraw)

		case ErrDrawFourHeld:
			o.Remedy = &module.Fact{
				LabelKey: "lastcard.remedy.followColourFirst",
				Params:   map[string]any{"colour": colourNameKey(s.colourInPlay())},
			}
			o.RemedyOfferID = enabled(OfferDraw)

		case ErrOnlyDrawnCard:
			o.Remedy = &module.Fact{
				LabelKey: "lastcard.remedy.playDrawnOrKeep",
				Params:   map[string]any{"card": s.DrawnCard},
			}
			o.RemedyOfferID = enabled(OfferPass)

		case ErrAlreadyDrew:
			o.Remedy = &module.Fact{
				LabelKey: "lastcard.remedy.playDrawnOrKeep",
				Params:   map[string]any{"card": s.DrawnCard},
			}
			o.RemedyOfferID = enabled(OfferPlay, OfferPass)

		case ErrNothingToKeep:
			o.Remedy = &module.Fact{LabelKey: "lastcard.remedy.playOrDraw"}
			o.RemedyOfferID = enabled(OfferPlay, OfferDraw)

		case ErrAnswerDrawFour:
			// A Wild Draw Four is waiting on this player, and the only
			// moves are to take it or dispute it.
			o.Remedy = &module.Fact{
				LabelKey: "lastcard.remedy.answerDrawFour",
				Params:   map[string]any{"player": drawFourPlayer(s)},
			}
			o.RemedyOfferID = enabled(OfferAccept, OfferChallenge)

		case ErrAlreadyCalled:
			o.Remedy = &module.Fact{LabelKey: "lastcard.remedy.nowPlay"}
			o.RemedyOfferID = enabled(OfferPlay, OfferDraw)

		case ErrColourRequired, ErrUnknownColour:
			o.Remedy = &module.Fact{LabelKey: "lastcard.remedy.nameAColour"}
			o.RemedyOfferID = enabled(OfferPlay)
		}
	}
}

func drawFourPlayer(s *GameState) string {
	if s.DrawFour == nil {
		return ""
	}
	return s.DrawFour.Player
}
