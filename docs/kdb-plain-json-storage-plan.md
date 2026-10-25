# Plain JSON for kdb storage; BSON only for Mongo

Status 2026-10-15 (later): **collapsed into one step.** Nothing has shipped to
players' devices yet, so there is no installed base that cannot read plain
JSON: the staged N / N+1 rollout below is unnecessary. On the integration
branch `claude/kdb-080-all` (worktree `/Users/davidj/devel/kdb080-all/zolik`,
kdb `3229aa64`) plain JSON is the default write format, `extjson` survives
only as an escape hatch, and the server, iOS core, Android core and Mac app are
all built from that one pair. The sections below are the original plan and the
first night's results, kept for the reasoning.

## The problem

`FEATURE_FLAG_DB_ENGINE` picks the store. On `mongo` the Go driver encodes
structs straight to binary BSON with the `bson:` tags; there is no text step,
and that path is fine. On `kdb` (production) every document goes through
`internal/db/kdbdoc.go`:

```go
func MarshalDoc(v any) ([]byte, error)    { return bson.MarshalExtJSON(v, false, false) }
func UnmarshalDoc(data []byte, v any) error { return bson.UnmarshalExtJSON(data, false, v) }
```

So kdb stores MongoDB Extended JSON, made and parsed by the Mongo driver's text
codec. A `MatchResult` today is stored as:

```json
{"_id":{"$oid":"6ad083a30d675ec0fbfb3eff"},
 "startedAt":{"$date":{"$numberLong":"-62135596800000"}},
 "completedAt":{"$date":"2026-10-15T07:00:00Z"}, ...}
```

There is no BSON anywhere on this path except the library used as a text codec,
and that parser (`bson.(*extJSONParser)`) was ~27% of server CPU in the
800-table profile. It is reached from 116 call sites in 11 packages, all of them
through those two functions, which makes the switch point small.

The reason it was done this way still holds and the plan must keep it: the
`bson:` tags name the stored fields, and several fields are `json:"-"` because
they must be stored but never sent to a client (`models.GuestID`, the notify
circle/device fields, `metrics` players, `localsaves` kind/subject, identity
`_id`). Plain `encoding/json` on these structs would rename or silently drop
stored data.

## The rule

| `FEATURE_FLAG_DB_ENGINE` | Stored as | Codec |
|---|---|---|
| `mongo` | binary BSON | the driver, unchanged |
| `kdb` | plain JSON | new storage codec: `bson:` tag names, JSON values |

Plain JSON means: ObjectID as its 24-character hex string, `time.Time` as an
RFC 3339 UTC string (nanosecond precision), int64 as a number, `JSONDoc`
(module state, actions) embedded as the raw JSON it already is rather than a
quoted string, no `$` wrappers.

One extra knob, temporary, for a safe rollout only (below):
`KDB_DOC_WRITE_FORMAT=extjson|json`. It changes only what is written. Reading
always accepts both.

## Design

### Storage codec (`internal/db/doccodec`)

* **Field names and rules from the `bson:` tag**, not the `json:` tag: name,
  `omitempty`, `inline`, `-`. This is what keeps `json:"-"` fields stored.
* **Encoder** writes the plain JSON above.
* **Decoder is tolerant**, and is the only decoder from the first release on.
  Each special type accepts both spellings:
  * ObjectID: `"hex"` or `{"$oid":"hex"}`
  * time: `"RFC3339"`, `{"$date":"RFC3339"}`, `{"$date":{"$numberLong":"ms"}}`
  * int64: number or `{"$numberLong":"n"}`; float: number or
    `{"$numberDouble":"Infinity|NaN|…"}`
  * `JSONDoc`: a raw object/array or a string holding JSON (legacy)
  * anything with a custom `UnmarshalBSONValue` gets an explicit rule (inventory
    in step 1; `JSONDoc` is the known one).
  
  This means no per-document format detection and no fallback parse: a legacy
  document and a new one decode through the same fast path.
* **Implementation choice**, decided by a one-day spike measured on
  `MatchResult` and `Match`:
  1. `json-iterator/go` configured with `TagKey: "bson"` plus type extensions
     for ObjectID, time and `JSONDoc` (least code; adds a dependency).
  2. Generated per-type codecs (`go generate`, easyjson-style, reading `bson:`
     tags) for the hot models only, with option 1 for the rest.
  
  Acceptance for the spike: decode at least 3x faster than
  `bson.UnmarshalExtJSON` on both models, allocations at least halved, and the
  parity tests below green.

### Wiring

`MarshalDoc`/`UnmarshalDoc` keep their signatures. `UnmarshalDoc` becomes the
tolerant decoder unconditionally. `MarshalDoc` writes Extended JSON or plain
JSON per `KDB_DOC_WRITE_FORMAT`. No call site changes. The Mongo repositories do
not call these functions and are untouched; confirm by grep in step 1.

