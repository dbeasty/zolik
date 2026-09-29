package rummytiles

import "zolik/server/internal/module"

// NarrateEvent says who did what, for the strip over the table. Every tile it
// names is on the table, in the workspace everyone at it can see; the tile
// drawn from the pool is never named.
//
// Each key is written out at its call site: module.CollectKeys reads source,
// so a key picked into a variable first would never reach serverKeys.json.
func (m *Module) NarrateEvent(_ module.State, ev module.Event) (module.Move, bool) {
	player, _ := ev.Data["playerId"].(string)
	if player == "" {
		return module.Move{}, false
	}
	set, _ := ev.Data["setId"].(string)
	move := module.Move{PlayerID: player, GroupID: set}
	switch ev.Type {
	case "tiles_placed":
		move.Fact = module.Fact{LabelKey: "rummytiles.move.placed", Params: map[string]any{
			"player": player, "cards": ev.Data["cards"],
		}}
	case "tiles_added":
		move.Fact = module.Fact{LabelKey: "rummytiles.move.added", Params: map[string]any{
			"player": player, "cards": ev.Data["cards"],
		}}
	case "tiles_taken":
		move.Fact = module.Fact{LabelKey: "rummytiles.move.took", Params: map[string]any{
			"player": player, "cards": ev.Data["cards"],
		}}
	case "run_split":
		move.Fact = module.Fact{LabelKey: "rummytiles.move.split", Params: map[string]any{"player": player}}
	case "joker_swapped":
		move.Fact = module.Fact{LabelKey: "rummytiles.move.swappedJoker", Params: map[string]any{
			"player": player, "card": ev.Data["replacement"],
		}}
	case "turn_reset":
		move.Fact = module.Fact{LabelKey: "rummytiles.move.reset", Params: map[string]any{"player": player}}
	case "turn_committed":
		move.Fact = module.Fact{LabelKey: "rummytiles.move.committed", Params: map[string]any{"player": player}}
	case "tile_drawn":
		move.Fact = module.Fact{LabelKey: "rummytiles.move.drew", Params: map[string]any{"player": player}}
	default:
		return module.Move{}, false
	}
	return move, true
}
