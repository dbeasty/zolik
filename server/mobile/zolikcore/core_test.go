package zolikcore

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// A phone alone with bots: a guest seat, a table, a bot, a started game and
// a socket carrying the state. This is Phase 1 of the offline plan end to
// end, against the real router, over a real listener.
func TestAGuestPlaysABotOnTheEmbeddedHost(t *testing.T) {
	dir := t.TempDir()
	h := startHost(t, dir)

	if again, err := Start(dir, false); err != nil || again != h {
		t.Fatalf("a second Start returned (%p, %v), want the running host %p", again, err, h)
	}

	guest := postJSON[map[string]any](t, h, "/auth/guest", "", map[string]any{"guestName": "Kitchen"}, http.StatusOK)
	access, _ := guest["accessToken"].(string)
	refresh, _ := guest["refreshToken"].(string)
	userID, _ := guest["userId"].(string)
	if access == "" || refresh == "" || userID == "" {
		t.Fatalf("guest sign-in answered %v", guest)
	}

	created := postJSON[map[string]any](t, h, "/matches", access, map[string]any{"moduleId": "blackjack"}, http.StatusOK)
	matchID, _ := created["matchId"].(string)
	if matchID == "" {
		t.Fatalf("create answered %v", created)
	}
	postJSON[map[string]any](t, h, "/matches/"+matchID+"/add-bot", access, map[string]any{}, http.StatusOK)
	postJSON[map[string]any](t, h, "/matches/"+matchID+"/start", access, map[string]any{}, http.StatusOK)

	conn := dial(t, h, matchID, access)
	defer func() { _ = conn.Close() }()
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var msg struct {
		Type string `json:"type"`
	}
	if err := conn.ReadJSON(&msg); err != nil {
		t.Fatalf("reading the first frame: %v", err)
	}
	if msg.Type != "match_state" {
		t.Fatalf("first frame is %q, want match_state", msg.Type)
	}
	_ = conn.Close()

	// The table and the seat survive the host restarting: same identity, the
	// same secrets, so the refresh token the client kept still works.
	id := h.InstanceID()
	h.Stop()
	h2 := startHost(t, dir)
	if h2.InstanceID() != id {
		t.Errorf("instance id changed across a restart: %s → %s", id, h2.InstanceID())
	}
	postJSON[map[string]any](t, h2, "/auth/refresh", "", map[string]any{"refreshToken": refresh}, http.StatusOK)
	resp := get(t, h2, "/matches/"+matchID, "")
	if resp.StatusCode != http.StatusOK {
		t.Errorf("the match is gone after a restart: %d", resp.StatusCode)
	}
}

// Other phones can reach a LAN host, so its surface is what an offline table
// needs and nothing more, and the dev-token shortcut is off.
func TestTheEmbeddedHostExposesOnlyWhatATableNeeds(t *testing.T) {
	h := startHost(t, t.TempDir())

	for _, path := range []string{"/healthz", "/version", "/modules"} {
		if got := get(t, h, path, "").StatusCode; got != http.StatusOK {
			t.Errorf("GET %s = %d, want 200", path, got)
		}
	}
	for _, path := range []string{"/users/me", "/leaderboard", "/lobby/waiting", "/debug/memory", "/auth/providers", "/scoring-sessions"} {
		if got := get(t, h, path, "").StatusCode; got != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", path, got)
		}
	}
	for _, path := range []string{"/auth/login", "/auth/register", "/auth/email/start"} {
		postJSON[map[string]any](t, h, path, "", map[string]any{}, http.StatusNotFound)
	}

	guest := postJSON[map[string]any](t, h, "/auth/guest", "", map[string]any{}, http.StatusOK)
	access := guest["accessToken"].(string)
	created := postJSON[map[string]any](t, h, "/matches", access, map[string]any{"moduleId": "blackjack"}, http.StatusOK)
	matchID := created["matchId"].(string)

	u := wsURL(h, matchID, "dev:"+guest["userId"].(string))
	conn, resp, err := websocket.DefaultDialer.Dial(u, nil)
	if err == nil {
		defer func() { _ = conn.Close() }()
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		var msg struct {
			Type string `json:"type"`
		}
		if conn.ReadJSON(&msg) == nil && msg.Type == "match_state" {
			t.Fatal("a dev: token was let into someone else's seat")
		}
		return
	}
	if resp != nil && resp.StatusCode == http.StatusSwitchingProtocols {
		t.Fatalf("dev: token upgraded: %v", err)
	}
}

func startHost(t *testing.T, dir string) *Host {
	t.Helper()
	h, err := Start(dir, false)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(h.Stop)
	return h
}

func get(t *testing.T, h *Host, path, token string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, h.BaseURL()+path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	return resp
}

func postJSON[T any](t *testing.T, h *Host, path, token string, body any, want int) T {
	t.Helper()
	raw, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, h.BaseURL()+path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != want {
		t.Fatalf("POST %s = %d (%s), want %d", path, resp.StatusCode, strings.TrimSpace(string(data)), want)
	}
	var out T
	_ = json.Unmarshal(data, &out)
	return out
}

func wsURL(h *Host, matchID, token string) string {
	return strings.Replace(h.BaseURL(), "http://", "ws://", 1) +
		"/ws/matches/" + url.PathEscape(matchID) + "?token=" + url.QueryEscape(token)
}

func dial(t *testing.T, h *Host, matchID, token string) *websocket.Conn {
	t.Helper()
	conn, _, err := websocket.DefaultDialer.Dial(wsURL(h, matchID, token), nil)
	if err != nil {
		t.Fatalf("dialling the match socket: %v", err)
	}
	return conn
}
