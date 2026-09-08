# Logging and reporting — the operator's console, and the door it lives behind

> **Status: implemented on `feat/ops-observability`, except where noted below.**
> Phases 0–4, 6 and 7 shipped whole. Phase 5 shipped as the console *shell*
> — guard, password login, embedded UI — and deliberately without the
> `admin-ui` branch's account-management screens. See
> [§5 Phase 5](#phase-5--port-the-console-forward) for what that leaves and why.

- **Baseline:** `main` @ `6d6ebc5`, plus the `admin-ui` branch @ `36d0001` ported forward
- **Goal:** answer, from one screen, how much was played today / this week / this month, by how
  many people, how many of them were new, how many tables were walked away from, how often the
  process died, and how often somebody was turned away because the box was full.
- **Constraint that shapes everything below:** that screen is not on the internet.

The deployment this has to fit is one container, one Go binary, one embedded database, 1 GiB,
one operator ([`docker-deploy-plan.md`](./docker-deploy-plan.md)). So the shape of the answer is
not a metrics pipeline. It is **a counter document per day**, written by the process that
already knows the number, read back by the admin console that already exists on a branch, and
served from **a second listener that nginx does not proxy and Docker does not publish to the
world**.

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

---

## 0.5 The admin console already exists, on a branch, and is the right home for this

The `admin-ui` branch (`36d0001`) has a working operator console: an embedded single-page app
under `/admin`, a bcrypt password login, an email allow-list, a rate limiter, user management,
and `stats.UsageSummary` — matches per day, per module, gap-filled. It is four commits and
~4,900 lines.

**This plan adopts it as the base rather than building a parallel endpoint.** An operator with
two places to look — a console for accounts and a `curl` for numbers — will keep exactly one of
them current, and it will not be the one nobody sees.

Two honest caveats about what that adoption costs.

**It is a port, not a merge.** The branch was last touched 2026-08-25 and `main` has moved 78
commits since, including the KDB port. It edits `app.go`, `config.go` and `db/collections.go`,
all of which have changed underneath it. More to the point, its `UsageSummary` is a method on
`mongoRepository` **with no KDB counterpart** — so as written it does not run on the engine
production actually uses. Porting means re-applying the console onto current `main` and giving
every repository method both implementations, which is the house rule anyway.

**It bundles four features, and only two are wanted here.** The branch carries the admin console,
a player-feedback triage system (its own collection, its own public route), Redis auto-detection,
and the password login. This plan takes the console and the password login. Feedback and Redis
auto-detection are separate changes that happen to share a branch; they should be ported on their
own merits or not at all, and dragging them along would make one reviewable change into four
unreviewable ones.

What the console gets right and should be kept verbatim: the CSP with no inline script, the
`no-store` / `noindex` headers, deriving the console token's signing key from the password hash
so that changing the password revokes every issued token, a console audience distinct from player
tokens so the two can never be traded for each other, and "configure no way in and everybody is
denied".

---

## 1. The rules this plan enforces

> **1. Every reportable number is either derivable from a row that already exists, or is a
> counter on one document per UTC day. Nothing else is stored, and nothing is stored twice.**

> **2. The admin console is not registered on the public listener. Not guarded on it — absent
> from it.**

Consequences of the first, stated so they are not quietly traded away later:

- **No event log table.** One row per refusal, per connect, per hand would be the natural
  instinct and it is wrong here: it puts unbounded write volume in front of an embedded database
  sized for gameplay, to answer questions that are all counts.
- **No weekly or monthly rollups.** A week is a sum of seven daily documents, computed in Go on
  read. Storing it would be a second source of truth that can disagree with the first.
- **Days are UTC**, matching how `stats.MatchResult` is already bucketed, and the console says so
  on the screen rather than letting a reader assume local time.
- **A number that cannot be exact says so** rather than being silently approximate — see the
  distinct-player cap in §2.

The second rule gets its own phase (§7) and its own reasoning, because it is the one the branch
as it stands gets wrong.

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
slog.Warn("admin sign-in failed", "user", username, "remote", ip)
```

That last one matters more once §7 exists: a console behind a tunnel should still say loudly
when somebody is knocking on it.

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
    // PlayersTruncated is set when the set hit playerSetCap. The console then
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
failure — a column that simply never appears on the screen.

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
| `admin.login.ok` / `admin.login.denied` | Phase 7 — the console's own door, counted like any other |

`SeePlayer` is called once per match participation with the subject key `stats` already computes
(`stats.Subject`), which is what makes guests, registered users and AI difficulties countable in
the same set with the vocabulary the rest of the statistics stack already uses.

**Games played does not become a counter's job to be right about.** `matches.completed` is a
convenience so the daily screen is one read; `stats.MatchResult` remains the source of truth, and
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
- The existing signal handler in `cmd/server/main.go` — it already blocks on `<-stop` before
  calling `srv.Shutdown` — sets `StoppedAt` and `Reason: "signal"` **before** `a.Close`, so a
  slow database teardown cannot lose the fact of a clean exit.
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
  of `matches.abandoned` on the day of the deploy, and the screen should not be read as a spike
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

The console shows all four, plus a completion rate defined explicitly as
`completed / (completed + abandoned)` — never over `created`, which would count an empty lobby as
a game somebody failed to finish.

---

## Phase 5 — Port the console forward

Re-apply the `admin-ui` branch onto current `main`, console and password login only:

- `internal/admin` — `guard.go`, `login.go`, `handlers.go`, `ui.go`, `ui/*`
- `internal/ratelimit`
- `cmd/adminpass` — the bcrypt hash generator, so a plaintext password never reaches an
  environment variable, a shell history, `docker inspect`, or a log that dumps the environment
- the `auth.SessionRepository` and `user.Repository` additions the console's user screens need,
  each with a **KDB implementation** as well as the Mongo one
- `stats.UsageSummary` promoted onto the `stats.Repository` interface, with `repository_kdb.go`
  gaining its counterpart — the piece that makes the console work on the engine production runs

Left on the branch: feedback triage, Redis auto-detection. Separate features, separate changes.

**What actually shipped, and what did not.** The console shell, the guard, the
password login, `cmd/adminpass`, the rate limiter and the embedded UI are in.
The **account-management screens are not** — listing users, deleting an
account, setting a password, revoking sessions.

That is a deliberate cut, and the reasoning is the same one this section
already applies to feedback triage, one step further. Those screens need eight
new repository methods (`ListUsers`, `CountUsers`, `CountUsersSeenSince`,
`DeleteByID`, `UpdateByID`, `DeleteIdentitiesForUser`, `DeleteByUserID`,
`CountActiveSessions`), each in **both** engines, plus the KDB scans and tests
that go with them — roughly as much surface again as everything else in this
plan. None of it is reporting, one of them deletes accounts, and porting it
alongside would have turned one reviewable change into two large ones sharing
a diff.

`stats.UsageSummary` was not ported either, and should not be: the report in
Phase 6 answers the same question from the daily counters, over both engines,
with the gap-filling and the bucketing it never had.

So the console today is *the reporting console*. Adding the account screens is
a follow-up with a clear shape: the eight methods, their KDB counterparts, and
the existing `admin_test.go` from the branch, which already tests them.

**Correction to make while porting.** The branch registers the admin group unconditionally, with
the comment that a route table changing shape by environment is worse than a guard that denies.
That reasoning is right for *the guard* and wrong for *the listener*: it is what puts a
user-deleting API on the public origin, one guard bug from the internet. Phase 7 moves it, and
the guard stays exactly as strict as it is.

---

## Phase 6 — The reporting screen

**`GET /admin/api/report?from=2026-08-01&to=2026-09-08&bucket=day|week|month`**, behind the
existing `Guard.Require`, alongside the console's `/admin/api/usage`.

- **Response:** the bucket list, each with `{ start, end, partial, matches: {created, started,
  completed, abandoned, byModule}, players: {distinct, distinctIsFloor}, users: {registered,
  total}, admission: {refused byReason, matchStartRefused}, ops: {boots, uncleanBoots} }`, plus a
  top-level `timezone: "UTC"` and the `from`/`to` actually used after clamping.
- **Gap-filled, always.** A day with nothing played is present with zeroes. Borrowed directly
  from `UsageSummary`, which is right about this: a chart drawn from a sparse series closes the
  gap and implies play that never happened.
- **Weeks are ISO weeks, months are calendar months, both UTC**, and a partial bucket at either
  end carries `partial: true` rather than being dropped or silently short.
- **In the console:** one new tab, four number tiles for the selected range (games, players, new
  users, unclean boots), a bar per bucket, and a table underneath. The console's CSP forbids
  inline script and loads nothing from a CDN, so this is hand-drawn SVG or a `<table>` — not a
  charting library. At four series that is the cheaper option anyway.
- `?format=csv` on the same route, because the operator's actual next step is a spreadsheet.

The previous draft of this plan proposed a standalone `/admin/report` behind a `METRICS_TOKEN`.
That is dropped: it existed only because there was no console to put this in, and a second
authentication mechanism guarding the same numbers is a second thing to get wrong.

---

## Phase 7 — The door

This is the phase the ask turns on, so the reasoning is spelled out rather than assumed.

### What is exposed today

The container publishes `127.0.0.1:8090:8090` — loopback on the host, good — and nginx's vhost
is a single catch-all:

```nginx
location / {
    proxy_pass http://127.0.0.1:8090;
}
```

That catch-all is deliberate and correct for the game: the server knows which paths are API and
which are the web client, so nginx enumerates nothing and cannot drift. But it means **every
route the Go router registers is on the public internet by construction**. Port the console as
the branch has it, and `https://play.limidus.com/admin/api/users/{id}` — a route that deletes
accounts — is public, protected by nothing but the guard.

The guard is good. It should not be the only thing.

### Layer 1 — a second listener, which is the one that matters

`cmd/server/main.go` builds **two routers and two `http.Server`s**:

| | Public | Admin |
|---|---|---|
| Port | `PORT` (8090) | `ADMIN_PORT` (8091) |
| Registers | game API, WebSocket, web client | `/admin`, `/admin/api` |
| Published by Compose | `127.0.0.1:8090:8090` | `127.0.0.1:8091:8091` |
| Proxied by nginx | yes, catch-all | **no — nginx has no knowledge of it** |

The public router never registers the admin group, so `https://play.limidus.com/admin` is a 404
served by the SPA fallback — not a 403, not a login page, nothing that says a console exists.

**Why this rather than `location /admin { deny all; }` in nginx.** A deny rule is a negative
patch on a positive-by-default catch-all, and it has to be exactly as clever as every path that
reaches the same handler: `/admin`, `/Admin`, `//admin`, `/./admin`, and whatever route the
console gains next year. A route that was never registered on the listener needs no rule and
cannot be outflanked by a path that normalises differently in nginx and in chi. The nginx config
stays exactly as it is — no new block, nothing to review, nothing to get wrong.

**The regression test that matters**, and it is three lines: build the public router, assert it
answers 404 to `/admin`, `/admin/api/login` and `/admin/api/users`. That is what stops somebody
tidily "unifying the routers" in a year.

**How the operator gets in:**

```bash
ssh -N -L 8091:127.0.0.1:8091 davja@192.168.13.13
```

then `http://127.0.0.1:8091/admin` in their own browser. The credential for reaching the console
at all is the SSH key that already deploys the thing.

**Why publish to the host's loopback rather than not publishing at all.** Not publishing is
marginally tighter — the port would exist only inside the container's network namespace — but
then reaching it means `docker exec` and a terminal browser, and an operator who cannot
comfortably look at the numbers will stop looking at them. Host loopback plus SSH is reachable
by exactly the people who can already `docker compose down`, and it is one `ssh -L` away from a
real browser.

### Layer 2 — authentication, unchanged in spirit

The tunnel is not the authentication. Keep the branch's guard as it stands:

- `ADMIN_USERNAME` + `ADMIN_PASSWORD_HASH` (bcrypt, generated by `cmd/adminpass`; the plaintext
  never enters the environment).
- A console JWT with its own audience, signed with a key **derived from the password hash**, so
  changing the password invalidates every token already issued and no separate secret has to be
  configured or rotated.
- Rate-limited sign-in (10 per 15 minutes per address), and now counted:
  `admin.login.denied` on the daily document, logged at `WARN`. Somebody knocking on a console
  that is supposed to be unreachable is the single most interesting line in the log.
- **Configure nothing and everybody is denied.** Kept exactly.

One change of emphasis: with the console off the public listener, the `ADMIN_EMAILS` allow-list
becomes optional rather than the primary door. Its whole justification was being a way in when
mail is broken — which is backwards once the password login is the primary. **Recommendation:
password-only in production**, allow-list left unset, one fewer dependency between "I need to see
the console" and "SMTP is up".

### Layer 3 — client certificates, if a tunnel is too awkward

Named in the ask, so designed here — but it is the alternative to Layer 1's tunnel, not an
addition to it, and it exists only for reaching the console from a device where `ssh -L` is
impractical (a phone, a borrowed machine).

- A **separate vhost**, `admin.play.limidus.com`, never the main one:

  ```nginx
  ssl_client_certificate /etc/nginx/admin-ca.crt;   # a private CA, not a public one
  ssl_verify_client on;                             # no cert, no TLS handshake
  location / {
      proxy_pass http://127.0.0.1:8091;
      proxy_set_header X-SSL-Client-Verify $ssl_client_verify;
      proxy_set_header X-SSL-Client-DN     $ssl_client_s_dn;
  }
  ```

- A small `scripts/admin-cert.sh` to mint the CA once and one client certificate per operator
  device. **Revocation is reissuing the CA**, not a CRL: at one to three devices that is less
  machinery and fewer ways to believe a revoked cert is dead when it isn't.
- **The trap, stated because it is the one that bites:** the origin must never trust
  `X-SSL-Client-*` from a caller that is not nginx. Two things make that safe, and both are
  required — the admin listener is loopback-only, so nothing off-host can set the header at all;
  and nginx names every `X-SSL-Client-*` header in `proxy_set_header`, which replaces whatever a
  client tried to send. Belt and braces: **the certificate does not replace the password.** A
  spoofed header still faces the bcrypt login, so the worst case of getting this wrong is losing
  one layer rather than the console.
- HSTS and `X-Robots-Tag: noindex` on the vhost. The console already sets the latter itself.

### Recommendation

**Layer 1 + Layer 2: the second listener, reached over SSH, password-protected.** Zero public
surface, no certificate authority to run, no nginx block to review, and the console cannot be
reached by anyone who cannot already reach the host. Layer 3 is a documented option to reach for
if and when browsing from an untunnelled device becomes a real need — not something to build
ahead of that need.

---

## What is deliberately not built

- **Prometheus / Grafana / OpenTelemetry.** A second process and a scrape endpoint on a 1 GiB
  box serving one operator. Revisit if there is ever a second host, which the scaling plan does
  not currently expect.
- **Per-event rows.** See §1.
- **Retention or expiry.** 365 small documents a year. Nothing needs sweeping.
- **Alerting.** The unclean-boot count on a screen read weekly is the alerting, until it isn't.
- **Per-user analytics, funnels, cohorts.** Not asked for, and a materially different privacy
  posture: this plan counts, and stores subject keys for one day's distinct count. It does not
  build a profile.
- **An admin console on the public origin, under any guard.** §7.

---

## Order of work, and what each phase is worth on its own

| Phase | Deliverable | Standalone value |
|---|---|---|
| 0 | `internal/obs`, slog default | Structured `docker logs` immediately, no call sites touched |
| 1 | `daily_metrics`, both engines, flusher | Nothing visible yet — the substrate |
| 2 | Counters wired, `rebuild-daily-metrics` | Games and users become answerable |
| 3 | Boot records | Crashes become visible, retroactively from the next boot |
| 4 | Abandon reaper | Closes a live bug; "unfinished" becomes a real number |
| 5 | Console shell ported forward | An operator console that runs on the engine production uses (account screens deferred — see Phase 5) |
| 6 | The reporting screen | The ask, end to end |
| 7 | Second listener, tunnel, password | The console is unreachable from the internet |

**Phases 5 and 7 ship together or not at all.** Porting the console without moving it off the
public listener is the one ordering that leaves the system worse than it is today, and it is the
easy mistake to make because Phase 5 is the interesting work and Phase 7 is plumbing.

Phases 0 and 4 stand alone and can go first in any order. Phase 4 is worth doing early
regardless of the reporting, because a table stuck in `suspended` forever is a defect whether or
not anybody is counting it.

---

## Open questions for the operator

1. **Should guests count as "users"?** They play, they finish matches, they have durable ids, and
   they never appear in the `users` collection. The plan counts them in `players.distinct` and
   separately in `sessions.guest`, and keeps `users.registered` to accounts. Say if "users" was
   meant to mean accounts only.
2. **`playerSetCap` at 5,000/day** — high enough to never trip at current volume, low enough to
   bound the document. Lower it if the daily document size matters more than exactness.
3. **Are bots players?** They hold seats and have subject keys. The plan excludes AI subjects from
   `players.distinct` and counts their matches normally, so "how many people played" means people.
4. **Feedback triage** — port it alongside the console, or leave it on the branch? It is a
   genuinely useful feature and it is not this ask; porting it doubles the review surface of
   Phase 5. *(Left on the branch, along with the account-management screens — see Phase 5.)*
5. **The account-management screens** — worth a follow-up now, or is the reporting console
   enough? They are the larger half of the `admin-ui` branch and the half that can delete
   accounts.

*(The earlier draft's question about `METRICS_TOKEN` versus the console is answered: the console
wins, and the token is gone.)*
