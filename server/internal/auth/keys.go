package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// The audiences this deployment signs for. A token says what it is for, and
// every verifier in this package insists on the one it wants, so an offline
// pass cannot be presented as an access token, a node's credential cannot
// open a match socket, and a seat receipt is good for nothing but proving
// which offline seats were whose.
const (
	// AudienceAccess is an ordinary access token, the thing a Bearer header
	// carries.
	AudienceAccess = "zolik"
	// AudienceOffline is the 30 day pass a signed-in player takes with them
	// to a host that has no internet. See offlinepass.go.
	AudienceOffline = "zolik-offline"
	// AudienceNode is a node's own credential, which it presents when it
	// syncs its replica rather than when it acts for a player.
	AudienceNode = "zolik-node"
	// AudienceSeat is a seat receipt, signed by the node that seated the
	// guest rather than by the cloud. See node.go.
	AudienceSeat = "zolik-seat"
)

// signingKey is one Ed25519 key together with the `kid` that names it in a
// token header. The name is derived from the public half rather than assigned
// by whoever generated the key, so the same key is called the same thing on
// every node that ever meets it, and two installs that happen to be
// configured in different orders cannot end up disagreeing about which key a
// token means.
type signingKey struct {
	kid  string
	priv ed25519.PrivateKey
}

// keyring is what this process signs with and what it is willing to believe.
//
// It is process-wide for the same reason the HS256 secret above it is: the
// functions that mint and read tokens are called from the match socket, the
// lobby, the admin guard and the notification endpoints, none of which has
// any business being handed a key. A deployment configures it once at startup
// and nothing changes it afterwards.
//
// The signer is separate from the verification set because they are not the
// same question. This server signs with exactly one key; it believes that key
// and every key it has retired but not yet forgotten, which is the whole of
// what makes a rotation something other than signing everybody out at once.
var keyring struct {
	sync.RWMutex
	signer *signingKey
	verify map[string]ed25519.PublicKey
	issuer string
	// legacyUntil is when this server stops believing a token signed with the
	// shared HS256 secret. The zero time means the migration window has no end
	// yet, which is where every existing deployment starts: every token in the
	// wild today is HS256, and refusing them the moment an Ed25519 key appears
	// would end every session that is currently being played.
	legacyUntil time.Time
}

// KeyConfig is how a deployment tells this process which key it signs with,
// which keys it has retired, and how long it still honours the tokens it
// signed with the shared secret before any of this existed.
type KeyConfig struct {
	// Seed is the private key itself, as JWT_SIGNING_KEY carries it: a PKCS#8
	// PEM block, or the raw 32 byte seed in hex or base64. Empty means the key
	// comes from SeedFile instead.
	Seed string
	// SeedFile is where an install that was given no key keeps the one it
	// generated for itself. It is written once, with the same atomic replace
	// the mobile host writes host.json with, and read on every later start: a
	// node that minted a fresh key on every boot would repudiate every token
	// it had ever issued.
	SeedFile string
	// Issuer is this server's public base URL, and becomes the `iss` claim.
	// A local node that verifies cloud tokens offline has no way to ask who
	// signed something, so the token has to say.
	Issuer string
	// Retired are public keys this server no longer signs with but still
	// verifies and still publishes, hex or base64. A key moves here for at
	// least as long as the longest token it ever signed can live, which for
	// an offline pass is 30 days.
	Retired []string
	// AcceptLegacyHS256Until closes the migration window. Until it passes (or
	// forever, while it is zero) a token signed with the shared secret is
	// still accepted, which is what keeps the sessions issued before this
	// change alive. After it, only Ed25519 will do.
	AcceptLegacyHS256Until time.Time
}

