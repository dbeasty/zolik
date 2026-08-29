// KDB backend: the embedded, in-process storage engine behind the
// FEATURE_FLAG_DB_ENGINE=kdb deployment shape — one binary, no database
// server, no Redis.
//
// The engine underneath (github.com/limidus/kdb/go) stores JSON documents
// keyed by UUID and gives us durable, fsynced commits. What it does not give
// us today — secondary indexes, unique constraints, conditional updates —
// this file provides at the application layer, and can provide *correctly*
// because the KDB deployment shape is single-process by design: every write
// to a namespace goes through that namespace's mutex here, so a
// read-check-write critical section really is atomic. That is the same
// bargain the Mongo backend strikes with its unique indexes, enforced one
// level higher.
package db

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	kdbauth "github.com/limidus/kdb/go/kdb/auth"
	"github.com/limidus/kdb/go/kdb/codec"
	"github.com/limidus/kdb/go/kdb/document"
	"github.com/limidus/kdb/go/kdb/embed"
	"github.com/limidus/kdb/go/kdb/schema"
	kdbserver "github.com/limidus/kdb/go/kdb/server"
	"github.com/limidus/kdb/go/kdb/storage"
	storio "github.com/limidus/kdb/go/kdb/storage/io"
)

// Namespace names, one per Mongo collection, so the two backends stay
// row-for-row comparable. The legacy "games" collection is deliberately
// absent: only the one-shot migration tooling reads it, and that tooling is
// Mongo-only.
const (
	NSMatches      = "matches"
	NSUsers        = "users"
	NSSessions     = "sessions"
	NSScoring      = "scoring_sessions"
	NSMatchResults = "match_results"
	NSPlayerStats  = "player_stats"
	NSIdentities   = "identities"
	NSLoginCodes   = "login_codes"
	NSOAuthFlows   = "oauth_flows"
)

var kdbNamespaceNames = []string{
	NSMatches, NSUsers, NSSessions, NSScoring, NSMatchResults,
	NSPlayerStats, NSIdentities, NSLoginCodes, NSOAuthFlows,
}

const kdbCatalog = "zolik"

// kdbSweepInterval is how often expired documents are physically removed.
// Reads filter on expiry themselves, exactly as they must under Mongo's TTL
// monitor (which also only sweeps periodically); the sweeper is about space,
// not correctness.
const kdbSweepInterval = time.Minute

// KDB is the embedded database: one engine runtime per namespace, plus the
// per-namespace write lock that makes compound operations atomic.
type KDB struct {
	nss       map[string]*kdbNamespace
	stop      chan struct{}
	done      chan struct{}
	closeOnce sync.Once
}

type kdbNamespace struct {
	id  string // "zolik/<name>"
	rt  *embed.EmbeddedKdbRuntime
	srv *kdbserver.KdbServerRuntime
	// mu serializes writes and read-check-write critical sections. Plain
	// reads do not take it: the engine is safe for concurrent reads, and a
	// reader racing a writer sees before-or-after, same as with Mongo.
	mu sync.Mutex
}

// KDBStorage is the write-durability tuning the engine opens with. The zero
// value is the default: durability "sync" with sync mode "fast" — every ack
// still means "synced to the device", via F_BARRIERFSYNC/fdatasync rather
// than the ~4ms full drive-cache flush. See KDBStorageFromEnv for the
// environment spelling and kdb's docs/benchmarks/write-durability-modes.md
// for the measured matrix.
type KDBStorage struct {
	// Durability: "sync" (ack ⇒ synced, the default) or "async" (ack ⇒
	// appended; a background flush runs on an interval, Mongo-journal style).
	Durability string
	// SyncMode: "fast" (F_BARRIERFSYNC/fdatasync, the default) or "full"
	// (F_FULLFSYNC/fsync — survives power loss, ~4ms per sync on Apple SSDs).
	SyncMode string
	// AsyncSyncIntervalMillis bounds the crash-loss window under "async".
	// Zero uses the engine default (5ms); 100 matches Mongo's default
	// journaling semantics.
	AsyncSyncIntervalMillis int64
}

