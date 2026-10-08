package lastcard

import "zolik/server/internal/module"

var _ module.EventNarrator = (*Module)(nil)

// NarrateEvent says what happened, one line a move, for the status box every
// table shows between the board and the hand.
//
// This game moves fast — a Skip, a Reverse and a Draw Two can all go down
// between two of a player's turns — and the board shows only where it ended:
// one card on the pile, a direction, a colour. Each line therefore says the
// play *and what it did*, so a player reading back to their own last move
// sees the whole chain: who reversed play, who drew two, what colour a wild
// named. The consequences an engine records as events of their own (the
// draw, the lost turn) are marked as effects and not narrated twice.
//
// Each key is written out at its call site: module.CollectKeys reads source,
// so a key picked into a variable would never reach serverKeys.json.
func (m *Module) NarrateEvent(raw module.State, ev module.Event) (module.Move, bool) {
	if effect, _ := ev.Data["effect"].(bool); effect {
		return module.Move{}, false
	}
	player, _ := ev.Data["playerId"].(string)
	switch ev.Type {
	case "card_played":
		return narratePlay(player, ev.Data), true

	case "cards_drawn":
		n := intOf(ev.Data["count"])
		switch {
		case n == 0:
			return module.Move{PlayerID: player, Fact: module.Fact{
				LabelKey: "lastcard.move.nothingToDraw", Params: map[string]any{"player": player},
			}}, true
		case n == 1:
			return module.Move{PlayerID: player, Fact: module.Fact{
				LabelKey: "lastcard.move.drew", Params: map[string]any{"player": player},
			}}, true
		}
		return module.Move{PlayerID: player, Fact: module.Fact{
			LabelKey: "lastcard.move.drewMany", Params: map[string]any{"player": player, "n": n},
		}}, true

	case "card_kept":
		return module.Move{PlayerID: player, Fact: module.Fact{
			LabelKey: "lastcard.move.kept", Params: map[string]any{"player": player},
		}}, true

	case "stack_taken":
		return module.Move{PlayerID: player, Fact: module.Fact{
			LabelKey: "lastcard.move.tookStack", Params: map[string]any{"player": player, "n": ev.Data["count"]},
		}}, true

	case "draw_four_accepted":
		return module.Move{PlayerID: player, Fact: module.Fact{
			LabelKey: "lastcard.move.accepted", Params: map[string]any{"player": player, "n": ev.Data["count"]},
		}}, true

	case "last_card_called":
		return module.Move{PlayerID: player, Fact: module.Fact{
			LabelKey: "lastcard.move.called", Params: map[string]any{"player": player},
		}}, true

	case "caught":
		by, _ := ev.Data["by"].(string)
		return module.Move{PlayerID: by, Fact: module.Fact{
			LabelKey: "lastcard.move.caught", Params: map[string]any{"player": by, "target": player, "n": ev.Data["count"]},
		}}, true

	case "challenge_won":
		return module.Move{PlayerID: player, Fact: module.Fact{
			LabelKey: "lastcard.move.challengeWon", Params: map[string]any{"player": player, "target": ev.Data["against"], "n": ev.Data["count"]},
		}}, true

	case "challenge_lost":
		return module.Move{PlayerID: player, Fact: module.Fact{
			LabelKey: "lastcard.move.challengeLost", Params: map[string]any{"player": player, "target": ev.Data["against"], "n": ev.Data["count"]},
		}}, true

	case "hands_swapped":
		return module.Move{PlayerID: player, Fact: module.Fact{
			LabelKey: "lastcard.move.swapped", Params: map[string]any{"player": player, "target": ev.Data["with"]},
		}}, true

	case "hands_passed":
		return module.Move{PlayerID: player, Fact: module.Fact{
			LabelKey: "lastcard.move.passedHands", Params: map[string]any{"player": player},
		}}, true

	case "deal_ended":
		winner, _ := ev.Data["winnerId"].(string)
		return module.Move{PlayerID: winner, Fact: module.Fact{
			LabelKey: "lastcard.move.wentOut", Params: map[string]any{"player": winner, "n": ev.Data["points"]},
		}}, true
	}
	return module.Move{}, false
}

// narratePlay is one card played and everything it did, as one line.
func narratePlay(player string, d map[string]any) module.Move {
	card := d["card"]
	victim := d["victim"]
	colour := ""
	if c, _ := d["declaredColour"].(string); c != "" {
		colour = colourKey(c)
	}
	effect, _ := d["effect"].(string)
	move := module.Move{PlayerID: player}

	switch effect {
	case effectSkip:
		move.Fact = module.Fact{LabelKey: "lastcard.move.playedSkip", Params: map[string]any{
			"player": player, "card": card, "target": victim,
		}}
	case effectReverse:
		if cw, _ := d["clockwise"].(bool); cw {
			move.Fact = module.Fact{LabelKey: "lastcard.move.playedReverseClockwise", Params: map[string]any{
				"player": player, "card": card,
			}}
		} else {
			move.Fact = module.Fact{LabelKey: "lastcard.move.playedReverseAnticlockwise", Params: map[string]any{
				"player": player, "card": card,
			}}
		}
	case effectDraw:
		if colour != "" {
			move.Fact = module.Fact{LabelKey: "lastcard.move.playedDrawColour", Params: map[string]any{
				"player": player, "card": card, "target": victim, "n": d["n"], "colour": colour,
			}}
		} else {
			move.Fact = module.Fact{LabelKey: "lastcard.move.playedDraw", Params: map[string]any{
				"player": player, "card": card, "target": victim, "n": d["n"],
			}}
		}
	case effectStack:
		if colour != "" {
			move.Fact = module.Fact{LabelKey: "lastcard.move.playedStackColour", Params: map[string]any{
				"player": player, "card": card, "target": victim, "n": d["n"], "colour": colour,
			}}
		} else {
			move.Fact = module.Fact{LabelKey: "lastcard.move.playedStack", Params: map[string]any{
				"player": player, "card": card, "target": victim, "n": d["n"],
			}}
		}
	case effectChallengeable:
		move.Fact = module.Fact{LabelKey: "lastcard.move.playedDrawFour", Params: map[string]any{
			"player": player, "card": card, "target": victim, "colour": colour,
		}}
	default:
		if colour != "" {
			move.Fact = module.Fact{LabelKey: "lastcard.move.playedWild", Params: map[string]any{
				"player": player, "card": card, "colour": colour,
			}}
		} else {
			move.Fact = module.Fact{LabelKey: "lastcard.move.played", Params: map[string]any{
				"player": player, "card": card,
			}}
		}
	}
	return move
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
