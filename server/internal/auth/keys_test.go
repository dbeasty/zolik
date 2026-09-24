package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// resetKeys puts the process keyring and node identity back the way the test
// found them. They are package-level on purpose (see keyring), which means a
// test that installs a key has to clean up after itself or the next test is
// reading a different server's tokens.
func resetKeys(t *testing.T) {
	t.Helper()
	keyring.Lock()
	signer, verify, issuer, until := keyring.signer, keyring.verify, keyring.issuer, keyring.legacyUntil
	keyring.Unlock()
	nodeIdentity.RLock()
	nodeID, nodePriv := nodeIdentity.id, nodeIdentity.priv
	nodeIdentity.RUnlock()

	t.Cleanup(func() {
		keyring.Lock()
		keyring.signer, keyring.verify, keyring.issuer, keyring.legacyUntil = signer, verify, issuer, until
		keyring.Unlock()
		nodeIdentity.Lock()
		nodeIdentity.id, nodeIdentity.priv = nodeID, nodePriv
		nodeIdentity.Unlock()
	})

	keyring.Lock()
	keyring.signer, keyring.verify, keyring.issuer, keyring.legacyUntil = nil, map[string]ed25519.PublicKey{}, "", time.Time{}
	keyring.Unlock()
	nodeIdentity.Lock()
	nodeIdentity.id, nodeIdentity.priv = "", nil
	nodeIdentity.Unlock()
}

// newSigningKey installs a fresh Ed25519 signing key for one test and returns
// its public half.
func newSigningKey(t *testing.T, issuer string) ed25519.PublicKey {
	t.Helper()
	resetKeys(t)
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := ConfigureSigningKeys(KeyConfig{
		Seed:   hex.EncodeToString(priv.Seed()),
		Issuer: issuer,
	}); err != nil {
		t.Fatalf("configuring signing keys: %v", err)
	}
	return pub
}

func TestConfigureSigningKeysAcceptsEveryShapeOfKey(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	pub, _ := priv.Public().(ed25519.PublicKey)

	for name, seed := range map[string]string{
		"hex seed":       hex.EncodeToString(priv.Seed()),
		"base64 seed":    base64.StdEncoding.EncodeToString(priv.Seed()),
		"base64url seed": base64.RawURLEncoding.EncodeToString(priv.Seed()),
		"full key":       hex.EncodeToString(priv),
	} {
		t.Run(name, func(t *testing.T) {
			resetKeys(t)
			if err := ConfigureSigningKeys(KeyConfig{Seed: seed, Issuer: "https://zolik.test"}); err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			signer := activeSigner()
			if signer == nil {
				t.Fatal("no signer installed")
			}
			if signer.kid != KeyIDFor(pub) {
				t.Fatalf("kid %q, want %q: the same key was given two names", signer.kid, KeyIDFor(pub))
			}
		})
	}
}

func TestConfigureSigningKeysRefusesADeploymentWithNoKey(t *testing.T) {
	resetKeys(t)
	if err := ConfigureSigningKeys(KeyConfig{Issuer: "https://zolik.test"}); err == nil {
		t.Fatal("accepted a configuration with neither a key nor a file")
	}
}

// A generated key is written once and read back for ever after. A node that
// minted a new one on every boot would repudiate every token it had issued.
func TestSigningKeyFileIsGeneratedOnceAndKept(t *testing.T) {
	path := filepath.Join(t.TempDir(), "signing.pem")

	resetKeys(t)
	if err := ConfigureSigningKeys(KeyConfig{SeedFile: path, Issuer: "https://zolik.test"}); err != nil {
		t.Fatal(err)
	}
	first := activeSigner().kid

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("the key was not persisted: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("key file mode %v, want 0600", mode)
	}

	resetKeys(t)
	if err := ConfigureSigningKeys(KeyConfig{SeedFile: path, Issuer: "https://zolik.test"}); err != nil {
		t.Fatal(err)
	}
	if second := activeSigner().kid; second != first {
		t.Fatalf("second start signed as %q, first as %q", second, first)
	}
}

func TestSigningKeyFileIsNotSilentlyReplacedWhenUnreadable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "signing.pem")
	if err := os.WriteFile(path, []byte("this is not a key"), 0o600); err != nil {
		t.Fatal(err)
	}
	resetKeys(t)
	if err := ConfigureSigningKeys(KeyConfig{SeedFile: path, Issuer: "https://zolik.test"}); err == nil {
		t.Fatal("a mangled key file was replaced rather than refused")
	}
}

