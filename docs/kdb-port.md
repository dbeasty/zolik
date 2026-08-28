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

## Performance

`go test ./internal/dbperf -bench . -benchmem` benchmarks both engines over
the paths the server leans on (Mongo rows skip unless the dev stack is up).
Representative numbers (M3 Max, dev-stack Mongo on localhost), as of KDB
`e2bbc82` (merged: `456c673`/`8fe306d` fixed the per-commit allocation and
added group-commit; `01d0654`/`41cf11c` then fixed four storage-layer
correctness issues the write-path change had left open — delete not
shadowing flushed data, size accounting drift, unbounded WAL segments, and
the integration CI job):

| Path | KDB | Mongo |
|---|---|---|
| Session lookup by token (per-request auth) | ~3.8 µs | ~205-255 µs |
| Match action cycle (load → CAS store) | ~4.2 ms | ~410-424 µs |
| Insert match / session / stats upsert | ~5-8 ms | ~200-260 µs |
| Resolve by join code (100 live matches) | ~0.75 ms | ~209-229 µs |
| Leaderboard (200 players) | ~3.1 ms | ~2.3 ms |
| History page (300 records) | ~3.8 ms | ~430 µs |

The read/write latency asymmetry is the honest shape of the trade: reads are
in-process function calls; every KDB write is an fsynced commit before it is
acked (Mongo acks first and journals on an interval). ~250 durable
writes/second is far beyond a card table's action rate.

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
