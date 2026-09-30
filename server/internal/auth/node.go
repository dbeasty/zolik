package auth

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
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

// nodeIdentity is what this install is, as distinct from who is using it.
//
// A cloud server, a LAN server in a club house and a phone hosting a table in
// a garden are all nodes holding a replica of the same database, and each of
// them needs to be able to say something the others can check. That is what
// this key is for. It is not the cloud's signing key and it never speaks for
// the cloud: it signs the two things a node is entitled to assert on its own,
// which are the guest seats it handed out and the receipts for them.
//
// The id is derived from the public key rather than chosen, so a node cannot
// claim to be another one by writing a different name into a file, and
// anything holding the key can confirm that the id belongs to it.
var nodeIdentity struct {
	sync.RWMutex
	id   string
	priv ed25519.PrivateKey
}

// SetNodeKey installs this node's identity from the seed the install has kept
// since it first ran. It is the node equivalent of SetAccessSecret, and the
// mobile host calls it with the key it stores beside host.json.
//
// On a host with no cloud signing key of its own, which is every offline
// host, the node key also becomes what access tokens are signed with. That is
// the point of item two: a guest seated at a kitchen table gets a token this
// node signed, verifiable by anything that knows the node's public key,
// instead of one signed with a shared secret that only this phone can check.
// The per-install accessSecret stays in the keyring behind it, so tokens
// issued to the guests of earlier builds keep working until they expire.
func SetNodeKey(seed string) error {
	priv, err := parseEd25519Private(seed)
	if err != nil {
		return fmt.Errorf("node key: %w", err)
	}
	installNodeKey(priv)
	return nil
}

// SetNodeKeyFromFile is the same thing for a server rather than a phone: the
// key lives in a file the install owns, and is generated the first time.
func SetNodeKeyFromFile(path string) error {
	priv, err := loadOrCreateSigningKey(path)
	if err != nil {
		return fmt.Errorf("node key: %w", err)
	}
	installNodeKey(priv)
	return nil
}

// ConfigureNodeKey is what a server deployment calls with NODE_KEY and
// NODE_KEY_FILE. Neither set is not an error: the cloud has no need of a node
// identity until it syncs with something, and a deployment that never does
// should not be made to carry a key it does not use.
func ConfigureNodeKey(seed, file string) error {
	switch {
	case strings.TrimSpace(seed) != "":
		return SetNodeKey(seed)
	case strings.TrimSpace(file) != "":
		return SetNodeKeyFromFile(file)
	default:
		return nil
	}
}

// NodeKeyConfigFromEnv reads the two variables a server sets.
func NodeKeyConfigFromEnv() (seed, file string) {
	return strings.TrimSpace(os.Getenv("NODE_KEY")), strings.TrimSpace(os.Getenv("NODE_KEY_FILE"))
}

func installNodeKey(priv ed25519.PrivateKey) {
	pub, _ := priv.Public().(ed25519.PublicKey)
	id := NodeIDFor(pub)

	nodeIdentity.Lock()
	nodeIdentity.id, nodeIdentity.priv = id, priv
	nodeIdentity.Unlock()

	// Trusted for verification either way, so this node can read back what it
	// signed itself, and promoted to signer only where nothing else has
	// claimed that job.
	trustPublicKey(pub)
	keyring.Lock()
	defer keyring.Unlock()
	if keyring.signer == nil {
		keyring.signer = &signingKey{kid: KeyIDFor(pub), priv: priv}
	}
	if keyring.issuer == "" {
		keyring.issuer = "node:" + id
	}
}

// NodeIDFor names a node after its key: the first eight bytes of the public
// key's digest, hex. Short enough to read out over a table, long enough that
// two installs will not collide, and checkable, which is what matters when a
// receipt claims to have come from a particular node.
func NodeIDFor(pub ed25519.PublicKey) string {
	sum := sha256.Sum256(pub)
	return hex.EncodeToString(sum[:8])
}

// NodeID is this node's id, or empty where no node key has been installed.
func NodeID() string {
	nodeIdentity.RLock()
	defer nodeIdentity.RUnlock()
	return nodeIdentity.id
}

// NodePublicKey is the half of the node key other nodes are given, in the
// enrolment request and wherever a receipt has to be checked.
func NodePublicKey() (ed25519.PublicKey, bool) {
	nodeIdentity.RLock()
	defer nodeIdentity.RUnlock()
	if nodeIdentity.priv == nil {
		return nil, false
	}
	pub, _ := nodeIdentity.priv.Public().(ed25519.PublicKey)
	return pub, true
}

// EncodeNodePublicKey renders a public key the way the enrolment endpoint
// expects it, base64url without padding, matching the `x` of a JWK.
func EncodeNodePublicKey(pub ed25519.PublicKey) string {
	return base64.RawURLEncoding.EncodeToString(pub)
}

