package identity

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// fakeProvider is a stand-in OIDC provider: a discovery document, a JWKS, and
// a token endpoint, all served from an httptest server so the whole
// verification path — discovery, key fetch, signature check, claim checks —
// runs for real against a key we control.
type fakeProvider struct {
	*httptest.Server
	key    *rsa.PrivateKey
	kid    string
	issuer string

	// idToken is what the token endpoint hands back on exchange.
	idToken string
	// tokenErr, when set, makes the token endpoint answer with an OAuth error.
	tokenErr string
}

func newFakeProvider(t *testing.T) *fakeProvider {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating a signing key: %v", err)
	}
	f := &fakeProvider{key: key, kid: "test-key-1"}

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                 f.issuer,
			"authorization_endpoint": f.issuer + "/authorize",
			"token_endpoint":         f.issuer + "/token",
			"jwks_uri":               f.issuer + "/jwks",
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) { f.writeJWKS(w) })
	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		if f.tokenErr != "" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error":             f.tokenErr,
				"error_description": "the fake provider refused",
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id_token": f.idToken})
	})

	f.Server = httptest.NewServer(mux)
	f.issuer = f.Server.URL
	t.Cleanup(f.Close)
	return f
}

// writeJWKS publishes the signing key in JWK form.
func (f *fakeProvider) writeJWKS(w http.ResponseWriter) {
	pub := f.key.Public().(*rsa.PublicKey)
	_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
		"kty": "RSA",
		"kid": f.kid,
		"use": "sig",
		"alg": "RS256",
		"n":   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
	}}})
}

// sign mints an ID token with the given claims, filling in the usual ones.
func (f *fakeProvider) sign(t *testing.T, claims jwt.MapClaims) string {
	t.Helper()
	if _, ok := claims["iss"]; !ok {
		claims["iss"] = f.issuer
	}
	if _, ok := claims["exp"]; !ok {
		claims["exp"] = time.Now().Add(time.Hour).Unix()
	}
	if _, ok := claims["iat"]; !ok {
		claims["iat"] = time.Now().Unix()
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = f.kid
	signed, err := tok.SignedString(f.key)
	if err != nil {
		t.Fatalf("signing the test id token: %v", err)
	}
	return signed
}

func (f *fakeProvider) provider(t *testing.T, mutate func(*OIDCConfig)) Provider {
	t.Helper()
	cfg := OIDCConfig{
		ID:          "fake",
		DisplayName: "Fake",
		Issuer:      f.issuer,
		ClientID:    "client-web",
		HTTPClient:  f.Client(),
	}
	if mutate != nil {
		mutate(&cfg)
	}
	p := NewOIDCProvider(cfg)
	if p == nil {
		t.Fatal("provider was not configured")
	}
	return p
}

func TestVerifyIDTokenAcceptsAWellFormedToken(t *testing.T) {
	f := newFakeProvider(t)
	p := f.provider(t, nil)

	raw := f.sign(t, jwt.MapClaims{
		"aud":            "client-web",
		"sub":            "google-subject-1",
		"email":          "player@example.com",
		"email_verified": true,
		"name":           "A Player",
		"nonce":          "n-1",
	})

	claims, err := p.VerifyIDToken(context.Background(), raw, "n-1")
	if err != nil {
		t.Fatalf("verifying a valid token: %v", err)
	}
	if claims.Subject != "google-subject-1" {
		t.Errorf("subject = %q, want google-subject-1", claims.Subject)
	}
	if claims.Email != "player@example.com" || !claims.EmailVerified {
		t.Errorf("email = %q verified = %v, want player@example.com/true", claims.Email, claims.EmailVerified)
	}
	if claims.Name != "A Player" {
		t.Errorf("name = %q, want A Player", claims.Name)
	}
	if claims.Provider != "fake" {
		t.Errorf("provider = %q, want fake", claims.Provider)
	}
}

// Each of these is a token that must not be accepted. They are the whole
// reason this verification exists — a bug in any one of them is somebody
// signing in as somebody else.
func TestVerifyIDTokenRejectsBadTokens(t *testing.T) {
	f := newFakeProvider(t)
	p := f.provider(t, nil)

	cases := []struct {
		name   string
		token  func() string
		nonce  string
		reason string
	}{
		{
			name: "audience belongs to another application",
			token: func() string {
				return f.sign(t, jwt.MapClaims{"aud": "somebody-elses-client", "sub": "s"})
			},
			reason: "a token minted for a different client would let any app's tokens sign in here",
		},
		{
			name: "issuer is not this provider",
			token: func() string {
				return f.sign(t, jwt.MapClaims{"aud": "client-web", "sub": "s", "iss": "https://evil.example"})
			},
			reason: "the issuer is what ties the signature to the provider we trust",
		},
		{
			name: "token has expired",
			token: func() string {
				return f.sign(t, jwt.MapClaims{
					"aud": "client-web", "sub": "s",
					"exp": time.Now().Add(-time.Minute).Unix(),
				})
			},
			reason: "an expired token is a replay",
		},
		{
			name: "token carries no expiry at all",
			token: func() string {
				tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
					"aud": "client-web", "sub": "s", "iss": f.issuer,
				})
				tok.Header["kid"] = f.kid
				signed, err := tok.SignedString(f.key)
				if err != nil {
					t.Fatalf("signing: %v", err)
				}
				return signed
			},
			reason: "a token with no expiry never stops working",
		},
		{
			name: "nonce does not match the flow we started",
			token: func() string {
				return f.sign(t, jwt.MapClaims{"aud": "client-web", "sub": "s", "nonce": "someone-elses"})
			},
			nonce:  "ours",
			reason: "the nonce is what binds the token to this sign-in attempt",
		},
		{
			name: "token is signed with an unknown key",
			token: func() string {
				other, err := rsa.GenerateKey(rand.Reader, 2048)
				if err != nil {
					t.Fatalf("generating a key: %v", err)
				}
				tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
					"aud": "client-web", "sub": "s", "iss": f.issuer,
					"exp": time.Now().Add(time.Hour).Unix(),
				})
				tok.Header["kid"] = f.kid
				signed, err := tok.SignedString(other)
				if err != nil {
					t.Fatalf("signing: %v", err)
				}
				return signed
			},
			reason: "anyone can mint a token; only the provider can sign one",
		},
		{
			name: "algorithm confusion: HMAC signed with the public key",
			token: func() string {
				// The classic attack: take the provider's *public* key, which
				// anybody can fetch, and use it as an HMAC secret. A verifier
				// that picks its algorithm from the token header accepts it.
				pub := f.key.Public().(*rsa.PublicKey)
				tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
					"aud": "client-web", "sub": "s", "iss": f.issuer,
					"exp": time.Now().Add(time.Hour).Unix(),
				})
				tok.Header["kid"] = f.kid
				signed, err := tok.SignedString(pub.N.Bytes())
				if err != nil {
					t.Fatalf("signing: %v", err)
				}
				return signed
			},
			reason: "the algorithm must come from our allow-list, never from the token",
		},
		{
			name:   "not a token at all",
			token:  func() string { return "not.a.jwt" },
			reason: "garbage must not reach the claim mapping",
		},
		{
			name:   "empty token",
			token:  func() string { return "" },
			reason: "an empty credential is not a credential",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := p.VerifyIDToken(context.Background(), tc.token(), tc.nonce); err == nil {
				t.Fatalf("token was accepted but must not be: %s", tc.reason)
			}
		})
	}
}

