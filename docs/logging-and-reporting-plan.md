# Logging and reporting — what the operator can actually see

- **Baseline:** `main` @ `6d6ebc5`
- **Goal:** answer, from one `curl`, how much was played today / this week / this month, by how
  many people, how many of them were new, how many tables were walked away from, how often the
  process died, and how often somebody was turned away because the box was full.

The deployment this has to fit is one container, one Go binary, one embedded database, 1 GiB,
one operator ([`docker-deploy-plan.md`](./docker-deploy-plan.md)). So the shape of the answer is
not a metrics pipeline. It is **a counter document per day**, written by the process that
already knows the number, and read back by one endpoint that buckets days into weeks and months.

No Prometheus, no Grafana, no Loki, no sidecar, no second container. Those are the right answer
for a fleet and the wrong answer for a box where the scrape target and the dashboard would be
competing with the game for the same 1 GiB. This is said here so that it is a decision on the
record rather than an omission somebody later "fixes".

---

## 0. What is already there, and what is missing

Half of this ask is already recorded and merely unqueried. Building it again would be the
expensive mistake, so the audit comes first.

| The question | Recorded today? | Where |
|---|---|---|
| Games played per day/week/month | **Yes** | `stats.MatchResult`, one immutable row per finished match, with `moduleId`, `completedAt`, `durationSeconds` |
| Which game, how long | **Yes** | same row |
| Total users, new users | **Yes** | `models.User.CreatedAt` |
| Active users per day | **No** | `User.LastSeenAt` is a moving field — it answers "active in the last N days, as of now", and can never answer "who played on the 3rd" |
| Guests playing | **No** | guests are `models.Session` rows with a `GuestID`; they finish matches and write no user row at all |
| Connections refused under load | **Partly** | `admission.Snapshot.Refused` counts them by `Reason` — **in memory**, reset by every restart, with no notion of a day |
| Crashes | **No** | nothing records that the process died; `restart: unless-stopped` brings it back silently |
| Games not finished | **No** | see below — the field exists and nothing reads it |

Three of those gaps are worth naming properly.

**Refusals evaporate.** `admission.Controller` already counts every turned-away arrival by
reason (`at_capacity`, `memory_pressure`, `waiting_room_closed`, `cpu_pressure`) and serves the
tally on `/healthz/capacity`. But the counter lives in a `map` on the controller. The one moment
an operator most wants that number — after the box was hammered — is the moment it is most
likely to have been zeroed by the restart that hammering caused.

**A crash cannot report itself.** A process that is OOM-killed does not get to write "I died".
The only honest detection is on the *next* boot: leave a marker saying "running", clear it on
graceful shutdown, and a boot that finds the previous marker uncleared knows the previous
process did not exit on purpose. That is the mechanism in §3.

**Unfinished games are a dead end in the code, not just in the reporting.**
`match.SuspendOnDisconnect` sets `Status = "suspended"`, `SuspendedAt`, and `AbandonAt = now +
AbandonWindow` — and **nothing anywhere reads `AbandonAt`**. `rules.StatusAbandoned` exists as a
constant and is never assigned to a `models.Match`. So a table whose player closed the tab sits
in `suspended` forever: not finished, not abandoned, not counted, not swept. "How many games
were not finished" cannot be reported because the runtime never decides that any game wasn't.
Fixing the report means fixing that first (§4), which is a real bug this plan happens to close.

### The stale admin branch

The `admin-ui` branch (`36d0001`, last touched 2026-08-25, four commits ahead of a `main` that
has since moved 78 commits) already contains an admin console, a login, a feedback triage
screen, and `stats.UsageSummary` — matches per day, per module, gap-filled. It is worth reading
before writing §5, and it is **not worth merging**: it predates the KDB port, so its
`UsageSummary` is a method on `mongoRepository` with no KDB counterpart, which means it does not
run on the engine production actually uses. Take its shape — particularly its insistence that
`ByDay` be gap-filled so a chart cannot close a gap and imply play that never happened — and
leave the branch where it is.

---

## 1. The rule this plan enforces

> **Every reportable number is either derivable from a row that already exists, or is a counter
> on one document per UTC day. Nothing else is stored, and nothing is stored twice.**

Consequences, stated so they are not quietly traded away later:

- **No event log table.** One row per refusal, per connect, per hand would be the natural
  instinct and it is wrong here: it puts unbounded write volume in front of an embedded database
  sized for gameplay, to answer questions that are all counts.