// ConfigureSigningKeys installs the key this process signs with. It is
// called once, at startup, before anything serves.
//
// A deployment with neither a key nor a file to keep one in is refused rather
// than quietly left signing HS256: silently doing the old thing is how a
// migration ends up never happening.
func ConfigureSigningKeys(cfg KeyConfig) error {
	var (
		priv ed25519.PrivateKey
		err  error
	)
	switch {
	case strings.TrimSpace(cfg.Seed) != "":
		priv, err = parseEd25519Private(cfg.Seed)
	case strings.TrimSpace(cfg.SeedFile) != "":
		priv, err = loadOrCreateSigningKey(strings.TrimSpace(cfg.SeedFile))
	default:
		return errors.New("signing key: set JWT_SIGNING_KEY, or JWT_SIGNING_KEY_FILE for a key this install keeps for itself")
	}
	if err != nil {
		return fmt.Errorf("signing key: %w", err)
	}

	retired := make(map[string]ed25519.PublicKey, len(cfg.Retired))
	for _, raw := range cfg.Retired {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		pub, perr := parseEd25519Public(raw)
		if perr != nil {
			return fmt.Errorf("retired signing key: %w", perr)
		}
		retired[KeyIDFor(pub)] = pub
	}

	pub, _ := priv.Public().(ed25519.PublicKey)
	keyring.Lock()
	defer keyring.Unlock()
	if keyring.verify == nil {
		keyring.verify = map[string]ed25519.PublicKey{}
	}
	keyring.signer = &signingKey{kid: KeyIDFor(pub), priv: priv}
	keyring.verify[keyring.signer.kid] = pub
	for kid, p := range retired {
		keyring.verify[kid] = p
	}
	keyring.issuer = strings.TrimRight(strings.TrimSpace(cfg.Issuer), "/")
	keyring.legacyUntil = cfg.AcceptLegacyHS256Until
	return nil
}

// KeyConfigFromEnv reads the signing configuration a server deployment sets
// in its environment. The public base URL is passed in rather than read here,
// because Config has already worked out what it is.
func KeyConfigFromEnv(publicBaseURL string) (KeyConfig, error) {
	cfg := KeyConfig{
		Seed:     strings.TrimSpace(os.Getenv("JWT_SIGNING_KEY")),
		SeedFile: strings.TrimSpace(os.Getenv("JWT_SIGNING_KEY_FILE")),
		Issuer:   publicBaseURL,
	}
	for _, raw := range strings.Split(os.Getenv("JWT_SIGNING_KEY_RETIRED"), ",") {
		if s := strings.TrimSpace(raw); s != "" {
			cfg.Retired = append(cfg.Retired, s)
		}
	}
	if raw := strings.TrimSpace(os.Getenv("JWT_LEGACY_HS256_UNTIL")); raw != "" {
		until, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return KeyConfig{}, fmt.Errorf("JWT_LEGACY_HS256_UNTIL is not an RFC3339 instant: %w", err)
		}
		cfg.AcceptLegacyHS256Until = until
	}
	return cfg, nil
}

// ConfigureSigningKeysFromEnv is the one call a server deployment makes at
// startup. A deployment that has set neither variable is left signing HS256
// and is not an error: the migration has to be able to happen one deployment
// at a time, and the sibling check on the shared secret (CheckAccessSecret)
// is what stops that state from being an insecure one.
//
// It reports whether an asymmetric key was installed, so a caller can say so
// in its startup log rather than leaving an operator to guess which half of
// the migration their server is on.
func ConfigureSigningKeysFromEnv(publicBaseURL string) (bool, error) {
	cfg, err := KeyConfigFromEnv(publicBaseURL)
	if err != nil {
		return false, err
	}
	if cfg.Seed == "" && cfg.SeedFile == "" {
		return false, nil
	}
	if err := ConfigureSigningKeys(cfg); err != nil {
		return false, err
	}
	return true, nil
}

// KeyIDFor is the `kid` a public key is known by everywhere: the first half
// of its SHA-256 digest, base64url. Truncated because a kid travels in the
// header of every token and its only job is to pick one key out of the two or
// three a server holds, and derived because a name computed from the key
// itself cannot be attached to the wrong one.
func KeyIDFor(pub ed25519.PublicKey) string {
	sum := sha256.Sum256(pub)
	return base64.RawURLEncoding.EncodeToString(sum[:16])
}

// activeSigner is the key to sign with, or nil where nothing has been
// configured and tokens are still HS256.
func activeSigner() *signingKey {
	keyring.RLock()
	defer keyring.RUnlock()
	return keyring.signer
}

// publicKeyByID is the verification side, and answers only for keys this
// process has been told about. An unknown kid gets no key and the token that
// named it fails, which is the whole point of naming it.
func publicKeyByID(kid string) (ed25519.PublicKey, bool) {
	keyring.RLock()
	defer keyring.RUnlock()
	pub, ok := keyring.verify[kid]
	return pub, ok
}

// trustPublicKey adds a key to the verification set without making it the
// signer. The node key goes in this way on a server that already has a cloud
// signing key: it must be able to read what its own node signed without
// signing anything with it.
func trustPublicKey(pub ed25519.PublicKey) {
	keyring.Lock()
	defer keyring.Unlock()
	if keyring.verify == nil {
		keyring.verify = map[string]ed25519.PublicKey{}
	}
	keyring.verify[KeyIDFor(pub)] = pub
}

