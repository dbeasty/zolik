package lobby_test

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// dialLobbyFor opens the waiting-room socket as a player waiting for the
// given games (none: any game).
func dialLobbyFor(t *testing.T, srvURL, tok string, modules ...string) *websocket.Conn {
	t.Helper()
	url := strings.Replace(srvURL, "http://", "ws://", 1) + "/ws/lobby?token=" + tok
	for _, m := range modules {
		url += "&moduleId=" + m
	}
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dialling the lobby: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

// waitForList reads broadcasts until one names exactly want, sorted.
func waitForList(t *testing.T, conn *websocket.Conn, want []string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	var last []string
	for time.Now().Before(deadline) {
		_ = conn.SetReadDeadline(deadline)
		var msg struct {
			Type    string `json:"type"`
			Players []struct {
				PlayerID string `json:"playerId"`
			} `json:"players"`
		}
		if err := conn.ReadJSON(&msg); err != nil {
			break
		}
		if msg.Type != "lobby_waiting" {
			continue
		}
		last = last[:0]
		for _, p := range msg.Players {
			last = append(last, p.PlayerID)
		}
		sort.Strings(last)
		if strings.Join(last, ",") == strings.Join(want, ",") {
			return
		}
	}
	t.Fatalf("last waiting list = %v, want %v", last, want)
}

func TestWaitingRoomIsSplitByGame(t *testing.T) {
	srv := lobbyServer(t, nil)

	holdem := dialLobbyFor(t, srv.URL, lobbyToken(t, "h1"), "holdem")
	waitForList(t, holdem, []string{"h1"})
	canasta := dialLobbyFor(t, srv.URL, lobbyToken(t, "c1"), "canasta")
	waitForList(t, canasta, []string{"c1"})
	anyGame := dialLobbyFor(t, srv.URL, lobbyToken(t, "a1"))

	// Everyone waiting for any game sees the whole room; the others see only
	// who they could end up at a table with.
	waitForList(t, anyGame, []string{"a1", "c1", "h1"})
	waitForList(t, holdem, []string{"a1", "h1"})
	waitForList(t, canasta, []string{"a1", "c1"})

	get := func(query string) []string {
		req, _ := http.NewRequest(http.MethodGet, srv.URL+"/lobby/waiting"+query, nil)
		req.Header.Set("Authorization", "Bearer "+lobbyToken(t, "browser"))
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET /lobby/waiting%s: %v", query, err)
		}
		defer res.Body.Close()
		var body struct {
			Players []struct {
				PlayerID  string   `json:"playerId"`
				ModuleIDs []string `json:"moduleIds"`
			} `json:"players"`
		}
		if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
			t.Fatalf("decoding /lobby/waiting%s: %v", query, err)
		}
		var ids []string
		for _, p := range body.Players {
			ids = append(ids, p.PlayerID+":"+strings.Join(p.ModuleIDs, "+"))
		}
		sort.Strings(ids)
		return ids
	}
	if got, want := strings.Join(get("?moduleId=holdem"), ","), "a1:,h1:holdem"; got != want {
		t.Errorf("hold'em list = %s, want %s", got, want)
	}
	if got, want := strings.Join(get(""), ","), "a1:,c1:canasta,h1:holdem"; got != want {
		t.Errorf("whole room = %s, want %s", got, want)
	}
}
