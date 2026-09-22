package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type AccessClaims struct {
	Username string `json:"username"`
	IsGuest  bool   `json:"isGuest"`
	jwt.RegisteredClaims
}

// devAccessSecret signs access tokens when JWT_ACCESS_SECRET is unset. It is
// in the repository, so anyone can mint a token for any player with it:
// CheckAccessSecret refuses to let a non-local deployment start on it.
const devAccessSecret = "dev_access_secret_change_me"

// embeddedSecret is the signing key an in-process host set with
// SetAccessSecret. A server reads its key from the environment. The copy
// embedded in the mobile app has no environment worth the name, and it must
// never fall back to devAccessSecret, because other phones on the local
// network can reach its listener.
var embeddedSecret struct {
	sync.RWMutex
	key string
}

// SetAccessSecret fixes the access-token signing key for this process. The
// environment and the development default are no longer consulted after it
// is called.
func SetAccessSecret(key string) {
	embeddedSecret.Lock()
	defer embeddedSecret.Unlock()
	embeddedSecret.key = key
}

func embeddedAccessSecret() string {
	embeddedSecret.RLock()
	defer embeddedSecret.RUnlock()
	return embeddedSecret.key
}

func accessSecret() string {
	if set := embeddedAccessSecret(); set != "" {
		return set
	}
	if v := strings.TrimSpace(os.Getenv("JWT_ACCESS_SECRET")); v != "" {
		return v
	}
	return devAccessSecret
}

// CheckAccessSecret reports whether the access-token signing key is fit for
// the deployment. Locally the built-in default is fine. Anywhere else the key
// must be set, and must not be the default or the placeholder that
// deploy/env.production.example ships with, because either is public and
// would let anyone sign a token as any player. A key set in-process with
// SetAccessSecret is the embedded host's own per-install secret, and passes.
//
// There is no matching check for JWT_REFRESH_SECRET: refresh tokens are
// opaque random strings looked up in the database, not signed, so that
// variable is never read.
func CheckAccessSecret(local bool) error {
	if local {
		return nil
	}
	if embeddedAccessSecret() != "" {
		return nil
	}
	switch strings.TrimSpace(os.Getenv("JWT_ACCESS_SECRET")) {
	case "":
		return errors.New("JWT_ACCESS_SECRET is unset; outside APP_ENV=local it must be set to a private random value")
	case devAccessSecret, "REPLACE_ON_FIRST_DEPLOY":
		return errors.New("JWT_ACCESS_SECRET is a published placeholder; outside APP_ENV=local it must be set to a private random value")
	}
	return nil
}

func getenv(k string) string {
	return os.Getenv(k)
}

// accessKeyfunc picks the key a token names, and refuses everything else.
//
// Two algorithms are live at once, deliberately. EdDSA is what this server
// signs with now: the token names its key in the `kid` header, and a node
// holding only the public half can check it without holding anything that
// could mint one. HS256 is what every token issued before that was signed
// with, and a deployment goes on accepting it until its migration window
// closes, because refusing it the moment an Ed25519 key appears would end
// every session being played at that moment.
//
// Nothing else is accepted, and neither is an EdDSA token whose kid is not
// one this process holds. An unknown key is an unknown signer, and a verifier
// that resolved it to "whatever key I have" would be checking a signature
// against a question nobody asked.
func accessKeyfunc(t *jwt.Token) (interface{}, error) {
	switch t.Method.Alg() {
	case jwt.SigningMethodEdDSA.Alg():
		kid, _ := t.Header["kid"].(string)
		if kid == "" {
			return nil, errors.New("token names no signing key")
		}
		pub, ok := publicKeyByID(kid)
		if !ok {
			return nil, fmt.Errorf("unknown signing key %q", kid)
		}
		return pub, nil
	case jwt.SigningMethodHS256.Alg():
		if !legacyHS256Accepted(time.Now()) {
			return nil, errors.New("shared-secret tokens are no longer accepted")
		}
		return []byte(accessSecret()), nil
	default:
		return nil, fmt.Errorf("unexpected signing method %q", t.Method.Alg())
	}
}

// parseAccess is the one place an access token is read. Both exported
// entry points go through it, so there is no second opinion anywhere about
// what makes a token acceptable.
func parseAccess(token string) (*AccessClaims, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, errors.New("empty token")
	}
	parsed, err := jwt.ParseWithClaims(token, &AccessClaims{}, accessKeyfunc)
	if err != nil {
		return nil, err
	}
	claims, ok := parsed.Claims.(*AccessClaims)
	if !ok || !parsed.Valid {
		return nil, errors.New("invalid token")
	}
	// A token signed with the asymmetric key has to say it is an access
	// token. The same key signs offline passes, node credentials and seat
	// receipts, and without this check any of them would open a match socket
	// as whoever they name. A legacy HS256 token predates audiences
	// altogether and carries none, which is read as what it is; one that does
	// carry an audience is held to it either way.
	if parsed.Method.Alg() == jwt.SigningMethodEdDSA.Alg() || len(claims.Audience) > 0 {
		if !slices.Contains(claims.Audience, AudienceAccess) {
			return nil, errors.New("token is not an access token")
		}
	}
	return claims, nil
}

// SubjectFromToken extracts the player subject from a signed access JWT. It is
// the only thing standing between a match socket and a player's hidden hand,
// so it accepts nothing but a token this server signed.
func SubjectFromToken(token string) (string, error) {
	claims, err := parseAccess(token)
	if err != nil {
		return "", err
	}
	if claims.Subject == "" {
		return "", errors.New("missing subject")
	}
	return claims.Subject, nil
}

func ParseAccessClaims(token string) (*AccessClaims, error) {
	return parseAccess(token)
}

func NewRandomToken(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func CreateAccessToken(subject, username string, isGuest bool, ttl time.Duration) (string, error) {
	now := time.Now().UTC()
	claims := AccessClaims{
		Username: username,
		IsGuest:  isGuest,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   subject,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	// The asymmetric key wins wherever there is one. Only then does the token
	// gain an issuer and an audience: a node verifying it offline has no way
	// to ask who signed it or what it was for, so the token has to say, and a
	// legacy HS256 token has to keep looking exactly as it always did.
	if signer := activeSigner(); signer != nil {
		claims.Issuer = tokenIssuer()
		claims.Audience = jwt.ClaimStrings{AudienceAccess}
		return signEdDSA(signer, claims)
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString([]byte(accessSecret()))
}

// signEdDSA signs claims and stamps the key's id into the header, which is
// how a verifier knows which of the keys the JWKS lists to check against.
func signEdDSA(k *signingKey, claims jwt.Claims) (string, error) {
	t := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	t.Header["kid"] = k.kid
	return t.SignedString(k.priv)
}

// CreateRefreshToken is opaque and persisted in MongoDB.
func CreateRefreshToken() (string, error) {
	// 32 bytes -> 64 hex chars.
	return NewRandomToken(32)
}
