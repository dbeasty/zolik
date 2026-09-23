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
	"sort"
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
	// NSMatchLog holds each match's moves and board snapshots, kept out of
	// NSMatches so the scans over matches never read them.
	NSMatchLog = "match_log"

	// NSReservations holds one document per claimed unique value - a username,
	// an email address, a friend code - keyed by the value itself. See
	// kdbreservations.go for why a scan cannot do this job once a namespace
	// replicates.
	NSReservations = "reservations"

	// NSNodes holds one document per enrolled replica: which install it is,
	// whose it is, and the key it signs with. NSGuestClaims holds one per
	// guest id a person has claimed, so a match played before they signed in
	// is credited to them however long afterwards it arrives.
	NSNodes       = "nodes"
	NSGuestClaims = "guest_claims"

	NSNotifyProfiles = "notify_profiles"
	NSNotifyCircle   = "notify_circle"
	NSNotifyDevices  = "notify_devices"
)

var kdbNamespaceNames = []string{
	NSMatches, NSUsers, NSSessions, NSScoring, NSMatchResults,
	NSPlayerStats, NSIdentities, NSLoginCodes, NSOAuthFlows,
	NSDailyMetrics, NSBoots, NSMatchLog, NSReservations, NSNodes, NSGuestClaims,
	NSNotifyProfiles, NSNotifyCircle, NSNotifyDevices,
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
	host *embed.Host
	// path is the data root, empty for an in-memory database. Kept so a
	// namespace can be asked for by name without creating it: a read must not
	// bring a namespace into existence.
	path string
	// mu guards nss, which grows at runtime: a namespace per user and per
	// match cannot be declared up front the way the fixed collections are.
	mu  sync.RWMutex
	nss map[string]*kdbNamespace
	// primary is the runtime every namespace opened later shares governance
	// with, so one memory budget covers the whole process however many
	// namespaces it ends up holding.
	primary *kdbserver.KdbServerRuntime
	// prepare, when set, is what internal/sync hooks a newly opened namespace
	// into: commit notification, the retention floor peers impose, and the
	// replicated definitions. Nil until this process is a sync node.
	prepare func(*kdbserver.KdbServerRuntime)
	// set commits transactions that span namespaces; see UpdateMulti.
	set       *kdbserver.NamespaceSet
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
//
// It is a quarter, not a half, because the pool is not a ceiling. kdb splits
// it into four caches whose shares sum to 125% of it (document versions 0.5,
// memtable, commit operations and historical trees 0.25 each), and those
// caches fill with match history nobody is reading. The garbage collector then
// lets the heap reach twice what is live before collecting. At a half, the
// history of a few days' play alone (485 MiB live of 568) put the process past
// kdbMemoryRejectFraction, and every new match was refused.
const kdbHotTierFraction = 0.25

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

// KDBMemoryLines is where kdb starts refusing writes and how large its cache
// pool is, for a process whose memory budget is totalBudgetBytes.
func KDBMemoryLines(totalBudgetBytes uint64) (writeShedding, hotTierPool int64) {
	return int64(float64(totalBudgetBytes) * kdbMemoryRejectFraction), kdbHotTierPoolBytes(totalBudgetBytes)
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

// isPrecondition reports whether err is a write refused because the document
// had changed since the caller read it — a peer's merge landing mid-critical
// section, since local writers are serialized by the namespace lock.
func isPrecondition(err error) bool {
	var pre *kdbserver.PreconditionFailedError
	return errors.As(err, &pre)
}

// duplicateIfPrecondition turns "the document was supposed to be absent" into
// the duplicate-key error every unique constraint in this package speaks, so
// a key a peer created reads the same to callers as one this process created.
func duplicateIfPrecondition(nsID, key string, err error) error {
	if isPrecondition(err) {
		return fmt.Errorf("kdb: %s %q: %w", nsID, key, ErrDuplicateKey)
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
		k.path = path
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
			// opens one of these per namespace. See kdbRescueReserveBytes.
			srv.SetMemoryBudget(
				sc.MemoryBudgetBytes,
				kdbMemoryRejectFraction,
				kdbRescueReserveBytes(),
				kdbserver.DefaultScanRowBudget,
			)
		}
		k.nss[name] = &kdbNamespace{id: nsID, rt: rt, srv: srv}
	}

	// Transactions across namespaces commit through the host's coordinator,
	// whose decision log is what replay consults to keep or drop each part.
	// The set holds the same server runtimes every single-namespace write
	// uses, so each namespace still has exactly one write gate.
	var coord *embed.TxnCoordinator
	if host != nil {
		coord = host.Transactions()
	}
	k.set = kdbserver.NewNamespaceSet(coord)
	for _, name := range kdbNamespaceNames {
		if err := k.set.Add(k.nss[name].srv); err != nil {
			k.closeRuntimes()
			return nil, fmt.Errorf("kdb: namespace %s: %w", name, err)
		}
	}
	// A namespace named by the engine rather than by this binary - a peer
	// pulling one, a cross-namespace commit reaching one - opens through the
	// same path every other namespace does, so there is still exactly one
	// runtime and one write gate per namespace.
	k.set.SetOpener(k.openForEngine)
	// Users is as good a primary as any: what the choice actually decides is
	// which runtime carries this node's identity and its auth engine, and
	// which one the others share a memory guard with.
	k.primary = k.nss[NSUsers].srv
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
	k.mu.Lock()
	defer k.mu.Unlock()
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

// openForEngine is what the engine calls for a namespace this process has not
// opened. create is false for a read, which must not bring a namespace into
// existence: a typo'd name would otherwise leave a directory behind that
// nothing ever reads again.
func (k *KDB) openForEngine(id string, create bool) (*kdbserver.KdbServerRuntime, error) {
	name := Unqualified(id)
	// A read must not bring a namespace into existence, but it must be able
	// to reach one that already exists on disk - including a declared one
	// this process closed for idleness and has to open again.
	if !create && k.path != "" && !embed.NamespaceExists(k.path, id) {
		return nil, fmt.Errorf("%w: %s", kdbserver.ErrUnknownNamespace, id)
	}
	n, err := k.Namespace(name)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", kdbserver.ErrUnknownNamespace, id)
	}
	return n.srv, nil
}

func (k *KDB) ns(name string) *kdbNamespace {
	n, err := k.Namespace(name)
	if err != nil {
		// A namespace this binary never declared, or one whose name cannot be
		// a namespace at all, is a programming error rather than a runtime
		// condition — the same class as asking Mongo for a collection handle
		// with a typo'd name, except caught loudly.
		panic("kdb: " + err.Error())
	}
	return n
}

// Namespace returns the handle for a namespace, opening it if this process has
// not held it before.
//
// The fixed collections are all opened at startup and are only ever looked up
// here. The ones that are not fixed — a namespace per user, per match, per
// node outbox — cannot be: their names are data. Opening on demand is what
// makes those possible, and it is also how a peer's sync reaches a namespace
// this process has never written to itself.
//
// An in-memory database can only open what it declared: without a host there
// is no data root to open a namespace under.
func (k *KDB) Namespace(name string) (*kdbNamespace, error) {
	k.mu.RLock()
	n, ok := k.nss[name]
	k.mu.RUnlock()
	if ok {
		return n, nil
	}
	if !IsDynamicNamespace(name) && !isDeclaredNamespace(name) && !k.HoldsNamespace(name) {
		return nil, fmt.Errorf("unknown namespace %q", name)
	}
	if k.host == nil {
		return nil, fmt.Errorf("namespace %q needs a file-backed database", name)
	}

	k.mu.Lock()
	defer k.mu.Unlock()
	// Another goroutine may have opened it while this one waited.
	if n, ok := k.nss[name]; ok {
		return n, nil
	}
	nsID := kdbCatalog + "/" + name
	if err := embed.ValidateNamespaceID(nsID); err != nil {
		return nil, err
	}
	// The set may still be serving this namespace even though this package
	// let go of it: a handle forgotten without the engine closing it, or a
	// namespace a peer's sync opened through the set's own opener. Two
	// runtimes over one namespace would be two uncoordinated writers, so the
	// one the set holds wins.
	if existing, ok := k.set.Get(nsID); ok {
		n := &kdbNamespace{id: nsID, rt: existing.Runtime, srv: existing}
		k.nss[name] = n
		return n, nil
	}
	rt, err := k.host.Namespace(kdbCatalog, nsID, schema.None())
	if err != nil {
		return nil, fmt.Errorf("opening namespace %s: %w", nsID, err)
	}
	srv := kdbserver.NewKdbServerRuntime(rt)
	if k.primary != nil {
		// One budget for the process, not one per namespace: a server holding
		// a namespace per match would otherwise grant itself the whole budget
		// again with every table that opens.
		srv.ShareGovernanceWith(k.primary)
		srv.AuthEngine = k.primary.AuthEngine
		srv.WriteTimeout = k.primary.WriteTimeout
	}
	if k.prepare != nil {
		k.prepare(srv)
	}
	if err := k.set.Add(srv); err != nil {
		rt.Close()
		return nil, fmt.Errorf("namespace %s: %w", nsID, err)
	}
	n = &kdbNamespace{id: nsID, rt: rt, srv: srv}
	k.nss[name] = n
	return n, nil
}

// isDeclaredNamespace reports whether a name is one of the fixed collections
// this binary declares. They are opened at startup, but can be closed again
// for idleness and have to be reopenable by name.
func isDeclaredNamespace(name string) bool {
	for _, declared := range kdbNamespaceNames {
		if declared == name {
			return true
		}
	}
	return false
}

// ForgetNamespace drops a namespace this process is no longer holding open.
//
// The engine closes namespaces nobody has used for a while, which is what
// makes a namespace per person and per match affordable. What it cannot do is
// know that this package kept a handle to the runtime it closed: a write
// through that handle reaches a runtime that has been drained and is refused
// as "server is shutting down", which is true and unhelpful. Forgetting it
// here means the next use opens the namespace again.
func (k *KDB) ForgetNamespace(namespaceID string) {
	name := Unqualified(namespaceID)
	k.mu.Lock()
	defer k.mu.Unlock()
	delete(k.nss, name)
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
// DocumentVersions lists the commits that wrote a key, oldest first.
//
// The commit DAG has held every one of them all along — under history=full
// nothing on that path is reclaimed — and embed.DocumentVersions is what
// finally makes them askable for a single document rather than for a whole
// namespace. Honestly O(the namespace's commit log): see that function's own
// note on why, and on the answer only ever growing at its end.
//
// KDB only, by construction. A caller wanting to work on both engines asks a
// repository, which declines on Mongo — see match.BoardSource.
func (k *KDB) DocumentVersions(ns, key string) ([]string, error) {
	n := k.ns(ns)
	if n == nil {
		return nil, fmt.Errorf("kdb: no namespace %q", ns)
	}
	versions, err := embed.DocumentVersions(n.rt, uuidForKey(key))
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(versions))
	for _, v := range versions {
		if v.Deleted {
			continue
		}
		out = append(out, v.Commit.Hex())
	}
	return out, nil
}

// RetainsHistory reports whether this namespace still keeps the past, and
// says why not when it does not.
//
// DocumentVersions and GetAt answer perfectly well under history=none — right
// up until the retention window passes and the commits they were walking stop
// being producible. So this is not a question about whether the call compiles;
// it is a question about whether the answer will still be there tomorrow. A
// caller that means to *offer* history to a player asks this first.
//
// feature is named in the error, so it reads as a sentence and carries the
// engine's own migrate-history remedy with it.
func (k *KDB) RetainsHistory(ns, feature string) error {
	n := k.ns(ns)
	if n == nil {
		return fmt.Errorf("kdb: no namespace %q", ns)
	}
	return n.rt.AssertRetainsHistory(n.id, feature)
}

// GetAt reads a key as it stood at one of those commits.
//
// Returns db.ErrNotFound when the key did not exist yet at that commit, which
// is an ordinary answer rather than a failure: a commit is a point in the
// whole namespace's history, not in this document's.
func (k *KDB) GetAt(ns, key, commitHex string) ([]byte, error) {
	n := k.ns(ns)
	if n == nil {
		return nil, fmt.Errorf("kdb: no namespace %q", ns)
	}
	at, err := codec.HashFromHex(commitHex)
	if err != nil {
		return nil, fmt.Errorf("kdb: %q is not a commit: %w", commitHex, err)
	}
	body, ok, err := embed.DocumentAt(n.rt, n.id, uuidForKey(key), at)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotFound
	}
	return []byte(body), nil
}