- **No weekly or monthly rollups.** A week is a sum of seven daily documents, computed in Go on
  read. Storing it would be a second source of truth that can disagree with the first.
- **Days are UTC**, matching how `stats.MatchResult` is already bucketed, and the endpoint says
  so in its own response rather than letting a reader assume local time.
- **A number that cannot be exact says so** rather than being silently approximate — see the
  distinct-player cap in §2.

---

## Phase 0 — Structured logging, and nothing else

**Thesis.** The logging half of the ask is one afternoon, because Go made it one afternoon.

Today: 17 files import stdlib `"log"` and call `log.Printf` with hand-formatted `match=%s
player=%s` strings. No levels, so nothing can be turned down. No structure, so `docker logs` is
`grep`. No request or match correlation beyond whatever the format string happened to include.

**Change.** A new `internal/obs` with one job: build a `*slog.Logger` — `slog.NewJSONHandler` on
stdout when `APP_ENV` is not `local`, `slog.NewTextHandler` when it is, level from `LOG_LEVEL`
(default `info`) — and call `slog.SetDefault` in `cmd/server/main.go` before anything else runs.

The reason this is cheap: `slog.SetDefault` also redirects the standard `log` package's default
logger through the slog handler. **Every existing `log.Printf` call site keeps compiling and
starts emitting structured records at `INFO` with no edit.** So Phase 0 is one new package, one
line in `main.go`, and zero changes to the 17 files.

Then, and only at the boundaries this plan actually reports on, replace the format strings with
attributes:

```go
slog.Info("admission refused", "class", class, "reason", reason, "live", live)
slog.Info("match abandoned", "match", id, "module", moduleID, "round", n)
slog.Warn("previous run did not exit cleanly", "bootId", prev.ID, "uptime", prev.Uptime)
```

**Deliberately not in scope:** log shipping, rotation, retention, tracing, correlation ids.
`docker logs` on one host with journald behind it is the pipeline. When there is a second host,
revisit; there is no second host.

---

## Phase 1 — The daily counter document

**One collection, `daily_metrics`; one KDB namespace, `NSDailyMetrics = "daily_metrics"`; one
document per UTC day, keyed by its own `2006-01-02` string.**

```go
// Day is every counter for one UTC calendar day, keyed by the day itself so a
// read for a date range is N direct gets and never a scan.
type Day struct {
    Date     string           `bson:"_id"      json:"date"`
    Counters map[string]int64 `bson:"counters" json:"counters"`
    // Players is the distinct set of subject keys seen playing that day —
    // a set rather than a counter because DAU is not additive, and because a
    // week's distinct players is the union of seven of these, which a counter
    // could never give.
    Players []string `bson:"players,omitempty" json:"-"`
    // PlayersTruncated is set when the set hit playerSetCap. The report then
    // renders the day's distinct-player figure as a floor ("≥ 5000") rather
    // than as a number that is quietly wrong.
    PlayersTruncated bool `bson:"playersTruncated,omitempty" json:"-"`
}
```

**Why a set for players and counters for everything else.** Distinct-players-per-day is the one
question in the ask that a counter genuinely cannot answer: incrementing per connect
double-counts a reconnecting player, and `User.LastSeenAt` is destroyed by the next visit. A set
of subject keys is small at this scale — a few hundred short strings, a few KB a day — and it
makes weekly and monthly distinct counts a union rather than an invention. `playerSetCap` (start
at 5,000) bounds the worst case, and `PlayersTruncated` is what stops the number lying if it is
ever hit.

**Write path.** A `metrics.Sink` with a deliberately tiny surface:

```go
type Sink interface {
    Add(name string, n int64)        // atomic, in-memory
    SeePlayer(subjectKey string)     // atomic, in-memory
}
```

Callers touch memory only. A background flusher folds the accumulated deltas into today's
document every `flushInterval` (30s) with an upsert-and-`$inc`, and once more from the shutdown
path. Rationale: a refusal happens exactly when the box is least able to afford an extra write,
so the refusal path must not write. Losing up to 30 seconds of counters to a crash is an
accepted, stated cost — and the crash itself is recorded by Phase 3, which does not go through
this path.

**Counter names are `const` string literals in one file**, `metrics/names.go`, with a test that
walks the AST and fails on an `Add` call whose argument is not one of them. The house already
learned this lesson with server message keys, where a key passed through a variable silently
dropped out of the manifest; a metric name assembled at runtime is the same bug with a quieter
failure — a column that simply never appears in the report.

