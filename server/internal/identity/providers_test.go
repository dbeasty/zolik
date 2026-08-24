package identity

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

// The registry is what makes "add Apple later" a configuration change, so the
// property that matters is that an unconfigured provider is absent rather than
// present-and-broken.
func TestUnconfiguredProvidersAreNotOffered(t *testing.T) {
	reg := FromConfig(Config{
		Google: ProviderConfig{ClientID: "g", ClientSecret: "s"},
		// Apple and Microsoft deliberately left unset.
	})

	if reg.Len() != 1 {
		t.Fatalf("registry holds %d providers, want only the configured one", reg.Len())
	}
	if _, err := reg.Get("google"); err != nil {
		t.Errorf("google was configured but is not available: %v", err)
	}
	for _, id := range []string{"apple", "microsoft"} {
		if _, err := reg.Get(id); !errors.Is(err, ErrUnknownProvider) {
			t.Errorf("Get(%q) error = %v, want ErrUnknownProvider", id, err)
		}
	}
}

func TestConfiguringAProviderIsAllItTakesToEnableIt(t *testing.T) {
	// The whole extensibility claim in one assertion: Microsoft needs no new
	// code, only a client id.
	reg := FromConfig(Config{
		Google:    ProviderConfig{ClientID: "g"},
		Microsoft: ProviderConfig{ClientID: "m", Tenant: "common"},
	})
	p, err := reg.Get("microsoft")
	if err != nil {
		t.Fatalf("microsoft was configured but is not available: %v", err)
	}
	if p.DisplayName() != "Microsoft" {
		t.Errorf("display name = %q, want Microsoft", p.DisplayName())
	}

	got := reg.Descriptors()
	if len(got) != 2 || got[0].ID != "google" || got[1].ID != "microsoft" {
		t.Fatalf("descriptors = %+v, want google then microsoft in id order", got)
	}
	for _, d := range got {
		if d.Kind != "oauth" {
			t.Errorf("descriptor %s kind = %q, want oauth", d.ID, d.Kind)
		}
	}
}

func TestRegistryLookupIsCaseAndSpaceInsensitive(t *testing.T) {
	reg := FromConfig(Config{Google: ProviderConfig{ClientID: "g"}})
	if _, err := reg.Get("  GOOGLE "); err != nil {
		t.Errorf("a differently-cased provider id was not found: %v", err)
	}
}

func TestMicrosoftIssuerIsMatchedByShape(t *testing.T) {
	// With the `common` tenant the ID token's issuer names the *user's*
	// tenant, so no single literal issuer can be compared against.
	p := Microsoft(ProviderConfig{ClientID: "m"}).(*oidcProvider)

	valid := "https://login.microsoftonline.com/9188040d-6c67-4c5b-b112-36a304b66dad/v2.0"
	if !p.issuerOK(valid) {
		t.Errorf("a real per-tenant issuer was rejected: %s", valid)
	}
	for _, bad := range []string{
		"https://login.microsoftonline.example.com/tenant/v2.0",
		"https://accounts.google.com",
		"https://login.microsoftonline.com/tenant/v1.0",
	} {
		if p.issuerOK(bad) {
			t.Errorf("issuer %q was accepted but is not Microsoft's", bad)
		}
	}
}

func TestGoogleAcceptsBothIssuerSpellings(t *testing.T) {
	p := Google(ProviderConfig{ClientID: "g"}).(*oidcProvider)
	for _, iss := range []string{"https://accounts.google.com", "accounts.google.com"} {
		if !p.issuerOK(iss) {
			t.Errorf("Google issuer %q was rejected; Google documents both spellings", iss)
		}
	}
	if p.issuerOK("https://accounts.google.com.evil.example") {
		t.Error("a look-alike issuer was accepted")
	}
}

// Apple's client secret is a signed JWT rather than a stored string, and
// getting it wrong means Apple sign-in fails at the token endpoint with an
// opaque error — so it is worth proving the shape here.
func TestAppleClientSecretIsASignedES256JWT(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generating a key: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshalling the key: %v", err)
	}
	pemKey := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))

	signer := NewAppleSecretSigner("TEAM123456", "KEY1234567", "com.example.zolik", pemKey)
	secret, err := signer.Secret()
	if err != nil {
		t.Fatalf("minting the apple client secret: %v", err)
	}

	parsed, err := jwt.ParseWithClaims(secret, &jwt.RegisteredClaims{}, func(tok *jwt.Token) (any, error) {
		if tok.Method.Alg() != "ES256" {
			return nil, errors.New("apple requires ES256")
		}
		if kid, _ := tok.Header["kid"].(string); kid != "KEY1234567" {
			return nil, errors.New("the key id must be in the header for Apple to find the key")
		}
		return key.Public(), nil
	})
	if err != nil {
		t.Fatalf("the minted secret does not verify: %v", err)
	}
	claims := parsed.Claims.(*jwt.RegisteredClaims)
	if claims.Issuer != "TEAM123456" {
		t.Errorf("issuer = %q, want the team id", claims.Issuer)
	}
	if claims.Subject != "com.example.zolik" {
		t.Errorf("subject = %q, want the client id", claims.Subject)
	}
	if len(claims.Audience) != 1 || claims.Audience[0] != "https://appleid.apple.com" {
		t.Errorf("audience = %v, want Apple", claims.Audience)
	}

	// Re-minting on every call would be pure waste; the secret is reusable
	// until it nears expiry.
	again, err := signer.Secret()
	if err != nil {
		t.Fatalf("second mint: %v", err)
	}
	if again != secret {
		t.Error("a fresh secret was minted while the cached one was still valid")
	}
}

func TestAppleWithoutAKeyFailsClearly(t *testing.T) {
	signer := NewAppleSecretSigner("", "", "com.example.zolik", "")
	if _, err := signer.Secret(); err == nil {
		t.Fatal("a signer with no key material reported success")
	}
}
