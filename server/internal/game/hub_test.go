package game

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