func (k *KDB) Scan(ns string, fn func(doc []byte) error) error {
	return k.ns(ns).scan(func(_ codec.UUID, doc []byte) error { return fn(doc) })
}

// Put creates or wholly replaces the document at key.
func (k *KDB) Put(ns, key string, doc []byte) error {
	n := k.ns(ns)
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.put(key, doc, nil)
}

// Insert creates the document at key, failing with ErrDuplicateKey if one is
// already there — the engine-level half of every unique constraint.
//
// The check and the write are one operation as far as any other writer is
// concerned: n.mu keeps this process's writers out, and Expect{Absent} keeps
// out a peer's merge that created the same key between the two, which no
// local lock can.
func (k *KDB) Insert(ns, key string, doc []byte) error {
	n := k.ns(ns)
	n.mu.Lock()
	defer n.mu.Unlock()
	if _, err := n.get(key); err == nil {
		return fmt.Errorf("kdb: %s %q: %w", ns, key, ErrDuplicateKey)
	}
	if err := n.put(key, doc, &kdbserver.Expect{Absent: true}); err != nil {
		return duplicateIfPrecondition(n.id, key, err)
	}
	return nil
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
	// read remembers the content hash of every key this critical section has
	// read, so the write that follows can say what it decided on. Empty means
	// "was not there when I looked".
	read map[string]string
}

