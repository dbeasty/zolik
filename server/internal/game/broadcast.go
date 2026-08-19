package game

import (
	"zolik/server/internal/models"
	"zolik/server/internal/rules"
)

// BroadcastRecipients returns every player id that should receive WS payloads for this game.
// Uses turn order when present (matches gameplay), otherwise lobby players.
func BroadcastRecipients(g models.Game) []string {
	if len(g.TurnOrder) > 0 {
		out := make([]string, len(g.TurnOrder))
		copy(out, g.TurnOrder)
		return out
	}
	out := make([]string, 0, len(g.Players))
	for _, p := range g.Players {
		if p.ID != "" {
			out = append(out, p.ID)
		}
	}
	return out
}

// broadcastEvents sends delta events to all game participants (local + Redis peers).
// Private draw card details are omitted for non-acting players on player_drew.
func (m *Manager) broadcastEvents(gameID, actorID string, events []rules.StateEvent, recipients []string) {
	if len(recipients) == 0 {
		return
	}
	for _, ev := range events {
		msgs := make([]PlayerMessage, 0, len(recipients))
		for _, pid := range recipients {
			payload := eventPayloadForPlayer(ev, pid, actorID)
			if payload == nil {
				continue
			}
			msgs = append(msgs, PlayerMessage{PlayerID: pid, Payload: payload})
		}
		if len(msgs) > 0 {
			m.hub.Publish(gameID, msgs)
		}
	}
}

func eventPayloadForPlayer(ev rules.StateEvent, viewerID, actorID string) map[string]interface{} {
	payload := map[string]interface{}{
		"type": ev.Type,
	}
	for k, v := range ev.Data {
		payload[k] = v
	}

	switch ev.Type {
	case "draw_deck", "draw_discard":
		if viewerID != actorID {
			return nil // full state sync covers hand updates
		}
	case "player_drew":
		if viewerID != actorID {
			delete(payload, "card")
		}
	}
	return payload
}
