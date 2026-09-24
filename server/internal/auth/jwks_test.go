package auth

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// cloudJWKSServer is a stand-in for the cloud: it publishes whatever this
// process is currently signing with, at the real path.
func cloudJWKSServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(LocalJWKS())
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestJWKSEndpointPublishesTheSigningKey(t *testing.T) {
	pub := newSigningKey(t, "https://zolik.test")
	h := NewHandlers(Deps{})

	rec := httptest.NewRecorder()
	h.jwks(rec, httptest.NewRequest(http.MethodGet, "/.well-known/jwks.json", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200", rec.Code)
	}

	var doc JWKS
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Keys) != 1 {
		t.Fatalf("published %d keys, want 1", len(doc.Keys))
	}
	k := doc.Keys[0]
	if k.Kty != "OKP" || k.Crv != "Ed25519" || k.Use != "sig" || k.Alg != "EdDSA" {
		t.Fatalf("unexpected JWK %+v", k)
	}
	if k.Kid != KeyIDFor(pub) {
		t.Errorf("kid %q, want %q", k.Kid, KeyIDFor(pub))
	}
	raw, err := base64.RawURLEncoding.DecodeString(k.X)
	if err != nil {
		t.Fatal(err)
	}
	if !ed25519.PublicKey(raw).Equal(pub) {
		t.Fatal("the published key is not the one this server signs with")
	}
}

// A deployment with no asymmetric key publishes an empty set rather than an
// error. A verifier that reads it then fails every token, which is right.
func TestJWKSEndpointIsEmptyWithoutAKey(t *testing.T) {
	resetKeys(t)
	h := NewHandlers(Deps{})

	rec := httptest.NewRecorder()
	h.jwks(rec, httptest.NewRequest(http.MethodGet, "/.well-known/jwks.json", nil))

	var doc JWKS
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Keys) != 0 {
		t.Fatalf("published %d keys on a server that signs nothing", len(doc.Keys))
	}
}

func TestJWKSCacheFetchesAndPersists(t *testing.T) {
	pub := newSigningKey(t, "https://zolik.test")
	srv := cloudJWKSServer(t)
	path := filepath.Join(t.TempDir(), "jwks.json")

	cache := NewJWKSCache(JWKSCacheConfig{URL: srv.URL, Path: path, HTTPClient: srv.Client()})
	got, ok := cache.PublicKeyByID(KeyIDFor(pub))
	if !ok {
		t.Fatal("the cache did not fetch the key")
	}
	if !got.Equal(pub) {
		t.Fatal("the cache returned a different key")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("the fetched document was not written to disk: %v", err)
	}
}

// The case the whole design is for: the node has been switched off for long
// enough that its copy is stale, there is no connection, and it still has to
// decide. It answers from disk.
func TestJWKSCacheVerifiesOfflineFromDisk(t *testing.T) {
	pub := newSigningKey(t, "https://zolik.test")
	srv := cloudJWKSServer(t)
	path := filepath.Join(t.TempDir(), "jwks.json")

	warm := NewJWKSCache(JWKSCacheConfig{URL: srv.URL, Path: path, HTTPClient: srv.Client()})
	if err := warm.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	// Old enough that the cache will try to refresh, against a cloud that is
	// no longer reachable.
	stale := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(path, stale, stale); err != nil {
		t.Fatal(err)
	}
	srv.Close()

	offline := NewJWKSCache(JWKSCacheConfig{URL: srv.URL, Path: path, HTTPClient: srv.Client()})
	got, ok := offline.PublicKeyByID(KeyIDFor(pub))
	if !ok {
		t.Fatal("an offline node could not read the keys it had cached")
	}
	if !got.Equal(pub) {
		t.Fatal("the cached key is not the one the cloud published")
	}

	// Fail closed all the same: a key it has never held is not invented
	// because the refresh that might have explained it could not be made.
	if _, ok := offline.PublicKeyByID("not-a-kid-this-node-holds"); ok {
		t.Fatal("an offline node answered for a key it has never seen")
	}
}

// A phone that has never been online since it was installed verifies against
// the copy compiled into the app.
func TestJWKSCacheFallsBackToTheBundledCopy(t *testing.T) {
	pub := newSigningKey(t, "https://zolik.test")
	bundled, err := json.Marshal(LocalJWKS())
	if err != nil {
		t.Fatal(err)
	}

	cache := NewJWKSCache(JWKSCacheConfig{
		Path:    filepath.Join(t.TempDir(), "jwks.json"),
		Bundled: bundled,
	})
	if _, ok := cache.PublicKeyByID(KeyIDFor(pub)); !ok {
		t.Fatal("the bundled keys were not used")
	}
	if _, ok := cache.PublicKeyByID("someone-elses-kid"); ok {
		t.Fatal("the bundled copy answered for a key it does not contain")
	}
}

// A document with nothing usable in it is rejected rather than cached: a
// captive portal answering instead of the cloud must not leave a node unable
// to verify anything.
func TestJWKSCacheRefusesAnEmptyDocument(t *testing.T) {
	pub := newSigningKey(t, "https://zolik.test")
	path := filepath.Join(t.TempDir(), "jwks.json")
	bundled, err := json.Marshal(LocalJWKS())
	if err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"keys":[]}`))
	}))
	t.Cleanup(srv.Close)

	cache := NewJWKSCache(JWKSCacheConfig{URL: srv.URL, Path: path, Bundled: bundled, HTTPClient: srv.Client()})
	if err := cache.Refresh(context.Background()); err == nil {
		t.Fatal("an empty key set was accepted")
	}
	if _, ok := cache.PublicKeyByID(KeyIDFor(pub)); !ok {
		t.Fatal("a failed refresh threw away the keys the node already had")
	}
	if _, err := os.Stat(path); err == nil {
		t.Fatal("an empty key set was written to disk")
	}
}
