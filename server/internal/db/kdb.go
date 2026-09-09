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
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"zolik/server/internal/module"

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
	NSDailyMetrics = "daily_metrics"
	NSBoots        = "boots"
)

var kdbNamespaceNames = []string{
	NSMatches, NSUsers, NSSessions, NSScoring, NSMatchResults,
	NSPlayerStats, NSIdentities, NSLoginCodes, NSOAuthFlows,
	NSDailyMetrics, NSBoots,
}

const kdbCatalog = "zolik"

// KDBCatalog is the catalog name every KDB namespace is opened under.
const KDBCatalog = kdbCatalog

// KDBNamespaceNames lists every namespace name the KDB backend declares, in
// the order OpenKDBWithStorage opens them. Exported for one-shot tooling
// (see cmd/kdb-consolidate) that must walk every namespace without keeping
// its own copy of the list to drift out of sync with this one.
func KDBNamespaceNames() []string {
	out := make([]string, len(kdbNamespaceNames))
	copy(out, kdbNamespaceNames)
	return out
}

// kdbSweepInterval is how often expired documents are physically removed.
// Reads filter on expiry themselves, exactly as they must under Mongo's TTL
// monitor (which also only sweeps periodically); the sweeper is about space,
// not correctness.
const kdbSweepInterval = time.Minute

// KDB is the embedded database: one shared engine runtime (a Host, for the
// file-backed case) holding every namespace under one data root, one
// directory lock and one memory pool, plus the per-namespace write lock that
// makes compound operations atomic.
type KDB struct {
	// host is non-nil only for file-backed KDBs. It owns the directory lock
	// and the shared memory pool every namespace draws from; Close on it
	// tears down every namespace's storage and releases the lock. Nil for
	// in-memory KDBs, which have no lock or shim to share and stay one
	// runtime per namespace.
	host      *embed.Host
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
	//
	// This lock is load-bearing and cannot be replaced by the engine's own
	// write gate: put() goes through embed.PutJSONDocument, which the gate
	// does not cover at all (kdb's docs/kdb-lld-concurrency.md §6 warns
	// embedded callers get no gate — verified empirically: 8 unserialized
	// goroutines racing PutJSONDocument lost 278 of 320 writes, each
	// returned as a "branch main moved" error, no silent corruption).
	// deleteByUUID() goes through srv.Commit, which the gate does cover, but
	// the gate only orders Commit calls against each other, not against an
	// ungated PutJSONDocument. mu is what makes a put and a delete on this
	// namespace mutually exclusive.
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
	// MemoryBudgetBytes opts every namespace's KdbServerRuntime into
	// kdb-spec-layer13 Component 48's memory admission: an operation reserves
	// its estimated cost before running and is rejected with a typed,
	// retryable error under pressure, with an orderly abort+restart if
	// pressure never clears — instead of the commit DAG (which retains every
	// write forever; there is no compaction on this path) growing until the
	// kernel OOM-kills the process with no signal to the client. Zero (the
	// default) leaves governance off, admitting everything, which is only
	// appropriate for tests and other bounded-workload runs.
	//
	// Every namespace shares one process and one cgroup limit, so each gets
	// the same full budget rather than a per-namespace slice: MemoryGuard
	// samples process-wide memory (runtime/metrics), not anything
	// namespace-scoped, so this is what makes all namespaces shed together
	// as the shared ceiling is approached.
	//
	// Also sizes the shared hot-tier cache pool (document/commit-ops/
	// history-tree/memtable — embed.Host's memory arbiter, see
	// kdbHotTierPoolBytes) every namespace draws its own share of on demand,
	// rather than each of the nine namespace runtimes independently
	// defaulting to 128 MiB.
	MemoryBudgetBytes uint64
}

// kdbMemoryRejectFraction is the fraction of MemoryBudgetBytes at which
// writes start being shed (ZoneHigh's entry point). Matches kdb-service's own
// default (go/cmd/kdb-service/main.go), which this mirrors; the scan row
// budget likewise comes from the engine's own default. The rescue reserve
// does not — see kdbRescueReserveBytes.
const kdbMemoryRejectFraction = 0.85