// KDBStorageFromEnv reads KDB_DURABILITY, KDB_SYNC_MODE and
// KDB_ASYNC_SYNC_INTERVAL_MS. Unset means the defaults; an unrecognised
// value is an error rather than a silent fallback, because it would silently
// change what an acknowledged write means.
func KDBStorageFromEnv() (KDBStorage, error) {
	sc := KDBStorage{
		Durability: strings.ToLower(strings.TrimSpace(os.Getenv("KDB_DURABILITY"))),
		SyncMode:   strings.ToLower(strings.TrimSpace(os.Getenv("KDB_SYNC_MODE"))),
	}
	if raw := strings.TrimSpace(os.Getenv("KDB_ASYNC_SYNC_INTERVAL_MS")); raw != "" {
		ms, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || ms < 0 {
			return KDBStorage{}, fmt.Errorf("kdb: KDB_ASYNC_SYNC_INTERVAL_MS=%q: want a non-negative integer", raw)
		}
		sc.AsyncSyncIntervalMillis = ms
	}
	if _, err := sc.engineOptions(); err != nil {
		return KDBStorage{}, err
	}
	return sc, nil
}

// engineOptions maps the string-spelled config onto the engine's option
// struct, starting from the engine's own env-derived options so KDB_S3_*
// replication keeps working exactly as it did under OpenFileRuntime.
func (sc KDBStorage) engineOptions() (embed.FileRuntimeOptions, error) {
	opts := embed.FileRuntimeOptionsFromEnv()
	switch sc.Durability {
	case "", "sync":
		opts.Storage.Durability = storage.DurabilitySync
	case "async":
		opts.Storage.Durability = storage.DurabilityAsync
	default:
		return opts, fmt.Errorf("kdb: KDB_DURABILITY=%q: want \"sync\" or \"async\"", sc.Durability)
	}
	switch sc.SyncMode {
	case "", "fast":
		opts.Storage.SyncMode = storio.SyncModeFast
	case "full":
		opts.Storage.SyncMode = storio.SyncModeFull
	default:
		return opts, fmt.Errorf("kdb: KDB_SYNC_MODE=%q: want \"fast\" or \"full\"", sc.SyncMode)
	}
	opts.Storage.AsyncSyncIntervalMillis = sc.AsyncSyncIntervalMillis
	return opts, nil
}

// OpenKDB opens (creating if needed) the embedded database rooted at path,
// with durability tuning read from the environment (see KDBStorageFromEnv).
// An empty path keeps everything in memory — no durability, which is only
// acceptable in tests.
func OpenKDB(path string) (*KDB, error) {
	sc, err := KDBStorageFromEnv()
	if err != nil {
		return nil, err
	}
	return OpenKDBWithStorage(path, sc)
}

// OpenKDBWithStorage opens the embedded database with explicit durability
// tuning, bypassing the environment.
func OpenKDBWithStorage(path string, sc KDBStorage) (*KDB, error) {
	opts, err := sc.engineOptions()
	if err != nil {
		return nil, err
	}
	k := &KDB{
		nss:  make(map[string]*kdbNamespace, len(kdbNamespaceNames)),
		stop: make(chan struct{}),
		done: make(chan struct{}),
	}
	for _, name := range kdbNamespaceNames {
		nsID := kdbCatalog + "/" + name
		var rt *embed.EmbeddedKdbRuntime
		var err error
		if path == "" {
			rt, err = embed.OpenMemoryRuntime(kdbCatalog, nsID, schema.None())
		} else {
			// One data root per namespace: the engine's directory lock is
			// per-root, so two runtimes over one root would refuse to open.
			root := filepath.Join(path, name)
			if err := os.MkdirAll(root, 0o755); err != nil {
				k.closeRuntimes()
				return nil, fmt.Errorf("kdb: creating %s: %w", root, err)
			}
			rt, err = embed.OpenFileRuntimeWithOptions(root, kdbCatalog, nsID, schema.None(), opts)
		}
		if err != nil {
			k.closeRuntimes()
			return nil, fmt.Errorf("kdb: opening namespace %s: %w", nsID, err)
		}
		k.nss[name] = &kdbNamespace{id: nsID, rt: rt, srv: kdbserver.NewKdbServerRuntime(rt)}
	}
	go k.sweep()
	return k, nil
}

// Close flushes and seals every namespace. Under the default "sync"
// durability every commit was already synced when it was acked, so this is
// orderliness; under "async" it is also what drains and flushes the tail of
// the commit log.
func (k *KDB) Close(ctx context.Context) error {
	k.closeOnce.Do(func() {
		close(k.stop)
		select {
		case <-k.done:
		case <-ctx.Done():
		}
		k.closeRuntimes()
	})
	return nil
}