func TestVerifyIDTokenAcceptsASiblingPlatformAudience(t *testing.T) {
	// Google mints a different client id per platform, so an ID token from the
	// iOS SDK carries the iOS client id rather than the server's. Both are
	// ours, and both have to work.
	f := newFakeProvider(t)
	p := f.provider(t, func(c *OIDCConfig) {
		c.ExtraAudiences = []string{"client-ios", "client-android"}
	})

	raw := f.sign(t, jwt.MapClaims{"aud": "client-ios", "sub": "s"})
	if _, err := p.VerifyIDToken(context.Background(), raw, ""); err != nil {
		t.Fatalf("a token from our own iOS client id was rejected: %v", err)
	}
}

func TestVerifyIDTokenHandlesAudienceArrays(t *testing.T) {
	f := newFakeProvider(t)
	p := f.provider(t, nil)
	raw := f.sign(t, jwt.MapClaims{"aud": []string{"other", "client-web"}, "sub": "s"})
	if _, err := p.VerifyIDToken(context.Background(), raw, ""); err != nil {
		t.Fatalf("an audience array containing our client id was rejected: %v", err)
	}
}

func TestEmailVerifiedIsNotAssumed(t *testing.T) {
	f := newFakeProvider(t)

	t.Run("absent claim is not trusted by default", func(t *testing.T) {
		p := f.provider(t, nil)
		raw := f.sign(t, jwt.MapClaims{"aud": "client-web", "sub": "s", "email": "a@example.com"})
		claims, err := p.VerifyIDToken(context.Background(), raw, "")
		if err != nil {
			t.Fatalf("verify: %v", err)
		}
		if claims.EmailVerified {
			// This is the account-takeover guard: an unverified address must
			// never be usable to attach to somebody's existing account.
			t.Error("an address with no email_verified claim was reported as verified")
		}
	})

	t.Run("string true is honoured", func(t *testing.T) {
		// Apple sends this as a string rather than a bool.
		p := f.provider(t, nil)
		raw := f.sign(t, jwt.MapClaims{
			"aud": "client-web", "sub": "s",
			"email": "a@example.com", "email_verified": "true",
		})
		claims, err := p.VerifyIDToken(context.Background(), raw, "")
		if err != nil {
			t.Fatalf("verify: %v", err)
		}
		if !claims.EmailVerified {
			t.Error(`email_verified:"true" was not honoured`)
		}
	})

	t.Run("opting in trusts an absent claim", func(t *testing.T) {
		p := f.provider(t, func(c *OIDCConfig) { c.TrustEmailWithoutVerifiedClaim = true })
		raw := f.sign(t, jwt.MapClaims{"aud": "client-web", "sub": "s", "email": "a@example.com"})
		claims, err := p.VerifyIDToken(context.Background(), raw, "")
		if err != nil {
			t.Fatalf("verify: %v", err)
		}
		if !claims.EmailVerified {
			t.Error("the opt-in did not take effect")
		}
	})
}

