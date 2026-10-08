package match_test

import (
	"net/http"
	"strings"
	"testing"
)

// Who may shape a table.
//
// Reported from production: adding a bot and dealing checked that the caller
// was signed in and then threw away who they were. Every other action on a
// table — Seat, Invite, DeleteAsHost — has always asked, so this was a pair
// of handlers that had been missed rather than a policy.
//
// It mattered because of what a join code is *for*. The code is the thing a
// host pastes into a chat, so it is not a secret and was never meant to be
// one; it is an invitation to sit down. Six characters were enough for anyone
// holding them to fill the last seat with a bot and deal the hand, and the
// friend the code was sent to then arrived to MATCH_ALREADY_STARTED. From the
// host's side that is indistinguishable from the invite being broken, which
// is how it was reported.
//
// The route is exercised by join code rather than by match id throughout,
// because that is the reachable one.

func TestOnlyTheHostMayAddABot(t *testing.T) {
	h := newInviteHarness(t)
	hostToken := token(t, "host-1", "Host", false)
	matchID := h.createMatch(t, hostToken)

	for _, who := range []struct {
		name  string
		token string
	}{
		// Somebody who took a seat is still not the host: the table screen
		// shows them "waiting for the host", not an Add-a-bot button.
		{"a seated player", token(t, "player-2", "Bo", false)},
		{"a passer-by with the code", token(t, "stranger", "Nobody", true)},
	} {
		t.Run(who.name, func(t *testing.T) {
			if who.name == "a seated player" {
				if res := h.do(http.MethodPost, "/matches/"+matchID+"/join", who.token, nil); res.status != http.StatusOK {
					t.Fatalf("joining: status %d body %s", res.status, res.raw)
				}
			}
			res := h.do(http.MethodPost, "/matches/"+matchID+"/add-bot", who.token, nil)
			if res.status != http.StatusForbidden {
				t.Fatalf("add-bot: status %d, want 403 (body %s)", res.status, res.raw)
			}
			if code := res.str("code"); code != "NOT_THE_HOST" {
				t.Errorf("code = %q, want NOT_THE_HOST", code)
			}
		})
	}

	// And the host is not locked out of their own table by the fix.
	if res := h.do(http.MethodPost, "/matches/"+matchID+"/add-bot", hostToken, nil); res.status != http.StatusOK {
		t.Fatalf("the host could not add a bot: status %d body %s", res.status, res.raw)
	}
}

func TestOnlyTheHostMayDeal(t *testing.T) {
	h := newInviteHarness(t)
	hostToken := token(t, "host-1", "Host", false)
	friendToken := token(t, "player-2", "Bo", false)
	matchID := h.createMatch(t, hostToken)
	if res := h.do(http.MethodPost, "/matches/"+matchID+"/join", friendToken, nil); res.status != http.StatusOK {
		t.Fatalf("joining: status %d body %s", res.status, res.raw)
	}

	// The whole of the reported story, in three calls: a stranger holding the
	// code stuffs the table and deals it, and the invited friend is locked
	// out of a game that was opened for them.
	strangerToken := token(t, "stranger", "Nobody", true)
	if res := h.do(http.MethodPost, "/matches/"+matchID+"/start", strangerToken, nil); res.status != http.StatusForbidden {
		t.Fatalf("start by a stranger: status %d, want 403 (body %s)", res.status, res.raw)
	}
	if res := h.do(http.MethodPost, "/matches/"+matchID+"/start", friendToken, nil); res.status != http.StatusForbidden {
		t.Fatalf("start by a seated non-host: status %d, want 403 (body %s)", res.status, res.raw)
	}

	// Untouched by either attempt, which is the part that matters: a refused
	// deal must leave a table somebody can still deal.
	if res := h.do(http.MethodGet, "/matches/"+matchID, "", nil); res.str("status") != "lobby" {
		t.Fatalf("status = %q, want the table still waiting", res.str("status"))
	}
	if res := h.do(http.MethodPost, "/matches/"+matchID+"/start", hostToken, nil); res.status != http.StatusOK {
		t.Fatalf("the host could not deal: status %d body %s", res.status, res.raw)
	}
}

