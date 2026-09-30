package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// newNodeKey installs a node identity for one test and returns its public
// half, the way an offline host has one from the seed in host.json.
func newNodeKey(t *testing.T) ed25519.PublicKey {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := SetNodeKey(hex.EncodeToString(priv.Seed())); err != nil {
		t.Fatal(err)
	}
	pub, _ := priv.Public().(ed25519.PublicKey)
	return pub
}

func TestSeatReceiptRoundTrip(t *testing.T) {
	resetKeys(t)
	pub := newNodeKey(t)

	guestID, err := NewRandomToken(16)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := CreateSeatReceipt(guestID)
	if err != nil {
		t.Fatal(err)
	}

	claims, err := VerifySeatReceipt(receipt, pub)
	if err != nil {
		t.Fatalf("a node could not verify its own receipt: %v", err)
	}
	if claims.GuestID != guestID || claims.Subject != guestID {
		t.Errorf("guest %q/%q, want %q", claims.GuestID, claims.Subject, guestID)
	}
	if claims.NodeID != NodeIDFor(pub) || claims.NodeID != NodeID() {
		t.Errorf("node %q, want %q", claims.NodeID, NodeIDFor(pub))
	}
	if claims.IssuedAtTime().IsZero() {
		t.Error("the receipt does not say when the guest sat down")
	}

	// The cloud reads the node id first, to know whose key to check against.
	named, err := PeekSeatReceiptNode(receipt)
	if err != nil || named != claims.NodeID {
		t.Fatalf("peeked %q (%v), want %q", named, err, claims.NodeID)
	}
}

// A receipt is worth nothing against the wrong key, and a node cannot write
// somebody else's id into one it signed itself.
func TestSeatReceiptFailsClosed(t *testing.T) {
	resetKeys(t)
	pub := newNodeKey(t)
	guestID, err := NewRandomToken(16)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := CreateSeatReceipt(guestID)
	if err != nil {
		t.Fatal(err)
	}

	otherPub, otherPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := VerifySeatReceipt(receipt, otherPub); err == nil {
		t.Fatal("a receipt verified against a key that did not sign it")
	}
	if _, err := VerifySeatReceipt(receipt, nil); err == nil {
		t.Fatal("a receipt verified against no key at all")
	}

	// Signed with a real key, but claiming to be a different node. This is
	// the only interesting lie available to somebody who holds a node key.
	impersonation, err := signEdDSA(&signingKey{kid: KeyIDFor(otherPub), priv: otherPriv}, SeatReceiptClaims{
		GuestID: guestID,
		NodeID:  NodeIDFor(pub),
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:  guestID,
			Audience: jwt.ClaimStrings{AudienceSeat},
			IssuedAt: jwt.NewNumericDate(time.Now()),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := VerifySeatReceipt(impersonation, otherPub); err == nil {
		t.Fatal("a node signed a receipt in another node's name and it was accepted")
	}

	// Wrong audience: an access token this node signed is not a receipt.
	wrongAudience, err := signEdDSA(&signingKey{kid: KeyIDFor(otherPub), priv: otherPriv}, SeatReceiptClaims{
		GuestID: guestID,
		NodeID:  NodeIDFor(otherPub),
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:  guestID,
			Audience: jwt.ClaimStrings{AudienceAccess},
			IssuedAt: jwt.NewNumericDate(time.Now()),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := VerifySeatReceipt(wrongAudience, otherPub); err == nil {
		t.Fatal("a token with the wrong audience passed as a receipt")
	}
}

func TestSeatReceiptNeedsANodeKey(t *testing.T) {
	resetKeys(t)
	guestID, err := NewRandomToken(16)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateSeatReceipt(guestID); err == nil {
		t.Fatal("a host with no node key issued a receipt")
	}
}

// On an offline host the node key is what signs the guest's access token:
// there is no cloud key, and a token nothing but this phone could check is
// what the node key replaces.
func TestNodeKeySignsGuestTokensOnAHostWithNoCloudKey(t *testing.T) {
	resetKeys(t)
	pub := newNodeKey(t)

	token, err := CreateAccessToken("abc123", "Some Guest", true, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	parsed, _, err := jwt.NewParser().ParseUnverified(token, &AccessClaims{})
	if err != nil {
		t.Fatal(err)
	}
	if alg := parsed.Method.Alg(); alg != "EdDSA" {
		t.Fatalf("a host with a node key signed a guest token with %q", alg)
	}
	if kid, _ := parsed.Header["kid"].(string); kid != KeyIDFor(pub) {
		t.Fatalf("kid %q, want the node key %q", kid, KeyIDFor(pub))
	}
	claims, _ := parsed.Claims.(*AccessClaims)
	if claims.Issuer != "node:"+NodeIDFor(pub) {
		t.Errorf("iss %q, want the node", claims.Issuer)
	}
	if _, err := ParseAccessClaims(token); err != nil {
		t.Fatalf("the host could not read back the token it signed: %v", err)
	}
}

// A cloud key already installed is not displaced by a node key: a server that
// signs for the deployment must go on doing so.
func TestNodeKeyDoesNotDisplaceTheCloudKey(t *testing.T) {
	cloud := newSigningKey(t, "https://zolik.test")
	node := newNodeKey(t)

	if signer := activeSigner(); signer.kid != KeyIDFor(cloud) {
		t.Fatal("the node key took over signing on a server that has a cloud key")
	}
	// It is trusted for verification all the same, so the server can read
	// what its own node signed.
	if _, ok := publicKeyByID(KeyIDFor(node)); !ok {
		t.Fatal("the node key is not in the verification set")
	}
}

func TestSetNodeKeyFromFileKeepsTheSameIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "node.pem")

	resetKeys(t)
	if err := SetNodeKeyFromFile(path); err != nil {
		t.Fatal(err)
	}
	first := NodeID()

	resetKeys(t)
	if err := SetNodeKeyFromFile(path); err != nil {
		t.Fatal(err)
	}
	if NodeID() != first {
		t.Fatalf("the node came back as %q, having been %q", NodeID(), first)
	}
}

func TestConfigureNodeKeyWithoutOneIsNotAnError(t *testing.T) {
	resetKeys(t)
	if err := ConfigureNodeKey("", ""); err != nil {
		t.Fatalf("a deployment with no node identity was refused: %v", err)
	}
	if NodeID() != "" {
		t.Fatal("a node id appeared from nowhere")
	}
}