**KDB note.** This adds the tenth namespace. Since `9af8174` the rescue reserve is one reserve
sliced across namespaces rather than 48 MiB each, so a tenth costs a slice and not 48 MiB;
`TestRescueReserveIsSharedAcrossNamespaces` is what keeps that true and must stay green. A year
of documents is 365 rows, so even a full `Scan` is trivial — but reads here are direct gets by
date, so it never comes to that.

**Both engines, as always.** `mongoRepository` and `kdbRepository`, the same interface, and the
existing `internal/dbperf` shape if the write path is ever suspected.

---

## Phase 2 — Instrument the four things asked for

| Counter | Incremented where |
|---|---|
| `matches.created` | lobby creates a `models.Match` |
| `matches.started` | status → `active` |
| `matches.completed` | `stats.Recorder.RecordMatch` succeeds — the same moment the permanent record is written, so the two can never disagree |
| `matches.abandoned` | the reaper, Phase 4 |
| `matches.completed.<moduleId>` | alongside `matches.completed` |
| `users.registered` | account creation |
| `sessions.guest` | a guest id is minted for the first time |
| `admission.refused.<reason>` | `Controller.reject`, one line, next to the counter that already lives there |
| `admission.refused.matchstart` | `AllowMatchStart` says no — a player told "not now" without a socket ever being opened |
| `ws.connected` | successful `Admit` |
| `boot` / `boot.unclean` | Phase 3 |

`SeePlayer` is called once per match participation with the subject key `stats` already computes
(`stats.Subject`), which is what makes guests, registered users and AI difficulties countable in
the same set with the vocabulary the rest of the statistics stack already uses.

**Games played does not become a counter's job to be right about.** `matches.completed` is a
convenience so the daily report is one read; `stats.MatchResult` remains the source of truth, and
a `cmd/rebuild-daily-metrics` one-shot recomputes every match-related counter from those rows.
That tool is the thing that makes a counter bug survivable, and it is Phase 2's deliverable, not
a later nicety.

---

## Phase 3 — Boot records, and knowing that the process died

**Collection `boots` / namespace `NSBoots`.** One document per process start:

```go
type Boot struct {
    ID        string     // ULID minted at start
    Version   string     // buildinfo
    StartedAt time.Time
    // StoppedAt is written by the shutdown path. Nil on a row that was never
    // closed, which is exactly the signal: the process did not get to run its
    // shutdown.
    StoppedAt *time.Time
    // Reason is "signal" for SIGTERM/SIGINT. Absent means unknown, which
    // together with a nil StoppedAt is what "crashed" is made of.
    Reason string
}
```

- `app.New` writes the row with `StoppedAt` nil.
- The existing signal handler in `cmd/server/main.go` sets `StoppedAt` and `Reason: "signal"` —
  before `a.Close`, so a slow database teardown cannot lose the fact of a clean exit.
- On start, before writing its own row, the process reads the most recent previous row. If its
  `StoppedAt` is nil, the previous process died: increment `boot.unclean`, log at `WARN` with the
  dead run's id and uptime, and stamp the old row so the next boot does not count it twice.

This distinguishes the three things an operator conflates as "it went down": a deploy (clean
exit, immediately followed by a boot at a new version), a crash or OOM kill (unclean), and a host
reboot (unclean, several services at once). Uptime per boot falls out for free, and it is the
figure that actually says whether the box is healthy.

**Not in scope:** panics. A recovered panic is a Go-level event that `slog` should log with a
stack; it is not a crash, and counting it as one would make the crash number useless. If panic
counting turns out to be wanted, it is a separate counter with a separate name.

---

## Phase 4 — Unfinished games, which first means finishing the abandon path

**The bug.** `AbandonAt` is written and never read. `rules.StatusAbandoned` exists and is never
assigned. There is no sweeper.

**The fix.** A reaper on the match manager, ticking every 30 seconds:

- Find `status == "suspended"` with `AbandonAt` in the past.
- Move to `status = "abandoned"`, stamp `EndedAt`, increment `matches.abandoned`, log it.
- **Write no `stats.MatchResult`.** An abandoned match has no standings anybody earned, and the
  lifetime aggregates are rebuilt from those rows — a half-played table folded into them would
  corrupt every average it touched. Abandonment is a runtime fact and belongs only to the daily
  counters. This is the same reasoning `stats.MatchResult.Rounds` already carries: a field can be
  recorded for a person to read and still be forbidden to anything derived.
- Existing suspended rows from before this ships are swept on first run. That is a one-time burst
  of `matches.abandoned` on the day of the deploy, and the report should not be read as a spike
  in players walking away. Worth a line in the release note rather than a special case in the
  code.

