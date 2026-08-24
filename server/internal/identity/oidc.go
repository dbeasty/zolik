package identity

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// OIDCConfig describes one OpenID Connect provider.
//
// Google, Apple and Microsoft are all plain OIDC providers, so all three are
// *this* type with different constants — no per-provider implementation. What
// little genuinely differs between them is expressed as data here (an extra
// authorization parameter, a second acceptable audience, a client secret that
// has to be minted rather than stored), which is what keeps "add Microsoft"
// a config change instead of a code change.
type OIDCConfig struct {
	// ID is the registry key: "google", "apple", "microsoft".
	ID string
	// DisplayName is the button label clients render.
	DisplayName string

	// Issuer is the value the ID token's `iss` claim must carry, and the base
	// for discovery when DiscoveryURL is empty.
	Issuer string
	// DiscoveryURL overrides the derived `{Issuer}/.well-known/openid-configuration`.
	DiscoveryURL string
	// IssuerMatches optionally replaces the exact issuer comparison. Microsoft
	// needs it: with the `common` tenant the token's issuer names the *user's*
	// tenant GUID, so no single literal can be compared against.
	IssuerMatches func(iss string) bool

	ClientID     string
	ClientSecret string
	// ClientSecretFunc supersedes ClientSecret when set, and is called per
	// request. Apple requires it: its "client secret" is a short-lived ES256
	// JWT the server signs itself, not a fixed string (see AppleSecretSigner).
	ClientSecretFunc func() (string, error)

	// Scopes requested in the redirect flow. Defaults to openid+email+profile.
	Scopes []string
	// AuthParams are extra authorization-URL parameters. Apple needs
	// `response_mode=form_post` whenever name/email scopes are requested;
	// Google needs `access_type`/`prompt` to control its consent screen.
	AuthParams map[string]string

	// ExtraAudiences are additional acceptable `aud` values for native ID
	// tokens. Google issues a *different* client id per platform, so an ID
	// token minted by the iOS SDK carries the iOS client id, not the server's.
	// Every value listed here must be a client id this project owns.
	ExtraAudiences []string

	// TrustEmailWithoutVerifiedClaim accepts the provider's email as verified
	// when it emits no `email_verified` claim at all. Off by default and it
	// should stay off unless the provider documents that its addresses are
	// always verified: an unverified address that is treated as verified lets
	// someone claim an existing account by signing up elsewhere with its
	// address. Microsoft is the reason this knob exists — it omits the claim
	// for personal accounts.
	TrustEmailWithoutVerifiedClaim bool

	// HTTPClient overrides the client used for discovery, JWKS and token
	// calls. Tests inject one pointed at an httptest server.
	HTTPClient *http.Client
	// JWKSCacheTTL overrides the one-hour default signing-key cache.
	JWKSCacheTTL time.Duration
}

// oidcProvider is the single Provider implementation behind every OIDC login.
type oidcProvider struct {
	cfg    OIDCConfig
	client *http.Client

	mu        sync.Mutex
	meta      *providerMetadata
	metaAt    time.Time
	keys      *keySet
	metaError error
}

// providerMetadata is the part of the OIDC discovery document we use.
type providerMetadata struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	JWKSURI               string `json:"jwks_uri"`
}

