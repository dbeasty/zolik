package auth

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testUserID = "652f1a2b3c4d5e6f70819293"

func TestOfflinePassRoundTrip(t *testing.T) {
	newSigningKey(t, "https://zolik.test")

	pass, err := CreateOfflinePass(testUserID, "alice", 3, OfflinePassTTL)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := VerifyOfflinePass(pass, LocalKeys())
	if err != nil {
		t.Fatalf("the cloud could not verify its own pass: %v", err)
	}
	if claims.Subject != testUserID {
		t.Errorf("sub %q, want the account id", claims.Subject)
	}
	if claims.Username != "alice" {
		t.Errorf("username %q, want alice", claims.Username)
	}
	if claims.CredentialVersion != 3 {
		t.Errorf("sv %d, want 3", claims.CredentialVersion)
	}
	if claims.Issuer != "https://zolik.test" {
		t.Errorf("iss %q, want the cloud's base URL", claims.Issuer)
	}
	if got := claims.SeatSubject(); got != "user:"+testUserID {
		t.Errorf("seat subject %q, want user:<hex>", got)
	}
	if claims.ExpiresAt == nil || claims.ExpiresAt.Time.After(time.Now().Add(OfflinePassTTL+time.Minute)) {
		t.Errorf("expiry %v, want at most thirty days out", claims.ExpiresAt)
	}
}

// The whole point of the thing: a host with no connection, verifying against
// the copy of the cloud's keys it cached while it had one.
func TestOfflinePassIsVerifiedFromTheCachedJWKS(t *testing.T) {
	newSigningKey(t, "https://zolik.test")
	pass, err := CreateOfflinePass(testUserID, "alice", 1, OfflinePassTTL)
	if err != nil {
		t.Fatal(err)
	}
	bundled, err := json.Marshal(LocalJWKS())
	if err != nil {
		t.Fatal(err)
	}

	// A node that holds the keys and nothing else: no URL, so there is no
	// network for it to fall back on.
	node := NewJWKSCache(JWKSCacheConfig{Path: filepath.Join(t.TempDir(), "jwks.json"), Bundled: bundled})
	if _, err := VerifyOfflinePass(pass, node); err != nil {
		t.Fatalf("an offline host refused a genuine pass: %v", err)
	}
}

