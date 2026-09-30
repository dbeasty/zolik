package prsi

import "zolik/server/internal/module"

// What to do *instead* — the third thing a refused player needs, after the
// reason and the rule.
//
// Prší has three controls and a lot of obligations, so almost every refusal
// here is really an instruction wearing a code: an unanswered seven means take
// the cards, an ace means take the skip, a queen means name a suit. Each of
// those is one press away, and naming which press is the whole of this file.
//
// It decides nothing about legality — it reads the refusal that already
// happened and the offers that were already built.
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

		case ErrCardDoesNotFit:
			// What would fit, named: the suit in play and the rank on top.
			// "That card doesn't match the suit or the rank" says which test
			// failed and not what passes it, and a player holding eight cards
			// should not have to try them one at a time to find out.
			// The suit as a message key and the top card as a card code, so
			// the sentence reads "Play a Hearts card, or one matching the
			// seven of spades" rather than naming both in codes only this
			// server uses — the client's labels.ts resolves both shapes.
			o.Remedy = &module.Fact{
				LabelKey: "prsi.remedy.matchOrDraw",
				Params:   map[string]any{"suit": "suit." + s.suitInPlay(), "card": s.top()},
			}
			o.RemedyOfferID = enabled(OfferDraw)

		case ErrMustAnswerDraw:
			// How many cards it now costs, because the debt stacks: a seven
			// answered twice is six cards, and "take the cards" without the
			// figure is the one detail a player wants before deciding.
			o.Remedy = &module.Fact{
				LabelKey: "prsi.remedy.answerSevenOrTake",
				Params:   map[string]any{"n": s.PendingDraw},
			}
			o.RemedyOfferID = enabled(OfferDraw)

		case ErrNothingToSkip:
			o.Remedy = &module.Fact{
				LabelKey: "prsi.remedy.playOrDraw",
				Params:   map[string]any{"suit": "suit." + s.suitInPlay()},
			}
			o.RemedyOfferID = enabled(OfferPlay, OfferDraw)

		case ErrSuitRequired, ErrUnknownSuit:
			o.Remedy = &module.Fact{LabelKey: "prsi.remedy.nameASuit"}
			o.RemedyOfferID = enabled(OfferPlay)

		case ErrNothingToDraw:
			// The pile is out and cannot be recycled, so drawing is not a move
			// any more. Play, or take the skip.
			o.Remedy = &module.Fact{LabelKey: "prsi.remedy.nothingLeftToDraw"}
			o.RemedyOfferID = enabled(OfferPlay, OfferPass)
		}
	}
}
