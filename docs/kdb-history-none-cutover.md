# Moving production to `KDB_HISTORY_MODE=none`

KDB 0.4.0 added history retention modes. A namespace either keeps every commit
forever (`full`) or keeps the current dataset plus a bounded window and reclaims
the rest (`none`). Nothing runs `none` yet — development and production both
leave `KDB_HISTORY_MODE` unset, which the engine reads as "whatever this
namespace already is", and every namespace anywhere was built before the modes
existed. This is the procedure for changing that.

`FEATURE_FLAG_KDB_HISTORY_RETENTION` is on by default in both compose files, and
that is not the same as choosing a mode. The flag decides whether a named mode
is *listened to*; with no mode named, it changes nothing. Turned off, the server
discards any `KDB_HISTORY_MODE` it finds — the escape hatch if a mode ever needs
to stop being honoured without hunting down where it is set.

Naming a mode is a procedure rather than an edit because **a namespace records
the mode it was built with and refuses to open under the other.** The two modes
do not leave the same bytes on disk. Setting `KDB_HISTORY_MODE: "none"` in
`deploy/compose/zolik.yml` and deploying it does not convert anything: the
server fails on open, the container exits, Docker restarts it, and it does that
forever. The symptom looks like a bad build, which is the worst thing it could
look like.

The flag is transitional. KDB 0.5.0 has already brought the thing that ends it —
see Route C — but nothing in this server calls it yet, so the sharp edge below is
still the one a deployment actually has.

## What it buys, and what it costs

`none` bounds the on-disk footprint. Under `full`, the data directory grows with
the number of commits ever made — every action of every match, permanently.
Under `none` it is a function of the live dataset plus the retention window
(24h by default; `KDB_RETAIN_DURATION` moves it). For a game server whose write
volume is "every card anyone plays", that is the difference between a footprint
that tracks the players and one that tracks the history of all play.

It costs write throughput, because the reclaim work lands in the write path.
Measured on this stack (`dbperf -bench '/kdb'`, kdb `330008d`, single runs,
arm64 — ratios transfer, absolute numbers do not):

| Benchmark | `full` | `none` | Δ |
|---|---|---|---|
| MatchActionCycle | 266,957 ns/op | 815,674 ns/op | +206% |
| SessionCreate | 222,852 ns/op | 400,311 ns/op | +80% |
| StatsUpsert | 231,910 ns/op | 406,346 ns/op | +75% |
| MatchInsert | 236,746 ns/op | 409,659 ns/op | +73% |
| RawInsert | 337,335 ns/op | 467,822 ns/op | +39% |
| every read path | — | — | within noise |

MatchActionCycle is the hot gameplay path, and its number swallowed a whole
compaction pass inside the timed window, so +206% overstates the steady state.
Even so: this trade is footprint for write speed, and it should be taken because
footprint is the constraint on a 1–2 GB host, not because it is faster. It is
not.

## Route A — wipe the data (chosen)

Accepted on 2026-09-09: nothing in production is worth keeping yet. **This
destroys every account, identity, session, match, match result, player stat and
daily metric.** Players are signed out and start over as new guests. The `.env`
on the host — JWT secrets, SMTP, admin console credentials — is not in the
volume and survives.

Order matters: the volume must be gone *before* a `none`-mode container starts,
or the first open crash-loops.

1. **Name the mode.** In `deploy/compose/zolik.yml`, `KDB_HISTORY_MODE: ""`
   becomes `KDB_HISTORY_MODE: "none"`. The flag above it is already `"true"`,
   so this one line is the whole configuration change. Commit it — the compose
   file on the host is written by `scripts/deploy.sh` from the repo, so an
   uncommitted edit deploys but leaves no record of when the host changed
   shape.

2. **Stop the stack and drop the volume.** The site is down from here until
   step 3 — a minute or two.

   ```
   ssh davja@192.168.13.13 "sudo -u zolik bash -c 'cd /home/zolik && docker compose down'"
   ssh davja@192.168.13.13 "sudo -u zolik docker volume rm zolik_kdb_data"
   ```

   `docker volume rm` refuses while a container holds it, which is the ordering
   check doing its job. If it refuses, something is still up — do not force it,
   find out what.

3. **Deploy.** From the repo, as usual:

   ```
   scripts/deploy.sh
   ```

   It ships the image, installs the new `compose.yml`, and starts the container,
   which finds no data directory and builds all eleven namespaces under `none`.
   Compose recreates the named volume on start; nothing needs creating by hand.