// kdbUpdateAttempts is how many times Update re-runs fn when a peer's merge
// landed between its read and its write. Contention like that is between this
// node and a replicating peer, not between local writers (n.mu covers those),
// so it is rare and does not compound: each retry reads the merged value.
const kdbUpdateAttempts = 5

// Update runs fn holding the namespace's write lock.
//
// When a write inside fn is refused because the document changed under it —
// which, with the lock held, only a peer's merge can do — fn runs again on
// what is there now. A read-check-write is therefore atomic against peers as
// well as against this process, which is what a replicated namespace needs
// and what the lock alone cannot give.
func (k *KDB) Update(ns string, fn func(tx *Tx) error) error {
	n := k.ns(ns)
	n.mu.Lock()
	defer n.mu.Unlock()
	var err error
	for attempt := 0; attempt < kdbUpdateAttempts; attempt++ {
		err = fn(&Tx{n: n, read: map[string]string{}})
		if !isPrecondition(err) {
			return err
		}
	}
	return err
}

// Get returns the document at key, or ErrNotFound. What it read is what a
// later Put or Insert on the same key requires to still be true.
func (t *Tx) Get(key string) ([]byte, error) {
	doc, hash, found, err := t.n.readForUpdate(key)
	if err != nil {
		return nil, err
	}
	t.read[key] = hash
	if !found {
		return nil, fmt.Errorf("kdb: %s %q: %w", t.n.id, key, ErrNotFound)
	}
	return doc, nil
}

