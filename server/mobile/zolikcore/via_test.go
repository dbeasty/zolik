package zolikcore

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// Each seat at a table a phone hosts says how its player is connected: the
// phone's own app, a guest on the room's Wi-Fi, one over Bluetooth. The
// Bluetooth guest's socket comes in on the phone's own loopback like the
// phone's app does, and is still told apart.
func TestEverySeatSaysHowItIsConnected(t *testing.T) {
	h := startHost(t, t.TempDir())
	port, err := h.OpenLAN()
	if err != nil {
		t.Fatalf("OpenLAN: %v", err)
	}
	room := lanAddr{port}

	host := postJSON[map[string]any](t, h, "/auth/guest", "", map[string]any{"guestName": "Host"}, http.StatusOK)
	wifi := postJSON[map[string]any](t, room, "/auth/guest", "", map[string]any{"guestName": "Wifi"}, http.StatusOK)
	hostTok, wifiTok := host["accessToken"].(string), wifi["accessToken"].(string)

	radio := connectGuest(t, h)
	res := radio.do("POST", "/auth/guest", "", map[string]string{"guestName": "Radio"})
	var rs struct{ AccessToken, UserID string }
	_ = json.Unmarshal([]byte(res.B), &rs)

	created := postJSON[map[string]any](t, h, "/matches", hostTok, map[string]any{"moduleId": "prsi"}, http.StatusOK)
	matchID, code := created["matchId"].(string), created["joinCode"].(string)
	postJSON[map[string]any](t, room, "/matches/"+code+"/join", wifiTok, map[string]any{}, http.StatusOK)
	if radio.do("POST", "/matches/"+code+"/join", rs.AccessToken, map[string]any{}).S != 200 {
		t.Fatal("the Bluetooth guest could not join")
	}
	postJSON[map[string]any](t, h, "/matches/"+matchID+"/start", hostTok, map[string]any{}, http.StatusOK)

	hostConn := dial(t, h, matchID, hostTok)
	defer func() { _ = hostConn.Close() }()
	wifiConn := dial(t, room, matchID, wifiTok)
	defer func() { _ = wifiConn.Close() }()
	radio.sendMsg(tunnelMsg{T: "wso", ID: 7, P: "/ws/matches/" + matchID + "?token=" + rs.AccessToken})
	radio.until(func(m tunnelMsg) bool { return m.T == "wsopen" && m.ID == 7 })

	ids := map[string]string{
		host["userId"].(string): "self",
		wifi["userId"].(string): "wifi",
		rs.UserID:               "bluetooth",
	}
	// The host's screen catches up as each socket arrives: read until it
	// shows all three.
	deadline := time.Now().Add(5 * time.Second)
	var got map[string]string
	for time.Now().Before(deadline) {
		got = viaOnState(t, hostConn)
		if len(got) == 3 {
			break
		}
	}
	for id, want := range ids {
		if got[id] != want {
			t.Errorf("seat %s shows via %q, want %q (all: %v)", id, got[id], want, got)
		}
	}

	// Gone is gone: a seat whose socket closed shows no route.
	_ = wifiConn.Close()
	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if got = viaOnState(t, hostConn); got[wifi["userId"].(string)] == "" {
			return
		}
	}
	t.Errorf("the Wi-Fi guest still shows a route after leaving: %v", got)
}

// viaOnState reads the next state message on a socket and returns each
// player's route.
func viaOnState(t *testing.T, c *websocket.Conn) map[string]string {
	t.Helper()
	for {
		_ = c.SetReadDeadline(time.Now().Add(5 * time.Second))
		var msg struct {
			Type    string `json:"type"`
			Players []struct {
				ID  string `json:"id"`
				Via string `json:"via"`
			} `json:"players"`
		}
		if err := c.ReadJSON(&msg); err != nil {
			t.Fatalf("reading the host's socket: %v", err)
		}
		if msg.Type != "match_state" {
			continue
		}
		out := map[string]string{}
		for _, p := range msg.Players {
			if p.Via != "" {
				out[p.ID] = p.Via
			}
		}
		return out
	}
}