// The join code is the reachable spelling of the id, and the one a host hands
// out, so the gate has to hold on that path too.
func TestTheHostGateHoldsWhenTheTableIsNamedByItsJoinCode(t *testing.T) {
	h := newInviteHarness(t)
	hostToken := token(t, "host-1", "Host", false)
	res := h.do(http.MethodPost, "/matches", hostToken, map[string]any{"moduleId": "prsi"})
	if res.status != http.StatusOK {
		t.Fatalf("create: status %d body %s", res.status, res.raw)
	}
	code := res.str("joinCode")
	if code == "" {
		t.Fatal("no join code was issued")
	}

	stranger := token(t, "stranger", "Nobody", true)
	if r := h.do(http.MethodPost, "/matches/"+code+"/add-bot", stranger, nil); r.status != http.StatusForbidden {
		t.Errorf("add-bot by code: status %d, want 403 (body %s)", r.status, r.raw)
	}
	if r := h.do(http.MethodPost, "/matches/"+code+"/start", stranger, nil); r.status != http.StatusForbidden {
		t.Errorf("start by code: status %d, want 403 (body %s)", r.status, r.raw)
	}
}

// The host may change a seated bot's strength at any point in the match —
// before the deal and during it — and nobody else may.
func TestOnlyTheHostMayChangeABotsStrength(t *testing.T) {
	h := newInviteHarness(t)
	hostToken := token(t, "host-1", "Host", false)
	matchID := h.createMatch(t, hostToken)

	added := h.do(http.MethodPost, "/matches/"+matchID+"/add-bot", hostToken, map[string]string{"skill": "easy"})
	if added.status != http.StatusOK {
		t.Fatalf("add-bot: status %d body %s", added.status, added.raw)
	}
	botID := added.str("playerId")
	// Encoded as the client's encodeURIComponent sends it: a bot id has a
	// colon in it, and Go's own PathEscape would leave that alone.
	path := "/matches/" + matchID + "/bots/" + strings.ReplaceAll(botID, ":", "%3A") + "/skill"

	stranger := token(t, "stranger", "Nobody", true)
	if res := h.do(http.MethodPost, path, stranger, map[string]string{"skill": "hard"}); res.status != http.StatusForbidden {
		t.Fatalf("a stranger changed the bot: status %d body %s", res.status, res.raw)
	}
	if res := h.do(http.MethodPost, path, hostToken, map[string]string{"skill": "superb"}); res.status != http.StatusBadRequest {
		t.Fatalf("an unknown skill: status %d, want 400 (body %s)", res.status, res.raw)
	}
	if res := h.do(http.MethodPost, "/matches/"+matchID+"/bots/host-1/skill", hostToken, map[string]string{"skill": "hard"}); res.str("code") != "NOT_AT_THIS_TABLE" {
		t.Fatalf("a person's seat was treated as a bot: status %d body %s", res.status, res.raw)
	}

	res := h.do(http.MethodPost, path, hostToken, map[string]string{"skill": "hard"})
	if res.status != http.StatusOK || res.str("skill") != "hard" {
		t.Fatalf("before the deal: status %d body %s", res.status, res.raw)
	}
	if res.str("name") == added.str("name") {
		t.Errorf("the bot kept the name %q of its old strength", res.str("name"))
	}
	if got := botSkillOnTable(t, h, matchID, botID); got != "hard" {
		t.Errorf("the table shows the bot at %q, want hard", got)
	}

	if res := h.do(http.MethodPost, "/matches/"+matchID+"/start", hostToken, nil); res.status != http.StatusOK {
		t.Fatalf("start: status %d body %s", res.status, res.raw)
	}
	if res := h.do(http.MethodPost, path, hostToken, map[string]string{"skill": "medium"}); res.status != http.StatusOK || res.str("skill") != "medium" {
		t.Fatalf("during the match: status %d body %s", res.status, res.raw)
	}
	if got := botSkillOnTable(t, h, matchID, botID); got != "medium" {
		t.Errorf("the table shows the bot at %q, want medium", got)
	}
}

func botSkillOnTable(t *testing.T, h *inviteHarness, matchID, botID string) string {
	t.Helper()
	res := h.do(http.MethodGet, "/matches/"+matchID, "", nil)
	players, _ := res.body["players"].([]any)
	for _, p := range players {
		if pm, _ := p.(map[string]any); pm["id"] == botID {
			s, _ := pm["skill"].(string)
			return s
		}
	}
	t.Fatalf("bot %s is not at the table: %s", botID, res.raw)
	return ""
}
