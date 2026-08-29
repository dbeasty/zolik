# KDB port — the database layer on the embedded engine

Companion to KDB's own plan for this integration,
[`kdb-spec-layer12-zolik-gap-analysis.md`](../../kdb/docs/kdb-spec-layer12-zolik-gap-analysis.md),
which this implements from the Zolik side. Where that document's Components
38/40 proposed reaching KDB over a socket (a Go-native server plus a Go
client SDK), this port takes the shorter road its §1.1 opened up: the Go
embed engine (`github.com/limidus/kdb/go`, `kdb/embed` + `kdb/server`) is
complete enough to link **into** the server process — which is also the
lowest-footprint deployment there is. One binary, no database server, no
Redis.

## What switched

Every repository behind the server's storage interfaces has two
implementations, selected at startup by `FEATURE_FLAG_DB_ENGINE`
(`mongo`, the default, or `kdb`; `FEATURE_FLAG_KDB=true` is the boolean
spelling — see `internal/app/config.go`):

| Interface | Mongo | KDB |
|---|---|---|
| `match.Repository` | `repository.go` | `repository_kdb.go` |
| `user.Repository` | `repository.go` | `repository_kdb.go` |
| `stats.Repository` | `repository.go` | `repository_kdb.go` |
| `auth.Store` | `store.go` | `store_kdb.go` |
| `auth.SessionRepository` | `session_repo.go` | `session_repo_kdb.go` |
| `scoring.Repository` | `repository.go` (new — handlers used raw collections before) | `repository_kdb.go` |

`internal/db/kdb.go` is the engine adapter: one embedded runtime per
former collection, documents stored as bson Extended JSON (so `bson` tags
and `json:"-"` server-only fields behave exactly as they do under the Mongo
driver — see `internal/db/kdbdoc.go`), document ids derived
deterministically from natural keys (ObjectID hex, session token,
`provider\x00subject`, subject key), so keyed lookups are direct reads.

## What the engine does not do, and where that landed

The Go engine today has no secondary indexes, no unique constraints, and no
conditional replace (its optimistic-concurrency check is inert on
file-backed runtimes). The contracts Zolik's schema built on those — unique
usernames/emails/identities/match-records, the match version CAS, the
consume-once login code, the take-once OAuth exchange — are enforced in
`internal/db` under per-namespace critical sections. That is *correct*, not
merely convenient, because the KDB deployment shape is single-process by
design and the engine's directory lock refuses a second process on the same
data. Mongo TTL indexes became read-time expiry checks plus a once-a-minute
sweeper. Lookups with no key (join code, verified email, subject history)
are namespace scans — the benchmarks below are what says when that stops
being fine.

## Tests

- Plain `go test ./...` exercises the KDB layer with nothing running:
  adapter tests (`internal/db/kdb_test.go`), per-repo conformance tests
  (`*_kdb_test.go`), and a full-stack boot test
  (`internal/app/kdb_boot_test.go`) that signs in a guest and plays a match
  create/read over HTTP with no Mongo and no Redis.
- The Mongo-backed integration suites run unchanged against the dev stack.
- `ZOLIK_TEST_DB_ENGINE=kdb go test ./internal/auth ./internal/match`
  re-runs those same end-to-end HTTP suites — sign-in races, code
  redemption, session rotation, invites, reconnection — on the embedded
  engine.

## Durability configuration

What an acknowledged KDB write means is configurable since KDB `5129201`
(`perf/fast-sync-and-interval-flush`; the measured matrix and semantics are
in KDB's `docs/benchmarks/write-durability-modes.md`). The adapter reads two
knobs from the environment (`internal/db/kdb.go`, `KDBStorageFromEnv`; an
unrecognised value refuses to start rather than silently changing the
guarantee):

- `KDB_DURABILITY` — `sync` (the default: ack ⇒ synced to the device) or
  `async` (ack ⇒ appended to the commit log in the OS page cache; a
  background flush runs every `KDB_ASYNC_SYNC_INTERVAL_MS` ms — engine
  default 5, `100` reproduces Mongo's default journaling semantics — which
  bounds the crash-loss window).
- `KDB_SYNC_MODE` — `fast` (the default: `F_BARRIERFSYNC` on macOS,
  `fdatasync` on Linux — an acked write survives process and OS crash) or
  `full` (`F_FULLFSYNC`/`fsync` — survives power loss too, at ~4ms per sync
  on Apple SSDs; the only behavior the adapter had before these knobs).

The default, sync+fast, is deliberately a **stronger** guarantee than
Mongo's default write concern (`w:1, j:false` acks from memory and journals
every ~100ms) while sitting in the same latency band — see the table below.
`KDB_DURABILITY=async KDB_ASYNC_SYNC_INTERVAL_MS=100` is the
Mongo-equivalent point in the space, for when even that band matters.

## Performance

`go test ./internal/dbperf -bench . -benchmem` benchmarks both engines over
the paths the server leans on (Mongo rows skip unless the dev stack is up;
for clean numbers run the engines in separate passes — `-bench '/kdb'` and
`-bench '/mongo'` — because an in-binary Mongo driver contends with the
embedded engine and inflates the KDB rows several-fold).
As of KDB `5129201` (`perf/fast-sync-and-interval-flush`, which made the
write-path durability cost configurable; before that, `456c673`/`8fe306d`
fixed the per-commit allocation and added group-commit, and
`01d0654`/`41cf11c` fixed four storage-layer correctness issues the
write-path change had left open — delete not shadowing flushed data, size
accounting drift, unbounded WAL segments, and the integration CI job). KDB
rows are the sync+fast default unless the row says otherwise.

