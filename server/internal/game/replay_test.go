package game

import (
	"testing"
	"time"

	"zolik/server/internal/models"
)

func TestRedactActionLog_HidesOtherPlayersDrawCard(t *testing.T) {
	log := []models.Action{{
		Seq:       1,
		Timestamp: time.Now(),
		Type:      "draw_deck",
		PlayerID:  "p2",
		Data:      map[string]interface{}{"card": "AS", "playerId": "p2"},
	}}

	out := RedactActionLogForPlayer(log, "p1")
	if _, ok := out[0].Data["card"]; ok {
		t.Fatal("expected card redacted for non-actor viewer")
	}
}

func TestRedactActionLog_KeepsOwnDrawCard(t *testing.T) {
	log := []models.Action{{
		Seq:       1,
		Timestamp: time.Now(),
		Type:      "draw_deck",
		PlayerID:  "p1",
		Data:      map[string]interface{}{"card": "AS"},
	}}

	out := RedactActionLogForPlayer(log, "p1")
	if out[0].Data["card"] != "AS" {
		t.Fatal("expected own draw card visible")
	}
}