// An access token now names its key, carries the issuer a node needs to know
// who signed it, and says what it is for.
func TestAccessTokenIsSignedWithTheAsymmetricKey(t *testing.T) {
	pub := newSigningKey(t, "https://zolik.test")

	token, err := CreateAccessToken("abc123", "alice", false, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	parsed, _, err := jwt.NewParser().ParseUnverified(token, &AccessClaims{})
	if err != nil {
		t.Fatal(err)
	}
	if alg := parsed.Method.Alg(); alg != "EdDSA" {
		t.Fatalf("signed with %q, want EdDSA", alg)
	}
	if kid, _ := parsed.Header["kid"].(string); kid != KeyIDFor(pub) {
		t.Fatalf("kid %q, want %q", kid, KeyIDFor(pub))
	}
	claims, _ := parsed.Claims.(*AccessClaims)
	if claims.Issuer != "https://zolik.test" {
		t.Errorf("iss %q, want the public base URL", claims.Issuer)
	}
	if len(claims.Audience) != 1 || claims.Audience[0] != AudienceAccess {
		t.Errorf("aud %v, want [%s]", claims.Audience, AudienceAccess)
	}

	subject, err := SubjectFromToken(token)
	if err != nil {
		t.Fatalf("this server could not read its own token: %v", err)
	}
	if subject != "abc123" {
		t.Fatalf("subject %q, want abc123", subject)
	}
}

// A token signed by a key this server has never heard of is refused, and no
// amount of looking at the header changes that.
func TestAccessTokenFromAnUnknownKeyIsRefused(t *testing.T) {
	newSigningKey(t, "https://zolik.test")

	_, strangerPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	strangerPub, _ := strangerPriv.Public().(ed25519.PublicKey)
	forged, err := signEdDSA(&signingKey{kid: KeyIDFor(strangerPub), priv: strangerPriv}, AccessClaims{
		Username: "mallory",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "abc123",
			Audience:  jwt.ClaimStrings{AudienceAccess},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseAccessClaims(forged); err == nil {
		t.Fatal("accepted a token signed by an unknown key")
	}

	// The same token, with the kid of a key this server does hold, must still
	// fail: the name in the header is a lookup, not a claim to be believed.
	relabelled, err := signEdDSA(&signingKey{kid: activeSigner().kid, priv: strangerPriv}, AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "abc123",
			Audience:  jwt.ClaimStrings{AudienceAccess},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseAccessClaims(relabelled); err == nil {
		t.Fatal("accepted a forged token wearing a known key's name")
	}
}

// Rotation: the retired key still verifies and is still published, so the
// tokens it signed keep working until they expire.
func TestRetiredKeysStillVerify(t *testing.T) {
	oldPub := newSigningKey(t, "https://zolik.test")
	issuedBefore, err := CreateAccessToken("abc123", "alice", false, time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	_, newPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := ConfigureSigningKeys(KeyConfig{
		Seed:    hex.EncodeToString(newPriv.Seed()),
		Retired: []string{base64.RawURLEncoding.EncodeToString(oldPub)},
		Issuer:  "https://zolik.test",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseAccessClaims(issuedBefore); err != nil {
		t.Fatalf("a token signed before the rotation stopped working: %v", err)
	}
	var published []string
	for _, k := range LocalJWKS().Keys {
		published = append(published, k.Kid)
	}
	if len(published) != 2 {
		t.Fatalf("published %v, want both the current and the retired key", published)
	}
}

// The migration window: tokens signed with the shared secret keep working
// while it is open, and stop the moment it closes.
func TestLegacyHS256IsAcceptedOnlyWhileTheWindowIsOpen(t *testing.T) {
	t.Setenv("JWT_ACCESS_SECRET", "test_secret")
	newSigningKey(t, "https://zolik.test")

	legacy := jwt.NewWithClaims(jwt.SigningMethodHS256, AccessClaims{
		Username: "alice",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "abc123",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	})
	token, err := legacy.SignedString([]byte("test_secret"))
	if err != nil {
		t.Fatal(err)
	}

	claims, err := ParseAccessClaims(token)
	if err != nil {
		t.Fatalf("a token from before the rotation was refused while the window was open: %v", err)
	}
	if claims.Subject != "abc123" {
		t.Fatalf("subject %q, want abc123", claims.Subject)
	}

	keyring.Lock()
	keyring.legacyUntil = time.Now().Add(-time.Second)
	keyring.Unlock()
	if _, err := ParseAccessClaims(token); err == nil {
		t.Fatal("a shared-secret token was accepted after the window closed")
	}
}

func TestKeyConfigFromEnv(t *testing.T) {
	t.Setenv("JWT_SIGNING_KEY", "seed")
	t.Setenv("JWT_SIGNING_KEY_FILE", "/tmp/key.pem")
	t.Setenv("JWT_SIGNING_KEY_RETIRED", " one , two ")
	t.Setenv("JWT_LEGACY_HS256_UNTIL", "2027-01-01T00:00:00Z")

	cfg, err := KeyConfigFromEnv("https://zolik.test/")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Seed != "seed" || cfg.SeedFile != "/tmp/key.pem" || cfg.Issuer != "https://zolik.test/" {
		t.Fatalf("unexpected configuration: %+v", cfg)
	}
	if len(cfg.Retired) != 2 || cfg.Retired[0] != "one" || cfg.Retired[1] != "two" {
		t.Fatalf("retired keys %v, want the two trimmed entries", cfg.Retired)
	}
	if cfg.AcceptLegacyHS256Until.IsZero() {
		t.Fatal("the deadline was not read")
	}

	t.Setenv("JWT_LEGACY_HS256_UNTIL", "next tuesday")
	if _, err := KeyConfigFromEnv("https://zolik.test"); err == nil {
		t.Fatal("accepted a deadline that is not an instant")
	}
}

// A deployment that has configured no asymmetric key is left exactly as it
// was, signing with the shared secret, rather than refused.
func TestConfigureSigningKeysFromEnv(t *testing.T) {
	t.Setenv("JWT_SIGNING_KEY", "")
	t.Setenv("JWT_SIGNING_KEY_FILE", "")
	resetKeys(t)

	installed, err := ConfigureSigningKeysFromEnv("https://zolik.test")
	if err != nil {
		t.Fatalf("a deployment with no key was refused: %v", err)
	}
	if installed {
		t.Fatal("a key was installed out of nowhere")
	}
	if activeSigner() != nil {
		t.Fatal("something is signing")
	}

	t.Setenv("JWT_SIGNING_KEY_FILE", filepath.Join(t.TempDir(), "signing.pem"))
	installed, err = ConfigureSigningKeysFromEnv("https://zolik.test")
	if err != nil || !installed {
		t.Fatalf("installed=%v err=%v", installed, err)
	}
	if activeSigner() == nil {
		t.Fatal("no signer after a key file was configured")
	}
}
