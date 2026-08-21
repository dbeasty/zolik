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

func accessSecret() string {
	if v := strings.TrimSpace(os.Getenv("JWT_ACCESS_SECRET")); v != "" {
		return v
	}
	return "dev_access_secret_change_me"
}

func refreshSecret() string {
	if v := strings.TrimSpace(os.Getenv("JWT_REFRESH_SECRET")); v != "" {
		return v
	}
	return "dev_refresh_secret_change_me"
}

func getenv(k string) string {
	return os.Getenv(k)
}

// TokenSubjectFromAccessToken extracts the player subject from an access JWT.
// Dev fallback: token = "dev:<playerId>".
func SubjectFromToken(token string) (string, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return "", errors.New("empty token")
	}
	if strings.HasPrefix(token, "dev:") {
		subj := strings.TrimPrefix(token, "dev:")
		if subj == "" {
			return "", errors.New("empty dev subject")
		}
		return subj, nil
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