// kdbRescueReserveBytes is one whole rescue reserve, divided by the number of
// namespaces that will each hold a share of it.
//
// The reserve is memory KDB holds back and drops on entry to Critical, so the
// abort sequence has room to finish in-flight commits and write its typed
// rejections instead of dying partway through for want of a few megabytes. It
// is deliberately a *real* allocation with every page touched — a reservation
// the allocator has not honoured yet is an intention, not headroom.
//
// Which is exactly why nine of them is not nine times as safe. Taking the
// engine's default per namespace, as SetMemoryLimit does, allocated and
// touched 9 x 48 MiB = 432 MiB before a single player connected: 42% of a 1 GiB
// container, live and uncollectable, at idle. With Go's collector then aiming
// at twice the live heap, the process idled at roughly 84% of that container —
// a hair under the 0.85 admission watermark — so the first few players to
// arrive pushed it over and were refused. The server looked like it could hold
// four people. It was holding nine copies of its own escape hatch.
//
// One process, one cgroup, one memory ceiling: one reserve's worth is what a
// single-runtime deployment holds, so that is what this one holds too, split so
// that every namespace still has its own share to drop when its guard goes
// critical. The guards all sample process-wide memory, so they go critical
// together and the shares are dropped together — which is the same 48 MiB
// arriving at the same moment, from nine places instead of one.
//
// See TestRescueReserveIsSharedAcrossNamespaces, which is what stops a tenth
// namespace quietly adding another 48 MiB.
func kdbRescueReserveBytes() int64 {
	share := kdbserver.DefaultRescueReserveBytes / int64(len(kdbNamespaceNames))
	if share < 1 {
		return 1
	}
	return share
}

// kdbHotTierFraction is the share of MemoryBudgetBytes set aside for KDB's
// own hot-tier caching (document versions, commit operations, historical
// trees, the memtable — embed.Host's shared memory arbiter pool, see
// embed.OpenFileHost). The rest is headroom for Go runtime overhead, the
// embedded web UI, and the admission grants an in-flight write burst needs
// before the kdbMemoryRejectFraction line trips.
//
// Left unset (the zero value engineOptions produces when MemoryBudgetBytes
// is 0), the host falls back to embed.DefaultHostMemoryBudgetBytes (64 MiB)
// shared across every namespace — fine for a dev machine, but a process
// that knows its real cgroup limit should size the pool from it instead of
// inheriting a default sized for a single namespace running alone.
const kdbHotTierFraction = 0.5

// kdbHotTierPoolBytes derives the host's shared hot-tier cache pool — see
// embed.Host.MemoryArbiter — from the same cgroup-derived total
// kdbMemoryRejectFraction governs write admission against. Every namespace
// draws its own share from this one pool on demand rather than each
// independently defaulting to a fixed size; a busy namespace can borrow
// from an idle one without a restart (see kdb's TestBusyNamespaceBorrowsFromTheIdleOne).
// Zero (no admission budget configured, e.g. a dev machine) returns zero,
// which leaves the host at its own default — the same "degrades to off"
// shape SetMemoryLimit already has.
func kdbHotTierPoolBytes(totalBudgetBytes uint64) int64 {
	if totalBudgetBytes == 0 {
		return 0
	}
	return int64(float64(totalBudgetBytes) * kdbHotTierFraction)
}

