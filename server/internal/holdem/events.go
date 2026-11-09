package holdem

import "zolik/server/internal/module"

var _ module.EventNarrator = (*Module)(nil)

// NarrateEvent says what happened, one line a move, for the box every table
// shows between the board and the hand.
//
// Poker is a game of who did what: the seat tiles show the bets as they stand,
// but not the order they came in. A street gets a line of its own, with no
// player on it, so the cards that just landed are read even when the table
// itself has been scrolled out of view. Nothing here names a hole card. Each
// key is written out at its call site so module.CollectKeys finds it.
func (m *Module) NarrateEvent(_ module.State, ev module.Event) (module.Move, bool) {
	player, _ := ev.Data["playerId"].(string)
	move := module.Move{PlayerID: player}
	switch ev.Type {
	case "folded":
		move.Fact = module.Fact{LabelKey: "holdem.move.folded", Params: map[string]any{"player": player}}
	case "checked":
		move.Fact = module.Fact{LabelKey: "holdem.move.checked", Params: map[string]any{"player": player}}
	case "called":
		n := intOf(ev.Data["amount"])
		if allIn, _ := ev.Data["allIn"].(bool); allIn {
			move.Fact = module.Fact{LabelKey: "holdem.move.calledAllIn", Params: map[string]any{"player": player, "n": n}}
		} else {
			move.Fact = module.Fact{LabelKey: "holdem.move.called", Params: map[string]any{"player": player, "n": n}}
		}
	case "raised":
		n := intOf(ev.Data["to"])
		if allIn, _ := ev.Data["allIn"].(bool); allIn {
			move.Fact = module.Fact{LabelKey: "holdem.move.raisedAllIn", Params: map[string]any{"player": player, "n": n}}
		} else {
			move.Fact = module.Fact{LabelKey: "holdem.move.raised", Params: map[string]any{"player": player, "n": n}}
		}
	case "stood_pat":
		move.Fact = module.Fact{LabelKey: "holdem.move.stoodPat", Params: map[string]any{"player": player}}
	case "drew":
		n := intOf(ev.Data["count"])
		if n == 1 {
			move.Fact = module.Fact{LabelKey: "holdem.move.drewOne", Params: map[string]any{"player": player}}
		} else {
			move.Fact = module.Fact{LabelKey: "holdem.move.drewMany", Params: map[string]any{"player": player, "n": n}}
		}
	case "street":
		return narrateStreet(ev)
	default:
		return module.Move{}, false
	}
	return move, true
}

// narrateStreet is the cards a street put on the board. Draw's streets deal
// nothing to the table, so they have no line.
func narrateStreet(ev module.Event) (module.Move, bool) {
	board := boardOf(ev.Data["board"])
	street, _ := ev.Data["street"].(string)
	switch {
	case street == streetFlop && len(board) >= 3:
		return module.Move{Fact: module.Fact{LabelKey: "holdem.move.flop", Params: map[string]any{"cards": board[:3]}}}, true
	case street == streetTurn && len(board) >= 4:
		return module.Move{Fact: module.Fact{LabelKey: "holdem.move.turn", Params: map[string]any{"card": board[3]}}}, true
	case street == streetRiver && len(board) >= 5:
		return module.Move{Fact: module.Fact{LabelKey: "holdem.move.river", Params: map[string]any{"card": board[4]}}}, true
	}
	return module.Move{}, false
}

// boardOf reads the board from an event, which is a []string straight out of
// Apply and a []any once it has been through JSON.
func boardOf(v any) []string {
	switch cards := v.(type) {
	case []string:
		return cards
	case []any:
		out := make([]string, 0, len(cards))
		for _, c := range cards {
			if code, ok := c.(string); ok {
				out = append(out, code)
			}
		}
		return out
	}
	return nil
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