// tokenIssuer is the `iss` every token this process signs carries.
func tokenIssuer() string {
	keyring.RLock()
	defer keyring.RUnlock()
	return keyring.issuer
}

// legacyHS256Accepted reports whether a token signed with the shared secret is
// still worth reading. Closed windows fail closed: once the deadline passes
// there is no configuration and no header that reopens it.
func legacyHS256Accepted(now time.Time) bool {
	keyring.RLock()
	defer keyring.RUnlock()
	return keyring.legacyUntil.IsZero() || now.Before(keyring.legacyUntil)
}

// localKeys is the PublicKeys view of this process's own keyring, for a
// server verifying what it signed itself. A node that verifies the cloud's
// tokens uses a JWKSCache instead.
type localKeys struct{}

// LocalKeys are the verification keys this process holds.
func LocalKeys() PublicKeys { return localKeys{} }

func (localKeys) PublicKeyByID(kid string) (ed25519.PublicKey, bool) {
	return publicKeyByID(kid)
}

// loadOrCreateSigningKey reads the install's key, generating and persisting
// one the first time. The file is PKCS#8 PEM at 0600, written to a temporary
// name and renamed into place, so a crash halfway through leaves the previous
// key rather than a truncated one that nothing can read.
func loadOrCreateSigningKey(path string) (ed25519.PrivateKey, error) {
	if raw, err := os.ReadFile(path); err == nil {
		key, perr := parseEd25519Private(string(raw))
		if perr != nil {
			// Refused rather than replaced. A key file that cannot be parsed
			// is far more likely to be a mangled deployment than a lost one,
			// and generating a fresh key over the top of it would invalidate
			// every token the old one signed.
			return nil, fmt.Errorf("read %s: %w", path, perr)
		}
		return key, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, err
	}
	block := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, block, 0o600); err != nil {
		return nil, fmt.Errorf("write %s: %w", path, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return nil, fmt.Errorf("write %s: %w", path, err)
	}
	return priv, nil
}

// parseEd25519Private accepts the shapes a key arrives in: a PKCS#8 PEM block
// from a key file or a secret manager, and the bare 32 byte seed in hex or
// base64 for an environment variable somebody has to paste.
func parseEd25519Private(s string) (ed25519.PrivateKey, error) {
	s = strings.TrimSpace(s)
	if strings.Contains(s, "-----BEGIN") {
		block, _ := pem.Decode([]byte(s))
		if block == nil {
			return nil, errors.New("no PEM block")
		}
		parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		key, ok := parsed.(ed25519.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("want an Ed25519 key, got %T", parsed)
		}
		return key, nil
	}
	raw, err := decodeKeyBytes(s)
	if err != nil {
		return nil, err
	}
	switch len(raw) {
	case ed25519.SeedSize:
		return ed25519.NewKeyFromSeed(raw), nil
	case ed25519.PrivateKeySize:
		return ed25519.PrivateKey(raw), nil
	default:
		return nil, fmt.Errorf("want %d or %d key bytes, got %d", ed25519.SeedSize, ed25519.PrivateKeySize, len(raw))
	}
}

// parseEd25519Public accepts a raw 32 byte public key, hex or base64, which
// is how a retired key and an enrolling node's key both arrive.
func parseEd25519Public(s string) (ed25519.PublicKey, error) {
	raw, err := decodeKeyBytes(strings.TrimSpace(s))
	if err != nil {
		return nil, err
	}
	if len(raw) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("want %d key bytes, got %d", ed25519.PublicKeySize, len(raw))
	}
	return ed25519.PublicKey(raw), nil
}

// decodeKeyBytes reads hex or any of the four base64 alphabets. Which one a
// key is written in depends on what produced it, and refusing three of them
// buys nothing but a confusing failure at somebody's first deploy.
func decodeKeyBytes(s string) ([]byte, error) {
	if raw, err := hex.DecodeString(s); err == nil && len(raw) > 0 {
		return raw, nil
	}
	for _, enc := range []*base64.Encoding{
		base64.RawURLEncoding,
		base64.URLEncoding,
		base64.RawStdEncoding,
		base64.StdEncoding,
	} {
		if raw, err := enc.DecodeString(s); err == nil && len(raw) > 0 {
			return raw, nil
		}
	}
	return nil, errors.New("not hex or base64")
}
