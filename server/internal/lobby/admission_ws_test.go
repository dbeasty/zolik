package lobby_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"

	"zolik/server/internal/admission"
	"zolik/server/internal/auth"
	"zolik/server/internal/lobby"
	"zolik/server/internal/ws"
)

// The waiting room is the first thing shed under pressure — a player idling
// here has no game to lose. These tests pin what that looks like on the wire:
// an ordinary 503 with the SERVER_BUSY vocabulary, before any upgrade.

func lobbyServer(t *testing.T, gate *admission.Controller) *httptest.Server {
	t.Helper()
	hub, err := ws.NewHub(ws.NewConnRegistry(), "")
	if err != nil {
		t.Fatalf("building a hub: %v", err)
	}
	t.Cleanup(func() { _ = hub.Close() })
	store, err := lobby.NewStore("")
	if err != nil {
		t.Fatalf("building the waiting room store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	h := lobby.NewHandlers(hub, store)
	h.SetAdmission(gate)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv
}

func lobbyToken(t *testing.T, subject string) string {
	t.Helper()
	tok, err := auth.CreateAccessToken(subject, "Player "+subject, true, time.Hour)
	if err != nil {
		t.Fatalf("minting a token: %v", err)
	}
	return tok
}

func dialLobby(t *testing.T, srv *httptest.Server, tok string) (*websocket.Conn, *http.Response, error) {
	t.Helper()
	url := strings.Replace(srv.URL, "http://", "ws://", 1) + "/ws/lobby?token=" + tok
	conn, resp, err := websocket.DefaultDialer.Dial(url, nil)
	if conn != nil {
		t.Cleanup(func() { _ = conn.Close() })
	}
	return conn, resp, err
}

func TestLobbySocketRefusedWhenWaitingRoomIsClosed(t *testing.T) {
	// Ceiling 5, waiting room closes at 4.
	gate := admission.New(admission.Limits{MaxConnections: 5, WaitingRoomRatio: 0.8})
	for i := 0; i < 4; i++ {
		if _, err := gate.Admit(admission.ClassGameplay); err != nil {
			t.Fatalf("occupying slot %d: %v", i, err)
		}
	}
	srv := lobbyServer(t, gate)

	_, resp, err := dialLobby(t, srv, lobbyToken(t, "shed-me"))
	if err == nil {
		t.Fatal("lobby dial succeeded with the waiting room closed, want a refused handshake")
	}
	if resp == nil || resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("refused handshake: response %+v, want status 503", resp)
	}
	if got := resp.Header.Get("Retry-After"); got == "" {
		t.Fatal("refused handshake has no Retry-After header")
	}
	var body struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil || body.Code != "SERVER_BUSY" {
		t.Fatalf("refusal body code = %q (decode err %v), want SERVER_BUSY", body.Code, err)
	}
}

func TestLobbySocketAdmittedUnderTheWaitingCeiling(t *testing.T) {
	gate := admission.New(admission.Limits{MaxConnections: 5, WaitingRoomRatio: 0.8})
	srv := lobbyServer(t, gate)

	conn, _, err := dialLobby(t, srv, lobbyToken(t, "welcome"))
	if err != nil {
		t.Fatalf("dialling an open lobby: %v", err)
	}
	if conn == nil {
		t.Fatal("no connection back from an open lobby")
	}
}

// A player already in the waiting room reconnects past a closed door: their
// new socket displaces their old one, so it sheds nobody else.
func TestWaitingPlayerReconnectsPastAClosedDoor(t *testing.T) {
	gate := admission.New(admission.Limits{MaxConnections: 4, WaitingRoomRatio: 0.5})
	srv := lobbyServer(t, gate)

	if _, _, err := dialLobby(t, srv, lobbyToken(t, "regular")); err != nil {
		t.Fatalf("first dial: %v", err)
	}

	// Close the waiting room behind them.
	for i := 0; i < 3; i++ {
		if _, err := gate.Admit(admission.ClassGameplay); err != nil {
			t.Fatalf("occupying slot %d: %v", i, err)
		}
	}
	if _, _, err := dialLobby(t, srv, lobbyToken(t, "too-late")); err == nil {
		t.Fatal("a new arrival got in with the waiting room closed")
	}

	if _, _, err := dialLobby(t, srv, lobbyToken(t, "regular")); err != nil {
		t.Fatalf("reconnect with the waiting room closed: %v — a player already waiting must get back in", err)
	}
}
