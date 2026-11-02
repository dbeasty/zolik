package match_test

import (
	"net/http"
	"testing"
)

// A seated person may change their own name and face after the deal, and the
// table has to keep telling its seats apart while they do.
func TestAPlayerChangesTheirOwnLookMidMatch(t *testing.T) {
	h := newInviteHarness(t)
	hostToken := token(t, "host-1", "Host", false)
	friendToken := token(t, "player-2", "Bo", true)
	matchID := h.createMatch(t, hostToken)
	if res := h.do(http.MethodPost, "/matches/"+matchID+"/join", friendToken, nil); res.status != http.StatusOK {
		t.Fatalf("joining: status %d body %s", res.status, res.raw)
	}
	added := h.do(http.MethodPost, "/matches/"+matchID+"/add-bot", hostToken, nil)
	if added.status != http.StatusOK {
		t.Fatalf("add-bot: status %d body %s", added.status, added.raw)
	}
	if res := h.do(http.MethodPost, "/matches/"+matchID+"/start", hostToken, nil); res.status != http.StatusOK {
		t.Fatalf("start: status %d body %s", res.status, res.raw)
	}
	look := "/matches/" + matchID + "/look"

	res := h.do(http.MethodPost, look, friendToken, map[string]string{"name": "  Lucky   Fox ", "avatar": "fox"})
	if res.status != http.StatusOK || res.str("name") != "Lucky Fox" || res.str("avatar") != "fox" {
		t.Fatalf("changing name and face: status %d body %s", res.status, res.raw)
	}
	if name, avatar := seatOnTable(t, h, matchID, "player-2"); name != "Lucky Fox" || avatar != "fox" {
		t.Errorf("the table shows %q wearing %q, want Lucky Fox wearing fox", name, avatar)
	}

	// The face alone, leaving the name as it is.
	if res := h.do(http.MethodPost, look, friendToken, map[string]string{"avatar": "owl"}); res.status != http.StatusOK || res.str("name") != "Lucky Fox" || res.str("avatar") != "owl" {
		t.Fatalf("changing the face alone: status %d body %s", res.status, res.raw)
	}

	for _, taken := range []string{"host", "LUCKY FOX", added.str("name")} {
		who, tok := "the friend", friendToken
		if taken == "LUCKY FOX" {
			who, tok = "the host", hostToken
		}
		res := h.do(http.MethodPost, look, tok, map[string]string{"name": taken})
		if res.status != http.StatusConflict || res.str("code") != "NAME_TAKEN" {
			t.Errorf("%s took %q: status %d body %s", who, taken, res.status, res.raw)
		}
	}
	// Taking back one's own name in another case is not a collision.
	if res := h.do(http.MethodPost, look, friendToken, map[string]string{"name": "lucky fox"}); res.status != http.StatusOK {
		t.Errorf("re-casing one's own name: status %d body %s", res.status, res.raw)
	}

	if res := h.do(http.MethodPost, look, friendToken, map[string]string{"name": " ​ "}); res.status != http.StatusBadRequest {
		t.Errorf("a blank name: status %d, want 400 (body %s)", res.status, res.raw)
	}
	// A face that is not a slug is no face, as at the door.
	if res := h.do(http.MethodPost, look, friendToken, map[string]string{"avatar": "<img>"}); res.status != http.StatusOK || res.str("avatar") != "" {
		t.Errorf("a malformed face: status %d body %s", res.status, res.raw)
	}

	stranger := token(t, "stranger", "Nobody", true)
	if res := h.do(http.MethodPost, look, stranger, map[string]string{"name": "Sly Owl"}); res.status != http.StatusForbidden || res.str("code") != "NOT_AT_THIS_TABLE" {
		t.Errorf("a stranger renamed a seat: status %d body %s", res.status, res.raw)
	}
}

func seatOnTable(t *testing.T, h *inviteHarness, matchID, playerID string) (name, avatar string) {
	t.Helper()
	res := h.do(http.MethodGet, "/matches/"+matchID, "", nil)
	players, _ := res.body["players"].([]any)
	for _, p := range players {
		if pm, _ := p.(map[string]any); pm["id"] == playerID {
			name, _ = pm["name"].(string)
			avatar, _ = pm["avatar"].(string)
			return name, avatar
		}
	}
	t.Fatalf("%s is not at the table: %s", playerID, res.raw)
	return "", ""
}
