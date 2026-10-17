# Release 1.8.0: production upgrade plan

Production today: `1.7.2.7+07f45ce` on kdb **v0.7.1** (`2d63fd9`), on all three
domains. This release moves it to zolik **1.8.0** on kdb **v0.9.1**
(`93a1996e`; server code identical to v0.9.0 `1aaf4eb4`).

## What changes

| Since 1.7.2.7 | Effect in production |
|---|---|
| kdb v0.7.1 → v0.9.1 | New storage layout, migrated in place on first open (below). Rollback of kdb alone to v0.8.2 is tested safe, but see "Rollback": the JSON change is the one-way part. |
| #346 plain-JSON documents | New and rewritten rows are stored as plain JSON. Old rows keep reading. **A build from before 1.8.0 cannot read plain-JSON rows.** |
| #346 finished matches close their namespace | Heap no longer grows with every match dealt since boot. |
| #346 sign-in index (`match_subjects`) | Guest sign-in stops scanning every match result. Built once, lazily, the first time it is needed. |
| #347 offers once per state, one probe copy | Roughly half the CPU per move. |
| #342, #343, #344, #345 | Client: Hints reachable when Controls is minimised, back arrow / browser-tab arrows, card values in the rules, a shorter pick sound. |

## Data migration

**No offline step.** Everything converts on its own:

- **kdb storage.** On first open, v0.8+ writes each namespace's tree segment and
  graph caches (the first open of a large namespace is slower, once), and
  rewrites v3–v5 checkpoints as v6 at the next checkpoint. Measured on the dev
  volume (~400–600 MB): no visible delay.
- **Documents.** Every reader accepts both the old Extended JSON and plain
  JSON, value by value; rows become plain JSON as they are rewritten. Nothing
  scans or rewrites the whole store.
- **Sign-in index.** The first sign-in that needs it scans `match_results` once
  and records the index; match recording waits for that one scan.

**Flags: none to change.** `deploy/compose/zolik.yml` needs no edit. Plain JSON
is the default (`KDB_DOC_WRITE_FORMAT` unset = `json`). The kdb graph opt-outs
discussed for v0.8.0 (`KDB_GRAPH_FILE=off`, `KDB_GRAPH_MEMORY=go`) are **not**
needed: v0.8.1 fixed the per-namespace 1 MiB chunk. `FEATURE_FLAG_*` unchanged.

One escape hatch exists and should stay unused: `KDB_DOC_WRITE_FORMAT=extjson`
makes 1.8.0 keep writing the old shape, which keeps a rollback to 1.7.2.7
possible without restoring a backup, at the cost of the CPU win on reads. Set it
in the host's `.env` *before* the deploy only if a no-restore rollback must stay
available.

## Pre-flight (read-only, on the host)

```bash
ssh davja@192.168.13.13 'docker ps --format "{{.Names}} {{.Image}} {{.Status}}"
docker run --rm -v zolik_kdb_data:/d:ro alpine:3 du -sh /d/kdb
df -h /var/lib/docker /home/zolik | tail -2
ls -1t /home/zolik/backups | head -3
docker logs zolik-app-1 2>&1 | grep -iE "corrupt|merge failed|could not compact|panic" | tail -5'
```

- Free disk must be at least **3× the data size**: the `--snapshot` tarball,
  the new tree/graph caches, and room for v0.9's larger bulk-commit bodies.
- A `could not compact … negative offset` line means a damaged SSTable like
  the one found on dev. On v0.9.1 it is reported as `corrupt table` during
  background merge and skipped; reads and writes are unaffected. It is a
  reason to note, not to stop.

## Deploy

1. Merge the 1.8.0 bump PR. Stage the pinned pair at the **merge** commit:
   ```bash
   git -C /Users/davidj/devel/zolik worktree add --detach /Users/davidj/devel/release-1.8.0/zolik <merge-sha>
   git -C /Users/davidj/devel/kdb worktree add --detach /Users/davidj/devel/release-1.8.0/kdb v0.9.1
   ```
2. `go build ./... && go test -short ./...` from the pair; rebuild zolik-dev
   (:8191) from it and play a guest game there.
3. In a quiet hour (the container restarts; tables pause and reconnect), run
   detached and keep the exit code:
   ```bash
   cd /Users/davidj/devel/release-1.8.0/zolik
   nohup sh -c 'scripts/deploy.sh --snapshot > ../deploy.log 2>&1; echo "rc=$?" >> ../deploy.log' &
   ```
   `--snapshot` tars `zolik_kdb_data` to `/home/zolik/backups` before the swap.
   That tarball is the rollback point; note its name.

## Verify (first hour)

- `deploy.sh`'s own `==> verification` block (healthz, `/version`,
  `/auth/login 200`, `/users/me 401`): read the log, not the exit status.
- `/version` on jokerless.com, jokerless.org and play.limidus.com all read
  `1.8.0.1` and the merge commit; host `docker ps` shows the new tag; the
  About footer's two version lines in a browser (the web bundle).
- `docker logs zolik-app-1`: `kdb documents: read extjson+json, write json`,
  no `panic`, no repeated `ERROR`.
- Sign in as a guest, play a bot game to the end, reopen it from Previous games.
- Memory: `docker stats` against the 2 GiB limit. Expect live heap in the
  hundreds of MB, flat over the hour rather than climbing with matches.

## Rollback

- **Within 1.8.0** (a bad client change, say): deploy an earlier 1.8.x image.
  Same storage, nothing to restore.
- **Back to 1.7.2.7** (`zolik:1.7.2.7-07f45ce`, kept on the host by
  `deploy.sh`): 1.7.2.7 cannot read rows 1.8.0 has written as plain JSON. So:
  stop the container, restore the `--snapshot` tarball into `zolik_kdb_data`,
  then start 1.7.2.7:
  ```bash
  ssh davja@192.168.13.13 "sudo -u zolik bash -c 'cd /home/zolik && docker compose down && \
    docker run --rm -v zolik_kdb_data:/d -v /home/zolik/backups:/b alpine:3 sh -c \"rm -rf /d/* && tar xzf /b/<snapshot>.tgz -C /d\" && \
    ZOLIK_RELEASE=1.7.2.7-07f45ce docker compose up -d'"
  ```
  Everything written after the snapshot is lost. The only way to avoid that is
  the `KDB_DOC_WRITE_FORMAT=extjson` hatch above, chosen before the deploy.

## Capacity on this release

Measured on a 2 CPU / 4 GB container, one client and two bots per Žolíky
table: p95 move round trip 5 ms at 400 tables, 9 ms at 800, ~0.5 s at 1,200;
saturation near 1,600. Production's container is 2 GiB; live heap at 800 tables
was ~730 MB.
