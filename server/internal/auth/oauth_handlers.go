package auth

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"zolik/server/internal/identity"
	"zolik/server/internal/models"
)

// The browser sign-in flow, in the shape that keeps credentials out of URLs.
//
//	client                     server                      provider
//	  │  POST /start ─────────────▶│                            │
//	  │  (Authorization header)    │  remembers state+nonce     │
//	  │◀──── authorizationUrl ─────│                            │
//	  │  opens in-app browser ─────┼───────────────────────────▶│
//	  │                            │◀──── GET /callback ────────│
//	  │                            │  exchanges code, verifies  │
//	  │◀──── redirect: returnTo?code=<one-time> ────────────────│
//	  │  POST /exchange ──────────▶│                            │
//	  │◀──── access + refresh ─────│                            │
//
// Two details are deliberate. Starting with a POST means the guest token or
// the signed-in token that scopes the flow travels in a header rather than a
// query string — so it never reaches browser history, a `Referer`, or the
// access logs of whatever sits in front of this server. And the callback hands
// back a one-time exchange code instead of the tokens themselves, for exactly
// the same reason: a redirect URL is the least private place in the system.

const oauthFlowTTL = 10 * time.Minute

type oauthStartReq struct {
	// ReturnTo is where the browser is sent when the flow finishes — the app's
	// deep link, or a web route. Must be on the allow-list.
	ReturnTo string `json:"returnTo"`
	// Link asks to attach this provider to the signed-in account rather than
	// to sign in with it. Requires a non-guest Authorization header.
	Link bool `json:"link,omitempty"`
}

// oauthStart begins a redirect sign-in and returns the provider URL to open.
func (h *Handlers) oauthStart(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	providerID := chi.URLParam(req, "provider")
	provider, err := h.providers.Get(providerID)
	if err != nil {
		http.Error(w, "unknown sign-in provider", http.StatusNotFound)
		return
	}

	var body oauthStartReq
	_ = json.NewDecoder(req.Body).Decode(&body)

	returnTo, err := h.resolveReturnTo(body.ReturnTo)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	flow := models.OAuthFlow{
		Provider:    providerID,
		RedirectURI: h.redirectURI(providerID),
		ReturnTo:    returnTo,
		CreatedAt:   time.Now().UTC(),
		ExpiresAt:   time.Now().UTC().Add(oauthFlowTTL),
	}

	// The caller's own token — if they sent one — is what decides whether this
	// is a link, a guest upgrade, or a plain sign-in. Taking it from a header
	// rather than from the request body is the point: a client cannot ask to
	// claim a guest id it does not hold a token for.
	if uc, ok := GetUserContext(req); ok {
		if uc.IsGuest {
			flow.GuestID = uc.UserID
		} else {
			flow.LinkUserID = uc.UserID
		}
	}
	if body.Link && flow.LinkUserID == "" {
		http.Error(w, "sign in before linking another account", http.StatusUnauthorized)
		return
	}

	if flow.State, err = NewRandomToken(32); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if flow.Nonce, err = NewRandomToken(16); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	authURL, err := provider.AuthCodeURL(flow.State, flow.Nonce, flow.RedirectURI)
	if err != nil {
		log.Printf("auth: building %s authorization url failed: %v", providerID, err)
		http.Error(w, "sign-in provider is unavailable", http.StatusBadGateway)
		return
	}
	if err := h.store.InsertFlow(ctx, flow); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]any{
		"authorizationUrl": authURL,
		"returnTo":         returnTo,
	})
}

