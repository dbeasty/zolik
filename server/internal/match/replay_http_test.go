package match_test

import (
	"fmt"
	"net/http"
	"testing"

	"zolik/server/internal/match"
)

// The replay endpoint's door policy. The fold itself is covered in package
// match (replay_test.go); what is worth pinning out here is who gets in, and
// what a caller is told when there is nothing to give them.

func (h *storedHarness) replay(t *testing.T, bearer, matchID, query string) (int, map[string]any) {
	t.Helper()
	res := h.do(http.MethodGet, "/matches/"+matchID+"/replay"+query, bearer, nil)
	return res.status, res.body
}

// A replay is for the people who played it. Not the unauthenticated spectator
// GET next door: this answers with every board the match passed through.
func TestReplayIsForPeopleAtTheTable(t *testing.T) {
	h := newStoredHarness(t)
	hostToken := token(t, "host-1", "Host", false)
	strangerToken := token(t, "nobody-1", "Passing Stranger", false)
	matchID := startedMatch(t, h.inviteHarness, hostToken)

	t.Run("no token at all", func(t *testing.T) {
		if status, _ := h.replay(t, "", matchID, ""); status != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", status)
		}
	})

	t.Run("somebody else's table", func(t *testing.T) {
		status, body := h.replay(t, strangerToken, matchID, "")
		if status != http.StatusForbidden {
			t.Errorf("status = %d, want 403", status)
		}
		if body["code"] != "NOT_AT_THIS_TABLE" {
			t.Errorf("code = %v, want NOT_AT_THIS_TABLE", body["code"])
		}
	})

	t.Run("a player at the table", func(t *testing.T) {
		status, body := h.replay(t, hostToken, matchID, "")
		if status != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body %v)", status, body)
		}
		if body["type"] != "match_replay" {
			t.Errorf("type = %v, want match_replay", body["type"])
		}
		frames, _ := body["frames"].([]any)
		if len(frames) == 0 {
			t.Fatal("a started match replayed to no frames at all")
		}
		// Frame 0 is the deal, and nobody made a move to reach it.
		first, _ := frames[0].(map[string]any)
		if first["index"] != float64(0) {
			t.Errorf("first frame index = %v, want 0", first["index"])
		}
		if _, ok := first["verb"]; ok {
			t.Errorf("the deal reported a move: %v", first)
		}
	})
}

// A table nobody dealt has no game in it to step through — and the caller did
// nothing wrong, so it is a conflict rather than a bad request.
func TestReplayRefusesALobby(t *testing.T) {
	h := newStoredHarness(t)
	hostToken := token(t, "host-1", "Host", false)
	matchID := h.createMatch(t, hostToken)

	status, body := h.replay(t, hostToken, matchID, "")
	if status != http.StatusConflict {
		t.Errorf("status = %d, want 409", status)
	}
	if body["code"] != "NOTHING_TO_REPLAY" {
		t.Errorf("code = %v, want NOTHING_TO_REPLAY", body["code"])
	}
}

// A match that was deleted, or that outlived its retention window, answers the
// same way every other route on a swept match does.
func TestReplayOfAMatchThatIsGone(t *testing.T) {
	h := newStoredHarness(t)
	hostToken := token(t, "host-1", "Host", false)

	status, body := h.replay(t, hostToken, "000000000000000000000000", "")
	if status != http.StatusNotFound {
		t.Errorf("status = %d, want 404", status)
	}
	if body["code"] != "MATCH_NOT_FOUND" {
		t.Errorf("code = %v, want MATCH_NOT_FOUND", body["code"])
	}
}

// The frame cap is the defense, not a preference: there is no rate limiter on
// gameplay routes, and every frame costs a fold.
func TestReplayClampsTheFrameCount(t *testing.T) {
	h := newStoredHarness(t)
	hostToken := token(t, "host-1", "Host", false)
	matchID := startedMatch(t, h.inviteHarness, hostToken)

	status, body := h.replay(t, hostToken, matchID,
		fmt.Sprintf("?limit=%d", match.MaxReplayFrames*10))
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	frames, _ := body["frames"].([]any)
	if len(frames) > match.MaxReplayFrames {
		t.Errorf("got %d frames, want at most %d", len(frames), match.MaxReplayFrames)
	}
}

// A stored-table row advertises replay, so the screen never has to work out
// for itself whether the button would do anything.
func TestStoredTableSaysWhetherItCanBeReplayed(t *testing.T) {
	h := newStoredHarness(t)
	hostToken := token(t, "host-1", "Host", false)

	h.createMatch(t, hostToken)
	rows := h.tables(t, hostToken, "")
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	if rows[0]["canReplay"] != false {
		t.Errorf("a lobby offered a replay: %v", rows[0]["canReplay"])
	}

	startedMatch(t, h.inviteHarness, hostToken)
	rows = h.tables(t, hostToken, "")
	var started map[string]any
	for _, r := range rows {
		if r["status"] != "lobby" {
			started = r
		}
	}
	if started == nil {
		t.Fatal("the started match is not in the list")
	}
	if started["canReplay"] != true {
		t.Errorf("a dealt match did not offer a replay: %v", started)
	}
}