## Steps

1. **Inventory** (half a day). Every type passed to `MarshalDoc`/`UnmarshalDoc`
   (follow the 116 call sites); every type implementing
   `MarshalBSONValue`/`UnmarshalBSONValue`/`MarshalBSON`; every `bson:",inline"`,
   map, `interface{}` and `bson.M` field inside a stored struct; every place that
   reads stored bytes without `UnmarshalDoc` (string matching, `kdb-cli`
   scripts, `cmd/export-games`, `cmd/migrate-*`, the sync/replica bundle code,
   tests with golden documents). Confirm nothing asks kdb to filter or index on
   a field path inside a document (none found so far; uniqueness goes through
   the reservations namespace keyed by natural keys, not document content).
2. **Spike** the codec choice (one day), against the acceptance bar above.
3. **Codec and parity tests** (two to three days). For every inventoried type:
   * legacy bytes from `bson.MarshalExtJSON` decode through the new decoder to
     a value deep-equal to `bson.UnmarshalExtJSON`'s;
   * new encode then decode is the identity;
   * every `json:"-"` field survives a round trip (the silent-drop guard);
   * edge values: zero `time.Time` (stored today as `$numberLong`
     -62135596800000), times with nanoseconds, zero/empty ObjectID, nil vs empty
     slices and maps under `omitempty`, int64 near 2^63, `JSONDoc` nil/empty/
     large, NaN/Inf if any float field can hold them;
   * a fuzz test feeding random values of each type through both paths.
   Plus a benchmark per hot model: Extended JSON vs new, decode and encode.
4. **Release N: reader only.** Ship the tolerant decoder with
   `KDB_DOC_WRITE_FORMAT` defaulting to `extjson`. Nothing on disk changes, so
   rollback is free. This must include the **phone and Mac builds**: they embed
   the same core, and documents replicate between phone and hub, so every node
   must be able to read plain JSON before any node writes it.
5. **Flip writes on dev.** `KDB_DOC_WRITE_FORMAT=json` on the dev stack; run the
   full Playwright suite, a soak, and the 2-CPU/4 GB capacity ramp against the
   CPU baseline. Check a mixed volume (old rows plus new rows) reads correctly,
   and that a phone on release N syncs with it.
6. **Release N+1: default `json`.** Production writes plain JSON from then on;
   documents convert lazily as they are rewritten. Old rows stay readable for
   ever through the tolerant decoder.
7. **Optional full conversion.** A `cmd/migrate-doc-format` offline rewriter
   (same shape as `cmd/migrate-match-log`: dry run first, run against a stopped
   server or a copy) if we want every row converted, for example before
   dropping the legacy branches of the decoder. Not needed for correctness.
8. **Release N+2: remove the knob.** `FEATURE_FLAG_DB_ENGINE` alone decides:
   `mongo` = BSON, `kdb` = plain JSON.

## Rollback

* N+1 back to N: safe; N reads both formats.
* Anything back to a build before N, after N+1 has written: **not safe**. A
  pre-N build cannot read plain-JSON rows (it expects `$oid`/`$date`). Same
  one-way rule as kdb attachments; write it into the release notes for N+1.
  Setting `KDB_DOC_WRITE_FORMAT=extjson` stops new plain-JSON writes but does
  not convert rows already written.

## Risks

* **Silent field loss** from a tag rule the new codec reads differently
  (`omitempty` on structs and zero times differs between the bson and json
  conventions; `inline`). Guarded by the `json:"-"` round-trip tests and the
  legacy-equivalence tests on every inventoried type, not on a sample.
* **Mixed-version sync**: a phone on a pre-N build receiving plain-JSON
  documents. Guarded by the N-before-N+1 ordering, and by refusing (or holding
  back) sync to a peer older than N if the sync handshake can see versions.
* **Precision**: `time` must keep nanoseconds the Extended JSON path dropped to
  milliseconds; decide whether equality tests compare at ms. Large int64 seeds
  live in `JSONDoc`, which the codec never parses.
* **Tools that read stored text** (`kdb-cli`, export, migration commands, debug
  endpoints) need the new shape or the shared decoder; covered by the
  inventory.

## Effort and payoff

About a week of work before release N (inventory, spike, codec, tests), then
two release cycles of calendar time for the rollout. The payoff is the ~27% of
CPU now spent in the Extended JSON parser, less whatever the sign-in fix
already removes, and smaller documents on disk. The capacity ramp in step 5 is
where that is measured.

Independent of the CPU work in progress (sign-in lookup, decode-once
broadcast); either can land first.

## Implementation and results (2026-10-15)

**What is in the branch (release N of the plan):**

