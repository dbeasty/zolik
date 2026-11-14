package blackpeter

import "zolik/server/internal/module"

var _ module.EventNarrator = (*Module)(nil)

// NarrateEvent says what happened, one line a move, for the status box every
// table shows between the board and the hand. Which card was taken is never
// said — only the taker saw it — but a pair laid down is public, and so is
// who went out and who was left holding Black Peter. Each key is written out
// at its call site so module.CollectKeys finds it.
func (m *Module) NarrateEvent(raw module.State, ev module.Event) (module.Move, bool) {
	player, _ := ev.Data["playerId"].(string)
	if player == "" {
		return module.Move{}, false
	}
	move := module.Move{PlayerID: player}
	switch ev.Type {
	case "card_taken":
		from, _ := ev.Data["from"].(string)
		move.Fact = module.Fact{LabelKey: "blackpeter.move.took", Params: map[string]any{"player": player, "from": from}}
	case "pair_laid":
		move.Fact = module.Fact{LabelKey: "blackpeter.move.pair", Params: map[string]any{"player": player, "cards": ev.Data["cards"]}}
	case "pairs_laid":
		move.Fact = module.Fact{LabelKey: "blackpeter.move.dealtPairs", Params: map[string]any{"player": player, "n": intOf(ev.Data["count"])}}
	case "went_out":
		move.Fact = module.Fact{LabelKey: "blackpeter.move.out", Params: map[string]any{"player": player}}
	case "round_lost":
		move.Fact = module.Fact{LabelKey: "blackpeter.move.lost", Params: map[string]any{"player": player}}
		move.Notable = true
	default:
		return module.Move{}, false
	}
	return move, true
}

// intOf reads a count from an event, which is an int straight out of Apply
// and a float64 once it has been through JSON.
func intOf(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case float64:
		return int(n)
	}
	return 0
}
