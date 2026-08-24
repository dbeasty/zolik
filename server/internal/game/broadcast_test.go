package game

import (
	"testing"

	"zolik/server/internal/models"
)

func TestBroadcastRecipients_TurnOrder(t *testing.T) {
	g := models.Game{
		TurnOrder: []string{"a", "b"},
		Players:   []models.Player{{ID: "x"}},
	}
	got := BroadcastRecipients(g)
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("expected turn order recipients, got %#v", got)
	}
}

func TestBroadcastRecipients_LobbyPlayers(t *testing.T) {
	g := models.Game{
		Players: []models.Player{{ID: "p1"}, {ID: "p2"}},
	}
	got := BroadcastRecipients(g)
	if len(got) != 2 {
		t.Fatalf("expected 2 lobby players, got %#v", got)
	}
}
