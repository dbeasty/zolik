package match_test

import (
	"net/http"
	"strings"
	"testing"
)

// handCards is how many of ownerID's cards the answer names. A hand the
// caller may not see arrives as a count with no cards in it.
func handCards(res apiResponse, ownerID string) int {
	view, _ := res.body["view"].(map[string]any)
	zones, _ := view["zones"].([]any)
	for _, z := range zones {
		zone, _ := z.(map[string]any)
		if zone["kind"] != "hand" || zone["ownerId"] != ownerID {
			continue
		}
		cards, _ := zone["cards"].([]any)
		return len(cards)
	}
	return 0
}

// hostsOffer is any offer in the answer that acts for ownerID: one the viewer
// may take, or one reaching into ownerID's hand. A spectator is still shown
// the disabled placeholders, so their mere presence proves nothing.
func hostsOffer(res apiResponse, ownerID string) map[string]any {
	offers, _ := res.body["legalActions"].([]any)
	for _, o := range offers {
		offer, _ := o.(map[string]any)
		if offer["enabled"] == true {
			return offer
		}
		for _, end := range []string{"source", "target"} {
			if ref, _ := offer[end].(map[string]any); ref["ownerId"] == ownerID {
				return offer
			}
		}
	}
	return nil
}

// GET /matches/{id} once projected for whatever ?as= said, with no token at
// all, and players[].id is broadcast to everyone at the table: anyone could
// read anyone's hand. The viewer is now the token's subject and nobody else.
func TestGetMatchProjectsForTheTokenNotTheQuery(t *testing.T) {
	h := newInviteHarness(t)
	hostID := "host-get-1"
	hostToken := token(t, hostID, "Host", false)
	matchID := startedMatch(t, h, hostToken)
	path := "/matches/" + matchID

	own := h.do(http.MethodGet, path, hostToken, nil)
	if own.status != http.StatusOK {
		t.Fatalf("own view: status %d body %s", own.status, own.raw)
	}
	if handCards(own, hostID) == 0 {
		t.Fatalf("the host's own token should show the host's hand: %s", own.raw)
	}
	if hostsOffer(own, hostID) == nil {
		t.Fatalf("the host's own token should carry the host's offers: %s", own.raw)
	}

	cases := map[string]string{
		"no token":             "",
		"someone else's token": token(t, "stranger-get-1", "Stranger", false),
		"a guest's token":      token(t, "0123456789abcdef0123456789abcdef", "Guest", true),
		"a malformed token":    "not-a-jwt",
	}
	for name, bearer := range cases {
		t.Run(name, func(t *testing.T) {
			res := h.do(http.MethodGet, path+"?as="+hostID, bearer, nil)
			if res.status != http.StatusOK {
				t.Fatalf("status %d body %s", res.status, res.raw)
			}
			if n := handCards(res, hostID); n != 0 {
				t.Fatalf("?as=%s showed %d of the host's cards to %s", hostID, n, name)
			}
			if offer := hostsOffer(res, hostID); offer != nil {
				t.Fatalf("?as=%s handed %s the host's legal action %v", hostID, name, offer)
			}
		})
	}

	// The same caller asking for their own seat by name still gets it — an
	// ?as= that agrees with the token is harmless, just redundant.
	if res := h.do(http.MethodGet, path+"?as="+hostID, hostToken, nil); handCards(res, hostID) == 0 {
		t.Fatalf("a matching ?as= should not blank the caller's own hand: %s", res.raw)
	}
}

// A seat link's token is scoped to one match. At that match it is its
// holder's own seat, hand included; at any other it is no token at all —
// even where the same player is seated.
func TestGetMatchHonoursASeatTokenAtItsOwnMatchOnly(t *testing.T) {
	h := newInviteHarness(t)
	matchID, annTok, link := tableWithASeatLink(t, h)
	claim := h.do(http.MethodPost, strings.Replace(link, "/seat/", "/seats/", 1)+"/claim", "", nil)
	if claim.status != http.StatusOK {
		t.Fatalf("claim: %d %s", claim.status, claim.raw)
	}
	seatTok := claim.str("accessToken")

	if res := h.do(http.MethodGet, "/matches/"+matchID, seatTok, nil); handCards(res, "bob") == 0 {
		t.Fatalf("the seat token should show Bob his own hand at its table: %s", res.raw)
	}

	other := h.createMatch(t, annTok)
	if res := h.do(http.MethodPost, "/matches/"+other+"/join", token(t, "bob", "Bob", true), nil); res.status != http.StatusOK {
		t.Fatalf("bob joining the other table: %d %s", res.status, res.raw)
	}
	if res := h.do(http.MethodPost, "/matches/"+other+"/start", annTok, nil); res.status != http.StatusOK {
		t.Fatalf("start the other table: %d %s", res.status, res.raw)
	}
	if res := h.do(http.MethodGet, "/matches/"+other, seatTok, nil); handCards(res, "bob") != 0 {
		t.Fatalf("a token scoped to %s showed Bob's hand at %s", matchID, other)
	}
}
