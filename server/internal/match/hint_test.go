package match_test

import (
	"net/http"
	"testing"
	"time"
)

// A hint is the move the caller's seat would make, suggested and never made.
func TestAHintSuggestsAMoveTheSeatCanMake(t *testing.T) {
	h := newInviteHarness(t)
	hostToken := token(t, "host-1", "Host", false)
	matchID := startedMatch(t, h, hostToken)

	// The bot may be first to act, so ask until it is the host's turn.
	deadline := time.Now().Add(10 * time.Second)
	for {
		res := h.do(http.MethodPost, "/matches/"+matchID+"/hint", hostToken, nil)
		if res.status == http.StatusOK {
			action, _ := res.body["action"].(map[string]any)
			if action["verb"] == "" || action["verb"] == nil {
				t.Fatalf("a hint with no move in it: %s", res.raw)
			}
			if id, _ := action["offerId"].(string); id == "" {
				t.Fatalf("a hint that names no control: %s", res.raw)
			}
			break
		}
		if res.str("code") != "NOT_YOUR_TURN" {
			t.Fatalf("hint: status %d body %s", res.status, res.raw)
		}
		if time.Now().After(deadline) {
			t.Fatal("the host's turn never came round")
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Asking changed nothing: the host still has the move.
	if res := h.do(http.MethodPost, "/matches/"+matchID+"/hint", hostToken, nil); res.status != http.StatusOK {
		t.Fatalf("a second hint, with nothing played in between: status %d body %s", res.status, res.raw)
	}
}

// A table that turned hints off gets none, and says why.
func TestAHintIsRefusedWhereTheTableTurnedThemOff(t *testing.T) {
	h := newInviteHarness(t)
	hostToken := token(t, "host-1", "Host", false)
	res := h.do(http.MethodPost, "/matches", hostToken, map[string]any{
		"moduleId": "prsi", "options": map[string]int{"hints": 0},
	})
	if res.status != http.StatusOK {
		t.Fatalf("create match: status %d body %s", res.status, res.raw)
	}
	matchID := res.str("matchId")
	h.do(http.MethodPost, "/matches/"+matchID+"/add-bot", hostToken, nil)
	h.do(http.MethodPost, "/matches/"+matchID+"/start", hostToken, nil)

	got := h.do(http.MethodPost, "/matches/"+matchID+"/hint", hostToken, nil)
	if got.status != http.StatusForbidden || got.str("code") != "HINTS_OFF" {
		t.Fatalf("hint at a table without them: status %d body %s", got.status, got.raw)
	}
}

// Only somebody seated at the table may ask what their seat should do.
func TestAHintIsForPlayersAtTheTable(t *testing.T) {
	h := newInviteHarness(t)
	hostToken := token(t, "host-1", "Host", false)
	matchID := startedMatch(t, h, hostToken)

	stranger := token(t, "stranger-1", "Stranger", false)
	got := h.do(http.MethodPost, "/matches/"+matchID+"/hint", stranger, nil)
	if got.status != http.StatusForbidden || got.str("code") != "NOT_AT_THIS_TABLE" {
		t.Fatalf("hint for a stranger: status %d body %s", got.status, got.raw)
	}
}
