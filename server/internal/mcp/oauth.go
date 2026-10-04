package mcp

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"

	"zolik/server/internal/auth"
)

// OAuth for MCP clients that cannot be handed a token — claude.ai's custom
// connectors above all, which discover how to sign in, register themselves and
// run an authorization-code flow.
//
// This is the MCP authorization profile: /mcp is a protected resource
// (RFC 9728) whose 401 points at its metadata; this server is also the
// authorization server (RFC 8414) with dynamic client registration (RFC 7591)
// and PKCE (RFC 7636, S256 only). Public clients only — there is no client
// secret to keep, which is the point of PKCE.
//
// Nothing here is stored. A registered client is a signed token naming its
// redirect URIs; an authorization request, a code and a refresh token are the
// same. Each kind is signed with its own derived key (auth.DeriveKey), so one
// can never be presented as another, nor as an access token. The cost of
// statelessness is that a refresh token cannot be revoked before it expires
// (30 days); the benefit is that nothing is lost on a restart and any instance
// can answer.
//
// The person is the real login. /oauth/authorize sends them to the app's
// consent page, where they are signed in as themselves and choose to allow or
// refuse; the agent that results is its own identity — one per (person, client
// name), so reconnecting keeps its seat — and its token can only play.

const (
	oauthScope      = "play"
	accessTTL       = time.Hour
	refreshTTL      = 30 * 24 * time.Hour
	codeTTL         = time.Minute
	requestTTL      = 10 * time.Minute
	registrationTTL = 365 * 24 * time.Hour
)

// otoken is every OAuth token's claims; Typ says which kind, and a different
// key per kind says it again cryptographically.
type otoken struct {
	Typ          string   `json:"typ"`
	ClientName   string   `json:"cn,omitempty"`
	RedirectURIs []string `json:"ru,omitempty"`
	ClientID     string   `json:"cid,omitempty"`
	RedirectURI  string   `json:"ruri,omitempty"`
	Challenge    string   `json:"cc,omitempty"`
	State        string   `json:"st,omitempty"`
	Owner        string   `json:"own,omitempty"`
	Agent        string   `json:"ag,omitempty"`
	jwt.RegisteredClaims
}