func TestEmailFallsBackToPreferredUsername(t *testing.T) {
	// Microsoft omits `email` for personal accounts and puts the address in
	// `preferred_username` instead.
	f := newFakeProvider(t)
	p := f.provider(t, nil)
	raw := f.sign(t, jwt.MapClaims{
		"aud": "client-web", "sub": "s", "preferred_username": "person@outlook.com",
	})
	claims, err := p.VerifyIDToken(context.Background(), raw, "")
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if claims.Email != "person@outlook.com" {
		t.Errorf("email = %q, want person@outlook.com", claims.Email)
	}
}

func TestNameIsAssembledFromGivenAndFamily(t *testing.T) {
	f := newFakeProvider(t)
	p := f.provider(t, nil)
	raw := f.sign(t, jwt.MapClaims{
		"aud": "client-web", "sub": "s",
		"given_name": "Ada", "family_name": "Lovelace",
	})
	claims, err := p.VerifyIDToken(context.Background(), raw, "")
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if claims.Name != "Ada Lovelace" {
		t.Errorf("name = %q, want Ada Lovelace", claims.Name)
	}
}

func TestExchangeCodeVerifiesTheReturnedToken(t *testing.T) {
	f := newFakeProvider(t)
	p := f.provider(t, nil)
	f.idToken = f.sign(t, jwt.MapClaims{
		"aud": "client-web", "sub": "exchanged-subject", "nonce": "n-9",
	})

	claims, err := p.ExchangeCode(context.Background(), "auth-code", "https://app.example/callback", "n-9")
	if err != nil {
		t.Fatalf("exchanging a code: %v", err)
	}
	if claims.Subject != "exchanged-subject" {
		t.Errorf("subject = %q, want exchanged-subject", claims.Subject)
	}
}

func TestExchangeCodeSurfacesProviderErrors(t *testing.T) {
	f := newFakeProvider(t)
	p := f.provider(t, nil)
	f.tokenErr = "invalid_grant"

	_, err := p.ExchangeCode(context.Background(), "used-code", "https://app.example/callback", "")
	if err == nil {
		t.Fatal("a rejected exchange was reported as success")
	}
	if !strings.Contains(err.Error(), "invalid_grant") {
		t.Errorf("error = %v, want it to name the provider's own reason", err)
	}
}

func TestAuthCodeURLCarriesTheFlowParameters(t *testing.T) {
	f := newFakeProvider(t)
	p := f.provider(t, func(c *OIDCConfig) {
		c.AuthParams = map[string]string{"prompt": "select_account"}
	})

	raw, err := p.AuthCodeURL("state-1", "nonce-1", "https://app.example/callback")
	if err != nil {
		t.Fatalf("building the authorization url: %v", err)
	}
	for _, want := range []string{
		"state=state-1",
		"nonce=nonce-1",
		"client_id=client-web",
		"response_type=code",
		"prompt=select_account",
		"redirect_uri=https%3A%2F%2Fapp.example%2Fcallback",
	} {
		if !strings.Contains(raw, want) {
			t.Errorf("authorization url is missing %s\n  got: %s", want, raw)
		}
	}
}

func TestKeySetIsCachedAcrossVerifications(t *testing.T) {
	// Refetching the key set per sign-in would put a third-party round trip on
	// the critical path of every login.
	f := newFakeProvider(t)
	fetches := 0
	mux := http.NewServeMux()
	var countingURL string
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                 f.issuer,
			"authorization_endpoint": f.issuer + "/authorize",
			"token_endpoint":         f.issuer + "/token",
			"jwks_uri":               countingURL + "/jwks",
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		fetches++
		f.writeJWKS(w)
	})
	counting := httptest.NewServer(mux)
	defer counting.Close()
	countingURL = counting.URL

	p := NewOIDCProvider(OIDCConfig{
		ID: "fake", Issuer: f.issuer, DiscoveryURL: counting.URL + "/.well-known/openid-configuration",
		ClientID: "client-web", HTTPClient: counting.Client(),
	})

	raw := f.sign(t, jwt.MapClaims{"aud": "client-web", "sub": "s"})
	for i := 0; i < 3; i++ {
		if _, err := p.VerifyIDToken(context.Background(), raw, ""); err != nil {
			t.Fatalf("verify %d: %v", i, err)
		}
	}
	if fetches != 1 {
		t.Errorf("jwks was fetched %d times for 3 verifications, want 1", fetches)
	}
}