### Insert and read, head to head

`BenchmarkRawInsert`/`BenchmarkRawRead` are the direct comparison: the same
fixed-shape document, written and read by the same key, with **no
application logic in between** — no uniqueness scan, no CAS, no repository
code, just each engine's floor cost for "write one document" and "read one
document by key". The insert runs once per KDB durability mode, because for
a one-document write the durability policy *is* the cost (M3 Max, dev-stack
Mongo on localhost, `-benchtime 3s`):

| Operation | KDB | Mongo | Ratio |
|---|---|---|---|
| **Insert**, sync+fast (default) | 240 µs | 312 µs | same band; KDB's guarantee is stronger |
| **Insert**, async-100ms (Mongo-equivalent semantics) | 61 µs | 312 µs | KDB ~5x faster |
| **Insert**, sync+full (the old hardwired mode) | 4.0 ms | 312 µs | Mongo ~13x faster |
| **Read** one document by key | 409 ns | 313 µs | KDB ~760x faster |

The read row is unchanged in kind: an in-process function call returning
bytes already in memory, against Mongo's unavoidable network round trip
(which is why Mongo's insert and read costs land in the same ~310 µs band
regardless of which one you're doing). The insert rows are the durability
matrix made concrete. The earlier revision of this table showed one KDB
insert row at 4.0 ms — "Mongo ~21x faster" — and that gap was never I/O
speed but a guarantee mismatch: KDB acked only after `F_FULLFSYNC` forced
the write through the drive cache (~4 ms on Apple SSDs), while Mongo's
default write concern acks from memory and journals every ~100 ms. With the
axes configurable, like-for-like comparisons exist in both directions:
sync+fast acks in Mongo's band while still promising "acked ⇒ on the
device" (barrier sync survives process and OS crash — the same guarantee
SQLite and PostgreSQL run with on macOS), and async-100ms makes exactly
Mongo's promise ~5x faster, with no network hop. sync+full remains available
for "acked ⇒ survives power loss", at its honest price.

### Repository-level paths

The table below goes through the actual repository methods the server
calls, so it includes each engine's real application-level cost (Mongo's
unique index vs KDB's locked uniqueness scan, Mongo's filtered replace vs
KDB's locked version check, etc.) — the numbers above are the fairer
apples-to-apples read; these are the honest end-to-end ones:

| Path | KDB (sync+fast) | Mongo |
|---|---|---|
| Session lookup by token (per-request auth) | ~4.1 µs | ~233 µs |
| Match action cycle (load → CAS store) | ~279 µs | ~452 µs |
| Insert match | ~246 µs | ~196 µs |
| Session create | ~201 µs | ~355 µs |
| Stats upsert | ~344 µs | ~704 µs |
| Resolve by join code (100 live matches) | ~0.80 ms | ~305 µs |
| Leaderboard (200 players) | ~3.4 ms | ~2.6 ms |
| History page (300 records) | ~4.1 ms | ~454 µs |

Under the old hardwired sync+full mode every KDB row with a write in it sat
at 4 ms or more — the fsync floor, paid once per commit. Under the sync+fast
default the write paths land in Mongo's own band (and the compound ones —
action cycle, session create, stats upsert — come out ahead, because KDB's
locked read-check-write is in-process function calls while Mongo pays a
round trip per step). What remains slower is what was always slower for a
different reason: the keyless scan-shaped reads (join code, leaderboard,
history), which are Zolik's own full-scan cost and would motivate real
indexes if they ever grew into a bottleneck.

**Allocation history, since it was flagged and then fixed upstream.** The
first pass through this port found the embed engine allocating ~21 MB per
commit — reported to the KDB side. Commit `456c673` fixed it; per-commit
allocation is now in the same order of magnitude as the Mongo driver's:

| Path (KDB) | Before (`0294299`) | After (`456c673`+) | Change |
|---|---|---|---|
| Insert match | 21.5 MB / 882 allocs | 83 KB / 576 allocs | ~260x less |
| Match action cycle | 21.5 MB / 1522 allocs | 113 KB / 1220 allocs | ~190x less |
| Session create | 21.5 MB / 606 allocs | 28.7 KB / 317 allocs | ~750x less |
| Stats upsert | 21.5 MB / 867 allocs | 69 KB / 555 allocs | ~310x less |

Latency did not move on this single-writer benchmark (still fsync-per-commit
durability, a separate cost from the allocation bug); the scan-shaped reads
(leaderboard, history, resolve-by-join-code) were never on the commit path
and are unaffected — their allocation is Zolik's own full-scan cost, not
KDB's, and is what would motivate adding real indexes if these queries ever
became a bottleneck.

## Deployment

- `docker compose up` — the Mongo stack, unchanged (the image now always
  contains both engines; the build pulls KDB source from the sibling
  checkout via a BuildKit named context).
- `docker compose -f docker-compose.kdb.yml up` — the KDB shape: one
  container, one volume.
- Bare: `FEATURE_FLAG_DB_ENGINE=kdb go run ./cmd/server`.

There is no cross-engine data migration: flipping the flag on an existing
deployment starts from an empty database. A migration tool is future work if
an existing Mongo deployment ever needs to move.