func TestOfflinePassFailsClosed(t *testing.T) {
	newSigningKey(t, "https://zolik.test")
	signer := activeSigner()

	sign := func(t *testing.T, claims OfflinePassClaims) string {
		t.Helper()
		token, err := signEdDSA(signer, claims)
		if err != nil {
			t.Fatal(err)
		}
		return token
	}
	now := time.Now().UTC()

	cases := map[string]string{
		"wrong audience": sign(t, OfflinePassClaims{
			Username: "alice",
			RegisteredClaims: jwt.RegisteredClaims{
				Subject:   testUserID,
				Audience:  jwt.ClaimStrings{AudienceAccess},
				ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
			},
		}),
		"no audience at all": sign(t, OfflinePassClaims{
			RegisteredClaims: jwt.RegisteredClaims{
				Subject:   testUserID,
				ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
			},
		}),
		"expired": sign(t, OfflinePassClaims{
			Username: "alice",
			RegisteredClaims: jwt.RegisteredClaims{
				Subject:   testUserID,
				Audience:  jwt.ClaimStrings{AudienceOffline},
				IssuedAt:  jwt.NewNumericDate(now.Add(-2 * time.Hour)),
				ExpiresAt: jwt.NewNumericDate(now.Add(-time.Hour)),
			},
		}),
		"no expiry": sign(t, OfflinePassClaims{
			Username: "alice",
			RegisteredClaims: jwt.RegisteredClaims{
				Subject:  testUserID,
				Audience: jwt.ClaimStrings{AudienceOffline},
				IssuedAt: jwt.NewNumericDate(now),
			},
		}),
		"subject is not an account": sign(t, OfflinePassClaims{
			Username: "alice",
			RegisteredClaims: jwt.RegisteredClaims{
				Subject:   "not-an-object-id",
				Audience:  jwt.ClaimStrings{AudienceOffline},
				ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
			},
		}),
	}
	for name, token := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := VerifyOfflinePass(token, LocalKeys()); err == nil {
				t.Fatalf("accepted a pass with a %s", name)
			}
		})
	}

	t.Run("signed by another cloud", func(t *testing.T) {
		_, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		pub, _ := priv.Public().(ed25519.PublicKey)
		token, err := signEdDSA(&signingKey{kid: KeyIDFor(pub), priv: priv}, OfflinePassClaims{
			Username: "mallory",
			RegisteredClaims: jwt.RegisteredClaims{
				Subject:   testUserID,
				Audience:  jwt.ClaimStrings{AudienceOffline},
				ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := VerifyOfflinePass(token, LocalKeys()); err == nil {
			t.Fatal("accepted a pass signed by a key this node does not hold")
		}
	})

	t.Run("signed with the shared secret", func(t *testing.T) {
		t.Setenv("JWT_ACCESS_SECRET", "test_secret")
		forged := jwt.NewWithClaims(jwt.SigningMethodHS256, OfflinePassClaims{
			Username: "mallory",
			RegisteredClaims: jwt.RegisteredClaims{
				Subject:   testUserID,
				Audience:  jwt.ClaimStrings{AudienceOffline},
				ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
			},
		})
		token, err := forged.SignedString([]byte("test_secret"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := VerifyOfflinePass(token, LocalKeys()); err == nil {
			t.Fatal("accepted a symmetric signature on a pass: any host that can check one could mint one")
		}
	})

	t.Run("no keys at all", func(t *testing.T) {
		pass, err := CreateOfflinePass(testUserID, "alice", 1, OfflinePassTTL)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := VerifyOfflinePass(pass, nil); err == nil {
			t.Fatal("a node with no keys verified a pass")
		}
	})
}

// A pass must not double as an access token, or an offline credential would
// open a match socket on the cloud.
func TestOfflinePassIsNotAnAccessToken(t *testing.T) {
	newSigningKey(t, "https://zolik.test")
	pass, err := CreateOfflinePass(testUserID, "alice", 1, OfflinePassTTL)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseAccessClaims(pass); err == nil {
		t.Fatal("an offline pass was accepted as an access token")
	}
	if _, err := SubjectFromToken(pass); err == nil {
		t.Fatal("a match socket would have opened on an offline pass")
	}
}

func TestOfflinePassNeedsAnAsymmetricKey(t *testing.T) {
	resetKeys(t)
	if _, err := CreateOfflinePass(testUserID, "alice", 1, OfflinePassTTL); err == nil {
		t.Fatal("a pass was issued by a deployment with no signing key")
	}
}

// The credential version is read at issue time, so a revocation shows up in
// the next pass rather than in the one already in somebody's pocket.
func TestOfflinePassCarriesTheCredentialVersion(t *testing.T) {
	newSigningKey(t, "https://zolik.test")
	versions := NewMemoryCredentialVersions()
	h := NewHandlers(Deps{Credentials: versions})
	ctx := context.Background()

	first := h.offlinePassFor(ctx, testUserID, "alice")
	if first == "" {
		t.Fatal("no pass was issued")
	}
	claims, err := VerifyOfflinePass(first, LocalKeys())
	if err != nil {
		t.Fatal(err)
	}
	if claims.CredentialVersion != 1 {
		t.Fatalf("sv %d, want 1 for an account nobody has revoked", claims.CredentialVersion)
	}

	versions.Bump(testUserID)
	next, err := VerifyOfflinePass(h.offlinePassFor(ctx, testUserID, "alice"), LocalKeys())
	if err != nil {
		t.Fatal(err)
	}
	if next.CredentialVersion != 2 {
		t.Fatalf("sv %d after a revocation, want 2", next.CredentialVersion)
	}
	if claims.CredentialVersion == next.CredentialVersion {
		t.Fatal("a revocation left the pass unchanged")
	}
}

// A guest id is not an account id, and a pass must not be issued for one.
func TestOfflinePassRefusesAGuestSubject(t *testing.T) {
	newSigningKey(t, "https://zolik.test")
	guestID, err := NewRandomToken(16)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateOfflinePass(guestID, "Some Guest", 1, OfflinePassTTL); err == nil {
		t.Fatalf("issued a pass for the guest id %q", guestID)
	}
}
