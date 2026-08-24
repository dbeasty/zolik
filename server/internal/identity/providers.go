package identity

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ProviderConfig is the deployment-supplied half of one provider: the secrets
// and ids that differ per environment. Everything else — issuer, endpoints,
// quirks — is a constant of the provider itself and lives in this file.
type ProviderConfig struct {
	ClientID     string
	ClientSecret string
	// ExtraAudiences lists sibling platform client ids (see
	// OIDCConfig.ExtraAudiences). Comma-separated in the environment.
	ExtraAudiences []string

	// Apple only: the values needed to mint its client secret.
	TeamID     string
	KeyID      string
	PrivateKey string

	// Microsoft only: the tenant segment of the authority ("common",
	// "organizations", "consumers", or a tenant id). Defaults to "common".
	Tenant string
}

// Config is the whole identity configuration, one entry per provider the
// deployment wants enabled. A provider with no client id is left out of the
// registry entirely (NewOIDCProvider returns nil), so the sign-in screen shows
// exactly what is actually usable.
type Config struct {
	Google    ProviderConfig
	Apple     ProviderConfig
	Microsoft ProviderConfig
}

// FromConfig builds the registry.
//
// This function is the complete list of "what can I sign in with", and the
// only place in the server that names a provider. Adding a fourth is a
// constructor below plus one line here — no handler, model, storage or client
// change, because everything past this point works in Provider and Claims.
func FromConfig(cfg Config) *Registry {
	return NewRegistry(
		Google(cfg.Google),
		Apple(cfg.Apple),
		Microsoft(cfg.Microsoft),
	)
}

// Google is a plain OIDC provider with no quirks beyond per-platform client
// ids: the iOS, Android and web SDKs each mint ID tokens under their own
// client id, all of which must be accepted here (see ExtraAudiences).
func Google(pc ProviderConfig) Provider {
	return NewOIDCProvider(OIDCConfig{
		ID:          "google",
		DisplayName: "Google",
		Issuer:      "https://accounts.google.com",
		IssuerMatches: func(iss string) bool {
			// Google documents both spellings as valid for the same tokens.
			return iss == "https://accounts.google.com" || iss == "accounts.google.com"
		},
		ClientID:       pc.ClientID,
		ClientSecret:   pc.ClientSecret,
		ExtraAudiences: pc.ExtraAudiences,
		Scopes:         []string{"openid", "email", "profile"},
		AuthParams: map[string]string{
			// Without this Google skips the account chooser for anyone with a
			// single signed-in session, which makes "sign in as someone else"
			// impossible on a shared device.
			"prompt": "select_account",
		},
	})
}

// Apple is OIDC with two documented deviations, both handled as data:
//
//   - the client secret is a short-lived ES256 JWT the server signs with a
//     downloaded .p8 key rather than a fixed string (ClientSecretFunc), and
//   - the person's name is delivered *once*, in the form post of the very
//     first authorization, and never again — so an account created through
//     Apple must take its display name from that first response or generate
//     one, and can never re-fetch it later.
//
// It is registered as soon as the four Apple values are configured; until
// then NewOIDCProvider drops it.
func Apple(pc ProviderConfig) Provider {
	if strings.TrimSpace(pc.ClientID) == "" {
		return nil
	}
	signer := NewAppleSecretSigner(pc.TeamID, pc.KeyID, pc.ClientID, pc.PrivateKey)
	return NewOIDCProvider(OIDCConfig{
		ID:             "apple",
		DisplayName:    "Apple",
		Issuer:         "https://appleid.apple.com",
		ClientID:       pc.ClientID,
		ExtraAudiences: pc.ExtraAudiences,
		// Apple's client secret expires (six months maximum), so it is minted
		// per request rather than configured.
		ClientSecretFunc: signer.Secret,
		Scopes:           []string{"openid", "email", "name"},
		AuthParams: map[string]string{
			// Apple requires form_post whenever name or email is requested.
			"response_mode": "form_post",
		},
	})
}