// oauthCallback is where the provider sends the browser back.
//
// It is registered for GET and POST: most providers redirect with a query
// string, Apple posts a form when name or email scopes are requested. Both
// carry the same two values, so both land here.
//
// Whatever happens, the person's browser is redirected back to the app — a
// failure returns `?error=...` rather than rendering a server error page,
// because at this point the browser is a tab floating over the game and the
// only useful thing it can do is hand control back.
func (h *Handlers) oauthCallback(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	if err := req.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	state := req.Form.Get("state")
	if state == "" {
		http.Error(w, "missing state", http.StatusBadRequest)
		return
	}

	flow, err := h.store.FindFlowByState(ctx, state)
	if err != nil {
		// An unknown state is the CSRF check firing: this callback does not
		// correspond to any sign-in this server started. There is no trusted
		// return URL to bounce to, so it ends here.
		http.Error(w, "this sign-in link has expired — please try again", http.StatusBadRequest)
		return
	}
	if time.Now().UTC().After(flow.ExpiresAt) {
		h.failFlow(w, req, flow, "expired")
		return
	}
	if provErr := req.Form.Get("error"); provErr != "" {
		// The person pressed cancel, or the provider refused. Neither is a
		// server fault and neither should look like one.
		h.failFlow(w, req, flow, provErr)
		return
	}
	code := req.Form.Get("code")
	if code == "" {
		h.failFlow(w, req, flow, "no_code")
		return
	}

	provider, err := h.providers.Get(flow.Provider)
	if err != nil {
		h.failFlow(w, req, flow, "unknown_provider")
		return
	}
	claims, err := provider.ExchangeCode(ctx, code, flow.RedirectURI, flow.Nonce)
	if err != nil {
		log.Printf("auth: %s code exchange failed: %v", flow.Provider, err)
		h.failFlow(w, req, flow, "exchange_failed")
		return
	}

	// Apple delivers the person's name once, in this very form post, and never
	// again — so if the claims carry none, take it from here before it is gone
	// for good.
	if claims.Name == "" {
		claims.Name = appleFormName(req.Form.Get("user"))
	}

	result, err := h.accounts.SignIn(ctx, claims, SignInOptions{
		GuestID:      flow.GuestID,
		LinkToUserID: flow.LinkUserID,
	})
	if errors.Is(err, ErrIdentityTaken) {
		h.failFlow(w, req, flow, "already_linked")
		return
	}
	if err != nil {
		log.Printf("auth: %s sign-in failed: %v", flow.Provider, err)
		h.failFlow(w, req, flow, "sign_in_failed")
		return
	}

	out := models.OAuthFlowResult{
		UserID:         result.User.ID.Hex(),
		Username:       result.User.Username,
		Linked:         result.Linked && flow.LinkUserID != "",
		ClaimedMatches: result.ClaimedMatches,
	}
	// A pure link flow deliberately mints no session: the person is already
	// signed in on the device that started it, and handing back a second set
	// of tokens would quietly replace their session with an identical one.
	if flow.LinkUserID == "" {
		tokens, err := h.issueUserSession(ctx, result.User)
		if err != nil {
			log.Printf("auth: issuing session failed: %v", err)
			h.failFlow(w, req, flow, "sign_in_failed")
			return
		}
		out.AccessToken, out.RefreshToken = tokens.AccessToken, tokens.RefreshToken
	}

	exchangeCode, err := NewRandomToken(32)
	if err != nil {
		h.failFlow(w, req, flow, "sign_in_failed")
		return
	}
	if err := h.store.CompleteFlow(ctx, flow.ID, exchangeCode, out); err != nil {
		// Already completed: a duplicate callback delivery. The first one won.
		h.failFlow(w, req, flow, "already_completed")
		return
	}

	redirectBack(w, req, flow.ReturnTo, url.Values{"code": {exchangeCode}})
}

type oauthExchangeReq struct {
	Code string `json:"code"`
}

// oauthExchange swaps the one-time code from the callback for tokens.
func (h *Handlers) oauthExchange(w http.ResponseWriter, req *http.Request) {
	var body oauthExchangeReq
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil || body.Code == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	// The read destroys the flow, so this code works exactly once.
	flow, err := h.store.TakeFlowResult(req.Context(), body.Code)
	if err != nil || flow.Result == nil {
		http.Error(w, "this sign-in has expired — please try again", http.StatusUnauthorized)
		return
	}
	r := flow.Result
	writeJSON(w, map[string]any{
		"accessToken":    r.AccessToken,
		"refreshToken":   r.RefreshToken,
		"userId":         r.UserID,
		"username":       r.Username,
		"isGuest":        false,
		"linked":         r.Linked,
		"provider":       flow.Provider,
		"claimedMatches": r.ClaimedMatches,
	})
}

type oauthTokenReq struct {
	// IDToken comes from a native sign-in SDK.
	IDToken string `json:"idToken"`
	// Nonce is the value the app asked the SDK to embed, when it used one.
	Nonce string `json:"nonce,omitempty"`
	// Link attaches the identity to the signed-in account instead of signing
	// in with it.
	Link bool `json:"link,omitempty"`
}

