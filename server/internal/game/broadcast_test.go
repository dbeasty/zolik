package game

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"

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

func TestHub_BroadcastGameState_PublishesWithoutLocalConnections(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()

	redisURL := "redis://" + mr.Addr() + "/0"

	regA := NewConnRegistry()
	hubA, err := NewHub(regA, redisURL)
	if err != nil {
		t.Fatal(err)
	}
	defer hubA.Close()

	regB := NewConnRegistry()
	hubB, err := NewHub(regB, redisURL)
	if err != nil {
		t.Fatal(err)
	}
	defer hubB.Close()

	got := make(chan bool, 1)
	regB.Add("g1", "p2", &fakeConn{onWrite: func(v interface{}) {
		if m, ok := v.(map[string]interface{}); ok && m["type"] == "game_state" {
			got <- true
		}
	}})

	time.Sleep(200 * time.Millisecond)

	// No connections on hubA — must still reach hubB via Redis.
	hubA.BroadcastGameState("g1", []string{"p2"}, func(pid string) interface{} {
		return map[string]interface{}{"type": "game_state", "round": 2}
	})

	select {
	case <-got:
	case <-time.After(2 * time.Second):
		t.Fatal("expected game_state on peer instance without local connections on publisher")
	}
}