// Microsoft is OIDC against the v2.0 endpoint. Its quirks:
//
//   - with the multi-tenant `common` authority the ID token's issuer names the
//     *user's* tenant, so the issuer is matched by shape rather than equality;
//   - it does not emit `email_verified` for personal accounts, so its
//     addresses are deliberately not trusted for matching an existing account
//     (see OIDCConfig.TrustEmailWithoutVerifiedClaim). Signing in with
//     Microsoft therefore creates or links by subject, never by address.
func Microsoft(pc ProviderConfig) Provider {
	tenant := strings.TrimSpace(pc.Tenant)
	if tenant == "" {
		tenant = "common"
	}
	return NewOIDCProvider(OIDCConfig{
		ID:           "microsoft",
		DisplayName:  "Microsoft",
		Issuer:       fmt.Sprintf("https://login.microsoftonline.com/%s/v2.0", tenant),
		DiscoveryURL: fmt.Sprintf("https://login.microsoftonline.com/%s/v2.0/.well-known/openid-configuration", tenant),
		IssuerMatches: func(iss string) bool {
			return strings.HasPrefix(iss, "https://login.microsoftonline.com/") &&
				strings.HasSuffix(strings.TrimSuffix(iss, "/"), "/v2.0")
		},
		ClientID:       pc.ClientID,
		ClientSecret:   pc.ClientSecret,
		ExtraAudiences: pc.ExtraAudiences,
		Scopes:         []string{"openid", "email", "profile"},
	})
}

// AppleSecretSigner mints Apple's client secret: an ES256 JWT signed with the
// private key downloaded from the Apple Developer portal, valid for at most
// six months. Cached and re-minted shortly before expiry rather than per call,
// since signing is cheap but not free and the token is reusable.
type AppleSecretSigner struct {
	teamID   string
	keyID    string
	clientID string
	pemKey   string

	mu        sync.Mutex
	cached    string
	expiresAt time.Time
}

// NewAppleSecretSigner accepts the private key either inline (PEM, as it comes
// out of the .p8 file) or as a path to the file, so a deployment can use an
// environment variable or a mounted secret without a wrapper.
func NewAppleSecretSigner(teamID, keyID, clientID, keyOrPath string) *AppleSecretSigner {
	return &AppleSecretSigner{teamID: teamID, keyID: keyID, clientID: clientID, pemKey: keyOrPath}
}

// Secret returns a valid client secret, minting a new one when the cached one
// is within a day of expiring.
func (s *AppleSecretSigner) Secret() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cached != "" && time.Until(s.expiresAt) > 24*time.Hour {
		return s.cached, nil
	}
	if s.teamID == "" || s.keyID == "" || s.pemKey == "" {
		return "", errors.New("apple sign-in is missing team id, key id or private key")
	}
	key, err := s.privateKey()
	if err != nil {
		return "", err
	}

	now := time.Now().UTC()
	// Apple caps the lifetime at six months; a shorter one limits the damage
	// from a leaked secret and costs nothing, since it is minted on demand.
	exp := now.Add(90 * 24 * time.Hour)
	tok := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.RegisteredClaims{
		Issuer:    s.teamID,
		Subject:   s.clientID,
		Audience:  jwt.ClaimStrings{"https://appleid.apple.com"},
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(exp),
	})
	tok.Header["kid"] = s.keyID

	signed, err := tok.SignedString(key)
	if err != nil {
		return "", fmt.Errorf("sign apple client secret: %w", err)
	}
	s.cached, s.expiresAt = signed, exp
	return signed, nil
}

func (s *AppleSecretSigner) privateKey() (*ecdsa.PrivateKey, error) {
	material := s.pemKey
	if !strings.Contains(material, "-----BEGIN") {
		b, err := os.ReadFile(material)
		if err != nil {
			return nil, fmt.Errorf("read apple private key: %w", err)
		}
		material = string(b)
	}
	block, _ := pem.Decode([]byte(material))
	if block == nil {
		return nil, errors.New("apple private key is not valid PEM")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse apple private key: %w", err)
	}
	key, ok := parsed.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errors.New("apple private key is not an ECDSA key")
	}
	return key, nil
}