// oauthNativeToken accepts an ID token minted by a platform SDK.
//
// This is the second door onto the same room: Google Sign-In on Android, Sign
// in with Apple on iOS and MSAL all hand the app a verified ID token directly,
// with no browser and no code to exchange. Verifying it yields the same
// identity.Claims the redirect flow produces, so everything after this line is
// shared — one account model, one linking rule, one guest claim.
func (h *Handlers) oauthNativeToken(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	providerID := chi.URLParam(req, "provider")
	provider, err := h.providers.Get(providerID)
	if err != nil {
		http.Error(w, "unknown sign-in provider", http.StatusNotFound)
		return
	}

	var body oauthTokenReq
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil || body.IDToken == "" {
		http.Error(w, "idToken required", http.StatusBadRequest)
		return
	}

	claims, err := provider.VerifyIDToken(ctx, body.IDToken, body.Nonce)
	if err != nil {
		log.Printf("auth: %s id token rejected: %v", providerID, err)
		http.Error(w, "that sign-in could not be verified", http.StatusUnauthorized)
		return
	}

	opts := SignInOptions{}
	if uc, ok := GetUserContext(req); ok {
		if uc.IsGuest {
			opts.GuestID = uc.UserID
		} else {
			opts.LinkToUserID = uc.UserID
		}
	}
	if body.Link && opts.LinkToUserID == "" {
		http.Error(w, "sign in before linking another account", http.StatusUnauthorized)
		return
	}

	h.completeSignIn(w, req, claims, opts, providerID)
}

// completeSignIn is the shared tail of every sign-in: resolve the account,
// mint a session unless this was a link, and answer.
func (h *Handlers) completeSignIn(w http.ResponseWriter, req *http.Request, claims identity.Claims, opts SignInOptions, providerID string) {
	ctx := req.Context()
	result, err := h.accounts.SignIn(ctx, claims, opts)
	if errors.Is(err, ErrIdentityTaken) {
		http.Error(w, "that account is already linked to another player", http.StatusConflict)
		return
	}
	if err != nil {
		log.Printf("auth: sign-in failed: %v", err)
		http.Error(w, "sign-in failed", http.StatusInternalServerError)
		return
	}

	out := map[string]any{
		"userId":         result.User.ID.Hex(),
		"username":       result.User.Username,
		"isGuest":        false,
		"created":        result.Created,
		"provider":       providerID,
		"claimedMatches": result.ClaimedMatches,
	}
	if opts.LinkToUserID != "" {
		out["linked"] = true
		writeJSON(w, out)
		return
	}

	tokens, err := h.issueUserSession(ctx, result.User)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	out["accessToken"] = tokens.AccessToken
	out["refreshToken"] = tokens.RefreshToken
	writeJSON(w, out)
}

// failFlow abandons a flow and returns the browser to the app with a reason.
func (h *Handlers) failFlow(w http.ResponseWriter, req *http.Request, flow models.OAuthFlow, reason string) {
	redirectBack(w, req, flow.ReturnTo, url.Values{"error": {reason}})
}

func redirectBack(w http.ResponseWriter, req *http.Request, returnTo string, params url.Values) {
	if returnTo == "" {
		// Nothing to return to — happens only for a flow started without a
		// return URL, which the API does not allow but a stored document from
		// an older build might.
		writeJSON(w, map[string]any{"result": params.Encode()})
		return
	}
	sep := "?"
	if strings.Contains(returnTo, "?") {
		sep = "&"
	}
	http.Redirect(w, req, returnTo+sep+params.Encode(), http.StatusFound)
}

// resolveReturnTo validates where the browser may be sent afterwards.
//
// This is an open-redirect guard, and it is not optional: the callback appends
// a code that can be swapped for a session, so an unchecked return URL would
// let anyone hand that session to a site they control. Only prefixes the
// deployment has declared are accepted.
func (h *Handlers) resolveReturnTo(candidate string) (string, error) {
	candidate = strings.TrimSpace(candidate)
	if candidate == "" {
		if len(h.allowedReturnURLs) == 0 {
			return "", errors.New("returnTo is required")
		}
		return h.allowedReturnURLs[0], nil
	}
	for _, allowed := range h.allowedReturnURLs {
		if strings.HasPrefix(candidate, allowed) {
			return candidate, nil
		}
	}
	return "", errors.New("returnTo is not an allowed address for this deployment")
}

func (h *Handlers) redirectURI(providerID string) string {
	return strings.TrimSuffix(h.publicBaseURL, "/") + "/auth/oauth/" + providerID + "/callback"
}

// appleFormName digs the display name out of Apple's `user` form field, which
// is a JSON blob delivered only on the very first authorization.
func appleFormName(raw string) string {
	if raw == "" {
		return ""
	}
	var payload struct {
		Name struct {
			FirstName string `json:"firstName"`
			LastName  string `json:"lastName"`
		} `json:"name"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return ""
	}
	return strings.TrimSpace(payload.Name.FirstName + " " + payload.Name.LastName)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
