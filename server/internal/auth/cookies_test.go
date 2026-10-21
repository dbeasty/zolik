package auth_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"testing"

	"github.com/golang-jwt/jwt/v5"

	"zolik/server/internal/auth"
)

// browser is a web client in cookie mode: it sends the opt-in header and keeps
// whatever cookies the server sets, the way the web build's fetch does.
type browser struct {
	h      *testHarness
	client *http.Client
	// last holds the cookies set by the most recent response.
	last []*http.Cookie
}

func (h *testHarness) browser() *browser {
	h.t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		h.t.Fatalf("cookie jar: %v", err)
	}
	return &browser{h: h, client: &http.Client{Jar: jar}}
}

func (b *browser) do(method, path, bearer string, body any) apiResponse {
	b.h.t.Helper()
	var reader io.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, b.h.server.URL+path, reader)
	if err != nil {
		b.h.t.Fatalf("building request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(auth.SessionCookieHeader, auth.SessionCookieMode)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := b.client.Do(req)
	if err != nil {
		b.h.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	b.last = resp.Cookies()
	raw, _ := io.ReadAll(resp.Body)
	out := apiResponse{status: resp.StatusCode, raw: string(raw)}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out.body)
	}
	return out
}

func (b *browser) setCookie(name string) *http.Cookie {
	for _, c := range b.last {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func TestWebSessionKeepsTheRefreshTokenOutOfTheBody(t *testing.T) {
	h := newTestHarness(t)
	b := h.browser()

	guest := b.do(http.MethodPost, "/auth/guest", "", map[string]any{"guestName": "Wendy"})
	if guest.status != http.StatusOK {
		t.Fatalf("guest: status %d body %s", guest.status, guest.raw)
	}
	if guest.str("refreshToken") != "" {
		t.Error("the refresh token was handed to the page in cookie mode")
	}
	if guest.str("accessToken") == "" || guest.str("guestKey") == "" {
		t.Errorf("guest answer lost its access token or guest key: %s", guest.raw)
	}
	for _, name := range []string{"zolik_refresh", "zolik_guest"} {
		c := b.setCookie(name)
		if c == nil {
			t.Fatalf("no %s cookie was set", name)
		}
		if !c.HttpOnly || c.SameSite != http.SameSiteStrictMode || c.Path != "/auth" {
			t.Errorf("%s: HttpOnly=%v SameSite=%v Path=%q, want HttpOnly, Strict, /auth", name, c.HttpOnly, c.SameSite, c.Path)
		}
	}

	// The browser refreshes with nothing in the body but the time zone.
	rotated := b.do(http.MethodPost, "/auth/refresh", "", map[string]any{})
	if rotated.status != http.StatusOK || rotated.str("accessToken") == "" {
		t.Fatalf("cookie refresh: status %d body %s", rotated.status, rotated.raw)
	}
	if rotated.str("refreshToken") != "" {
		t.Error("the rotated refresh token was handed to the page")
	}
	if b.setCookie("zolik_refresh") == nil {
		t.Error("the rotated refresh token was not written back to the cookie")
	}

	if res := b.do(http.MethodPost, "/auth/logout", "", map[string]any{}); res.status != http.StatusOK {
		t.Fatalf("logout: status %d", res.status)
	}
	if c := b.setCookie("zolik_refresh"); c == nil || c.MaxAge >= 0 {
		t.Error("logout did not clear the refresh cookie")
	}
	if c := b.setCookie("zolik_guest"); c != nil {
		t.Error("logout touched the guest cookie, which is meant to outlive a sign-out")
	}
	if res := b.do(http.MethodPost, "/auth/refresh", "", map[string]any{}); res.status == http.StatusOK {
		t.Error("refresh still worked after logout")
	}
}

// The case cookies exist for: the page's storage was wiped (Safari does this
// after a week away) and the client asks for a guest session with no proof.
func TestWebGuestResumesFromItsCookieAfterStorageIsLost(t *testing.T) {
	h := newTestHarness(t)
	b := h.browser()

	first := b.do(http.MethodPost, "/auth/guest", "", map[string]any{"guestName": "Ivy"})
	b.do(http.MethodPost, "/auth/logout", "", map[string]any{})

	again := b.do(http.MethodPost, "/auth/guest", "", map[string]any{})
	if again.str("guestId") != first.str("guestId") {
		t.Errorf("guest id = %q, want the cookie's %q", again.str("guestId"), first.str("guestId"))
	}

	// A different browser, holding no cookie, is a different guest.
	other := h.browser().do(http.MethodPost, "/auth/guest", "", map[string]any{})
	if other.str("guestId") == first.str("guestId") {
		t.Error("a browser with no cookie resumed somebody else's guest")
	}
}

// A body proof is a deliberate choice, such as opening one's own guest link,
// and wins over whatever guest the cookie holds.
func TestWebGuestBodyKeyOverridesTheCookie(t *testing.T) {
	h := newTestHarness(t)
	elsewhere := h.do(http.MethodPost, "/auth/guest", "", map[string]any{"guestName": "Linked"})

	b := h.browser()
	b.do(http.MethodPost, "/auth/guest", "", map[string]any{"guestName": "Local"})
	res := b.do(http.MethodPost, "/auth/guest", "", map[string]any{"guestKey": elsewhere.str("guestKey")})
	if res.str("guestId") != elsewhere.str("guestId") {
		t.Fatalf("guest id = %q, want the linked %q", res.str("guestId"), elsewhere.str("guestId"))
	}
	// And the cookie now follows the identity just chosen.
	b.do(http.MethodPost, "/auth/logout", "", map[string]any{})
	if res := b.do(http.MethodPost, "/auth/guest", "", map[string]any{}); res.str("guestId") != elsewhere.str("guestId") {
		t.Error("the guest cookie still names the guest that was replaced")
	}
}

// A browser that stored its refresh token before cookies existed sends it in
// the body; the server takes it and moves it into the cookie.
func TestWebRefreshMovesAStoredTokenIntoTheCookie(t *testing.T) {
	h := newTestHarness(t)
	legacy := h.do(http.MethodPost, "/auth/guest", "", map[string]any{"guestName": "Old"})

	b := h.browser()
	moved := b.do(http.MethodPost, "/auth/refresh", "", map[string]any{"refreshToken": legacy.str("refreshToken")})
	if moved.status != http.StatusOK || moved.str("refreshToken") != "" {
		t.Fatalf("migrating refresh: status %d body %s", moved.status, moved.raw)
	}
	if res := b.do(http.MethodPost, "/auth/refresh", "", map[string]any{}); res.status != http.StatusOK {
		t.Errorf("refreshing from the migrated cookie: status %d body %s", res.status, res.raw)
	}
}

func TestWebRefusedRefreshClearsTheCookie(t *testing.T) {
	h := newTestHarness(t)
	b := h.browser()
	res := b.do(http.MethodPost, "/auth/refresh", "", map[string]any{"refreshToken": "not-a-token"})
	if res.status != http.StatusUnauthorized {
		t.Fatalf("refresh with a bad token: status %d, want 401", res.status)
	}
	if c := b.setCookie("zolik_refresh"); c == nil || c.MaxAge >= 0 {
		t.Error("a refused refresh left its cookie in place")
	}
}

func TestWebClaimGuestReadsTheCookieAndClearsIt(t *testing.T) {
	h := newTestHarness(t)
	b := h.browser()

	guest := b.do(http.MethodPost, "/auth/guest", "", map[string]any{"guestName": "Gail"})
	h.seedGuestMatch(guest.str("guestId"))

	account := b.do(http.MethodPost, "/auth/oauth/fake/token", "", map[string]any{
		"idToken": h.oidc.sign(t, jwt.MapClaims{"sub": "gail-account", "email": "gail@example.com"}),
	})
	if account.status != http.StatusOK || account.str("refreshToken") != "" {
		t.Fatalf("account sign-in: status %d body %s", account.status, account.raw)
	}

	claim := b.do(http.MethodPost, "/auth/claim-guest", account.str("accessToken"), map[string]any{})
	if claim.status != http.StatusOK || claim.num("claimedMatches") != 1 {
		t.Fatalf("claim from the cookie: status %d body %s, want 1 match", claim.status, claim.raw)
	}
	if c := b.setCookie("zolik_guest"); c == nil || c.MaxAge >= 0 {
		t.Error("claiming the guest left its cookie behind")
	}

	// The account's own session still refreshes: only the guest went.
	if res := b.do(http.MethodPost, "/auth/refresh", "", map[string]any{}); res.status != http.StatusOK || res.str("userId") != account.str("userId") {
		t.Errorf("account refresh after the claim: status %d body %s", res.status, res.raw)
	}
}

// Native clients never ask for cookies, and get exactly what they always did.
func TestWithoutTheHeaderNoCookiesAreSet(t *testing.T) {
	h := newTestHarness(t)
	req, _ := http.NewRequest(http.MethodPost, h.server.URL+"/auth/guest", bytes.NewReader([]byte(`{}`)))
	resp, err := h.server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if len(resp.Cookies()) != 0 {
		t.Errorf("cookies set without the opt-in header: %v", resp.Cookies())
	}
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if s, _ := body["refreshToken"].(string); s == "" {
		t.Error("the refresh token went missing from a native client's answer")
	}
}

func TestSameHostOrigin(t *testing.T) {
	for _, tc := range []struct {
		host, origin string
		want         bool
	}{
		{"localhost:8080", "http://localhost:8081", true},
		{"jokerless.com", "https://jokerless.com", true},
		{"192.168.1.20:8080", "http://192.168.1.20:8081", true},
		{"[::1]:8080", "http://[::1]:8081", true},
		{"jokerless.com", "https://evil.example", false},
		{"jokerless.com", "https://jokerless.com.evil.example", false},
		{"localhost:8080", "", false},
		{"localhost:8080", "null", false},
	} {
		req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
		req.Host = tc.host
		if got := auth.SameHostOrigin(req, tc.origin); got != tc.want {
			t.Errorf("SameHostOrigin(host %q, origin %q) = %v, want %v", tc.host, tc.origin, got, tc.want)
		}
	}
}
