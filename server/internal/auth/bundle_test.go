package auth

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"testing"
)

// withNodeKey installs a node key for one test and puts the process's key
// state back afterwards.
//
// Both the node identity and the keyring are process-wide, which is what they
// have to be: a server has one identity. That makes them shared state between
// tests, and a test that installs a key and leaves it there changes how every
// later test's tokens are signed - which is how installing one here first
// broke the access-token tests that expect the shared secret.
func withNodeKey(t *testing.T, priv ed25519.PrivateKey) {
	t.Helper()
	nodeIdentity.Lock()
	oldID, oldPriv := nodeIdentity.id, nodeIdentity.priv
	nodeIdentity.Unlock()
	keyring.Lock()
	oldSigner, oldIssuer := keyring.signer, keyring.issuer
	keyring.Unlock()

	t.Cleanup(func() {
		nodeIdentity.Lock()
		nodeIdentity.id, nodeIdentity.priv = oldID, oldPriv
		nodeIdentity.Unlock()
		keyring.Lock()
		keyring.signer, keyring.issuer = oldSigner, oldIssuer
		keyring.Unlock()
	})
	if err := SetNodeKey(hex.EncodeToString(priv.Seed())); err != nil {
		t.Fatalf("installing the node key: %v", err)
	}
}

func TestOnlyAnEnrolledNodesSignatureIsAccepted(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("key: %v", err)
	}
	withNodeKey(t, priv)
	nodes := NewMemoryNodeRepository()
	id := NodeIDFor(pub)
	if err := nodes.SaveNode(context.Background(), Node{
		ID: id, OwnerID: "65f0", Kind: NodeKindPhone, PublicKey: EncodeNodePublicKey(pub),
	}); err != nil {
		t.Fatalf("enrolling: %v", err)
	}

	payload := []byte(`{"match":"65f0aa"}`)
	signature, err := SignBundle(payload)
	if err != nil {
		t.Fatalf("signing: %v", err)
	}
	v := BundleVerifier{Nodes: nodes}
	if err := v.VerifyBundle(context.Background(), id, payload, signature); err != nil {
		t.Fatalf("a match handed up by an enrolled node was refused: %v", err)
	}

	// Anything else is refused: a node nobody enrolled, a bundle nobody
	// signed, a signature over different bytes, and a signature from another
	// key altogether.
	if err := v.VerifyBundle(context.Background(), "deadbeefdeadbeef", payload, signature); err == nil {
		t.Fatal("a match from an unenrolled node was accepted")
	}
	if err := v.VerifyBundle(context.Background(), id, payload, ""); err == nil {
		t.Fatal("an unsigned match was accepted")
	}
	if err := v.VerifyBundle(context.Background(), id, []byte(`{"match":"65f0bb"}`), signature); err == nil {
		t.Fatal("a signature over different content was accepted")
	}
	_, other, _ := ed25519.GenerateKey(nil)
	forged := base64.RawURLEncoding.EncodeToString(ed25519.Sign(other, payload))
	if err := v.VerifyBundle(context.Background(), id, payload, forged); err == nil {
		t.Fatal("a signature from another key was accepted")
	}
}
