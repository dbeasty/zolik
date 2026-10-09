package zolikcore

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"zolik/server/internal/relay"
)

// The guest's end of the tunnel, against the host's end, through the relay:
// a table across the internet answers on a loopback address like any other.
func TestAGuestTableAnswersOnLoopbackThroughTheRelay(t *testing.T) {
	srv := relayCloud(t)
	if err := SetCloudBaseURL(srv.URL); err != nil {
		t.Fatal(err)
	}
	h := startHost(t, t.TempDir())
	h.nodeCredential, h.cloudBaseURL = "node-n1", srv.URL
	code, err := h.OpenRelay("Ada's phone")
	if err != nil {
		t.Fatalf("OpenRelay: %v", err)
	}

	// A first sit-down has nothing pinned, and learns the table's key.
	g, err := JoinRelay(code, h.InstanceID(), "")
	if err != nil {
		t.Fatalf("JoinRelay: %v", err)
	}
	t.Cleanup(LeaveGuestTables)
	if g.HostKey() != h.PublicKey() {
		t.Errorf("host key %s, want %s", g.HostKey(), h.PublicKey())
	}
	if len(g.CheckCode()) != 4 {
		t.Errorf("check code %q", g.CheckCode())
	}

	// Plain HTTP, through the tunnel.
	res, err := http.Post(g.BaseURL()+"/auth/guest", "application/json", strings.NewReader(`{"guestName":"Rita"}`))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 || !strings.Contains(string(body), "accessToken") {
		t.Fatalf("sign-in through the gateway: %d %s", res.StatusCode, body)
	}
	var who struct {
		AccessToken string `json:"accessToken"`
	}
	_ = json.Unmarshal(body, &who)

	// A table, and a socket on it.
	req, _ := http.NewRequest("POST", g.BaseURL()+"/matches", strings.NewReader(`{"moduleId":"prsi"}`))
	req.Header.Set("Authorization", "Bearer "+who.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ = io.ReadAll(res.Body)
	res.Body.Close()
	var made struct {
		MatchID string `json:"matchId"`
	}
	_ = json.Unmarshal(body, &made)
	if res.StatusCode != 200 && res.StatusCode != 201 || made.MatchID == "" {
		t.Fatalf("creating a table through the gateway: %d %s", res.StatusCode, body)
	}
	ws, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(g.BaseURL(), "http")+"/ws/matches/"+made.MatchID+"?token="+who.AccessToken, nil)
	if err != nil {
		t.Fatalf("socket through the gateway: %v", err)
	}
	_ = ws.SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, data, err := ws.ReadMessage(); err != nil || !strings.Contains(string(data), made.MatchID) {
		t.Fatalf("the socket sent %q, %v", data, err)
	}
	_ = ws.Close()

	// The link drops (the host's door closes and opens): the next request
	// reconnects by itself, on the same address.
	addr := g.BaseURL()
	g.tun.mu.Lock()
	live := g.tun.live
	g.tun.mu.Unlock()
	live.link.close()
	deadline := time.Now().Add(5 * time.Second)
	for {
		res, err = http.Post(addr+"/auth/guest", "application/json", strings.NewReader(`{"guestName":"Rita"}`))
		if err == nil && res.StatusCode == 200 {
			res.Body.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("no reconnect: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}

	// A table that answers with another key than the pinned one is refused.
	if _, err := JoinRelay(code, h.InstanceID(), "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="); err != ErrHostKeyChanged {
		t.Errorf("joining with the wrong pin: %v", err)
	}

	// Closing the host's door tells the guest why the table went.
	h.CloseRelay()
	deadline = time.Now().Add(5 * time.Second)
	for g.HostAway() != "ended" && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if g.HostAway() != "ended" {
		t.Errorf("after the host closed the table, HostAway = %q (relay close code %d)", g.HostAway(), relay.CloseHostEnded)
	}
}
