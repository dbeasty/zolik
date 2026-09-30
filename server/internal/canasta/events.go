package canasta

import "zolik/server/internal/module"

// NarrateEvent says who did what, for the strip over the table. Melds belong
// to a partnership, so the board cannot say which partner laid a card on
// one; the event can.
//
// Each key is written out at its call site: module.CollectKeys reads source,
// so a key picked into a variable first would never reach serverKeys.json.
func (m *Module) NarrateEvent(_ module.State, ev module.Event) (module.Move, bool) {
	player, _ := ev.Data["playerId"].(string)
	if player == "" {
		return module.Move{}, false
	}
	meld, _ := ev.Data["meldId"].(string)
	move := module.Move{PlayerID: player, GroupID: meld}
	switch ev.Type {
	case "cards_drawn":
		// Only a count travels; the cards stay in the drawer's hand.
		move.Fact = module.Fact{LabelKey: "canasta.move.drewStock", Params: map[string]any{"player": player}}
	case "red_threes_laid":
		move.Fact = module.Fact{LabelKey: "canasta.move.redThrees", Params: map[string]any{
			"player": player, "cards": ev.Data["cards"],
		}}
	case "pile_taken":
		move.Fact = module.Fact{LabelKey: "canasta.move.tookPile", Params: map[string]any{
			"player": player, "card": ev.Data["top"],
		}}
	case "top_card_taken":
		move.Fact = module.Fact{LabelKey: "canasta.move.tookTopCard", Params: map[string]any{
			"player": player, "card": ev.Data["card"],
		}}
	case "meld_laid":
		move.Fact = module.Fact{LabelKey: "canasta.move.melded", Params: map[string]any{
			"player": player, "cards": ev.Data["cards"],
		}}
	case "cards_laid_off":
		move.Fact = module.Fact{LabelKey: "canasta.move.laidOff", Params: map[string]any{
			"player": player, "cards": ev.Data["cards"],
		}}
	case "card_discarded":
		move.Fact = module.Fact{LabelKey: "canasta.move.discarded", Params: map[string]any{
			"player": player, "card": ev.Data["card"],
		}}
	case "take_pile_undone", "lay_off_undone", "meld_undone":
		move.Fact = module.Fact{LabelKey: "canasta.move.undid", Params: map[string]any{"player": player}}
	default:
		return module.Move{}, false
	}
	return move, true
}
