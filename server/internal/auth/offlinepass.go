package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// OfflinePassTTL is how long a pass is good for. Thirty days is the same
// window a refresh token gets, and is chosen for the same reason: it is about
// as long as somebody can plausibly be away from a connection and still
// expect their account to be theirs when they come back. It is also the floor
// under how long a retired signing key has to stay published, because a pass
// signed on the day of a rotation is still being presented a month later.
const OfflinePassTTL = 30 * 24 * time.Hour

// ErrNoSigningKey is returned where something that must be signed
// asymmetrically is asked for on a deployment that has no Ed25519 key. It is
// a configuration gap, not a refusal, and the sign-in paths treat it as "this
// deployment issues no passes" rather than as a failure.
var ErrNoSigningKey = errors.New("no asymmetric signing key is configured")

// OfflinePassClaims is the cloud saying, to whoever can read it later: this
// person is a real account, and here is who they are.
//
// An offline host has no accounts, no database of users and, by definition,
// no way to ask. The pass is the whole of what it has to go on, so it carries
// everything a seat needs: the account id, the name to show, and the
// credential version.
//
// CredentialVersion is what makes revocation possible at all in a place that
// cannot be told anything. The cloud raises an account's version whenever its
// credentials change (a sign-out everywhere, a stolen device, an address
// taken over), and every pass issued before that carries the older number. A
// host that has synced since then refuses the old ones; a host that has not
// synced cannot know, and will go on accepting a pass until it expires, which
// is the honest bound on how much revocation an offline system can offer.
type OfflinePassClaims struct {
	Username          string `json:"username"`
	CredentialVersion int    `json:"sv"`
	jwt.RegisteredClaims
}

// OfflineSeatPrefix marks a subject an offline host seated from a pass. A
// guest id is bare hex and an account id on the cloud is bare hex too, so the
// prefix is what keeps "this host decided you were this account" distinct
// from "the cloud says you are".
const OfflineSeatPrefix = "user:"

// SeatSubject is how an offline host records the player.
func (c *OfflinePassClaims) SeatSubject() string { return OfflineSeatPrefix + c.Subject }

// CreateOfflinePass issues the pass alongside a session. The subject is the
// account's ObjectID hex, unadorned: the `user:` prefix is a seating detail
// of the host, and putting it in the token would make the same account two
// different subjects depending on who was reading.
func CreateOfflinePass(userID, username string, credentialVersion int, ttl time.Duration) (string, error) {
	userID = strings.TrimSpace(userID)
	if !isObjectIDHex(userID) {
		return "", errors.New("offline pass: subject is not an account id")
	}
	signer := activeSigner()
	if signer == nil {
		return "", ErrNoSigningKey
	}
	if ttl <= 0 {
		ttl = OfflinePassTTL
	}
	if credentialVersion < 1 {
		credentialVersion = 1
	}
	now := time.Now().UTC()
	return signEdDSA(signer, OfflinePassClaims{
		Username:          username,
		CredentialVersion: credentialVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			Issuer:    tokenIssuer(),
			Audience:  jwt.ClaimStrings{AudienceOffline},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	})
}

// VerifyOfflinePass checks a pass against the keys this node holds, which on
// a phone is the disk-cached copy of the cloud's JWKS.
//
// Everything here fails closed. There is no HS256 path: a pass is a new thing
// with nothing in the wild to migrate, and accepting a symmetric signature
// would mean every host that can check a pass could also mint one. The
// audience must be the offline one, so an ordinary access token cannot be
// presented as a pass. An expiry is required rather than merely honoured,
// because a pass with no expiry is a permanent credential nobody agreed to
// issue. And a key set that has never been populated verifies nothing, rather
// than verifying everything.
func VerifyOfflinePass(token string, keys PublicKeys) (*OfflinePassClaims, error) {
	if keys == nil {
		return nil, errors.New("offline pass: no keys to check against")
	}
	parsed, err := jwt.ParseWithClaims(strings.TrimSpace(token), &OfflinePassClaims{},
		func(t *jwt.Token) (interface{}, error) {
			kid, _ := t.Header["kid"].(string)
			if kid == "" {
				return nil, errors.New("pass names no signing key")
			}
			pub, ok := keys.PublicKeyByID(kid)
			if !ok {
				return nil, fmt.Errorf("unknown signing key %q", kid)
			}
			return pub, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodEdDSA.Alg()}),
		jwt.WithAudience(AudienceOffline),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, err
	}
	claims, ok := parsed.Claims.(*OfflinePassClaims)
	if !ok || !parsed.Valid {
		return nil, errors.New("offline pass: invalid")
	}
	if !isObjectIDHex(claims.Subject) {
		return nil, errors.New("offline pass: subject is not an account id")
	}
	return claims, nil
}

// isObjectIDHex accepts only the shape an account id has, for the same reason
// sanitizeGuestID does: the value becomes a seat subject, a map key and part
// of a document id, and none of those should ever hold something a caller
// made up.
func isObjectIDHex(s string) bool {
	if len(s) != 24 {
		return false
	}
	for _, r := range s {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

// CredentialVersions reports the version an account's credentials are
// currently at, which is the `sv` every offline pass carries.
//
// It is an interface here because the number belongs on the account document
// and this package may not reach into the database layer while the storage
// work is in flight. The in-memory implementation below is what a deployment
// runs until the persistent one lands: everybody is at version one, which
// matches what the cloud believes about an account nobody has ever revoked.
//
// TODO: back this with the accounts store, holding the counter on the user
// document in the "users" namespace, so that raising a version survives a
// restart and reaches every node through the ordinary database sync. Until
// then a revocation is forgotten when the process is.
type CredentialVersions interface {
	CredentialVersion(ctx context.Context, userID string) (int, error)
}

// MemoryCredentialVersions is the stand-in implementation.
type MemoryCredentialVersions struct {
	mu       sync.Mutex
	versions map[string]int
}

func NewMemoryCredentialVersions() *MemoryCredentialVersions {
	return &MemoryCredentialVersions{versions: map[string]int{}}
}

var _ CredentialVersions = (*MemoryCredentialVersions)(nil)

func (m *MemoryCredentialVersions) CredentialVersion(_ context.Context, userID string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if v, ok := m.versions[userID]; ok {
		return v, nil
	}
	return 1, nil
}

// Bump invalidates every pass issued to an account so far, and returns the
// version the next one will carry.
func (m *MemoryCredentialVersions) Bump(userID string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	next := m.versions[userID]
	if next < 1 {
		next = 1
	}
	next++
	m.versions[userID] = next
	return next
}
