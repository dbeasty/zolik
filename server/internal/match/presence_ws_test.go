package match_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// A socket that was *displaced* is not a player who left.
//
// `ConnRegistry.Add` closes whatever connection a new one supersedes, and the
// closed connection's handler then runs its own defers. Suspending the match
// there is wrong for the same reason `RemoveIfCurrent` returns a bool and says
// so in its doc comment: the player is still connected, on the newer socket.
//
// Getting this wrong is not a cosmetic bug. A client that opened one extra
// socket — a reconnect chain left running by a re-rendered hook was enough —
// put a real table into a loop of suspend/resume every couple of seconds for
// fifteen minutes. Every action sent during a paused half was refused with
// MATCH_NOT_ACTIVE, the bot loop hit one and stopped for good, and the player
// pressing Discard on a perfectly legal card saw nothing happen at all.

// wsHarness reuses the invite harness's Mongo-backed server, since what it
// builds — a real router, a real hub, a real repository — is exactly what a
// presence test needs and nothing about it is invite-specific.
func startedMatch(t *testing.T, h *inviteHarness, hostToken string) string {
	t.Helper()
	matchID := h.createMatch(t, hostToken)
	if res := h.do(http.MethodPost, "/matches/"+matchID+"/add-bot", hostToken, nil); res.status != http.StatusOK {
		t.Fatalf("add bot: status %d body %s", res.status, res.raw)
	}
	if res := h.do(http.MethodPost, "/matches/"+matchID+"/start", hostToken, nil); res.status != http.StatusOK {
		t.Fatalf("start match: status %d body %s", res.status, res.raw)
	}
	return matchID
}

func dialMatch(t *testing.T, h *inviteHarness, matchID, tok string) *websocket.Conn {
	t.Helper()
	url := strings.Replace(h.server.URL, "http://", "ws://", 1) + "/ws/matches/" + matchID + "?token=" + tok
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dialling the match socket: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	// The server writes this viewer's state as soon as it has registered the
	// connection, so reading one message is how a test knows the handler has
	// got as far as its defers being armed.
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, _, err := conn.ReadMessage(); err != nil {
		t.Fatalf("reading the first state message: %v", err)
	}
	_ = conn.SetReadDeadline(time.Time{})
	return conn
}

func (h *inviteHarness) status(t *testing.T, matchID string) string {
	t.Helper()
	res := h.do(http.MethodGet, "/matches/"+matchID, "", nil)
	if res.status != http.StatusOK {
		t.Fatalf("get match: status %d body %s", res.status, res.raw)
	}
	return res.str("status")
}

// waitForStatus polls until the match reaches want, or fails. Suspension and
// resumption are both driven off a socket's lifetime rather than off a
// request, so there is nothing to synchronise on but the result.
func (h *inviteHarness) waitForStatus(t *testing.T, matchID, want string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	var got string
	for time.Now().Before(deadline) {
		got = h.status(t, matchID)
		if got == want {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("match status = %q, want %q", got, want)
}

func TestReconnectingDoesNotSuspendTheMatch(t *testing.T) {
	h := newInviteHarness(t)
	hostToken := token(t, "host-ws-1", "Host", false)
	matchID := startedMatch(t, h, hostToken)

	first := dialMatch(t, h, matchID, hostToken)
	// The second connection displaces the first: the server closes it, and the
	// first handler's defers run.
	dialMatch(t, h, matchID, hostToken)

	// Wait until the displaced socket is genuinely gone, so the assertion below
	// is about what the handler decided rather than about it not having run.
	_ = first.SetReadDeadline(time.Now().Add(5 * time.Second))
	for {
		if _, _, err := first.ReadMessage(); err != nil {
			break
		}
	}

	// Give the deferred suspend the same window it would have had, then insist
	// the table never paused.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if got := h.status(t, matchID); got != "active" {
			t.Fatalf("match status = %q after a reconnect, want it to stay active", got)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// The other half of the same rule: a socket that really was the player's last
// one still pauses the table, and their return still un-pauses it. Without
// this, "never suspend" would pass the test above and break reconnection.
func TestTheLastSocketClosingStillSuspendsAndReturningResumes(t *testing.T) {
	h := newInviteHarness(t)
	hostToken := token(t, "host-ws-2", "Host", false)
	matchID := startedMatch(t, h, hostToken)

	only := dialMatch(t, h, matchID, hostToken)
	_ = only.Close()
	h.waitForStatus(t, matchID, "suspended")

	dialMatch(t, h, matchID, hostToken)
	h.waitForStatus(t, matchID, "active")
}
