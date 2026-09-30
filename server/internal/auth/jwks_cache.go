package auth

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
)

// JWKSCache is how a local node verifies what the cloud signed.
//
// A phone hosting a table in a cottage with no signal still has to decide
// whether the offline pass somebody is presenting is genuine, and the only
// thing that can answer is a copy of the cloud's public keys it already has.
// So the document is kept on disk, beside the rest of what the install owns,
// and read back at startup. The network is where a fresher copy comes from,
// never where the answer comes from.
//
// The consequences of that are deliberate. A cached key is used however stale
// it is, because the alternative is a node that stops working the moment it
// loses its connection, which is the one situation it exists for. Expiry
// schedules a refresh; it never forgets a key. What a node will not do is
// invent one: an unknown kid is a refusal, and it stays a refusal if the
// refresh that might have explained it cannot be made.
type JWKSCache struct {
	url     string
	path    string
	ttl     time.Duration
	client  *http.Client
	bundled []byte

	mu   sync.Mutex
	keys map[string]ed25519.PublicKey
	// fetchedAt drives the ordinary refresh and lastMiss rate-limits the one
	// an unknown kid triggers. Without that limit anyone who can reach this
	// node could turn a stream of forged tokens into a stream of outbound
	// requests to the cloud.
	fetchedAt time.Time
	lastMiss  time.Time
}

// JWKSCacheConfig describes where the keys come from and where the copy of
// them lives.
type JWKSCacheConfig struct {
	// URL is the cloud's /.well-known/jwks.json. Empty means this node never
	// refreshes and lives on the disk copy and the bundled fallback alone,
	// which is a legitimate configuration for a host that is offline by
	// design.
	URL string
	// Path is the file the fetched document is kept in. It sits beside
	// host.json, so a player who deletes the app's data loses it along with
	// everything else the install owned rather than keeping a stale copy that
	// nothing else agrees with.
	Path string
	// Bundled is a JWKS compiled into the app, used only when there is no
	// disk copy at all. It is what makes the very first offline verification
	// possible on a phone that has never been online since it was installed.
	// A disk copy, once there is one, wins outright: it is the fresher of the
	// two, and a bundled key that the cloud has since retired must not
	// outlive the app update that would have removed it.
	Bundled []byte
	// TTL is how long a fetched document is considered current. An hour
	// matches internal/identity's key cache, and for the same reason.
	TTL time.Duration
	// HTTPClient overrides the client used to fetch, for tests.
	HTTPClient *http.Client
}

// NewJWKSCache builds the cache and loads whatever this node already has. It
// does not reach the network: a constructor that blocked on a fetch would put
// the cloud on the critical path of an offline host starting up.
func NewJWKSCache(cfg JWKSCacheConfig) *JWKSCache {
	c := &JWKSCache{
		url:     cfg.URL,
		path:    cfg.Path,
		ttl:     cfg.TTL,
		client:  cfg.HTTPClient,
		bundled: cfg.Bundled,
		keys:    map[string]ed25519.PublicKey{},
	}
	if c.ttl <= 0 {
		c.ttl = time.Hour
	}
	if c.client == nil {
		c.client = &http.Client{Timeout: 10 * time.Second}
	}
	if raw, err := os.ReadFile(c.path); err == nil {
		if keys, err := parseJWKS(raw); err == nil {
			c.keys = keys
			// Dated by the file rather than by now, so a node that has been
			// switched off for a week refreshes when it comes back instead of
			// believing the copy it just read is an hour old.
			if info, err := os.Stat(c.path); err == nil {
				c.fetchedAt = info.ModTime()
			}
			return c
		}
	}
	if len(c.bundled) > 0 {
		if keys, err := parseJWKS(c.bundled); err == nil {
			c.keys = keys
		}
	}
	return c
}

var _ PublicKeys = (*JWKSCache)(nil)

// PublicKeyByID answers from what this node holds, refreshing first when the
// copy is old or the kid is one it has never seen. A refresh that fails
// changes nothing: the keys already held still answer.
func (c *JWKSCache) PublicKeyByID(kid string) (ed25519.PublicKey, bool) {
	c.mu.Lock()
	key, held := c.keys[kid]
	fresh := time.Since(c.fetchedAt) < c.ttl
	if held && fresh {
		c.mu.Unlock()
		return key, true
	}
	if c.url == "" {
		c.mu.Unlock()
		return key, held
	}
	if fresh && time.Since(c.lastMiss) < time.Minute {
		c.mu.Unlock()
		return key, held
	}
	c.lastMiss = time.Now()
	c.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := c.Refresh(ctx); err != nil {
		c.mu.Lock()
		key, held = c.keys[kid]
		c.mu.Unlock()
		return key, held
	}

	c.mu.Lock()
	key, held = c.keys[kid]
	c.mu.Unlock()
	return key, held
}

// Refresh fetches the document and, if it parses, replaces what this node
// holds and writes the copy on disk. A node with a connection calls it at
// startup so that it is ready for the next time it has none.
func (c *JWKSCache) Refresh(ctx context.Context) error {
	if c.url == "" {
		return errors.New("jwks: no url configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("fetch jwks %s: %w", c.url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch jwks %s: unexpected status %d", c.url, resp.StatusCode)
	}
	// Bounded, so a fetch that lands on something other than the cloud
	// cannot hand a phone an arbitrarily large body to hold in memory.
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if err != nil {
		return fmt.Errorf("fetch jwks %s: %w", c.url, err)
	}
	keys, err := parseJWKS(raw)
	if err != nil {
		return err
	}

	c.mu.Lock()
	c.keys = keys
	c.fetchedAt = time.Now()
	c.mu.Unlock()
	c.persist(raw)
	return nil
}

// persist writes the document out the way the mobile host writes host.json:
// to a temporary name and then renamed, so a node interrupted mid-write keeps
// the copy it had rather than gaining one it cannot parse. A failure is
// logged nowhere and ignored, because a node that cannot write its cache is
// still a node that can verify tokens for as long as it stays up.
func (c *JWKSCache) persist(raw []byte) {
	if c.path == "" {
		return
	}
	tmp := c.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return
	}
	if err := os.Rename(tmp, c.path); err != nil {
		_ = os.Remove(tmp)
	}
}

func parseJWKS(raw []byte) (map[string]ed25519.PublicKey, error) {
	var doc JWKS
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("parse jwks: %w", err)
	}
	return doc.publicKeys()
}
