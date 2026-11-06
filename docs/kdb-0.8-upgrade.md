# Moving zolik from kdb v0.7.1 to v0.8.0

Status 2026-10-14: built, deployed to dev (`zolik-dev`, 127.0.0.1:8191) as
`1.7.2.16+a622795` on kdb `v0.8.0` (`b85e4cc8`), end-to-end tested. Production
still runs `1.7.2.7+07f45ce`. This document is the plan for production.

## What was verified on dev

| Check | Result |
|---|---|
| `go build ./...` against v0.8.0, no source change | clean |
| Image build from a pinned worktree pair (zolik `a622795`, kdb `v0.8.0`) | clean |
| Open the existing dev volume (393 MB, history=full) under v0.8.0 | opens, no refusal, ~0.4 s to listening |
| Full Playwright suite against the container | 415 passed, 3 skipped, 4 did not run, 3 failed (below) |
| Rollback: the 1.7.2.7 (v0.7.1) image opening the volume v0.8.0 wrote | opens and serves |
| Control: same zolik commit built on kdb v0.7.1, the two failing specs | fail identically |

The three failures: `mobile-board-controls:71` passed on re-run (flake).
`every-game-ui:171` (phone; React error #418, a hydration mismatch) and
`home-waiting-status:120` fail on this commit with kdb v0.7.1 too, so they are
client/test problems on main, not kdb. They passed against the older 1.7.2.7
client, so they arrived in the 9 commits since (the nav/hints changes are the
likely source).

## Feature flags: REVISED - set two opt-outs until the memory regression is fixed

Load testing (botsoak, 2 CPU / 4 GB container, one namespace per match) found
that v0.8.0 costs about 1 MiB of heap per opened namespace: the commit graph's
first record chunk (4096 slots x 256 B) is allocated per namespace, and this
server never closes a finished match's namespace in standalone mode (idle close
is wired only for the sync hub, internal/app/sync.go). Same load:

| build | heap after ~600 matches |
|---|---|
| v0.7.1 | 299 MB |
| v0.8.0 defaults | 3.3 GB after ~2,200 matches (~1.5 MB/match; 72% is `dag.(*nodeStore).put`) |
| v0.8.0 with `KDB_GRAPH_FILE=off KDB_GRAPH_MEMORY=go` | 389 MB |

So until kdb stops allocating a full chunk for a tiny namespace (or zolik closes
finished matches), add to `deploy/compose/zolik.yml` environment:
`KDB_GRAPH_FILE: "off"` and `KDB_GRAPH_MEMORY: "go"` (p95 move latency was
unchanged at 17 ms). This gives up the mapped graph, which matters for one huge
namespace, not for thousands of small ones. The original recommendation below
("change nothing") is superseded for these two options.

### Original assessment (superseded for the graph options)

* `FEATURE_FLAG_DB_ENGINE=kdb` stays.
* The three storage options v0.8.0 turns on (`KDB_GRAPH_FILE`, `KDB_TREE_FILE`,
  `KDB_BOUNDED_LIVE_BODIES`) default to **on** in
  `embed.FileRuntimeOptionsFromEnv`, which `internal/db/kdb.go` already calls.
  So v0.8.0 gets them with no code or compose edit. They are the point of the
  release (data larger than memory) and need no setting.
* Keep them as emergency opt-outs only: `KDB_GRAPH_FILE=off KDB_TREE_FILE=off
  KDB_BOUNDED_LIVE_BODIES=off` restores v0.7.1's in-heap behaviour without a
  downgrade. Do not add them to `deploy/compose/zolik.yml` unless needed.
* `FEATURE_FLAG_KDB_HISTORY_RETENTION=true` with `KDB_HISTORY_MODE=""` stays.
  v0.8.0 does not change history modes; the `none` cutover
  (`docs/kdb-history-none-cutover.md`) remains a separate procedure. Do not
  combine the two.
* `FEATURE_FLAG_SYNC`: unused in production; the v0.8.0 peer-sync limits do
  not apply.
* The server uses no attachments, no SQL and no unique index over an array, so
  the one-way attachment rule, the SQL paging change and the multikey change do
  not reach it. That is also what keeps the rollback below safe.
* `GODEBUG=disablethp=1` is in the image and `mem_limit: 2g` in the compose
  file; leave both.

## Clients

No client in this repository depends on `@kdb/client` or `@kdb/replica`
(nothing in any `package.json`), so the 0.7.x-client-refuses-0.8.0-server rule
does not bite. The clients that carry kdb are the embedded Go cores: the phone
(`scripts/build-mobile-core.sh`) and the Mac app (gobind), both compiled from
`server/go.mod`, whose `replace` points at the sibling `../kdb`. They move to
0.8 by being **rebuilt from the same pinned pair** as the server:

* iOS/Android and macOS builds must come from a pair with kdb at `v0.8.0`, or
  they stay on whatever the sibling checkout happened to hold.
* A device's offline database goes through the same one-time open-and-convert
  as the server's, on the first launch of the new build. Phone data is small,
  so this is quick, but it is one-way in practice: an app rolled back to a
  0.7.1 build replays the log in full.