// NewOIDCProvider builds a provider from config. It returns nil when the
// config carries no client id, so a deployment that has not set up (say)
// Microsoft simply has no Microsoft provider rather than a broken one — see
// NewRegistry, which drops nils.
func NewOIDCProvider(cfg OIDCConfig) Provider {
	if strings.TrimSpace(cfg.ClientID) == "" || strings.TrimSpace(cfg.ID) == "" {
		return nil
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	if len(cfg.Scopes) == 0 {
		cfg.Scopes = []string{"openid", "email", "profile"}
	}
	if cfg.DisplayName == "" {
		cfg.DisplayName = strings.ToUpper(cfg.ID[:1]) + cfg.ID[1:]
	}
	return &oidcProvider{cfg: cfg, client: client}
}

func (p *oidcProvider) ID() string          { return p.cfg.ID }
func (p *oidcProvider) DisplayName() string { return p.cfg.DisplayName }

// metadata fetches and caches the discovery document. Discovery is cached for
// a day: these endpoints change on the order of never, and a provider's
// discovery URL being briefly down should not break sign-in.
func (p *oidcProvider) metadata(ctx context.Context) (*providerMetadata, error) {
	p.mu.Lock()
	if p.meta != nil && time.Since(p.metaAt) < 24*time.Hour {
		m := p.meta
		p.mu.Unlock()
		return m, nil
	}
	p.mu.Unlock()

	discoveryURL := p.cfg.DiscoveryURL
	if discoveryURL == "" {
		discoveryURL = strings.TrimSuffix(p.cfg.Issuer, "/") + "/.well-known/openid-configuration"
	}
	var m providerMetadata
	if err := getJSON(ctx, p.client, discoveryURL, &m); err != nil {
		p.mu.Lock()
		cached := p.meta
		p.mu.Unlock()
		if cached != nil {
			return cached, nil
		}
		return nil, fmt.Errorf("%s: discovery failed: %w", p.cfg.ID, err)
	}
	if m.JWKSURI == "" || m.TokenEndpoint == "" || m.AuthorizationEndpoint == "" {
		return nil, fmt.Errorf("%s: discovery document is missing endpoints", p.cfg.ID)
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	p.meta, p.metaAt = &m, time.Now()
	if p.keys == nil || p.keys.url != m.JWKSURI {
		p.keys = newKeySet(m.JWKSURI, p.client, p.cfg.JWKSCacheTTL)
	}
	return &m, nil
}

func (p *oidcProvider) keySet(ctx context.Context) (*keySet, error) {
	if _, err := p.metadata(ctx); err != nil {
		return nil, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.keys, nil
}

// AuthCodeURL builds the provider's authorization URL.
func (p *oidcProvider) AuthCodeURL(state, nonce, redirectURI string) (string, error) {
	m, err := p.metadata(context.Background())
	if err != nil {
		return "", err
	}
	q := url.Values{}
	q.Set("client_id", p.cfg.ClientID)
	q.Set("response_type", "code")
	q.Set("scope", strings.Join(p.cfg.Scopes, " "))
	q.Set("redirect_uri", redirectURI)
	q.Set("state", state)
	q.Set("nonce", nonce)
	for k, v := range p.cfg.AuthParams {
		q.Set(k, v)
	}
	sep := "?"
	if strings.Contains(m.AuthorizationEndpoint, "?") {
		sep = "&"
	}
	return m.AuthorizationEndpoint + sep + q.Encode(), nil
}

func (p *oidcProvider) clientSecret() (string, error) {
	if p.cfg.ClientSecretFunc != nil {
		return p.cfg.ClientSecretFunc()
	}
	return p.cfg.ClientSecret, nil
}

// ExchangeCode redeems an authorization code at the token endpoint and
// verifies the ID token that comes back.
func (p *oidcProvider) ExchangeCode(ctx context.Context, code, redirectURI, nonce string) (Claims, error) {
	m, err := p.metadata(ctx)
	if err != nil {
		return Claims{}, err
	}
	secret, err := p.clientSecret()
	if err != nil {
		return Claims{}, fmt.Errorf("%s: client secret: %w", p.cfg.ID, err)
	}

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	form.Set("client_id", p.cfg.ClientID)
	if secret != "" {
		form.Set("client_secret", secret)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return Claims{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return Claims{}, err
	}
	defer resp.Body.Close()

	var body struct {
		IDToken          string `json:"id_token"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if err := decodeJSONBody(resp, &body); err != nil {
		return Claims{}, err
	}
	if body.Error != "" {
		// The provider's own words are the most useful thing we can log here,
		// and they never contain the code or the secret.
		return Claims{}, fmt.Errorf("%s: token exchange rejected: %s %s", p.cfg.ID, body.Error, body.ErrorDescription)
	}
	if body.IDToken == "" {
		return Claims{}, fmt.Errorf("%s: token response carried no id_token", p.cfg.ID)
	}
	return p.VerifyIDToken(ctx, body.IDToken, nonce)
}

// VerifyIDToken checks an ID token's signature, issuer, audience, expiry and
// nonce, and maps it to Claims.
//
// This is the security boundary of the whole OAuth path: everything after it
// treats the subject as proven. Signature verification is done against the
// provider's published keys, and the algorithm allow-list is explicit so a
// token claiming `alg: none` (or an HMAC over the public key) cannot be talked
// into verifying.
func (p *oidcProvider) VerifyIDToken(ctx context.Context, rawIDToken, nonce string) (Claims, error) {
	rawIDToken = strings.TrimSpace(rawIDToken)
	if rawIDToken == "" {
		return Claims{}, errors.New("empty id token")
	}
	ks, err := p.keySet(ctx)
	if err != nil {
		return Claims{}, err
	}

	claims := jwt.MapClaims{}
	_, err = jwt.ParseWithClaims(rawIDToken, claims, func(t *jwt.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		return ks.key(ctx, kid)
	},
		jwt.WithValidMethods([]string{"RS256", "RS384", "RS512", "ES256", "ES384", "ES512"}),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return Claims{}, fmt.Errorf("%s: id token rejected: %w", p.cfg.ID, err)
	}

	iss, _ := claims["iss"].(string)
	if !p.issuerOK(iss) {
		return Claims{}, fmt.Errorf("%s: unexpected issuer %q", p.cfg.ID, iss)
	}
	if !p.audienceOK(claims["aud"]) {
		return Claims{}, fmt.Errorf("%s: id token was not issued for this application", p.cfg.ID)
	}
	// The nonce ties the token to the flow we started. It is only checked when
	// we started one: a native SDK flow has no server-side nonce to compare.
	if nonce != "" {
		got, _ := claims["nonce"].(string)
		if subtle.ConstantTimeCompare([]byte(got), []byte(nonce)) != 1 {
			return Claims{}, fmt.Errorf("%s: id token nonce mismatch", p.cfg.ID)
		}
	}

	sub, _ := claims["sub"].(string)
	if sub == "" {
		return Claims{}, fmt.Errorf("%s: id token carried no subject", p.cfg.ID)
	}
	return Claims{
		Provider:      p.cfg.ID,
		Subject:       sub,
		Email:         p.emailFrom(claims),
		EmailVerified: p.emailVerified(claims),
		Name:          nameFrom(claims),
		Picture:       stringClaim(claims, "picture"),
	}, nil
}

func (p *oidcProvider) issuerOK(iss string) bool {
	if p.cfg.IssuerMatches != nil {
		return p.cfg.IssuerMatches(iss)
	}
	// Google is issued under both spellings and documents both as valid.
	want := strings.TrimSuffix(p.cfg.Issuer, "/")
	return strings.TrimSuffix(iss, "/") == want
}

// audienceOK accepts the server's own client id or any sibling platform client
// id the deployment has declared. `aud` is a string or an array of strings per
// the JWT spec, so both shapes are handled.
func (p *oidcProvider) audienceOK(aud any) bool {
	accept := func(v string) bool {
		if v == p.cfg.ClientID {
			return true
		}
		for _, extra := range p.cfg.ExtraAudiences {
			if extra != "" && v == extra {
				return true
			}
		}
		return false
	}
	switch v := aud.(type) {
	case string:
		return accept(v)
	case []any:
		for _, item := range v {
			s, ok := item.(string)
			if ok && accept(s) {
				return true
			}
		}
	case []string:
		for _, s := range v {
			if accept(s) {
				return true
			}
		}
	}
	return false
}

func (p *oidcProvider) emailFrom(claims jwt.MapClaims) string {
	if e := stringClaim(claims, "email"); e != "" {
		return e
	}
	// Microsoft personal accounts put the address here and omit `email` unless
	// the optional claim is configured on the app registration.
	upn := stringClaim(claims, "preferred_username")
	if strings.Contains(upn, "@") {
		return upn
	}
	return ""
}

// emailVerified reads the provider's verification flag, which is a bool in the
// spec but a string in practice for Apple and some Microsoft configurations.
func (p *oidcProvider) emailVerified(claims jwt.MapClaims) bool {
	v, present := claims["email_verified"]
	if !present {
		// Microsoft's `xms_edov` ("email domain owner verified") is the
		// documented substitute where it is configured.
		v, present = claims["xms_edov"]
	}
	if !present {
		return p.cfg.TrustEmailWithoutVerifiedClaim
	}
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return strings.EqualFold(t, "true")
	default:
		return false
	}
}

func nameFrom(claims jwt.MapClaims) string {
	if n := stringClaim(claims, "name"); n != "" {
		return n
	}
	given, family := stringClaim(claims, "given_name"), stringClaim(claims, "family_name")
	return strings.TrimSpace(strings.TrimSpace(given) + " " + strings.TrimSpace(family))
}

func stringClaim(claims jwt.MapClaims, key string) string {
	s, _ := claims[key].(string)
	return strings.TrimSpace(s)
}

func decodeJSONBody(resp *http.Response, out any) error {
	// Token endpoints answer 400 with a JSON error body that is far more
	// useful than the status alone, so the body is decoded either way and the
	// status is only consulted if it turns out to be unparseable.
	if err := getJSONFromResponse(resp, out); err != nil {
		return fmt.Errorf("unexpected status %d: %w", resp.StatusCode, err)
	}
	return nil
}
