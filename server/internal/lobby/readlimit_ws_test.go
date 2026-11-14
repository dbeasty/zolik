package lobby_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"zolik/server/internal/ws"
)

// Nothing a client sends to the waiting room means anything, so a frame over
// the idle limit is refused with "message too big" and the socket closed —
// rather than read into memory, which with no limit at all let one guest
// exhaust the process with a single huge frame.
func TestLobbySocketClosesOnAnOversizedFrame(t *testing.T) {
	srv := lobbyServer(t, nil)
	conn, _, err := dialLobby(t, srv, lobbyToken(t, "too-big"))
	if err != nil {
		t.Fatalf("dialling the lobby: %v", err)
	}
	if err := conn.WriteMessage(websocket.TextMessage, []byte(strings.Repeat("x", ws.MaxIdleMessageBytes+1))); err != nil {
		t.Fatalf("sending the oversized frame: %v", err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	for {
		_, _, err := conn.ReadMessage()
		if err == nil {
			continue // the waiting list broadcast on join
		}
		var ce *websocket.CloseError
		if !errors.As(err, &ce) || ce.Code != websocket.CloseMessageTooBig {
			t.Fatalf("read error = %v, want a close with code %d", err, websocket.CloseMessageTooBig)
		}
		return
	}
}
