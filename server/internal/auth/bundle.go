package auth

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"fmt"
)

// A finished match handed up by a node carries that node's signature over it.
//
// The signature answers a narrower question than it might look like. It does
// not say the match was played fairly - the cloud settles that by replaying
// the moves - and it does not say who sat in which seat. It says which
// enrolled install handed this up, which is what lets the cloud credit an
// account only when the node that attested the seat is one that account
// enrolled, and what lets a bundle from an install nobody has ever enrolled be
// refused before any of it is believed.

// SignBundle signs a handed-up match with this node's key.
func SignBundle(payload []byte) (string, error) {
	nodeIdentity.RLock()
	priv := nodeIdentity.priv
	nodeIdentity.RUnlock()
	if priv == nil {
		return "", ErrNoNodeKey
	}
	return base64.RawURLEncoding.EncodeToString(ed25519.Sign(priv, payload)), nil
}

// NodeSigner is this process signing as itself, for internal/sync's outbox.
type NodeSigner struct{}

func (NodeSigner) SignBundle(payload []byte) (string, error) { return SignBundle(payload) }

// BundleVerifier checks a handed-up match against the key of the node that
// claims to have hosted it.
type BundleVerifier struct {
	Nodes NodeRepository
}

// VerifyBundle refuses anything it cannot attribute to an enrolled node.
//
// An unsigned bundle is refused outright rather than accepted as anonymous.
// The namespace it arrived in already proves which node pushed it, but not
// that the node that pushed it is the one that played the match: a device can
// push its own outbox, and without a signature it could fill that outbox with
// matches it invented on behalf of an install it does not own.
func (v BundleVerifier) VerifyBundle(ctx context.Context, nodeID string, payload []byte, signature string) error {
	if v.Nodes == nil {
		return errors.New("bundle: this server keeps no record of enrolled nodes")
	}
	if signature == "" {
		return fmt.Errorf("bundle: node %s handed up an unsigned match", nodeID)
	}
	n, err := v.Nodes.FindNode(ctx, nodeID)
	if err != nil {
		return fmt.Errorf("bundle: node %s: %w", nodeID, err)
	}
	pub, err := parseEd25519Public(n.PublicKey)
	if err != nil {
		return fmt.Errorf("bundle: node %s has an unreadable key: %w", nodeID, err)
	}
	raw, err := base64.RawURLEncoding.DecodeString(signature)
	if err != nil {
		return fmt.Errorf("bundle: node %s: unreadable signature: %w", nodeID, err)
	}
	if !ed25519.Verify(pub, payload, raw) {
		return fmt.Errorf("bundle: node %s: the signature does not match what was handed up", nodeID)
	}
	return nil
}