// Put creates or wholly replaces the document at key.
func (t *Tx) Put(key string, doc []byte) error { return t.n.put(key, doc, t.expect(key)) }

// Insert creates the document at key, failing with ErrDuplicateKey if
// present.
func (t *Tx) Insert(key string, doc []byte) error {
	if _, err := t.n.get(key); err == nil {
		return fmt.Errorf("kdb: %s %q: %w", t.n.id, key, ErrDuplicateKey)
	}
	if err := t.n.put(key, doc, &kdbserver.Expect{Absent: true}); err != nil {
		return duplicateIfPrecondition(t.n.id, key, err)
	}
	return nil
}

// expect is the precondition for writing key: what this critical section read,
// or nothing when it is writing a key it never looked at (a blind overwrite,
// which is what it asked for).
func (t *Tx) expect(key string) *kdbserver.Expect {
	hash, ok := t.read[key]
	if !ok {
		return nil
	}
	if hash == "" {
		return &kdbserver.Expect{Absent: true}
	}
	return &kdbserver.Expect{ContentHash: hash}
}

// Delete removes the document at key, reporting whether it existed.
func (t *Tx) Delete(key string) (bool, error) { return t.n.deleteByUUID(uuidForKey(key)) }

// Scan streams every document in the namespace.
func (t *Tx) Scan(fn func(doc []byte) error) error {
	return t.n.scan(func(_ codec.UUID, doc []byte) error { return fn(doc) })
}

// MultiTx is a transaction over several namespaces: reads see its own writes,
// and nothing it writes is applied unless the function returns nil.
type MultiTx struct {
	k   *KDB
	nss map[string]*kdbNamespace
	ops []multiOp
	// read is the content hash of everything this transaction has read, keyed
	// ns\x00key; empty means it was not there. Its writes assert it, so a
	// peer's merge landing between the read and the commit refuses the
	// transaction instead of silently overwriting the merged value.
	read map[string]string
}

type multiOp struct {
	ns, key string
	doc     []byte // nil for a delete
}

