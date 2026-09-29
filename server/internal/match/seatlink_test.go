package match_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

// A table Ann and Bob are playing, and a link Ann made for Bob's seat.
func tableWithASeatLink(t *testing.T, h *inviteHarness) (matchID, annTok, link string) {
	t.Helper()
	annTok = token(t, "ann", "Ann", false)
	matchID = h.createMatch(t, annTok)
	if res := h.do(http.MethodPost, "/matches/"+matchID+"/join", token(t, "bob", "Bob", true), nil); res.status != http.StatusOK {
		t.Fatalf("bob joining: %d %s", res.status, res.raw)
	}
	if res := h.do(http.MethodPost, "/matches/"+matchID+"/start", annTok, nil); res.status != http.StatusOK {
		t.Fatalf("start: %d %s", res.status, res.raw)
	}
	res := h.do(http.MethodPost, "/matches/"+matchID+"/seats/bob/link", annTok, nil)
	if res.status != http.StatusOK {
		t.Fatalf("minting bob's link: %d %s", res.status, res.raw)
	}
	link = res.str("path")
	if !strings.HasPrefix(link, "/seat/"+matchID+"/") {
		t.Fatalf("link %q is not a seat link for this table", link)
	}
	return matchID, annTok, link
}

func dialFails(t *testing.T, h *inviteHarness, matchID, tok string) bool {
	t.Helper()
	url := strings.Replace(h.server.URL, "http://", "ws://", 1) + "/ws/matches/" + matchID + "?token=" + tok
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err == nil {
		_ = conn.Close()
		return false
	}
	return true
}

// The whole of it, as Bob on a new phone meets it: the link says whose seat
// it is, taking it gives him a token that plays that seat, and that token
// reaches nothing else.
func TestASeatLinkBringsThatPersonBackToThatSeatAndNowhereElse(t *testing.T) {
	h := newInviteHarness(t)
	matchID, annTok, link := tableWithASeatLink(t, h)

	// Anybody holding the link may see whose it is — and only names.
	preview := h.do(http.MethodGet, strings.Replace(link, "/seat/", "/seats/", 1), "", nil)
	if preview.status != http.StatusOK {
		t.Fatalf("preview: %d %s", preview.status, preview.raw)
	}
	seat, _ := preview.body["seat"].(map[string]any)
	if seat["name"] != "Bob" || seat["id"] != "bob" {
		t.Fatalf("the link should name Bob's seat, got %v", seat)
	}
	if strings.Contains(preview.raw, "seatKeyHash") || strings.Contains(preview.raw, "hand") {
		t.Fatalf("the preview says more than who is at the table: %s", preview.raw)
	}

	// A new phone, signed in as nobody, takes it.
	claim := h.do(http.MethodPost, strings.Replace(link, "/seat/", "/seats/", 1)+"/claim", "", nil)
	if claim.status != http.StatusOK || claim.str("userId") != "bob" {
		t.Fatalf("claim: %d %s", claim.status, claim.raw)
	}
	seatTok := claim.str("accessToken")

	// It plays Bob's seat at this table…
	dialMatch(t, h, matchID, seatTok)
	// …and while it is, nobody else can take the seat with the same link.
	again := h.do(http.MethodPost, strings.Replace(link, "/seat/", "/seats/", 1)+"/claim", "", nil)
	if again.status != http.StatusConflict || again.str("code") != "SEAT_IN_USE" {
		t.Fatalf("claiming a seat being played: %d %s", again.status, again.raw)
	}
	// It reaches the routes a seated player uses at this table — refused on
	// the rules (the table is not abandoned), not on who is asking.
	if res := h.do(http.MethodPost, "/matches/"+matchID+"/resume", seatTok, nil); res.status == http.StatusUnauthorized {
		t.Fatalf("resume turned the seat token away: %s", res.raw)
	}

	// …and nothing else: not "about me", not another table.
	if res := h.do(http.MethodGet, "/users/me/tables", seatTok, nil); res.status != http.StatusUnauthorized {
		t.Fatalf("/users/me/tables with a seat token: %d, want 401", res.status)
	}
	other := h.createMatch(t, annTok)
	if !dialFails(t, h, other, seatTok) {
		t.Fatal("a seat token opened another table's socket")
	}
	if res := h.do(http.MethodPost, "/matches", seatTok, map[string]any{"moduleId": "prsi"}); res.status != http.StatusUnauthorized {
		t.Fatalf("creating a table with a seat token: %d, want 401", res.status)
	}
}

// Somebody who already is that seat is just sent to the table.
func TestClaimingYourOwnSeatIsANoOp(t *testing.T) {
	h := newInviteHarness(t)
	_, _, link := tableWithASeatLink(t, h)
	res := h.do(http.MethodPost, strings.Replace(link, "/seat/", "/seats/", 1)+"/claim", token(t, "bob", "Bob", true), nil)
	if res.status != http.StatusOK || res.body["alreadyYours"] != true || res.str("accessToken") != "" {
		t.Fatalf("claim by bob himself: %d %s", res.status, res.raw)
	}
}

// Who may make a link, for whom, and when; and making another kills the old.
func TestSeatLinkRefusals(t *testing.T) {
	h := newInviteHarness(t)
	matchID, annTok, link := tableWithASeatLink(t, h)

	if res := h.do(http.MethodPost, "/matches/"+matchID+"/seats/bob/link", token(t, "eve", "Eve", false), nil); res.str("code") != "NOT_AT_THIS_TABLE" {
		t.Errorf("a stranger minting: %d %s", res.status, res.raw)
	}
	if res := h.do(http.MethodPost, "/matches/"+matchID+"/seats/nobody/link", annTok, nil); res.str("code") != "NOT_AT_THIS_TABLE" {
		t.Errorf("minting for nobody: %d %s", res.status, res.raw)
	}
	lobby := h.createMatch(t, annTok)
	if res := h.do(http.MethodPost, "/matches/"+lobby+"/seats/ann/link", annTok, nil); res.str("code") != "SEAT_LINK_EXPIRED" {
		t.Errorf("minting at a lobby (the join link's job): %d %s", res.status, res.raw)
	}

	// A second link for the same seat replaces the first.
	if res := h.do(http.MethodPost, "/matches/"+matchID+"/seats/bob/link", annTok, nil); res.status != http.StatusOK {
		t.Fatalf("minting again: %d %s", res.status, res.raw)
	}
	old := h.do(http.MethodGet, strings.Replace(link, "/seat/", "/seats/", 1), "", nil)
	if old.status != http.StatusNotFound || old.str("code") != "SEAT_LINK_EXPIRED" {
		t.Errorf("the replaced link still opens: %d %s", old.status, old.raw)
	}
	// And a guessed one opens nothing.
	if res := h.do(http.MethodGet, "/seats/"+matchID+"/00000000000000000000000000000000", "", nil); res.str("code") != "SEAT_LINK_EXPIRED" {
		t.Errorf("a guessed link: %d %s", res.status, res.raw)
	}
}
