package auth_test

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

// browserLinkFlow starts a link flow with bearer's token and finishes the
// provider half as subject, returning the exchange code the browser that
// finished it is sent back with.
func browserLinkFlow(t *testing.T, h *testHarness, bearer, subject string) string {
	t.Helper()
	start := h.do(http.MethodPost, "/auth/oauth/fake/start", bearer, map[string]any{"returnTo": "app://return", "link": true})
	if start.status != http.StatusOK {
		t.Fatalf("start: status %d body %s", start.status, start.raw)
	}
	authURL, err := url.Parse(start.str("authorizationUrl"))
	if err != nil {
		t.Fatal(err)
	}
	h.oidc.idToken = h.oidc.sign(t, jwt.MapClaims{
		"sub": subject, "email": subject + "@example.com", "nonce": authURL.Query().Get("nonce"),
	})
	resp := h.doNoRedirect(http.MethodGet, "/auth/oauth/fake/callback?state="+url.QueryEscape(authURL.Query().Get("state"))+"&code=c")
	defer resp.Body.Close()
	loc, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	if e := loc.Query().Get("error"); e != "" {
		t.Fatalf("callback failed: %s", e)
	}
	code := loc.Query().Get("code")
	if code == "" {
		t.Fatalf("callback returned no code: %s", loc)
	}
	return code
}

func signInNative(t *testing.T, h *testHarness, subject string) apiResponse {
	t.Helper()
	res := h.do(http.MethodPost, "/auth/oauth/fake/token", "", map[string]any{
		"idToken": h.oidc.sign(t, jwt.MapClaims{"sub": subject, "email": subject + "@example.com"}),
	})
	if res.status != http.StatusOK {
		t.Fatalf("sign-in %s: status %d body %s", subject, res.status, res.raw)
	}
	return res
}

func linkedTo(t *testing.T, h *testHarness, subject string) string {
	t.Helper()
	id, err := h.store.FindIdentity(context.Background(), "fake", subject)
	if err != nil {
		return ""
	}
	return id.UserID
}

// The attacker starts a link flow with their own token and sends the
// provider URL to a victim already signed in to the provider. Before the fix
// the callback made the link there and then, so the victim's provider
// identity became a way into the attacker's account (and the victim's next
// "sign in with" landed there). Now the callback holds the identity and only
// the account that started the flow can attach it.
func TestALinkFlowFinishedByAnotherBrowserLinksNothing(t *testing.T) {
	h := newTestHarness(t)
	attacker := signInNative(t, h, "attacker-base")
	victim := signInNative(t, h, "victim-base")

	code := browserLinkFlow(t, h, attacker.str("accessToken"), "victim-provider")

	if got := linkedTo(t, h, "victim-provider"); got != "" {
		t.Fatalf("the callback linked the victim's identity to %s before anybody exchanged", got)
	}
	// The victim's app collects the code with the victim's own token.
	res := h.do(http.MethodPost, "/auth/oauth/exchange", victim.str("accessToken"), map[string]any{"code": code})
	if res.status != http.StatusForbidden {
		t.Fatalf("exchange by another account: status %d body %s, want 403", res.status, res.raw)
	}
	if got := linkedTo(t, h, "victim-provider"); got != "" {
		t.Fatalf("the victim's identity was linked to %s", got)
	}
	// The code is spent: the attacker cannot collect it afterwards either.
	if res := h.do(http.MethodPost, "/auth/oauth/exchange", attacker.str("accessToken"), map[string]any{"code": code}); res.status == http.StatusOK {
		t.Fatal("a refused link code was collected a second time")
	}
}

func TestALinkFlowExchangedWithoutATokenLinksNothing(t *testing.T) {
	h := newTestHarness(t)
	attacker := signInNative(t, h, "attacker-base-2")
	code := browserLinkFlow(t, h, attacker.str("accessToken"), "victim-provider-2")
	if res := h.do(http.MethodPost, "/auth/oauth/exchange", "", map[string]any{"code": code}); res.status != http.StatusForbidden {
		t.Fatalf("anonymous exchange: status %d body %s, want 403", res.status, res.raw)
	}
	if got := linkedTo(t, h, "victim-provider-2"); got != "" {
		t.Fatalf("linked to %s", got)
	}
}

// The person who started the link and finished it themselves gets it.
func TestALinkFlowExchangedByItsOwnAccountLinks(t *testing.T) {
	h := newTestHarness(t)
	me := signInNative(t, h, "me-base")
	code := browserLinkFlow(t, h, me.str("accessToken"), "me-second")

	res := h.do(http.MethodPost, "/auth/oauth/exchange", me.str("accessToken"), map[string]any{"code": code})
	if res.status != http.StatusOK {
		t.Fatalf("exchange: status %d body %s", res.status, res.raw)
	}
	if !res.boolean("linked") || res.str("accessToken") != "" {
		t.Errorf("a link answers linked with no new session, got %s", res.raw)
	}
	if got := linkedTo(t, h, "me-second"); got != me.str("userId") {
		t.Fatalf("identity linked to %q, want %s", got, me.str("userId"))
	}
}

// An identity that already belongs elsewhere is still refused with the reason
// code the app explains, at the callback.
func TestALinkFlowForAnIdentityOwnedElsewhereFailsAtTheCallback(t *testing.T) {
	h := newTestHarness(t)
	signInNative(t, h, "owned-elsewhere")
	me := signInNative(t, h, "me-base-3")

	start := h.do(http.MethodPost, "/auth/oauth/fake/start", me.str("accessToken"), map[string]any{"returnTo": "app://return", "link": true})
	authURL, _ := url.Parse(start.str("authorizationUrl"))
	h.oidc.idToken = h.oidc.sign(t, jwt.MapClaims{"sub": "owned-elsewhere", "nonce": authURL.Query().Get("nonce")})
	resp := h.doNoRedirect(http.MethodGet, "/auth/oauth/fake/callback?state="+url.QueryEscape(authURL.Query().Get("state"))+"&code=c")
	defer resp.Body.Close()
	loc, _ := url.Parse(resp.Header.Get("Location"))
	if got := loc.Query().Get("error"); got != "already_linked" {
		t.Fatalf("callback error = %q, want already_linked (Location %s)", got, loc)
	}
}
