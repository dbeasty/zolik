package ginrummy

import "zolik/server/internal/module"

// NarrateEvent says who did what, for the strip over the table. The end of a
// hand is not narrated; the results panel already covers it.
//
// Each key is written out at its call site: module.CollectKeys reads source,
// so a key picked into a variable first would never reach serverKeys.json.
func (m *Module) NarrateEvent(raw module.State, ev module.Event) (module.Move, bool) {
	player, _ := ev.Data["playerId"].(string)
	if player == "" {
		return module.Move{}, false
	}
	meld, _ := ev.Data["meldId"].(string)
	move := module.Move{PlayerID: player, GroupID: meld}
	switch ev.Type {
	case "card_drawn":
		// A card from the discard pile was face up, so it is named; one from
		// the stock is not.
		if card, _ := ev.Data["card"].(string); card != "" {
			move.Fact = module.Fact{LabelKey: "ginrummy.move.tookDiscard", Params: map[string]any{
				"player": player, "card": card,
			}}
		} else {
			move.Fact = module.Fact{LabelKey: "ginrummy.move.drewStock", Params: map[string]any{"player": player}}
		}
	case "upcard_taken":
		move.Fact = module.Fact{LabelKey: "ginrummy.move.tookUpcard", Params: map[string]any{
			"player": player, "card": ev.Data["card"],
		}}
	case "upcard_passed":
		move.Fact = module.Fact{LabelKey: "ginrummy.move.passedUpcard", Params: map[string]any{"player": player}}
	case "card_discarded":
		move.Fact = module.Fact{LabelKey: "ginrummy.move.discarded", Params: map[string]any{
			"player": player, "card": ev.Data["card"],
		}}
	case "knocked":
		if gin, _ := ev.Data["gin"].(bool); gin {
			move.Fact = module.Fact{LabelKey: "ginrummy.move.wentGin", Params: map[string]any{"player": player}}
		} else {
			move.Fact = module.Fact{LabelKey: "ginrummy.move.knocked", Params: map[string]any{
				"player": player, "deadwood": ev.Data["deadwood"],
			}}
		}
	case "laid_off":
		// Only the knocker has melds to lay off on, and only the other player
		// lays off.
		owner := ""
		if s, err := decode(raw); err == nil {
			owner = other(s.Players, player)
		}
		move.Fact = module.Fact{LabelKey: "ginrummy.move.laidOff", Params: map[string]any{
			"player": player, "card": ev.Data["card"], "owner": owner,
		}}
	default:
		return module.Move{}, false
	}
	return move, true
}