func signO(typ string, c otoken, ttl time.Duration) (string, error) {
	c.Typ = typ
	now := time.Now()
	c.RegisteredClaims = jwt.RegisteredClaims{
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		ID:        randID(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(auth.DeriveKey("oauth-" + typ))
}

func parseO(typ, tok string) (*otoken, error) {
	var c otoken
	parsed, err := jwt.ParseWithClaims(tok, &c, func(*jwt.Token) (any, error) {
		return auth.DeriveKey("oauth-" + typ), nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil || !parsed.Valid || c.Typ != typ {
		return nil, fmt.Errorf("invalid %s", typ)
	}
	return &c, nil
}

func randID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// usedCodes makes an authorization code single-use. In memory, and that is
// enough: a code lives a minute, and a restart inside it loses only the replay
// guard for codes that were about to expire anyway.
type usedCodes struct {
	mu sync.Mutex
	m  map[string]time.Time
}

func (u *usedCodes) take(id string, until time.Time) bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.m == nil {
		u.m = map[string]time.Time{}
	}
	now := time.Now()
	for k, exp := range u.m {
		if now.After(exp) {
			delete(u.m, k)
		}
	}
	if _, dup := u.m[id]; dup {
		return false
	}
	u.m[id] = until
	return true
}

// SetConsentBase fixes where the person is sent to approve a connection: the
// origin serving the app. Defaults to the server's own origin, which is right
// wherever the app is compiled into the binary.
func (h *Handlers) SetConsentBase(u string) { h.consentBase = strings.TrimRight(u, "/") }

func (h *Handlers) registerOAuth(r chi.Router) {
	r.Get("/.well-known/oauth-protected-resource", h.protectedResource)
	r.Get("/.well-known/oauth-protected-resource/mcp", h.protectedResource)
	r.Get("/.well-known/oauth-authorization-server", h.authServerMetadata)
	r.Post("/oauth/register", h.oauthRegister)
	r.Get("/oauth/authorize", h.oauthAuthorize)
	r.Get("/oauth/request", h.oauthRequest)
	r.With(auth.AuthMiddleware).Post("/oauth/approve", h.oauthApprove)
	r.Post("/oauth/token", h.oauthToken)
}

func writeJSONStatus(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func oauthErr(w http.ResponseWriter, status int, code, desc string) {
	writeJSONStatus(w, status, map[string]string{"error": code, "error_description": desc})
}

func (h *Handlers) protectedResource(w http.ResponseWriter, req *http.Request) {
	o := h.origin(req)
	writeJSONStatus(w, http.StatusOK, map[string]any{
		"resource":                 o + "/mcp",
		"authorization_servers":    []string{o},
		"bearer_methods_supported": []string{"header"},
		"scopes_supported":         []string{oauthScope},
		"resource_name":            "Zolik card tables",
	})
}

func (h *Handlers) authServerMetadata(w http.ResponseWriter, req *http.Request) {
	o := h.origin(req)
	writeJSONStatus(w, http.StatusOK, map[string]any{
		"issuer":                                o,
		"authorization_endpoint":                o + "/oauth/authorize",
		"token_endpoint":                        o + "/oauth/token",
		"registration_endpoint":                 o + "/oauth/register",
		"response_types_supported":              []string{"code"},
		"grant_types_supported":                 []string{"authorization_code", "refresh_token"},
		"code_challenge_methods_supported":      []string{"S256"},
		"token_endpoint_auth_methods_supported": []string{"none"},
		"scopes_supported":                      []string{oauthScope},
	})
}

// validRedirect accepts https, and http only to a loopback host (a CLI's
// callback). Anything else — custom schemes, fragments — is refused: a redirect
// the server cannot vouch for is where a code would leak.
func validRedirect(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Fragment != "" || u.Host == "" {
		return false
	}
	switch u.Scheme {
	case "https":
		return true
	case "http":
		h := u.Hostname()
		return h == "localhost" || h == "127.0.0.1" || h == "::1"
	}
	return false
}

func (h *Handlers) oauthRegister(w http.ResponseWriter, req *http.Request) {
	var in struct {
		RedirectURIs []string `json:"redirect_uris"`
		ClientName   string   `json:"client_name"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, req.Body, 1<<16)).Decode(&in); err != nil {
		oauthErr(w, http.StatusBadRequest, "invalid_client_metadata", "unreadable body")
		return
	}
	if len(in.RedirectURIs) == 0 || len(in.RedirectURIs) > 10 {
		oauthErr(w, http.StatusBadRequest, "invalid_redirect_uri", "one to ten redirect_uris are required")
		return
	}
	for _, u := range in.RedirectURIs {
		if !validRedirect(u) {
			oauthErr(w, http.StatusBadRequest, "invalid_redirect_uri", "redirect_uris must be https, or http on a loopback host")
			return
		}
	}
	name := clean(in.ClientName, 32)
	if name == "" {
		name = "AI agent"
	}
	id, err := signO("client", otoken{ClientName: name, RedirectURIs: in.RedirectURIs}, registrationTTL)
	if err != nil {
		oauthErr(w, http.StatusInternalServerError, "server_error", "")
		return
	}
	writeJSONStatus(w, http.StatusCreated, map[string]any{
		"client_id":                  id,
		"client_name":                name,
		"redirect_uris":              in.RedirectURIs,
		"grant_types":                []string{"authorization_code", "refresh_token"},
		"response_types":             []string{"code"},
		"token_endpoint_auth_method": "none",
		"scope":                      oauthScope,
	})
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// oauthAuthorize checks the request and hands the person to the consent page.
// A request it cannot trust the redirect of is answered here and never
// redirected to; any other fault goes back to the client as the spec says.
func (h *Handlers) oauthAuthorize(w http.ResponseWriter, req *http.Request) {
	q := req.URL.Query()
	client, err := parseO("client", q.Get("client_id"))
	if err != nil {
		http.Error(w, "unknown client", http.StatusBadRequest)
		return
	}
	redirect := q.Get("redirect_uri")
	if !contains(client.RedirectURIs, redirect) {
		http.Error(w, "redirect_uri is not registered for this client", http.StatusBadRequest)
		return
	}
	back := func(code string) {
		u, _ := url.Parse(redirect)
		v := u.Query()
		v.Set("error", code)
		if s := q.Get("state"); s != "" {
			v.Set("state", s)
		}
		u.RawQuery = v.Encode()
		http.Redirect(w, req, u.String(), http.StatusFound)
	}
	switch {
	case q.Get("response_type") != "code":
		back("unsupported_response_type")
		return
	case q.Get("code_challenge") == "" || q.Get("code_challenge_method") != "S256":
		back("invalid_request")
		return
	}
	if res := q.Get("resource"); res != "" && res != h.origin(req)+"/mcp" {
		back("invalid_target")
		return
	}
	tok, err := signO("req", otoken{
		ClientID: q.Get("client_id"), ClientName: client.ClientName,
		RedirectURI: redirect, Challenge: q.Get("code_challenge"), State: q.Get("state"),
	}, requestTTL)
	if err != nil {
		back("server_error")
		return
	}
	base := h.consentBase
	if base == "" {
		base = h.origin(req)
	}
	http.Redirect(w, req, base+"/oauth/consent?req="+url.QueryEscape(tok), http.StatusFound)
}

// oauthRequest tells the consent page what it is asking about.
func (h *Handlers) oauthRequest(w http.ResponseWriter, req *http.Request) {
	r, err := parseO("req", req.URL.Query().Get("req"))
	if err != nil {
		oauthErr(w, http.StatusBadRequest, "invalid_request", "this request has expired — start again from the app that sent you here")
		return
	}
	host := ""
	if u, err := url.Parse(r.RedirectURI); err == nil {
		host = u.Host
	}
	writeJSONStatus(w, http.StatusOK, map[string]any{"clientName": r.ClientName, "redirectHost": host})
}

// oauthApprove is the person's answer. It needs their ordinary login — that is
// the whole authentication of this flow — and returns where to send them next.
func (h *Handlers) oauthApprove(w http.ResponseWriter, req *http.Request) {
	uc, _ := auth.GetUserContext(req)
	var in struct {
		Req     string `json:"req"`
		Approve bool   `json:"approve"`
	}
	_ = json.NewDecoder(http.MaxBytesReader(w, req.Body, 1<<16)).Decode(&in)
	r, err := parseO("req", in.Req)
	if err != nil {
		oauthErr(w, http.StatusBadRequest, "invalid_request", "this request has expired")
		return
	}
	u, _ := url.Parse(r.RedirectURI)
	v := u.Query()
	if r.State != "" {
		v.Set("state", r.State)
	}
	if !in.Approve {
		v.Set("error", "access_denied")
	} else {
		code, err := signO("code", otoken{
			ClientID: r.ClientID, ClientName: r.ClientName, RedirectURI: r.RedirectURI,
			Challenge: r.Challenge, Owner: uc.UserID, Agent: agentIDFor(uc.UserID, r.ClientName),
		}, codeTTL)
		if err != nil {
			oauthErr(w, http.StatusInternalServerError, "server_error", "")
			return
		}
		v.Set("code", code)
	}
	u.RawQuery = v.Encode()
	writeJSONStatus(w, http.StatusOK, map[string]string{"redirectUrl": u.String()})
}

// agentIDFor is the agent's identity: stable for one person and one client, so
// a reconnect keeps its seats, and unrelated to the person's own id.
func agentIDFor(owner, clientName string) string {
	mac := hmac.New(sha256.New, auth.DeriveKey("oauth-agent-id"))
	mac.Write([]byte(owner + "|" + clientName))
	return "agent:" + hex.EncodeToString(mac.Sum(nil))[:16]
}

func (h *Handlers) oauthToken(w http.ResponseWriter, req *http.Request) {
	if err := req.ParseForm(); err != nil {
		oauthErr(w, http.StatusBadRequest, "invalid_request", "unreadable form")
		return
	}
	switch req.PostForm.Get("grant_type") {
	case "authorization_code":
		h.grantCode(w, req)
	case "refresh_token":
		h.grantRefresh(w, req)
	default:
		oauthErr(w, http.StatusBadRequest, "unsupported_grant_type", "")
	}
}

func (h *Handlers) grantCode(w http.ResponseWriter, req *http.Request) {
	f := req.PostForm
	c, err := parseO("code", f.Get("code"))
	if err != nil {
		oauthErr(w, http.StatusBadRequest, "invalid_grant", "the code is invalid or expired")
		return
	}
	if f.Get("redirect_uri") != c.RedirectURI || f.Get("client_id") != c.ClientID {
		oauthErr(w, http.StatusBadRequest, "invalid_grant", "client or redirect_uri does not match the request")
		return
	}
	sum := sha256.Sum256([]byte(f.Get("code_verifier")))
	if f.Get("code_verifier") == "" || base64.RawURLEncoding.EncodeToString(sum[:]) != c.Challenge {
		oauthErr(w, http.StatusBadRequest, "invalid_grant", "PKCE verification failed")
		return
	}
	if !h.used.take(c.ID, c.ExpiresAt.Time) {
		oauthErr(w, http.StatusBadRequest, "invalid_grant", "the code was already used")
		return
	}
	h.issue(w, c.Agent, c.ClientName, c.Owner, c.ClientID)
}

func (h *Handlers) grantRefresh(w http.ResponseWriter, req *http.Request) {
	c, err := parseO("refresh", req.PostForm.Get("refresh_token"))
	if err != nil {
		oauthErr(w, http.StatusBadRequest, "invalid_grant", "the refresh token is invalid or expired")
		return
	}
	h.issue(w, c.Agent, c.ClientName, c.Owner, c.ClientID)
}

// issue mints an access token that can play and a fresh refresh token.
func (h *Handlers) issue(w http.ResponseWriter, agent, name, owner, clientID string) {
	access, err := auth.CreateAgentToken(agent, name, "", accessTTL)
	if err != nil {
		oauthErr(w, http.StatusInternalServerError, "server_error", "")
		return
	}
	refresh, err := signO("refresh", otoken{Agent: agent, ClientName: name, Owner: owner, ClientID: clientID}, refreshTTL)
	if err != nil {
		oauthErr(w, http.StatusInternalServerError, "server_error", "")
		return
	}
	writeJSONStatus(w, http.StatusOK, map[string]any{
		"access_token":  access,
		"token_type":    "Bearer",
		"expires_in":    int(accessTTL.Seconds()),
		"refresh_token": refresh,
		"scope":         oauthScope,
	})
}