* Phone-to-hub sync is negotiated per connection, so a 0.7.1 phone and a 0.8.0
  server (or the reverse) still sync, with the 0.8 features (compressed pages,
  squash catch-up) switching on once both ends run 0.8. Ship the server first.
* Squash catch-up (`catchup=auto`, per peer) is the one worthwhile setting for
  offline devices that sync after long gaps. It is not an environment variable;
  `internal/sync` would have to pass it to the sync node. Not done here.
* `KDB_DURABILITY=async` and `KDB_SYNC_MODE` stay as they are (sync; the latter
  has no effect on Linux).

## Production data migration

There is no data conversion step. v0.8.0 migrates in place on first open:
v3-v5 checkpoints are read and rewritten as v6 at the next checkpoint, and the
tree/graph/index caches (`graph/`, `tree/`, `deltaidx/`, `sstidx/`) are built
once. The first open of a large namespace is slower, once. The dev volume
showed no visible cost at 393 MB, but prod's size is unmeasured (see below).

### 0. Pre-flight (read-only; run from your own shell, my ssh read was blocked)

```bash
ssh davja@192.168.13.13 'docker ps --format "{{.Names}} {{.Image}} {{.Status}}"
docker run --rm -v zolik_kdb_data:/d:ro alpine:3 du -sh /d/kdb
df -h /var/lib/docker | tail -1; free -m | head -2
ls -1t /home/zolik/backups | head -3
docker logs zolik-app-1 2>&1 | grep -E "merge failed|could not compact|did not exit cleanly" | tail'
```

* Free disk must exceed **2x the data size** (snapshot tarball plus the new
  caches).
* A "could not compact ... negative offset or length" line is the dev finding
  below. It is existing damage, not a reason to stop.

### 1. Release

1. Bump `VERSION` (`chore(release): X.Y.Z`, body listing commits and
   `kdb v0.8.0 / b85e4cc8`), PR to main, merge.
2. Build and deploy from a **pinned pair** (zolik at the merged commit, kdb
   worktree at tag `v0.8.0`), never the shared checkouts:

```bash
git -C ../kdb worktree add --detach ../release-X.Y.Z/kdb v0.8.0
git worktree add --detach ../release-X.Y.Z/zolik origin/main
cd ../release-X.Y.Z/zolik && scripts/deploy.sh --snapshot
```

3. `--snapshot` tars `zolik_kdb_data` before the image switch; it is the
   rollback point. Deploy during a quiet hour: the container restarts, so
   active matches pause and sockets reconnect.
4. `deploy.sh`'s exit code is masked by pipes; confirm with
   `curl https://jokerless.com/version` (expect the new version and commit).

### 2. Watch (first hour)

* `docker logs` for `merge failed`, `could not compact`, `refus`, panic.
* `/healthz`, then play one guest game on each of the three domains.
* Memory: with `ENABLE_DEBUG_ENDPOINTS=true` only on dev, read
  `/debug/memory?gc=1` there. On prod watch `docker stats` against the 2 GiB
  limit; the cgroup number includes mapped files, so judge by heap and by
  restarts (`docker inspect -f '{{.RestartCount}}'`), not by a high raw figure.
* The first checkpoint after the upgrade writes v6; expect a brief CPU/disk
  bump, not a stall.

### 3. Rollback

Safe because production has no attachments (v0.8.0 only forbids older builds
once a namespace stores its first attachment) and every namespace has its own
full log (`history=full`). Rehearsed on dev: the v0.7.1 image opened the
volume v0.8.0 had written.

```bash
ssh davja@192.168.13.13 "sudo -u zolik bash -c 'cd /home/zolik && ZOLIK_RELEASE=<older-tag> docker compose up -d'"
```

Cost: v0.7.1 cannot read v6 checkpoints, so its first open replays each log in
full (24 s at 600k writes in kdb's own test). Expect a slow start, not data
loss. If the app is not safe to run on the new caches but a downgrade is not
wanted, set the three opt-out variables above instead.

Do **not** roll back if a namespace has joined by snapshot or squash catch-up
(not a case here), and never go below v0.7.1.

If the volume itself is damaged: stop the container, restore the
`backups/kdb-*.tgz` from step 1 into `zolik_kdb_data`, start the older tag.
That loses everything written since the snapshot.

## Open items

* **Damaged table on dev.** `ns/zolik/sessions/sstable/L4/60d3f762-...` (written
  2026-10-07) cannot be compacted under v0.8.0 (`footer overruns the file`) or
  under v0.7.1 (`negative offset or length`). Reads and writes work; only that
  namespace's compaction is stuck. Check prod's logs for the same message. If
  present, report to kdb with the file; a `sessions` namespace is rebuildable
  data, but do not delete anything without a backup.
* **Heap after load.** Dev read 719 MB live heap right after the 425-test run
  (112 MB at boot). Re-read after idle before concluding anything; compare
  with a v0.7.1 control under the same load if it stays high.
* The two failing specs above need their own fix on main.