func (k *KDB) closeRuntimes() {
	for _, n := range k.nss {
		n.mu.Lock()
		n.rt.Close()
		n.mu.Unlock()
	}
}

func (k *KDB) ns(name string) *kdbNamespace {
	n, ok := k.nss[name]
	if !ok {
		// A namespace this binary never declared is a programming error, not
		// a runtime condition — same class as asking Mongo for a collection
		// handle with a typo'd name, except caught loudly.
		panic("kdb: unknown namespace " + name)
	}
	return n
}

// uuidForKey maps a natural key — an ObjectID hex, a session token, a
// "provider:subject" pair — onto the UUID document id the engine requires.
// Deterministic, so a lookup by natural key is a direct document read rather
// than a scan.
func uuidForKey(key string) codec.UUID {
	sum := sha256.Sum256([]byte(key))
	u, err := codec.UUIDFromBytes(sum[:16])
	if err != nil {
		// UUIDFromBytes only rejects a wrong length, and sum[:16] cannot be one.
		panic(err)
	}
	return u
}

// Get returns the stored document for key, or ErrNotFound.
func (k *KDB) Get(ns, key string) ([]byte, error) {
	return k.ns(ns).get(key)
}

// Scan streams every document in the namespace, in no particular order.
func (k *KDB) Scan(ns string, fn func(doc []byte) error) error {
	return k.ns(ns).scan(func(_ codec.UUID, doc []byte) error { return fn(doc) })
}

// Put creates or wholly replaces the document at key.
func (k *KDB) Put(ns, key string, doc []byte) error {
	n := k.ns(ns)
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.put(key, doc)
}

// Insert creates the document at key, failing with ErrDuplicateKey if one is
// already there — the engine-level half of every unique constraint.
func (k *KDB) Insert(ns, key string, doc []byte) error {
	n := k.ns(ns)
	n.mu.Lock()
	defer n.mu.Unlock()
	if _, err := n.get(key); err == nil {
		return fmt.Errorf("kdb: %s %q: %w", ns, key, ErrDuplicateKey)
	}
	return n.put(key, doc)
}

// Delete removes the document at key, reporting whether it existed.
func (k *KDB) Delete(ns, key string) (bool, error) {
	n := k.ns(ns)
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.deleteByUUID(uuidForKey(key))
}

// Tx is a critical section on one namespace, not a rollback-able
// transaction: writes land as they are made, and an error simply stops the
// caller's function. What the lock buys is atomicity of read-check-write —
// uniqueness scans, version checks, consume-once updates — against every
// other writer in this (single) process.
type Tx struct {
	n *kdbNamespace
}

// Update runs fn holding the namespace's write lock.
func (k *KDB) Update(ns string, fn func(tx *Tx) error) error {
	n := k.ns(ns)
	n.mu.Lock()
	defer n.mu.Unlock()
	return fn(&Tx{n: n})
}

// Get returns the document at key, or ErrNotFound.
func (t *Tx) Get(key string) ([]byte, error) { return t.n.get(key) }

// Put creates or wholly replaces the document at key.
func (t *Tx) Put(key string, doc []byte) error { return t.n.put(key, doc) }

// Insert creates the document at key, failing with ErrDuplicateKey if
// present.
func (t *Tx) Insert(key string, doc []byte) error {
	if _, err := t.n.get(key); err == nil {
		return fmt.Errorf("kdb: %s %q: %w", t.n.id, key, ErrDuplicateKey)
	}
	return t.n.put(key, doc)
}

// Delete removes the document at key, reporting whether it existed.
func (t *Tx) Delete(key string) (bool, error) { return t.n.deleteByUUID(uuidForKey(key)) }

// Scan streams every document in the namespace.
func (t *Tx) Scan(fn func(doc []byte) error) error {
	return t.n.scan(func(_ codec.UUID, doc []byte) error { return fn(doc) })
}

// --- namespace primitives ---

func (n *kdbNamespace) get(key string) ([]byte, error) {
	doc, _, found, err := n.srv.GetDocument(n.id, uuidForKey(key))
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("kdb: %s %q: %w", n.id, key, ErrNotFound)
	}
	return []byte(doc), nil
}

