package match_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"zolik/server/internal/ws"
)

// The match socket reads actions, and nothing a client legitimately sends is
// anywhere near ws.MaxActionMessageBytes. A frame over it is refused with
// "message too big", the socket closed and the seat taken out of the registry
// — rather than read into memory, which with no limit let a guest send one
// multi-gigabyte frame and take the whole process down.
func TestMatchSocketClosesOnAnOversizedFrame(t *testing.T) {
	h := newInviteHarness(t)
	const host = "host-readlimit"
	hostToken := token(t, host, "Host", false)
	matchID := startedMatch(t, h, hostToken)
	conn := dialMatch(t, h, matchID, hostToken)

	// Under the limit, even far over any real action: refused as bad JSON,
	// and the socket stays open.
	if err := conn.WriteMessage(websocket.TextMessage, []byte(strings.Repeat("x", ws.MaxActionMessageBytes/2))); err != nil {
		t.Fatalf("sending a large frame: %v", err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("socket closed on a frame under the limit: %v", err)
		}
		var msg map[string]any
		_ = json.Unmarshal(data, &msg)
		if msg["code"] == "BAD_JSON" {
			break
		}
	}

	if err := conn.WriteMessage(websocket.TextMessage, []byte(strings.Repeat("x", ws.MaxActionMessageBytes+1))); err != nil {
		t.Fatalf("sending the oversized frame: %v", err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	for {
		_, _, err := conn.ReadMessage()
		if err == nil {
			continue // state pushes from the bot's turn
		}
		var ce *websocket.CloseError
		if !errors.As(err, &ce) || ce.Code != websocket.CloseMessageTooBig {
			t.Fatalf("read error = %v, want a close with code %d", err, websocket.CloseMessageTooBig)
		}
		break
	}

	deadline := time.Now().Add(5 * time.Second)
	for h.hub.Registry().Has(matchID, host) {
		if time.Now().After(deadline) {
			t.Fatal("the seat is still registered after its socket was closed")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
