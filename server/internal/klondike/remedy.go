package klondike

import "zolik/server/internal/module"

// annotate puts the rule and the way out on every disabled offer — what to do
// instead of what was refused. It decides nothing about legality: it reads the
// refusal that already happened and the offers already built.
func (m *Module) annotate(s *GameState, offers []module.ActionOffer) {
	enabled := func(id string) string {
		for _, o := range offers {
			if o.Enabled && o.ID == id {
				return o.ID
			}
		}
		return ""
	}
	for i := range offers {
		o := &offers[i]
		if o.Enabled || o.WhyNot == "" {
			continue
		}
		o.RuleIDs = ruleIDsFor(s, o.WhyNot)
		switch o.WhyNot {
		case ErrStockEmpty:
			if id := enabled(OfferRecycle); id != "" {
				o.Remedy = &module.Fact{LabelKey: "klondike.remedy.turnWasteOver"}
				o.RemedyOfferID = id
			} else {
				o.Remedy = &module.Fact{LabelKey: "klondike.remedy.playTheTable"}
			}
		case ErrStockNotEmpty:
			o.Remedy = &module.Fact{LabelKey: "klondike.remedy.drawFirst"}
			o.RemedyOfferID = enabled(OfferDraw)
		case ErrWasteEmpty:
			o.Remedy = &module.Fact{LabelKey: "klondike.remedy.drawFirst"}
			o.RemedyOfferID = enabled(OfferDraw)
		case ErrNoRedeals:
			o.Remedy = &module.Fact{LabelKey: "klondike.remedy.playTheTable"}
		case ErrNotFinishable:
			o.Remedy = &module.Fact{LabelKey: "klondike.remedy.turnEverythingUp"}
		case ErrNothingToUndo:
			o.Remedy = &module.Fact{LabelKey: "klondike.remedy.playTheTable"}
		}
	}
}
