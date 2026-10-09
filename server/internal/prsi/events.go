package prsi

import "zolik/server/internal/module"

var _ module.EventNarrator = (*Module)(nil)

// NarrateEvent says what happened, one line a move, for the status box every
// table shows between the board and the hand.
//
// Each line says the play and what it now asks of whom — a seven that the
// next player must answer or pay for, an ace they must answer or sit out, a
// queen's suit — read from the state after the move, which is where the
// stacked debt and the player now facing it are. Each key is written out at
// its call site so module.CollectKeys finds it.
func (m *Module) NarrateEvent(raw module.State, ev module.Event) (module.Move, bool) {
	player, _ := ev.Data["playerId"].(string)
	if player == "" {
		return module.Move{}, false
	}
	s, err := decode(raw)
	if err != nil {
		return module.Move{}, false
	}
	move := module.Move{PlayerID: player}
	switch ev.Type {
	case "card_played":
		card, _ := ev.Data["card"].(string)
		switch {
		case s.Status == "completed" && s.WinnerID == player:
			move.Fact = module.Fact{LabelKey: "prsi.move.wentOut", Params: map[string]any{"player": player, "card": card}}
		case rankOf(card) == rankDrawTwo:
			move.Fact = module.Fact{LabelKey: "prsi.move.playedSeven", Params: map[string]any{
				"player": player, "card": card, "target": s.Current, "n": s.PendingDraw,
			}}
		case rankOf(card) == rankSkip:
			move.Fact = module.Fact{LabelKey: "prsi.move.playedAce", Params: map[string]any{
				"player": player, "card": card, "target": s.Current,
			}}
		case rankOf(card) == rankWild:
			suit, _ := ev.Data["declaredSuit"].(string)
			move.Fact = module.Fact{LabelKey: "prsi.move.playedQueen", Params: map[string]any{
				"player": player, "card": card, "suit": module.GermanSuitKey(suit),
			}}
		default:
			move.Fact = module.Fact{LabelKey: "prsi.move.played", Params: map[string]any{"player": player, "card": card}}
		}
	case "cards_drawn":
		switch n := intOf(ev.Data["count"]); {
		case n == 1:
			move.Fact = module.Fact{LabelKey: "prsi.move.drew", Params: map[string]any{"player": player}}
		case n > 1:
			move.Fact = module.Fact{LabelKey: "prsi.move.tookMany", Params: map[string]any{"player": player, "n": n}}
		default:
			return module.Move{}, false
		}
	case "turn_skipped":
		move.Fact = module.Fact{LabelKey: "prsi.move.missedTurn", Params: map[string]any{"player": player}}
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
