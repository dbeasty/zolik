package ws

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

func TestHub_PublishCrossInstance(t *testing.T) {
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

	// Simulate player on instance B only.
	got := make(chan map[string]interface{}, 1)
	regB.Add("game1", "p1", &fakeConn{onWrite: func(v interface{}) {
		if m, ok := v.(map[string]interface{}); ok {
			got <- m
		}
	}})

	time.Sleep(200 * time.Millisecond)

	hubA.Publish("game1", []PlayerMessage{{
		PlayerID: "p1",
		Payload:  map[string]interface{}{"type": "game_state", "round": 1},
	}})

	select {
	case msg := <-got:
		if msg["type"] != "game_state" {
			t.Fatalf("unexpected payload %#v", msg)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for cross-instance delivery")
	}
}

type fakeConn struct {
	onWrite func(interface{})
}

func (f *fakeConn) WriteJSON(v interface{}) error {
	if f.onWrite != nil {
		f.onWrite(v)
	}
	return nil
}

func (f *fakeConn) Close() error { return nil }

func (f *fakeConn) Ping() error { return nil }

func (f *fakeConn) CloseWithCode(int, string) error { return nil }

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

// Somebody watching a table without a seat is sent each new board, built for
// them — not left on the snapshot they were given when they arrived.
func TestHub_BroadcastGameState_ReachesAnOnlookerLocally(t *testing.T) {
	reg := NewConnRegistry()
	hub, err := NewHub(reg, "")
	if err != nil {
		t.Fatal(err)
	}
	defer hub.Close()

	got := map[string][]string{}
	add := func(id string) {
		reg.Add("g1", id, &fakeConn{onWrite: func(v interface{}) {
			got[id] = append(got[id], v.(map[string]interface{})["for"].(string))
		}})
	}
	add("seated")
	add("watcher")

	hub.BroadcastGameState("g1", []string{"seated"}, func(pid string) interface{} {
		return map[string]interface{}{"type": "match_state", "for": pid}
	})
	if len(got["seated"]) != 1 || got["seated"][0] != "seated" {
		t.Fatalf("seated player got %v, want their own board once", got["seated"])
	}
	if len(got["watcher"]) != 1 || got["watcher"][0] != "watcher" {
		t.Fatalf("onlooker got %v, want a board built for them", got["watcher"])
	}
}

func TestHub_BroadcastGameState_ReachesAnOnlookerOnAnotherInstance(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()
	url := "redis://" + mr.Addr() + "/0"

	hubA, err := NewHub(NewConnRegistry(), url)
	if err != nil {
		t.Fatal(err)
	}
	defer hubA.Close()
	regB := NewConnRegistry()
	hubB, err := NewHub(regB, url)
	if err != nil {
		t.Fatal(err)
	}
	defer hubB.Close()

	got := make(chan string, 4)
	regB.Add("g1", "watcher", &fakeConn{onWrite: func(v interface{}) {
		got <- v.(map[string]interface{})["for"].(string)
	}})
	time.Sleep(200 * time.Millisecond)

	hubA.BroadcastGameState("g1", []string{"seated"}, func(pid string) interface{} {
		return map[string]interface{}{"type": "match_state", "for": pid}
	})
	select {
	case who := <-got:
		if who != "" {
			t.Fatalf("a peer's onlooker got a board built for %q, want the unseated view", who)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("onlooker on another instance never got the new board")
	}
}
