package auth

import (
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// A web client keeps its long-lived secrets in cookies rather than in
// localStorage.
//
// Two reasons, both about the browser. Safari erases storage a page script
// wrote once the site has gone seven days without a visit, which turned a
// guest who stayed away for a week into a stranger and lost exactly the
// history the guest key exists to keep; a cookie the server sets is not
// subject to that. And an HttpOnly cookie is out of reach of any script on
// the page, where localStorage is readable by all of them.
//
// It is opt-in per request, by SessionCookieHeader. Native clients keep their
// tokens in the platform keychain and never send it, so nothing changes for
// them, and a request body that carries a token still wins over the cookie:
// that is how a browser that stored its tokens before this existed moves
// them across on its first refresh.
const (
	// SessionCookieHeader asks for the session to be kept in cookies. Its
	// value is SessionCookieMode.
	SessionCookieHeader = "X-Zolik-Session"
	SessionCookieMode   = "cookie"

	refreshCookie = "zolik_refresh"
	guestCookie   = "zolik_guest"
	// cookiePath confines both to the auth endpoints, the only routes that
	// read them, so they ride along with nothing else.
	cookiePath = "/auth"
	// guestCookieTTL is as long as browsers keep any cookie. The key never
	// expires on the server, so the cookie is the only thing that can.
	guestCookieTTL = 400 * 24 * time.Hour
)

// wantsSessionCookies reports whether this request asked for cookie mode.
func wantsSessionCookies(req *http.Request) bool {
	return req.Header.Get(SessionCookieHeader) == SessionCookieMode
}

func cookieValue(req *http.Request, name string) string {
	c, err := req.Cookie(name)
	if err != nil {
		return ""
	}
	return c.Value
}

// refreshTokenOf is the refresh token a request presents: the body's when it
// has one, otherwise the browser's cookie.
func refreshTokenOf(req *http.Request, fromBody string) string {
	if fromBody != "" {
		return fromBody
	}
	return cookieValue(req, refreshCookie)
}

// guestProofOf is the same for the guest proof. Either half of the body's
// proof overrides both cookies: a body proof is a deliberate choice of
// identity, such as opening one's own guest link on a new device.
func guestProofOf(req *http.Request, key, refresh string) (string, string) {
	if key != "" || refresh != "" {
		return key, refresh
	}
	return cookieValue(req, guestCookie), cookieValue(req, refreshCookie)
}

func setAuthCookie(w http.ResponseWriter, req *http.Request, name, value string, ttl time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     cookiePath,
		MaxAge:   int(ttl / time.Second),
		HttpOnly: true,
		Secure:   isHTTPS(req),
		// Strict, because every request that should carry these comes from
		// the app's own page. It is also what makes the credentialed CORS in
		// cmd/server safe: a request started by any other site never has
		// them.
		SameSite: http.SameSiteStrictMode,
	})
}

func clearAuthCookie(w http.ResponseWriter, req *http.Request, name string) {
	setAuthCookie(w, req, name, "", -time.Second)
}

// isHTTPS decides the Secure flag. Behind the production proxy the request
// arrives as plain HTTP, so the forwarded scheme is the one that counts; a
// dev server on http://localhost gets no Secure flag, which Safari would
// otherwise refuse to store.
func isHTTPS(req *http.Request) bool {
	return req.TLS != nil || strings.EqualFold(req.Header.Get("X-Forwarded-Proto"), "https")
}

// writeTokens answers any request that hands out a session. In cookie mode
// the refresh token goes into its cookie and leaves the body empty, so it
// never reaches a script; the guest key goes into one too but stays in the
// body, because the app shows it as the guest's own link.
//
// A sign-in that claimed a guest's history clears the guest cookie, as the
// client forgets its stored identity then: the history has moved to the
// account, and the key is worth nothing after it.
func writeTokens(w http.ResponseWriter, req *http.Request, out map[string]any) {
	if wantsSessionCookies(req) {
		if token, _ := out["refreshToken"].(string); token != "" {
			setAuthCookie(w, req, refreshCookie, token, refreshTokenTTL)
			out["refreshToken"] = ""
		}
		if key, _ := out["guestKey"].(string); key != "" {
			setAuthCookie(w, req, guestCookie, key, guestCookieTTL)
		}
		if claimed, _ := out["claimedMatches"].(int); claimed > 0 {
			clearAuthCookie(w, req, guestCookie)
		}
	}
	writeJSON(w, out)
}

// SameHostOrigin reports whether a browser Origin names the host this request
// was addressed to, on whatever port. That is the web app talking to its own
// server: the same origin in production, and Metro on another port of the
// same machine in development. Only these get credentialed CORS.
func SameHostOrigin(req *http.Request, origin string) bool {
	u, err := url.Parse(origin)
	if err != nil || u.Hostname() == "" {
		return false
	}
	host := req.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	return strings.EqualFold(u.Hostname(), strings.Trim(host, "[]"))
}