**Then the reporting definition**, which is the part worth being pedantic about, because
"unfinished" has three meanings and mixing them produces a number nobody can act on:

| Bucket | Meaning | Source |
|---|---|---|
| Completed | played to an outcome | `matches.completed` |
| Abandoned | started, then walked away from | `matches.abandoned` |
| Never started | a lobby that never filled | `matches.created − matches.started` |
| In flight | started today, not yet resolved | derived at read time, and only ever non-zero for today |

The report shows all four, plus a completion rate defined explicitly as
`completed / (completed + abandoned)` — never over `created`, which would count an empty lobby as
a game somebody failed to finish.

---

## Phase 5 — The report

**`GET /admin/report?from=2026-08-01&to=2026-09-08&bucket=day|week|month`**

- **Auth:** a bearer token from `METRICS_TOKEN`, and **the route is not registered when the
  variable is unset**. This is the pattern `ENABLE_DEBUG_ENDPOINTS` already uses in
  `deploy/compose/zolik.yml`, and it means a misconfigured host serves 404 rather than serving
  the numbers. Reviving the `admin-ui` branch's session login is a larger job with a larger
  surface; if an admin console is wanted later it can adopt this endpoint, which is why the
  endpoint is JSON first.
- **Response:** the bucket list, each with `{ start, end, matches: {created, started, completed,
  abandoned, byModule}, players: {distinct, distinctIsFloor}, users: {registered, total},
  admission: {refused byReason, matchStartRefused}, ops: {boots, uncleanBoots} }`, plus a
  top-level `timezone: "UTC"` and the `from`/`to` actually used after clamping.
- **Gap-filled, always.** A day with nothing played is present with zeroes. Borrowed directly
  from the stale branch's `UsageSummary`, which is right about this.
- **Weeks are ISO weeks, months are calendar months, both UTC**, and a partial bucket at either
  end is returned with `partial: true` rather than being dropped or silently short.
- `?format=csv` for the same rows, because the operator's actual next step is a spreadsheet.

**Phase 5b, optional and explicitly deferred:** a single static HTML page served under the same
token that draws four sparklines from that JSON. Worth doing only once the JSON has been read by
a human a few times and the four numbers worth charting are known, rather than guessed at now.

---

## What is deliberately not built

- **Prometheus / Grafana / OpenTelemetry.** A second process and a scrape endpoint on a 1 GiB
  box serving one operator. Revisit if there is ever a second host, which the scaling plan does
  not currently expect.
- **Per-event rows.** See §1.
- **Retention or expiry.** 365 small documents a year. Nothing needs sweeping.
- **Alerting.** The unclean-boot count in a report read weekly is the alerting, until it isn't.
- **Per-user analytics, funnels, cohorts.** Not asked for, and a materially different privacy
  posture: this plan counts, and stores subject keys for one day's distinct count. It does not
  build a profile.

---

## Order of work, and what each phase is worth on its own

| Phase | Deliverable | Standalone value |
|---|---|---|
| 0 | `internal/obs`, slog default | Structured `docker logs` immediately, no call sites touched |
| 1 | `daily_metrics`, both engines, flusher | Nothing visible yet — the substrate |
| 2 | Counters wired, `rebuild-daily-metrics` | Games and users become answerable |
| 3 | Boot records | Crashes become visible, retroactively from the next boot |
| 4 | Abandon reaper | Closes a live bug; "unfinished" becomes a real number |
| 5 | `/admin/report` | The ask, end to end |

Phases 0, 3 and 4 each stand alone and could ship in any order. Phase 4 is the one worth doing
early regardless of the reporting, because a table stuck in `suspended` forever is a defect
whether or not anybody is counting it.

---

## Open questions for the operator

1. **Should guests count as "users"?** They play, they finish matches, they have durable ids, and
   they never appear in the `users` collection. The plan counts them in `players.distinct` and
   separately in `sessions.guest`, and keeps `users.registered` to accounts. Say if "users" was
   meant to mean accounts only.
2. **`METRICS_TOKEN`, or wait for the admin console?** The token is one env var and ships in
   Phase 5. Reviving the console is a bigger piece of work that this endpoint would slot into.
3. **`playerSetCap` at 5,000/day** — high enough to never trip at current volume, low enough to
   bound the document. Lower it if the daily document size matters more than exactness.
4. **Are bots players?** They hold seats and have subject keys. The plan excludes AI subjects from
   `players.distinct` and counts their matches normally, so "how many people played" means people.
