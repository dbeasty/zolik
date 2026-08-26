package admin

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"zolik/server/internal/ratelimit"
)

// PasswordLogin is a username and a bcrypt hash from configuration, letting an
// operator into the console without the email flow working.
//
// It exists because the allow-list path needs mail delivery: signing in there
// means receiving a one-time code, so a deployment with no SMTP — or one whose
// SMTP has just broken — has no way into the console at all, which is exactly
// when somebody needs to get in. This is the way in that depends on nothing.
//
// Only the hash is ever configured. A plaintext password in an environment
// variable would sit in shell history, process listings, `docker inspect`
// output and any log that dumps the environment; the hash is useless in all
// of those places.
type PasswordLogin struct {
	Username string
	// Hash is a bcrypt hash of the password, as produced by
	// `go run ./cmd/adminpass`.
	Hash string
}

// Enabled reports whether a password sign-in is configured. Both halves are
// required: half-configured means off, never means open.
func (p PasswordLogin) Enabled() bool {
	return strings.TrimSpace(p.Username) != "" && strings.TrimSpace(p.Hash) != ""
}

// consoleTokenTTL is how long a password sign-in lasts. Longer than the
// player access token because there is no refresh flow behind it — the console
// would otherwise log an operator out mid-job — and short enough that a
// token taken from a browser is not a lasting key.
const consoleTokenTTL = 12 * time.Hour

// Sign-in attempts allowed per address per window, before the ceiling.
const (
	loginWindow = 15 * time.Minute
	loginLimit  = 10
)

// consoleAudience marks a token as belonging to the console rather than to a
// player.
const consoleAudience = "zolik-admin-console"

// signingKey derives the console token's HMAC key from the password hash.
//
// Deriving it rather than configuring a separate secret has two payoffs:
// nothing extra to set, and changing the password invalidates every console
// token already issued, because the key they were signed with no longer
// exists. It is also necessarily distinct from the player token secret, so a
// console token is not a player token and cannot be traded for one.
func (p PasswordLogin) signingKey() []byte {
	sum := sha256.Sum256([]byte(consoleAudience + "\x00" + p.Hash))
	return sum[:]
}

func (p PasswordLogin) mintToken(now time.Time) (string, error) {
	claims := jwt.RegisteredClaims{
		Subject:   p.Username,
		Audience:  jwt.ClaimStrings{consoleAudience},
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(consoleTokenTTL)),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(p.signingKey())
}

// parseToken validates a console token and returns the username it names.
func (p PasswordLogin) parseToken(token string) (string, error) {
	if !p.Enabled() {
		return "", errors.New("password sign-in is not configured")
	}
	parsed, err := jwt.ParseWithClaims(
		strings.TrimSpace(token),
		&jwt.RegisteredClaims{},
		func(t *jwt.Token) (any, error) {
			if t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
				return nil, errors.New("unexpected signing method")
			}
			return p.signingKey(), nil
		},
		jwt.WithAudience(consoleAudience),
	)
	if err != nil {
		return "", err
	}
	claims, ok := parsed.Claims.(*jwt.RegisteredClaims)
	if !ok || !parsed.Valid || claims.Subject == "" {
		return "", errors.New("invalid console token")
	}
	// The subject is checked against the configured name as well as the
	// signature, so a token minted under a previous username stops working
	// when the username changes, not only when the password does.
	if subtle.ConstantTimeCompare([]byte(claims.Subject), []byte(p.Username)) != 1 {
		return "", errors.New("token names a different administrator")
	}
	return claims.Subject, nil
}

// check verifies a submitted username and password.
//
// The username comparison is constant-time and the bcrypt comparison always
// runs, even when the username is already wrong. Returning early on a bad
// username would make the wrong-username case measurably faster than the
// wrong-password one, which turns the endpoint into an oracle for whether a
// guessed username is the right one.
func (p PasswordLogin) check(username, password string) bool {
	nameOK := subtle.ConstantTimeCompare(
		[]byte(strings.TrimSpace(username)),
		[]byte(strings.TrimSpace(p.Username)),
	) == 1
	passwordOK := bcrypt.CompareHashAndPassword([]byte(p.Hash), []byte(password)) == nil
	return nameOK && passwordOK
}

type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// login exchanges a configured username and password for a console token.
func (h *Handlers) login(w http.ResponseWriter, r *http.Request) {
	login := h.deps.Guard.password
	if !login.Enabled() {
		http.Error(w, "password sign-in is not enabled", http.StatusNotFound)
		return
	}

	// Keyed by address, before the password is even looked at, so a guessing
	// run is slowed whether or not it is guessing the right username.
	key := ratelimit.ClientIP(r)
	if !h.deps.Guard.logins.Allow(key, time.Now().UTC()) {
		http.Error(w, "too many attempts — wait a few minutes", http.StatusTooManyRequests)
		return
	}

	var body loginReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if !login.check(body.Username, body.Password) {
		// One message for both halves being wrong: saying which was wrong
		// would confirm a guessed username.
		http.Error(w, "wrong username or password", http.StatusUnauthorized)
		return
	}

	token, err := login.mintToken(time.Now().UTC())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// A successful sign-in clears the address's history, so a few fumbled
	// attempts before the right one do not eat into the next sign-in.
	h.deps.Guard.logins.Reset(key)

	writeJSON(w, map[string]any{
		"token":            token,
		"username":         login.Username,
		"expiresInSeconds": int(consoleTokenTTL.Seconds()),
	})
}