4. **Verify the marker, not the health check.** `/healthz` goes green on a
   container that is wrong in every way that matters here, and a fresh empty
   database is exactly what a silent misconfiguration also produces. Read what
   is actually on disk:

   ```
   ssh davja@192.168.13.13 "sudo -u zolik docker run --rm -v zolik_kdb_data:/data alpine:3 \
     sh -c 'find /data -name meta.json -exec cat {} \; -exec echo \; '"
   ```

   Every line must say `"historyMode":"none"`. Eleven namespaces, one marker
   each.

5. **Verify a player can play.** Load https://play.limidus.com, take a guest
   session, start a match against a bot, play one action. That exercises the
   write path the benchmarks say is three times slower, on the host that has to
   run it.

6. **Watch the footprint do the thing it is for.** Nothing is reclaimed until
   commits fall outside the 24h window, so the payoff is not visible on day
   one — it is visible as the absence of unbounded growth over weeks. Baseline
   it now and check back:

   ```
   ssh davja@192.168.13.13 "sudo -u zolik docker run --rm -v zolik_kdb_data:/data alpine:3 du -sh /data/kdb"
   ```

   `/debug/memory` is shut on this host (`ENABLE_DEBUG_ENDPOINTS: "false"`, and
   it should stay shut), so disk is measured from outside the process like this.

### Rollback

The images `deploy.sh` keeps are the rollback path for the *code*, and they are
not a rollback path for this change. The boundary is the KDB version a release
was built against, not its zolik number: anything older than kdb 0.4.0 predates
the retention modes entirely, and pointing it at a `none`-mode volume is
undefined at best. `1.1.1.101` is the first release carrying 0.4.0, so **rolling
back past it means wiping the volume again.** Check what a candidate image was
built with before trusting it — `deploy.sh` keeps only the last three, and they
will not all be on the same side of that line for long.

## Route B — convert instead of wiping

Not taken, recorded because the next time this comes up the data may matter.

KDB ships an offline conversion, and the mismatch error names it:

```
kdb-inspect migrate-history --data-dir /data/kdb --namespace <ns> --history-mode none
```

Offline is literal — the server must be stopped, and it runs per namespace, so
eleven invocations. The obstacle specific to this deployment is that
`kdb-inspect` is not in the image: production runs a distroless container
holding one Go binary and no tooling. Converting in place would mean building
the tool for linux/amd64, shipping it, and running it against the stopped
volume from a container that has both the binary and the volume mounted. That is
a real afternoon, and the reason Route A is chosen while the data is still
disposable.

## Route C — switch it at runtime (KDB 0.5.0, not yet wired up)

The route that makes the other two look like what they are. KDB 0.5.0 added
`EmbeddedKdbRuntime.SetHistoryMode(mode, reclaim)`, and its defining property is
that **the switch destroys nothing**: a namespace moved to `none` holds exactly
the bytes it held a moment earlier. Reclamation is a separate question
(`storage.ReclaimMode` and `CompactHistory`), which is what makes the move
reversible — an operator can turn `none` on, look at what it *would* reclaim,
and turn it back off. An unspecified reclaim mode means *manual* rather than the
global default, precisely so that changing the mode does not start deleting as a
side effect.

Two things stay true even here. `none → full` cannot bring back what a
compaction already removed, and the runtime reports that as `HistoryLost` read
from the segments actually on disk — anything showing it to a person should
carry that through, or "full history" gets read as "the history is back". And
the *open-time* refusal is unchanged in 0.5.0: a `KDB_HISTORY_MODE` that
disagrees with a namespace's marker still fails on open. The live switch is an
API call, not an environment variable, so nothing above changes.

**This server cannot do it today.** Nothing in `internal/` calls
`SetHistoryMode`, and zolik embeds the runtime rather than running KDB's control
plane, so the HTTP endpoint 0.5.0 exposes (`control/retention.go`) is not
reachable either. Wiring it up — an admin-console action that switches the mode
and reports what reclamation would cost — is the follow-up that retires both the
wipe and the flag. Until then Route A is what is actually executable.

## Development

The dev stack has the same defaults as production — flag on, no mode named — so
it is `full` until asked otherwise. To run `none` locally, name it and wipe in
the same breath:

```
ZOLIK_KDB_HISTORY_MODE=none ZOLIK_DEV_DATA=clean scripts/dev-stack.sh up
```

The wipe is not optional: an existing dev volume holds full-mode namespaces and
hits exactly the refusal production would. `dev-stack.sh` recognises that
specific crash and prints the fix rather than letting it read as a broken build.
`ZOLIK_KDB_HISTORY_RETENTION=false` turns the feature off outright.

Which mode a running server actually got is in its startup log
(`kdb history retention: ...`), so answering it no longer means mounting the
volume to read `meta.json`.
