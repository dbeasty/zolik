package identity

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"sync"
	"time"
)

// keySet is a cached, self-refreshing view of one provider's JWKS endpoint.
//
// Every OIDC provider signs its ID tokens with keys it rotates on its own
// schedule and publishes at a well-known URL. Caching is not an optimisation
// here so much as a requirement: fetching the key set on every sign-in would
// put a third-party HTTP call on the critical path of every login and would
// get us rate-limited. Refreshing on an unseen `kid` is the other half — a
// provider that rotates a key mid-TTL must not lock every user out until the
// cache expires.
type keySet struct {
	url    string
	client *http.Client
	ttl    time.Duration

	mu sync.Mutex
	// keys is the parsed cache, by key id.
	keys map[string]any
	// fetchedAt drives normal expiry, lastMiss rate-limits the refresh that an
	// unknown kid triggers. Without that limit an attacker could force one
	// upstream fetch per forged token simply by inventing key ids.
	fetchedAt time.Time
	lastMiss  time.Time
}

func newKeySet(url string, client *http.Client, ttl time.Duration) *keySet {
	if ttl <= 0 {
		ttl = time.Hour
	}
	return &keySet{url: url, client: client, ttl: ttl, keys: map[string]any{}}
}

// key returns the public key for a key id, fetching or refreshing as needed.
func (k *keySet) key(ctx context.Context, kid string) (any, error) {
	k.mu.Lock()
	fresh := time.Since(k.fetchedAt) < k.ttl
	key, ok := k.keys[kid]
	if ok && fresh {
		k.mu.Unlock()
		return key, nil
	}
	// An unknown kid on an otherwise fresh cache means either a rotation or a
	// forged token. Allow one refresh per minute to cover the first without
	// letting the second turn into an outbound request amplifier.
	if fresh && time.Since(k.lastMiss) < time.Minute {
		k.mu.Unlock()
		if ok {
			return key, nil
		}
		return nil, fmt.Errorf("unknown signing key %q", kid)
	}
	k.lastMiss = time.Now()
	k.mu.Unlock()

	keys, err := fetchJWKS(ctx, k.client, k.url)
	if err != nil {
		// A refresh failure must not throw away a cache that still works: a
		// provider's JWKS endpoint being briefly unreachable should not sign
		// everyone out.
		k.mu.Lock()
		cached, hit := k.keys[kid]
		k.mu.Unlock()
		if hit {
			return cached, nil
		}
		return nil, err
	}

	k.mu.Lock()
	k.keys = keys
	k.fetchedAt = time.Now()
	key, ok = k.keys[kid]
	k.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("unknown signing key %q", kid)
	}
	return key, nil
}

// jwk is the subset of a JSON Web Key this server can use: RSA and P-256/384/521
// EC public keys, which between them cover every provider we target (Google and
// Microsoft sign RS256, Apple signs RS256 for ID tokens and needs ES256 only for
// the client secret we mint ourselves).
type jwk struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

func fetchJWKS(ctx context.Context, client *http.Client, url string) (map[string]any, error) {
	var doc struct {
		Keys []jwk `json:"keys"`
	}
	if err := getJSON(ctx, client, url, &doc); err != nil {
		return nil, fmt.Errorf("fetch jwks %s: %w", url, err)
	}
	out := map[string]any{}
	for _, k := range doc.Keys {
		if k.Use != "" && k.Use != "sig" {
			continue
		}
		pub, err := k.publicKey()
		if err != nil {
			// One malformed or unsupported key must not invalidate the rest of
			// the set — providers publish keys for algorithms we never use.
			continue
		}
		out[k.Kid] = pub
	}
	if len(out) == 0 {
		return nil, errors.New("jwks contained no usable signing keys")
	}
	return out, nil
}

func (k jwk) publicKey() (any, error) {
	switch k.Kty {
	case "RSA":
		n, err := b64uBigInt(k.N)
		if err != nil {
			return nil, err
		}
		eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
		if err != nil {
			return nil, err
		}
		if len(eBytes) == 0 || len(eBytes) > 4 {
			return nil, errors.New("bad rsa exponent")
		}
		padded := make([]byte, 4)
		copy(padded[4-len(eBytes):], eBytes)
		return &rsa.PublicKey{N: n, E: int(binary.BigEndian.Uint32(padded))}, nil
	case "EC":
		var curve elliptic.Curve
		switch k.Crv {
		case "P-256":
			curve = elliptic.P256()
		case "P-384":
			curve = elliptic.P384()
		case "P-521":
			curve = elliptic.P521()
		default:
			return nil, fmt.Errorf("unsupported curve %q", k.Crv)
		}
		x, err := b64uBigInt(k.X)
		if err != nil {
			return nil, err
		}
		y, err := b64uBigInt(k.Y)
		if err != nil {
			return nil, err
		}
		return &ecdsa.PublicKey{Curve: curve, X: x, Y: y}, nil
	default:
		return nil, fmt.Errorf("unsupported key type %q", k.Kty)
	}
}

func b64uBigInt(s string) (*big.Int, error) {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}
	if len(b) == 0 {
		return nil, errors.New("empty key parameter")
	}
	return new(big.Int).SetBytes(b), nil
}

func getJSON(ctx context.Context, client *http.Client, url string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return getJSONFromResponse(resp, out)
}

// getJSONFromResponse decodes a response body that the caller has already
// decided to read regardless of status — token endpoints put their most useful
// diagnostics in the body of a 400.
func getJSONFromResponse(resp *http.Response, out any) error {
	return json.NewDecoder(resp.Body).Decode(out)
}