// put stores doc at key, create-or-full-replace. It goes through
// embed.PutJSONDocument rather than the runtime's Upsert: Upsert routes the
// body through a WriteOp, and a WriteOp onto an existing document is a
// shallow *merge* — a field the model dropped (a cleared suspension, an
// emptied option) would silently survive. PutJSONDocument swaps the whole
// body, which is what ReplaceOne semantics require. Callers hold n.mu, which
// is what stands in for the write serialization Upsert would have provided.
func (n *kdbNamespace) put(key string, doc []byte) error {
	withID, err := injectDocID(doc, uuidForKey(key))
	if err != nil {
		return err
	}
	_, err = embed.PutJSONDocument(n.rt, n.id, string(withID))
	return err
}

func (n *kdbNamespace) deleteByUUID(id codec.UUID) (bool, error) {
	_, _, found, err := n.srv.GetDocument(n.id, id)
	if err != nil {
		return false, err
	}
	if !found {
		return false, nil
	}
	head, err := n.rt.DAG.Head()
	if err != nil {
		return false, err
	}
	txID, err := codec.RandomUUID()
	if err != nil {
		return false, err
	}
	tx := document.Transaction{
		ID:          txID,
		BaseVersion: head,
		Operations:  []document.Op{document.DeleteOp{DocID: id}},
		Timestamp:   codec.TimestampNow(),
	}
	if _, err := n.srv.Commit(n.id, tx, "", kdbauth.Principal{}); err != nil {
		return false, err
	}
	return true, nil
}

func (n *kdbNamespace) scan(fn func(id codec.UUID, doc []byte) error) error {
	head, err := n.rt.DAG.Head()
	if err != nil {
		return err
	}
	commit, err := n.rt.DAG.GetCommitOrThrow(head)
	if err != nil {
		return err
	}
	return n.rt.Storage.ScanDocuments(n.id, commit.DocumentTreeHash, 256,
		func(batch []document.Document) error {
			for _, d := range batch {
				if err := fn(d.ID, []byte(d.JSON)); err != nil {
					return err
				}
			}
			return nil
		})
}

// injectDocID sets the engine-level "id" field the put path derives the
// document id from. The models never use a root-level "id" bson field, so
// nothing collides, and decoders ignore the extra key on the way back out.
func injectDocID(doc []byte, id codec.UUID) ([]byte, error) {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(doc, &root); err != nil {
		return nil, fmt.Errorf("kdb: document is not a JSON object: %w", err)
	}
	idJSON, err := json.Marshal(id.String())
	if err != nil {
		return nil, err
	}
	root["id"] = idJSON
	return json.Marshal(root)
}

// --- expiry sweep ---

// expiryProbe reads just the fields the sweeper cares about, whichever the
// namespace has. Mongo expires these documents with TTL indexes; here the
// reads filter on the same fields (correctness) and this sweep reclaims the
// space (parity).
type expiryProbe struct {
	ExpiresAt time.Time  `bson:"expiresAt"`
	AbandonAt *time.Time `bson:"abandonAt"`
}

// sweptNamespaces mirrors mongo.go's TTL indexes: sessions, login codes and
// OAuth flows die at expiresAt; a suspended match dies at abandonAt.
var sweptNamespaces = []string{NSSessions, NSLoginCodes, NSOAuthFlows, NSMatches}

func (k *KDB) sweep() {
	defer close(k.done)
	ticker := time.NewTicker(kdbSweepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-k.stop:
			return
		case <-ticker.C:
			for _, name := range sweptNamespaces {
				k.sweepNamespace(k.nss[name])
			}
		}
	}
}

func (k *KDB) sweepNamespace(n *kdbNamespace) {
	now := time.Now().UTC()
	var dead []codec.UUID
	err := n.scan(func(id codec.UUID, doc []byte) error {
		if expired(doc, now) {
			dead = append(dead, id)
		}
		return nil
	})
	if err != nil {
		return
	}
	for _, id := range dead {
		n.mu.Lock()
		// Re-check under the lock: a suspended match can be resumed — its
		// abandonAt cleared — between the scan above and this delete, and
		// deleting it then would kill a live table.
		if doc, _, found, err := n.srv.GetDocument(n.id, id); err == nil && found && expired([]byte(doc), now) {
			_, _ = n.deleteByUUID(id)
		}
		n.mu.Unlock()
	}
}

// expired reports whether the document's own expiry fields say it is dead.
func expired(doc []byte, now time.Time) bool {
	var p expiryProbe
	if err := UnmarshalDoc(doc, &p); err != nil {
		return false
	}
	if !p.ExpiresAt.IsZero() && p.ExpiresAt.Before(now) {
		return true
	}
	return p.AbandonAt != nil && !p.AbandonAt.IsZero() && p.AbandonAt.Before(now)
}
