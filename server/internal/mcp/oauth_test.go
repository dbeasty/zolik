package mcp_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"zolik/server/internal/auth"
)

func noRedirect() *http.Client {
	return &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

func getJSON(t *testing.T, u string) map[string]any {
	t.Helper()
	resp, err := http.Get(u)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return out
}

// The whole flow a connector runs: discover, register, authorize, a person
// approves, exchange the code, call /mcp, refresh.
func TestOAuthFlow(t *testing.T) {
	r := newRig(t)
	base := r.srv.URL

	// A bare /mcp URL tells the client where to sign in.
	resp, _ := http.Post(base+"/mcp", "application/json", strings.NewReader("{}"))
	if resp.StatusCode != 401 || !strings.Contains(resp.Header.Get("WWW-Authenticate"), "/.well-known/oauth-protected-resource") {
		t.Fatalf("401 challenge = %d %q", resp.StatusCode, resp.Header.Get("WWW-Authenticate"))
	}
	prm := getJSON(t, base+"/.well-known/oauth-protected-resource")
	if prm["resource"] != base+"/mcp" {
		t.Fatalf("resource metadata = %v", prm)
	}
	asm := getJSON(t, base+"/.well-known/oauth-authorization-server")
	for _, k := range []string{"authorization_endpoint", "token_endpoint", "registration_endpoint"} {
		if asm[k] == nil {
			t.Fatalf("authorization server metadata lacks %s: %v", k, asm)
		}
	}

	post := func(path, ct string, body []byte, bearer string) (*http.Response, map[string]any) {
		req, _ := http.NewRequest("POST", base+path, bytes.NewReader(body))
		req.Header.Set("Content-Type", ct)
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
		resp, err := noRedirect().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var out map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&out)
		return resp, out
	}

	// Registration refuses a redirect it cannot vouch for.
	bad, _ := json.Marshal(map[string]any{"redirect_uris": []string{"http://evil.example/cb"}})
	if resp, _ := post("/oauth/register", "application/json", bad, ""); resp.StatusCode != 400 {
		t.Fatalf("plain-http non-loopback redirect registered: %d", resp.StatusCode)
	}
	redirect := "https://claude.ai/api/mcp/auth_callback"
	reg, _ := json.Marshal(map[string]any{"redirect_uris": []string{redirect}, "client_name": "Claude"})
	resp, client := post("/oauth/register", "application/json", reg, "")
	if resp.StatusCode != 201 {
		t.Fatalf("register = %d %v", resp.StatusCode, client)
	}
	clientID := client["client_id"].(string)

	verifier := "a-very-long-random-pkce-verifier-0123456789abcdefghijk"
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	authz := func(extra url.Values) *http.Response {
		q := url.Values{"response_type": {"code"}, "client_id": {clientID}, "redirect_uri": {redirect},
			"code_challenge": {challenge}, "code_challenge_method": {"S256"}, "state": {"xyz"}}
		for k, v := range extra {
			q[k] = v
		}
		resp, err := noRedirect().Get(base + "/oauth/authorize?" + q.Encode())
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp
	}

	// A redirect that was not registered is never redirected to.
	if resp := authz(url.Values{"redirect_uri": {"https://evil.example/cb"}}); resp.StatusCode != 400 {
		t.Fatalf("unregistered redirect_uri = %d, want 400", resp.StatusCode)
	}
	// PKCE is mandatory, and goes back to the client as an error.
	if resp := authz(url.Values{"code_challenge_method": {"plain"}}); resp.StatusCode != 302 ||
		!strings.Contains(resp.Header.Get("Location"), "error=invalid_request") {
		t.Fatalf("plain PKCE: %d %q", resp.StatusCode, resp.Header.Get("Location"))
	}

	resp = authz(nil)
	loc, _ := url.Parse(resp.Header.Get("Location"))
	if resp.StatusCode != 302 || loc.Path != "/oauth/consent" {
		t.Fatalf("authorize = %d %v", resp.StatusCode, loc)
	}
	reqTok := loc.Query().Get("req")
	if info := getJSON(t, base+"/oauth/request?req="+url.QueryEscape(reqTok)); info["clientName"] != "Claude" {
		t.Fatalf("request info = %v", info)
	}

	// Approving needs a signed-in person.
	approve, _ := json.Marshal(map[string]any{"req": reqTok, "approve": true})
	if resp, _ := post("/oauth/approve", "application/json", approve, ""); resp.StatusCode != 401 {
		t.Fatalf("approve without login = %d", resp.StatusCode)
	}
	// And a person can say no.
	deny, _ := json.Marshal(map[string]any{"req": reqTok, "approve": false})
	_, d := post("/oauth/approve", "application/json", deny, token(t, "alice"))
	if !strings.Contains(d["redirectUrl"].(string), "error=access_denied") {
		t.Fatalf("deny = %v", d)
	}

	_, ok := post("/oauth/approve", "application/json", approve, token(t, "alice"))
	back, _ := url.Parse(ok["redirectUrl"].(string))
	if back.Query().Get("state") != "xyz" || back.Host != "claude.ai" {
		t.Fatalf("approval redirect = %v", back)
	}
	code := back.Query().Get("code")

	exchange := func(v url.Values) (*http.Response, map[string]any) {
		return post("/oauth/token", "application/x-www-form-urlencoded", []byte(v.Encode()), "")
	}
	form := func(c, ver string) url.Values {
		return url.Values{"grant_type": {"authorization_code"}, "code": {c}, "redirect_uri": {redirect},
			"client_id": {clientID}, "code_verifier": {ver}}
	}
	if resp, _ := exchange(form(code, "the-wrong-verifier-the-wrong-verifier-the-wrong")); resp.StatusCode != 400 {
		t.Fatalf("wrong PKCE verifier accepted: %d", resp.StatusCode)
	}
	resp, tokens := exchange(form(code, verifier))
	if resp.StatusCode != 200 || tokens["access_token"] == nil || tokens["refresh_token"] == nil {
		t.Fatalf("token = %d %v", resp.StatusCode, tokens)
	}
	if resp, _ := exchange(form(code, verifier)); resp.StatusCode != 400 {
		t.Fatalf("authorization code redeemed twice: %d", resp.StatusCode)
	}

	// The access token plays — and can do nothing else.
	access := tokens["access_token"].(string)
	if _, err := auth.ParseAccessClaims(access); err == nil {
		t.Fatal("an OAuth access token was accepted as an ordinary one")
	}
	rr := &rig{t: t, srv: r.srv}
	out := rr.rpcWith(access, "tools/call", map[string]any{"name": "register_agent", "arguments": map[string]any{}})
	if out["error"] != nil || out["result"] == nil {
		t.Fatalf("register with an OAuth token: %v", out)
	}
	if a, found := r.reg.Get(agentOf(t, access)); !found || a.Name != "Claude" {
		t.Fatalf("agent record = %+v, %v", a, found)
	}

	// A code or a refresh token is not an access token, and vice versa.
	if resp := rr.status(code); resp != 401 {
		t.Fatalf("an authorization code opened /mcp: %d", resp)
	}
	if resp := rr.status(tokens["refresh_token"].(string)); resp != 401 {
		t.Fatalf("a refresh token opened /mcp: %d", resp)
	}

	resp, again := exchange(url.Values{"grant_type": {"refresh_token"}, "refresh_token": {tokens["refresh_token"].(string)}})
	if resp.StatusCode != 200 || again["access_token"] == nil {
		t.Fatalf("refresh = %d %v", resp.StatusCode, again)
	}
	// Same person, same client: same agent, so its seats survive a reconnect.
	if agentOf(t, again["access_token"].(string)) != agentOf(t, access) {
		t.Fatal("refresh changed the agent identity")
	}
	if resp, _ := exchange(url.Values{"grant_type": {"refresh_token"}, "refresh_token": {access}}); resp.StatusCode != 400 {
		t.Fatalf("an access token refreshed: %d", resp.StatusCode)
	}
}

func agentOf(t *testing.T, tok string) string {
	t.Helper()
	c, err := auth.ParseAccessClaimsForAgent(tok)
	if err != nil {
		t.Fatal(err)
	}
	return c.Subject
}

func (r *rig) rpcWith(bearer, method string, params any) map[string]any {
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	req, _ := http.NewRequest("POST", r.srv.URL+"/mcp", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+bearer)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		r.t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return out
}

func (r *rig) status(bearer string) int {
	req, _ := http.NewRequest("POST", r.srv.URL+"/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
	req.Header.Set("Authorization", "Bearer "+bearer)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		r.t.Fatal(err)
	}
	resp.Body.Close()
	return resp.StatusCode
}
