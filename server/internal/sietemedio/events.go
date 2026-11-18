package sietemedio

import "zolik/server/internal/module"

var _ module.EventNarrator = (*Module)(nil)

// NarrateEvent says what happened, one line a move, for the table's history.
//
// A seat tile shows a stake and a hand as they stand, not the order the table
// played them in, nor who asked for which card. Every card a draw names is a
// face-up one: only the first card dealt is ever face down in this game, and
// the deal is not narrated. A draw that goes over says so, because going over
// turns the whole hand face up. A draw that makes 7½ does not, since with the
// first card down that total is the player's secret until the hand is shown.
// Each key is written out at its call site so module.CollectKeys finds it.
func (m *Module) NarrateEvent(_ module.State, ev module.Event) (module.Move, bool) {
	player, _ := ev.Data["playerId"].(string)
	move := module.Move{PlayerID: player}
	switch ev.Type {
	case "stake_placed":
		move.Fact = module.Fact{LabelKey: "sietemedio.move.staked", Params: map[string]any{"player": player, "n": intOf(ev.Data["amount"])}}
	case "card_drawn":
		card, _ := ev.Data["card"].(string)
		if bust, _ := ev.Data["bust"].(bool); bust {
			move.Fact = module.Fact{LabelKey: "sietemedio.move.drewBust", Params: map[string]any{"player": player, "card": card}}
			move.Notable = true
		} else {
			move.Fact = module.Fact{LabelKey: "sietemedio.move.drew", Params: map[string]any{"player": player, "card": card}}
		}
	case "stood":
		move.Fact = module.Fact{LabelKey: "sietemedio.move.stood", Params: map[string]any{"player": player}}
	case "bank_turn":
		move.Fact = module.Fact{LabelKey: "sietemedio.move.bankTurn", Params: map[string]any{"player": player}}
	case "no_room":
		move.Fact = module.Fact{LabelKey: "sietemedio.move.noRoom", Params: map[string]any{"player": player}}
	case "leaving":
		move.Fact = module.Fact{LabelKey: "sietemedio.move.leaving", Params: map[string]any{"player": player}}
	case "left":
		move.Fact = module.Fact{LabelKey: "sietemedio.move.left", Params: map[string]any{"player": player, "n": intOf(ev.Data["chips"])}}
	case "round_settled":
		// The table's own line says the round is over; this one says how the
		// bank finished, which is what every stake was settled against.
		total := intOf(ev.Data["bankTotal"])
		if total > Target {
			move.Fact = module.Fact{LabelKey: "sietemedio.move.bankBust"}
		} else {
			move.Fact = module.Fact{LabelKey: "sietemedio.move.bankStands", Params: map[string]any{"total": halvesText(total)}}
		}
	default:
		return module.Move{}, false
	}
	return move, true
}

// intOf reads a number from an event, which is an int straight out of Apply
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