// UpdateMulti runs fn holding the write lock of every namespace in nss —
// taken in sorted order, so two of them can never deadlock — and commits its
// writes when fn returns nil: one kdb commit when they touch one namespace,
// one cross-namespace transaction when they touch more. Either way they land
// together or not at all, including across a crash.
func (k *KDB) UpdateMulti(nss []string, fn func(tx *MultiTx) error) error {
	names := append([]string(nil), nss...)
	sort.Strings(names)
	held := make(map[string]*kdbNamespace, len(names))
	for _, name := range names {
		if _, dup := held[name]; dup {
			continue
		}
		n := k.ns(name)
		n.mu.Lock()
		defer n.mu.Unlock()
		held[name] = n
	}
	// Re-run on a precondition failure for the same reason Update does: with
	// every lock held, the only thing that can have moved a document under
	// this transaction is a peer's merge, and the answer is to decide again on
	// what the merge left.
	var err error
	for attempt := 0; attempt < kdbUpdateAttempts; attempt++ {
		tx := &MultiTx{k: k, nss: held, read: map[string]string{}}
		if err = fn(tx); err != nil {
			return err
		}
		if err = tx.commit(); !isPrecondition(err) {
			return err
		}
	}
	return err
}

// commit turns the buffered writes into one transaction per namespace.
//
// A put is a delete followed by a write of the whole body. A write onto an
// existing document is a shallow merge, which would keep a field the model
// has since dropped; the engine applies a transaction's operations in order,
// so deleting first makes the write a replacement — the same whole-document
// semantics put gets from PutJSONDocument.
func (t *MultiTx) commit() error {
	type final struct {
		key string
		doc []byte
	}
	byNS := map[string][]final{}
	seen := map[string]int{}
	for _, op := range t.ops {
		id := op.ns + "\x00" + op.key
		if i, ok := seen[id]; ok {
			byNS[op.ns][i].doc = op.doc
			continue
		}
		seen[id] = len(byNS[op.ns])
		byNS[op.ns] = append(byNS[op.ns], final{key: op.key, doc: op.doc})
	}

	var parts []kdbserver.NamespaceTransaction
	for _, name := range sortedKeys(byNS) {
		n := t.nss[name]
		var ops []document.Op
		var pres []document.Precondition
		for _, f := range byNS[name] {
			id := uuidForKey(f.key)
			_, _, exists, err := n.srv.GetDocument(n.id, id)
			if err != nil {
				return err
			}
			// The first operation on a document is the one that carries what
			// the caller read: a delete when there is something to replace,
			// otherwise the write itself.
			pre, err := t.precondition(name, f.key)
			if err != nil {
				return err
			}
			if pre != nil {
				pre.OpIndex = len(ops)
				pres = append(pres, *pre)
			}
			if exists {
				ops = append(ops, document.DeleteOp{DocID: id})
			}
			if f.doc == nil {
				continue
			}
			body, err := injectDocID(f.doc, id)
			if err != nil {
				return err
			}
			ops = append(ops, document.WriteOp{DocID: id, Patch: string(body)})
		}
		if len(ops) > 0 {
			parts = append(parts, kdbserver.NamespaceTransaction{
				Namespace: n.id,
				Tx:        document.Transaction{Operations: ops, Preconditions: pres},
			})
		}
	}

	switch len(parts) {
	case 0:
		return nil
	case 1:
		n := t.nss[strings.TrimPrefix(parts[0].Namespace, kdbCatalog+"/")]
		head, err := n.rt.DAG.Head()
		if err != nil {
			return err
		}
		txID, err := codec.RandomUUID()
		if err != nil {
			return err
		}
		tx := parts[0].Tx
		tx.ID, tx.BaseVersion, tx.Timestamp = txID, head, codec.TimestampNow()
		_, err = n.srv.Commit(n.id, tx, "", kdbauth.Principal{})
		return busyIfShed(err)
	default:
		_, err := t.k.set.CommitAcross(parts, kdbauth.Principal{})
		return busyIfShed(err)
	}
}