// busyIfShed translates the engine's admission refusals into SERVER_BUSY, the
// same refusal vocabulary a player turned away at the door already gets
// (admission.WriteBusy). Both spellings mean one thing to a caller — the
// server is full right now, the request was fine, come back shortly — and the
// client already renders that code from its locale bundle and backs off on it,
// so a shed write reads as a busy server rather than as the 500 an untranslated
// engine error would produce.
//
// Returned as a bare module.Error rather than wrapped: module.CodeOf type-
// asserts, so a fmt.Errorf("%w") around this would silently degrade the code
// to a generic ERROR at exactly the moment the vocabulary matters. The engine's
// own text is kept as the Message so the server log still says which zone shed
// the write and for how long it wanted the caller to wait; clients key off the
// code, not the prose. Written as a literal so the key scanner sees SERVER_BUSY
// and keeps it in serverKeys.json.
//
// Anything else — including ResourceExhaustedError, which means "resubmit
// smaller" and so will not pass on a retry — travels on unchanged.
func busyIfShed(err error) error {
	var pressure *kdbserver.MemoryPressureError
	var busy *kdbserver.BusyError
	if errors.As(err, &pressure) || errors.As(err, &busy) {
		return module.Error{Code: "SERVER_BUSY", Message: err.Error()}
	}
	return err
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
	if hotTier := kdbHotTierPoolBytes(sc.MemoryBudgetBytes); hotTier > 0 {
		opts.Storage.MemoryBudgetBytes = hotTier
	}
	k := &KDB{
		nss:  make(map[string]*kdbNamespace, len(kdbNamespaceNames)),
		stop: make(chan struct{}),
		done: make(chan struct{}),
	}

	var host *embed.Host
	if path != "" {
		// One data root for every namespace: the Host owns the directory
		// lock, the I/O shim and the memory pool, and lends them to each
		// namespace opened under it — see embed.OpenFileHost. Nine
		// namespaces used to mean nine data roots, nine locks and nine
		// independent budgets; now they share one root and one pool.
		if err := os.MkdirAll(path, 0o755); err != nil {
			return nil, fmt.Errorf("kdb: creating %s: %w", path, err)
		}
		var err error
		host, err = embed.OpenFileHost(path, opts)
		if err != nil {
			return nil, fmt.Errorf("kdb: opening host at %s: %w", path, err)
		}
		k.host = host
	}

	for _, name := range kdbNamespaceNames {
		nsID := kdbCatalog + "/" + name
		var rt *embed.EmbeddedKdbRuntime
		var err error
		if path == "" {
			rt, err = embed.OpenMemoryRuntime(kdbCatalog, nsID, schema.None())
		} else {
			rt, err = host.Namespace(kdbCatalog, nsID, schema.None())
		}
		if err != nil {
			k.closeRuntimes()
			return nil, fmt.Errorf("kdb: opening namespace %s: %w", nsID, err)
		}
		srv := kdbserver.NewKdbServerRuntime(rt)
		if sc.MemoryBudgetBytes > 0 {
			// SetMemoryBudget rather than SetMemoryLimit: the narrow form
			// takes the engine's default rescue reserve, and this process
			// opens nine of these. See kdbRescueReserveBytes.
			srv.SetMemoryBudget(
				sc.MemoryBudgetBytes,
				kdbMemoryRejectFraction,
				kdbRescueReserveBytes(),
				kdbserver.DefaultScanRowBudget,
			)
		}
		k.nss[name] = &kdbNamespace{id: nsID, rt: rt, srv: srv}
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
	// Each rt.Close() above only closed that namespace under the host (see
	// EmbeddedKdbRuntime.storageClose set by Host.openNamespace); the host
	// itself — and the directory lock it holds — is released here.
	if k.host != nil {
		_ = k.host.Close()
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
	// PutJSONDocument appends to the DAG through the embedded runtime, so it
	// bypasses the memory admission srv.Commit goes through (nothing in
	// package embed consults Admission at all) — and it is the path behind
	// every Put, Insert and Tx.Put, which is essentially all of this server's
	// write volume. Reserving here is therefore what actually holds the line:
	// without it the budget set at open time would govern only deletes and
	// reads, while the writes that grow the commit DAG forever stayed
	// unmetered. Mirrors what KdbServerRuntime.commitWith does around its own
	// commits, bounded wait included. Acquire on a nil Admission (no budget
	// configured) returns an empty grant and no error, so this is a no-op
	// wherever governance is off.
	ctx, cancel := context.WithTimeout(context.Background(), n.srv.WriteTimeout)
	defer cancel()
	grant, err := n.srv.Admission().Acquire(ctx, kdbserver.ClassWrite, len(withID))
	if err != nil {
		return busyIfShed(err)
	}
	defer grant.Release()

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
		// Commit reserves against the same budget put() does, so a delete is
		// shed under pressure exactly as a write is.
		return false, busyIfShed(err)
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
	ExpiresAt time.Time `bson:"expiresAt"`
}

// sweptNamespaces mirrors mongo.go's TTL indexes: sessions, login codes and
// OAuth flows die at expiresAt.
//
// Matches are deliberately not here, and abandonAt is deliberately not an
// expiry field. It used to be both, and the pair destroyed games: abandonAt is
// not "delete me at", it is "decide about me at", and match.StartReaper is
// what decides — it marks the table abandoned, clears abandonAt, and leaves
// the row. Sweeping the same field turned that into a race between two
// timers, and when this one won there was no abandoned match to report, no
// row for the statistics, and nothing left of the game: a player following
// their own link got a table that had never existed rather than one that had
// ended. The reaper's 30s tick beat this one's minute often enough that the
// losses looked random.
//
// A resolved match is history and stays, exactly like a completed one. What
// bounds the collection is that suspended tables stop being suspended within
// AbandonWindow, not that they are eventually deleted.
var sweptNamespaces = []string{NSSessions, NSLoginCodes, NSOAuthFlows}

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
		// Re-check under the lock: a document can be rewritten with a later
		// expiry between the scan above and this delete, and deleting it then
		// would drop a session that had just been extended.
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
	return !p.ExpiresAt.IsZero() && p.ExpiresAt.Before(now)
}
