package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"strings"
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

func accessSecret() string {
	if v := strings.TrimSpace(os.Getenv("JWT_ACCESS_SECRET")); v != "" {
		return v
	}
	return devAccessSecret
}

// CheckAccessSecret reports whether the access-token signing key is fit for
// the deployment. Locally the built-in default is fine. Anywhere else the key
// must be set, and must not be the default or the placeholder that
// deploy/env.production.example ships with, because either is public and
// would let anyone sign a token as any player.
//
// There is no matching check for JWT_REFRESH_SECRET: refresh tokens are
// opaque random strings looked up in the database, not signed, so that
// variable is never read.
func CheckAccessSecret(local bool) error {
	if local {
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

// SubjectFromToken extracts the player subject from a signed access JWT. It is
// the only thing standing between a match socket and a player's hidden hand,
// so it accepts nothing but a token this server signed.
func SubjectFromToken(token string) (string, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return "", errors.New("empty token")
	}

	parsed, err := jwt.ParseWithClaims(token, &AccessClaims{}, func(t *jwt.Token) (interface{}, error) {
		if t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(accessSecret()), nil
	})
	if err != nil {
		return "", err
	}
	claims, ok := parsed.Claims.(*AccessClaims)
	if !ok || !parsed.Valid {
		return "", errors.New("invalid token")
	}
	if claims.Subject == "" {
		return "", errors.New("missing subject")
	}
	return claims.Subject, nil
}

func ParseAccessClaims(token string) (*AccessClaims, error) {
	token = strings.TrimSpace(token)
	parsed, err := jwt.ParseWithClaims(token, &AccessClaims{}, func(t *jwt.Token) (interface{}, error) {
		if t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(accessSecret()), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := parsed.Claims.(*AccessClaims)
	if !ok || !parsed.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
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
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString([]byte(accessSecret()))
}

// CreateRefreshToken is opaque and persisted in MongoDB.
func CreateRefreshToken() (string, error) {
	// 32 bytes -> 64 hex chars.
	return NewRandomToken(32)
}
