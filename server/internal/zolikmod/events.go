package zolikmod

import "zolik/server/internal/module"

// ProjectEvent keeps the card drawn blind from the deck with the player who
// drew it. Everyone else learns that a card was drawn from the player_drew
// event that follows it, which is all their board shows either.
func (m *Module) ProjectEvent(ev module.Event, viewerID string) (module.Event, bool) {
	if ev.Type == "draw_deck" && ev.Data["playerId"] != viewerID {
		return ev, false
	}
	return ev, true
}

// NarrateEvent says who did what, for the strip over the table. Only the moves
// that change what other players see are narrated. Draws, discards and
// lay-downs are; the end of a deal is not, because the results panel already
// covers it.
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
	case "player_drew":
		// From the discard pile, the draw_discard event says which card.
		if ev.Data["from"] != "deck" {
			return module.Move{}, false
		}
		move.Fact = module.Fact{LabelKey: "zolik.move.drewStock", Params: map[string]any{"player": player}}
	case "draw_discard":
		move.Fact = module.Fact{LabelKey: "zolik.move.tookDiscard", Params: map[string]any{
			"player": player, "card": ev.Data["card"],
		}}
	case "meld_played":
		move.Fact = module.Fact{LabelKey: "zolik.move.melded", Params: map[string]any{
			"player": player, "cards": ev.Data["cards"],
		}}
	case "card_laid_off":
		owner := meldOwner(raw, meld)
		if owner == "" || owner == player {
			move.Fact = module.Fact{LabelKey: "zolik.move.laidOffOwn", Params: map[string]any{
				"player": player, "cards": ev.Data["cards"],
			}}
		} else {
			move.Fact = module.Fact{LabelKey: "zolik.move.laidOff", Params: map[string]any{
				"player": player, "cards": ev.Data["cards"], "owner": owner,
			}}
		}
	case "joker_swapped":
		move.Fact = module.Fact{LabelKey: "zolik.move.swappedJoker", Params: map[string]any{
			"player": player, "card": ev.Data["card"],
		}}
	case "player_discarded":
		move.Fact = module.Fact{LabelKey: "zolik.move.discarded", Params: map[string]any{
			"player": player, "card": ev.Data["card"],
		}}
	case "undo_draw_discard", "undo_lay_off", "undo_lay_meld", "undo_turn":
		move.Fact = module.Fact{LabelKey: "zolik.move.undid", Params: map[string]any{"player": player}}
	default:
		return module.Move{}, false
	}
	return move, true
}

// meldOwner is whose spread a meld is in, or "" if it is no longer on the
// table.
func meldOwner(raw module.State, meldID string) string {
	if meldID == "" {
		return ""
	}
	s, err := decode(raw)
	if err != nil {
		return ""
	}
	for owner, metas := range s.Rules.MeldMeta {
		for _, meta := range metas {
			if meta.MeldID == meldID {
				return owner
			}
		}
	}
	return ""
}