* `internal/db/doccodec`: the storage codec, no new dependency. Reads `bson:`
  tags (name, `omitempty`, `inline`, `-`), writes plain JSON, and reads plain
  JSON, relaxed Extended JSON and a mix of both, value by value. Copies the
  driver's rules found by probing it: default key is the lowercased field name;
  `omitempty` drops zero times and ObjectIDs (they have `IsZero`) but not
  all-zero structs; nil slice/map is `null`; an embedded struct of an
  unexported type is skipped; a nil top-level map is `{}`; times are UTC to the
  millisecond; decoding into a used value keeps absent fields, merges maps and
  replaces slices; an untyped `$date` reads as `bson.DateTime`; empty binary
  reads as empty, not nil.
* `models.JSONDoc` stores itself raw when it is exactly a valid JSON object or
  array, and as a string otherwise (so a stored string always means "the
  document's text", unambiguous against legacy rows).
* `db.MarshalDoc`/`UnmarshalDoc` (`internal/db/kdbdoc.go`): reading always goes
  through doccodec; writing follows `KDB_DOC_WRITE_FORMAT` (`extjson` default,
  `json`), validated at startup (a typo refuses to start) and logged
  (`kdb documents: read extjson+json, write …`). No call site changed.
* Tests: `doccodec` rule tests (every tag rule, edge values, mixed rows, reuse
  semantics, malformed input, refused shapes); `doccodectest.Check`, a
  random-fill parity harness, called from a `storeddoc_parity_test.go` in each
  of the 10 packages that store documents, covering every stored type found in
  the inventory (about 30, including unexported ones) - for each: legacy bytes
  read by doccodec == read by the driver, and doccodec's own bytes read back the
  same; replica test (client JSON identical for both shapes, history sort key
  identical); sync test (plain vs Extended JSON `updatedAt` compare correctly).
* Benchmarks (`storeddoc_bench_test.go` in stats and match).

**Inventory findings worth knowing:** the replica's `completedAt` sorts stored
times as text and `asClientJSON` decodes into `bson.M`; both give identical
results for the two shapes (tested). `sync/authority.go` already parsed both
time shapes. Nothing asks kdb to index or filter on a field path inside a
document. `json.RawMessage` fields (sync bundles, legacy match parts) are
binary to BSON and stay base64 in plain JSON.

**Full suite** (`ZOLIK_TEST_DB_ENGINE=kdb go test ./...`, once with each write
format): every package passes except two failures that also occur without
this change - `internal/canasta` `TestSelfPlaySweepNeverStalls` (40 min
timeout; it timed out the same way on origin/main + kdb v0.8.0), and
`mobile/zolikcore` `TestAGuestTableAnswersOnLoopbackOverBluetooth` (flaky
"bad handshake"; failed 1 of 18 runs on origin/main and 2 of 18 here).

**Microbenchmarks** (Apple M3 Max, noisy machine):

| | driver Extended JSON | doccodec, legacy bytes | doccodec, plain bytes |
|---|---|---|---|
| MatchResult decode | 24.1 us, 479 allocs | 4.7 us, 36 allocs | 4.1 us, 31 allocs |
| MatchResult encode | 12.0 us, 207 allocs | - | 2.0 us, 3 allocs |
| Snapshot (3 KB state) decode | 39.7 us | 18.0 us | 10.5 us |

Stored snapshot size: 4,264 B legacy, 3,167 B plain (the state is no longer an
escaped string).

**Server, 2 CPU / 4 GB, copy of the dev data, same kdb, one build at a time:**

| | origin/main | branch, writes extjson | branch, writes json |
|---|---|---|---|
| 100 guest sign-ins, p50 (two rounds) | 67.3 / 67.4 ms | - | 17.2 / 17.5 ms |
| 400 tables: p50 / p95 / p99 | 3 / 80 / 480 ms | 4 / 38 / 84 ms | 3 / 32 / 75 ms |
| 800 tables: p50 / p95 / p99 | 498 ms / 1.39 s / 2.12 s | 550 ms / 1.18 s / 1.50 s | 527 ms / 1.11 s / 1.44 s |

Reading the numbers honestly: in steady play the stored-row decode is about 1%
of CPU in every build (profiles taken 50-80 s into each run, after the
sign-in burst). The 27% measured on 2026-10-14 was the guest sign-in scan of
`match_results`, which decodes every row; this change makes that scan 3.8x
faster, and the separate CPU branch (`claude/cut-signin-and-broadcast-cpu`)
removes the scan altogether. At 800 tables every build is CPU-bound on the
per-viewer state decode that the CPU branch fixes; the codec does not move
that. The 400-table p95/p99 gains are real in direction but the runs were
sequential on a shared machine, so treat their size as approximate.

**Not done:** step 5 (flip dev to `json`), the phone/Mac release ordering
(release N must reach every client before any node writes `json`), the
optional offline converter (step 7), and removal of the knob (step 8). The
Docker Hub registry timed out during the night, so the comparison images were
built by layering locally compiled amd64 server binaries onto an existing
image (`FROM zolik-fix-app`), the same binary shape the normal image ships.
