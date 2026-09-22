package auth

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
)

// PublicKeys is where a verifier looks up the key a token names in its `kid`
// header. The cloud answers from its own keyring; a phone or a LAN server
// answers from the copy of the cloud's JWKS it keeps on disk, which is what
// lets it check a pass at a kitchen table with no internet.
//
// A lookup that comes back empty is a refusal, not a question: the caller
// fails the token rather than trying something else.
type PublicKeys interface {
	PublicKeyByID(kid string) (ed25519.PublicKey, bool)
}

// JWK is one Ed25519 public key as RFC 8037 writes it. Only the OKP shape
// appears here, because this server signs with nothing else; the RSA and EC
// forms live in internal/identity, which has to read whatever Google, Apple
// and Microsoft happen to publish.
type JWK struct {
	Kty string `json:"kty"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
}

// JWKS is the document served at /.well-known/jwks.json.
type JWKS struct {
	Keys []JWK `json:"keys"`
}

// JWKFor renders a public key for publication.
func JWKFor(pub ed25519.PublicKey) JWK {
	return JWK{
		Kty: "OKP",
		Crv: "Ed25519",
		X:   base64.RawURLEncoding.EncodeToString(pub),
		Kid: KeyIDFor(pub),
		Use: "sig",
		Alg: "EdDSA",
	}
}

// LocalJWKS is every key this process will verify a token against, which is
// exactly what it publishes. The retired keys are in it too: a node that
// fetched the document a month ago and a node that fetches it now must both
// be able to read a token signed before the last rotation.
//
// Sorted by kid so the document is byte-for-byte the same between restarts,
// which keeps an ETag or a cached copy meaningful.
func LocalJWKS() JWKS {
	keyring.RLock()
	keys := make([]JWK, 0, len(keyring.verify))
	for _, pub := range keyring.verify {
		keys = append(keys, JWKFor(pub))
	}
	keyring.RUnlock()
	sort.Slice(keys, func(i, j int) bool { return keys[i].Kid < keys[j].Kid })
	return JWKS{Keys: keys}
}

// publicKeys parses a fetched or cached document. One unreadable key does not
// spoil the set, but a document with nothing usable in it is an error: it
// almost certainly means a captive portal answered instead of the cloud, and
// caching that would leave a node unable to verify anything until it happened
// to try again.
func (s JWKS) publicKeys() (map[string]ed25519.PublicKey, error) {
	out := make(map[string]ed25519.PublicKey, len(s.Keys))
	for _, k := range s.Keys {
		if k.Kty != "OKP" || k.Crv != "Ed25519" {
			continue
		}
		if k.Use != "" && k.Use != "sig" {
			continue
		}
		if k.Alg != "" && k.Alg != "EdDSA" {
			continue
		}
		raw, err := base64.RawURLEncoding.DecodeString(k.X)
		if err != nil || len(raw) != ed25519.PublicKeySize {
			continue
		}
		pub := ed25519.PublicKey(raw)
		// The kid is derived from the key, so a document that names a key
		// something else is describing a key it does not have. Publishing
		// under the derived name regardless keeps a lookup by kid honest.
		out[KeyIDFor(pub)] = pub
		if k.Kid != "" {
			out[k.Kid] = pub
		}
	}
	if len(out) == 0 {
		return nil, errors.New("jwks contained no usable Ed25519 signing keys")
	}
	return out, nil
}

// jwks publishes the verification keys. It is unauthenticated and meant to be
// cached: a public key is public, and every node that verifies a cloud token
// offline starts by fetching this once while it still has a connection.
//
// An empty set is served rather than an error where no Ed25519 key has been
// configured. The honest answer to "which keys do you sign with" is then
// "none", and a verifier that reads it fails every token it is shown, which
// is the correct outcome and a legible one.
func (h *Handlers) jwks(w http.ResponseWriter, _ *http.Request) {
	// Five minutes is short enough that a rotation propagates within one
	// coffee break and long enough that the document is not fetched per
	// sign-in by every node in the field.
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(LocalJWKS())
}