// ErrNoNodeKey is returned where something needs this node's identity and the
// install has none. It is a configuration failure rather than a refusal.
var ErrNoNodeKey = errors.New("this install has no node key")

// SeatReceiptClaims is the long-lived proof that a particular guest sat at a
// particular node.
//
// A guest at an offline table is a device, not an account, and their play is
// recorded against a guest id that means nothing to the cloud. The receipt is
// what makes that reconcilable later: when the guest eventually signs in, they
// hand back the receipts they have collected, and the cloud can see that the
// node which issued them really did sign them and really is the node it says
// it is. It has no expiry on purpose. A season of games in a cottage with no
// signal is exactly the case it exists for, and a receipt that had gone stale
// by the time anyone got a connection would be worth nothing.
//
// The subject mirrors GuestID so that anything in this codebase which reads a
// token's subject sees the same value the named claim carries.
type SeatReceiptClaims struct {
	GuestID string `json:"guestId"`
	NodeID  string `json:"nodeId"`
	jwt.RegisteredClaims
}

// IssuedAtTime is the receipt's issuedAt as a time.
func (c *SeatReceiptClaims) IssuedAtTime() time.Time {
	if c.IssuedAt == nil {
		return time.Time{}
	}
	return c.IssuedAt.Time
}

// CreateSeatReceipt signs a receipt with the node key. A node with no key
// issues none, and the guest seat it hands out is simply unreconcilable
// later, which is what every seat was before this existed.
func CreateSeatReceipt(guestID string) (string, error) {
	guestID = sanitizeGuestID(guestID)
	if guestID == "" {
		return "", errors.New("seat receipt: not a guest id")
	}
	nodeIdentity.RLock()
	id, priv := nodeIdentity.id, nodeIdentity.priv
	nodeIdentity.RUnlock()
	if priv == nil {
		return "", ErrNoNodeKey
	}

	now := time.Now().UTC()
	pub, _ := priv.Public().(ed25519.PublicKey)
	return signEdDSA(&signingKey{kid: KeyIDFor(pub), priv: priv}, SeatReceiptClaims{
		GuestID: guestID,
		NodeID:  id,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:  guestID,
			Issuer:   "node:" + id,
			Audience: jwt.ClaimStrings{AudienceSeat},
			IssuedAt: jwt.NewNumericDate(now),
		},
	})
}

// VerifySeatReceipt checks a receipt against the public key of the node that
// is supposed to have issued it.
//
// Three things have to line up, and any one of them failing fails the whole
// receipt: the signature, the audience, and the claimed node id matching the
// key that signed it. The last is what stops a node from writing somebody
// else's id into a receipt it signed itself, which is the only interesting
// lie available to a node that holds a valid key of its own.
func VerifySeatReceipt(token string, pub ed25519.PublicKey) (*SeatReceiptClaims, error) {
	if len(pub) != ed25519.PublicKeySize {
		return nil, errors.New("seat receipt: no node key to check against")
	}
	parsed, err := jwt.ParseWithClaims(strings.TrimSpace(token), &SeatReceiptClaims{}, func(t *jwt.Token) (interface{}, error) {
		if t.Method.Alg() != jwt.SigningMethodEdDSA.Alg() {
			return nil, fmt.Errorf("unexpected signing method %q", t.Method.Alg())
		}
		return pub, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := parsed.Claims.(*SeatReceiptClaims)
	if !ok || !parsed.Valid {
		return nil, errors.New("seat receipt: invalid")
	}
	if !slices.Contains(claims.Audience, AudienceSeat) {
		return nil, errors.New("seat receipt: wrong audience")
	}
	if claims.NodeID == "" || claims.NodeID != NodeIDFor(pub) {
		return nil, errors.New("seat receipt: claims a node other than the one that signed it")
	}
	if claims.GuestID == "" || (claims.Subject != "" && claims.Subject != claims.GuestID) {
		return nil, errors.New("seat receipt: no coherent guest")
	}
	if claims.IssuedAt == nil {
		return nil, errors.New("seat receipt: no issue time")
	}
	return claims, nil
}

// PeekSeatReceiptNode reads which node a receipt names without checking
// anything at all.
//
// It exists because the cloud has to look the node's key up before it can
// verify, and the id is in the token it has not verified yet. Nothing may be
// decided on what it returns: it is a database lookup key, and the answer
// only becomes true when VerifySeatReceipt passes against the key that lookup
// found.
func PeekSeatReceiptNode(token string) (string, error) {
	var claims SeatReceiptClaims
	if _, _, err := jwt.NewParser().ParseUnverified(strings.TrimSpace(token), &claims); err != nil {
		return "", err
	}
	if claims.NodeID == "" {
		return "", errors.New("seat receipt: names no node")
	}
	return claims.NodeID, nil
}
