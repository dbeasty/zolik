package match_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"zolik/server/internal/admission"
)

// These are the handler-level halves of the admission story: the controller's
// own tests prove the arithmetic, these prove a player actually meets it as a
// 503 with the SERVER_BUSY vocabulary — and that the exemptions (reconnects,
// retrying a start) really exist on the wire.

// occupy fills n gameplay slots and returns their releases.
func occupy(t *testing.T, gate *admission.Controller, n int) []*admission.Release {
	t.Helper()
	out := make([]*admission.Release, 0, n)
	for i := 0; i < n; i++ {
		rel, err := gate.Admit(admission.ClassGameplay)
		if err != nil {
			t.Fatalf("occupying slot %d: %v", i, err)
		}
		out = append(out, rel)
	}
	return out
}

func TestCreateMatchRefusedAtCapacity(t *testing.T) {
	gate := admission.New(admission.Limits{MaxConnections: 1, RetryAfter: 7 * time.Second})
	h := newInviteHarnessWithAdmission(t, gate)
	hostToken := token(t, "busy-host-1", "Host", false)

	held := occupy(t, gate, 1)
	defer held[0].Release()

	res := h.do(http.MethodPost, "/matches", hostToken, map[string]any{"moduleId": "prsi"})
	if res.status != http.StatusServiceUnavailable {
		t.Fatalf("create at capacity: status = %d body %s, want 503", res.status, res.raw)
	}
	if got := res.str("code"); got != "SERVER_BUSY" {
		t.Fatalf("create at capacity: code = %q, want SERVER_BUSY", got)
	}
}

// The Retry-After hint has to reach the wire — it is the client's honest
// "when to try again", and the header is the only place a plain HTTP client
// looks for it.
func TestBusyRefusalCarriesRetryAfter(t *testing.T) {
	gate := admission.New(admission.Limits{MaxConnections: 1, RetryAfter: 7 * time.Second})
	h := newInviteHarnessWithAdmission(t, gate)
	hostToken := token(t, "busy-host-2", "Host", false)

	held := occupy(t, gate, 1)
	defer held[0].Release()

	req, err := http.NewRequest(http.MethodPost, h.server.URL+"/matches", strings.NewReader(`{"moduleId":"prsi"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+hostToken)
	resp, err := h.server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if got := resp.Header.Get("Retry-After"); got != "7" {
		t.Fatalf("Retry-After = %q, want %q", got, "7")
	}
}

// A refused start must strand nothing: the table keeps its seats and the same
// request succeeds once the pressure passes.
func TestStartMatchRefusedThenSucceedsAfterRelease(t *testing.T) {
	gate := admission.New(admission.Limits{MaxConnections: 1})
	h := newInviteHarnessWithAdmission(t, gate)
	hostToken := token(t, "busy-host-3", "Host", false)

	matchID := h.createMatch(t, hostToken)
	if res := h.do(http.MethodPost, "/matches/"+matchID+"/add-bot", hostToken, nil); res.status != http.StatusOK {
		t.Fatalf("add bot: status %d body %s", res.status, res.raw)
	}

	held := occupy(t, gate, 1)
	res := h.do(http.MethodPost, "/matches/"+matchID+"/start", hostToken, nil)
	if res.status != http.StatusServiceUnavailable || res.str("code") != "SERVER_BUSY" {
		t.Fatalf("start under pressure: status %d code %q, want 503 SERVER_BUSY", res.status, res.str("code"))
	}

	held[0].Release()
	if res := h.do(http.MethodPost, "/matches/"+matchID+"/start", hostToken, nil); res.status != http.StatusOK {
		t.Fatalf("start after release: status %d body %s, want 200 — a refused start must be retryable", res.status, res.raw)
	}
}

// Seating moves — join, add-bot, invite — stay open under pressure: they
// attach players to a match that was already admitted, and the start gate
// still guards the expensive step.
func TestSeatingAnExistingMatchIsNotGated(t *testing.T) {
	gate := admission.New(admission.Limits{MaxConnections: 1})
	h := newInviteHarnessWithAdmission(t, gate)
	hostToken := token(t, "busy-host-4", "Host", false)

	matchID := h.createMatch(t, hostToken)
	held := occupy(t, gate, 1)
	defer held[0].Release()

	if res := h.do(http.MethodPost, "/matches/"+matchID+"/add-bot", hostToken, nil); res.status != http.StatusOK {
		t.Fatalf("add bot under pressure: status %d body %s, want 200", res.status, res.raw)
	}
	guestToken := token(t, "busy-guest-4", "Guest", true)
	if res := h.do(http.MethodPost, "/matches/"+matchID+"/join", guestToken, nil); res.status != http.StatusOK {
		t.Fatalf("join under pressure: status %d body %s, want 200", res.status, res.raw)
	}
}

// The socket-level gate, end to end: a stranger's handshake is refused with a
// readable 503, while the player already at the table reconnects freely.
func TestMatchSocketRefusalAndReconnectExemption(t *testing.T) {
	gate := admission.New(admission.Limits{MaxConnections: 1})
	h := newInviteHarnessWithAdmission(t, gate)
	hostToken := token(t, "busy-host-5", "Host", false)
	matchID := startedMatch(t, h, hostToken)

	// The host's first socket takes the only slot.
	dialMatch(t, h, matchID, hostToken)

	// A second arrival has no slot to take.
	url := strings.Replace(h.server.URL, "http://", "ws://", 1) + "/ws/matches/" + matchID +
		"?token=" + token(t, "busy-stranger-5", "Stranger", false)
	conn, resp, err := websocket.DefaultDialer.Dial(url, nil)
	if err == nil {
		conn.Close()
		t.Fatal("a stranger's dial succeeded at capacity, want a refused handshake")
	}
	if resp == nil || resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("refused handshake: response %+v, want status 503", resp)
	}

	// The host reconnecting displaces their own socket and is never refused.
	dialMatch(t, h, matchID, hostToken)
}