// precondition is what this transaction read for ns/key, as an assertion for
// the commit to make. Nil when the key was never read: a blind write, which is
// what the caller asked for.
func (t *MultiTx) precondition(ns, key string) (*document.Precondition, error) {
	hash, ok := t.read[ns+"\x00"+key]
	if !ok {
		return nil, nil
	}
	if hash == "" {
		return &document.Precondition{Kind: document.ExpectAbsent}, nil
	}
	h, err := codec.HashFromHex(hash)
	if err != nil {
		return nil, err
	}
	return &document.Precondition{Kind: document.ExpectContentHash, ContentHash: h}, nil
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func (t *MultiTx) held(ns string) *kdbNamespace {
	n, ok := t.nss[ns]
	if !ok {
		panic(fmt.Sprintf("kdb: namespace %q is not part of this transaction", ns))
	}
	return n
}

// Get returns the document at key as this transaction would leave it, and
// remembers what was stored so the commit can assert it is still there.
func (t *MultiTx) Get(ns, key string) ([]byte, error) {
	n := t.held(ns)
	for i := len(t.ops) - 1; i >= 0; i-- {
		if op := t.ops[i]; op.ns == ns && op.key == key {
			if op.doc == nil {
				return nil, fmt.Errorf("kdb: %s %q: %w", n.id, key, ErrNotFound)
			}
			return op.doc, nil
		}
	}
	doc, hash, found, err := n.readForUpdate(key)
	if err != nil {
		return nil, err
	}
	t.read[ns+"\x00"+key] = hash
	if !found {
		return nil, fmt.Errorf("kdb: %s %q: %w", n.id, key, ErrNotFound)
	}
	return doc, nil
}

// Put creates or wholly replaces the document at key when the transaction
// applies.
func (t *MultiTx) Put(ns, key string, doc []byte) {
	t.held(ns)
	t.ops = append(t.ops, multiOp{ns: ns, key: key, doc: append([]byte(nil), doc...)})
}

// Delete removes the document at key, if there is one, when the transaction
// applies.
func (t *MultiTx) Delete(ns, key string) {
	t.held(ns)
	t.ops = append(t.ops, multiOp{ns: ns, key: key})
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
// KdbServerRuntime.PutJSON rather than the runtime's Upsert: Upsert routes
// the body through a WriteOp, and a WriteOp onto an existing document is a
// shallow *merge* — a field the model dropped (a cleared suspension, an
// emptied option) would silently survive. PutJSON swaps the whole body,
// which is what ReplaceOne semantics require.
//
// PutJSON — rather than embed.PutJSONDocument, which this used to call —
// takes the engine's write gate, so a local write is serialized against peer
// ingest as well as against every other local writer. That matters the
// moment a namespace replicates: a merge landing from a peer and a put from
// here would otherwise race for the branch head, and the loser used to come
// back as "branch main moved". It also goes through the engine's own memory
// admission, so the manual Acquire this function used to do is gone with it.
//
// expect, when non-nil, is the value the caller read and decided on: the
// write fails with *kdbserver.PreconditionFailedError if the document has
// changed since. Callers holding n.mu need it only against peers — see
// Update, which retries on it.
func (n *kdbNamespace) put(key string, doc []byte, expect *kdbserver.Expect) error {
	id := uuidForKey(key)
	withID, err := injectDocID(doc, id)
	if err != nil {
		return err
	}
	if _, err := n.srv.PutJSON(n.id, id, string(withID), expect, kdbauth.Principal{}); err != nil {
		return busyIfShed(err)
	}
	return nil
}

// readForUpdate returns the document at key together with the content hash to
// hand back as a precondition, and whether it is there at all.
func (n *kdbNamespace) readForUpdate(key string) (doc []byte, hash string, found bool, err error) {
	body, h, found, err := n.srv.ReadForUpdate(n.id, uuidForKey(key))
	if err != nil || !found {
		return nil, "", false, err
	}
	return []byte(body), h, true, nil
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
	// The engine's document id is written as "id", so a model that stored a
	// field of its own under that name would have it silently replaced by a
	// UUID it never chose, and would read back wrong long afterwards. Every
	// model here keys its own id as "_id".
	//
	// A document that already carries a UUID there is one being written
	// again, or moved to another key by a migration, and its id is simply
	// replaced. Anything else is a model naming a field "id", which is worth
	// failing on loudly.
	if existing, ok := root["id"]; ok && string(existing) != string(idJSON) && !isDocumentID(existing) {
		return nil, fmt.Errorf("kdb: document carries its own \"id\" field (%s), which the engine's document id would replace: store it under another name", existing)
	}
	root["id"] = idJSON
	return json.Marshal(root)
}

// isDocumentID reports whether a JSON value is an engine document id, which is
// what tells a document being re-keyed from a model that named a field "id".
func isDocumentID(raw json.RawMessage) bool {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return false
	}
	_, err := codec.UUIDFromString(s)
	return err == nil
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
			// Only node-local namespaces are swept. Every one of these holds
			// something this node issued and this node alone consumes — a
			// refresh token, a login code, an OAuth flow — so none of them
			// replicates, and no peer can be reading what is being deleted.
			// A swept replicated namespace would have every node deleting
			// every other node's expired rows, which is a great deal of
			// commit traffic to say the same thing many times.
			for _, name := range sweptNamespaces {
				k.sweepNamespace(k.ns(name))
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
