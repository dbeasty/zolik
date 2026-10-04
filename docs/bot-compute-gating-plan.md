# Bot compute gating — plan

> **Status: Phases 0–4 built.** The governor ships in `observe` on servers (flip with
> `BOT_GOVERNOR=enforce` once its counts look right) and enforces on phones. §4 records what
> each phase built and how it was tested; §5 holds the measurements it rests on.

Deep AI agents are a limited resource. The server works out how many it can run comfortably
("5 deep agents on this box"), hands out that many **leases**, and plays every other bot seat
with the rule-based engine. When resources tighten, a resource monitor publishes a
notification, and deep seats **downgrade to the rule engine** at their next turn boundary.
When resources recover, waiting seats **upgrade** back.

*As built*, the leases cover measured cost classes rather than "deep agents". The most
expensive bot turned out to be a rule bot (Mariáš Hard), so a class is (game, skill, engine),
and the fallback is the next class down. See §4, Phase 2.

- **Baseline:** `origin/main` @ `7a61924`
- **Phase 0:** instrumentation and `cmd/botcost` on `claude/bot-compute-phase0`; results in §5.
- **Deliverable:**
  - A capacity model that turns CPU and memory into a count of deep agents.
  - A lease pool.
  - A resource monitor that emits up and down events.
  - Engine switching at turn boundaries.
  - A lobby that offers deep bots only when a lease is free.
  - Metrics that show all of it working.

---

## 1. What is actually there today

| Piece | State |
|---|---|
| Bots | Every shipped bot is a **rule/heuristic engine**: Žolíky's `ai.HeuristicAgent`, Hold'em's formula + bounded Monte Carlo, and rule bots for Canasta, Gin, Prší, Tiles and Blackjack. Easy, Medium and Hard are all rule-engine profiles (`module/skill.go`). |
| Deep agents | Trained nets (`learn.NetBot`, `ZLNET1` MLPs). The library merged to main in PR #221, but **no model is embedded and nothing in live play uses a net** (as of 2026-10-03). The trainer README plans to embed `models/hard.bin`. No search agent exists; there is only a `Searchable` sketch in `architecture.md`. |
| Bot loop | One goroutine per match with bots (`match/bots.go`). No cap, pool or semaphore. The 0.9–1.8 s think time is a `time.Sleep` that uses no CPU. |
| Hint | `POST /matches/{id}/hint` runs a Hard `Act` synchronously in the request, with no rate limit. |
| Resource awareness | `admission` reads cgroup memory, PSI `cpu.pressure` and GOMEMLIMIT, and refuses **new matches** under pressure. It only answers when asked; nothing is notified when pressure changes. Nothing reads `cpu.max` or sets `GOMAXPROCS`. |
| Deployment | One container, `mem_limit: 2g`, **no CPU limit**, with KDB embedded in the same process. |
| Skill offer | `botSkill` enum served statically via `GET /modules`. The table screen hard-codes `BOT_SKILLS` (`client-react-native/app/lobby/table.tsx:35`). |

The downgrade target already exists and is proven: the rule engines are today's product.
This plan adds a second tier above them and a way to move seats between the two tiers safely.

## 2. Principles

- **Two engines, one fallback.** A bot seat plays either **deep** or **rule**. Rule is always
  available and never needs a lease, so it is the floor. Moves are never scaled partway.
  Either an agent has its full resources, or the seat plays the rule engine.
- **Capacity is a count, not a feeling.** The operator can read "deep agents: 3 / 5" on
  `/debug/memory`, a metrics gauge or the admin console, and can pin the count with an env
  var.
- **Downgrade at a turn boundary, upgrade at a round boundary.** A seat never changes engine
  in the middle of a multi-action turn (draw → meld → meld → discard), so the rule engine never
  inherits half a plan it doesn't understand. A downgrade is forced, so it happens at the next
  turn. An upgrade waits for the next round or deal, so an opponent never gets stronger in the
  middle of a hand. *(Decided 2026-09-29.)*
- **Degrade the bot, never the table.** A downgrade must still produce a legal move on every
  turn (see "never end a turn in a dead position"). Nothing about a downgrade pauses, stalls or
  evicts a table.
- **Gate growth at the offer; degrade in place.** This follows admission's existing split.
  New deep seats are only offered while a lease is free. Seats already running lose their
  lease only on a pressure event.
- **Compute inside the think window.** Start computing when the turn arrives, then sleep for
  whatever part of `thinkFor` is left. Deep moves then add no visible latency, and the pace is
  unchanged for e2e tests (`BOT_THINK_MIN_MS` keeps its defaults for Playwright).
- **Off / observe / enforce.** The monitor ships in observe mode first. It emits events and
  counts the leases it *would* grant or revoke, so the thresholds can be tuned before it
  changes any seat.

## 3. Design

### 3.1 Engines: rule is always there, deep is optional

Add an optional interface next to `Botted` in `module/bot.go`:

```go
// DeepBotted is implemented by a module that has an agent stronger than its
// rule bot. The rule bot (Botted / OfferBot) stays the fallback.
type DeepBotted interface {
    DeepBot() DeepBot
}

type DeepBot interface {
    Bot
    // Kind names the agent for capacity accounting ("holdem-net", "canasta-net",
    // "zolik-search"). Agents of different kinds have different costs.
    Kind() string
    // Warm loads whatever the agent needs resident (model weights) before its
    // first move. Idempotent; shared across seats of the same kind.
    Warm() error
}
```

Extend `BotSeat` with the engine the seat is playing this turn. **The zero value is `rule`**,
so every existing bot and test is unchanged:

```go
type Engine uint8
const (
    EngineRule Engine = iota // today's bots; no lease
    EngineDeep               // requires a lease
)

type BotSeat struct {
    PlayerID string
    Skill    Skill
    Seed     int64
    Engine   Engine
}
```

Skill keeps its current meaning inside each engine. A seat that asked for a deep agent falls
back to the rule engine **at Hard**, the strongest play that needs no lease.

### 3.2 Capacity model: how many deep agents fit comfortably

This lives in a new package, `internal/capacity`, reusing admission's cgroup readers.

**Available resources**
- CPU quota from `cpu.max` (cgroup v2) or `cfs_quota/period` (v1). The fallback is
  `runtime.NumCPU()`.
- Set `GOMAXPROCS` from the quota at startup, unless it is set in the environment. This
  follows the GOMEMLIMIT pattern in `app/app.go:458`.
- Memory limit from the existing `admission.MemoryLimit()`.

**Cost per agent kind.** This is a table in code, filled in from Phase 0 measurements and
overridable by env:

| Field | Meaning | Example (net, to be measured) |
|---|---|---|
| `CPUPerMove` | CPU time for one decision, p95 | 4 ms |
| `MovesPerSec` | Peak decision rate of one seat in active play | 1 (think time bounds it) |
| `MemResident` | Shared once per kind: model weights | 0.5 MB |
| `MemPerSeat` | Per seat: belief state, search tree, scratch | 2 MB |

**Slot count**

```
cpuCores   = quota × BOT_CPU_SHARE (default 0.5 — the rest stays with KDB, sockets, HTTP)
cpuSlots   = floor(cpuCores / Σ(CPUPerMove × MovesPerSec) per seat) × comfort
memSlots   = floor((limit × BOT_MEM_SHARE − Σ MemResident) / MemPerSeat) × comfort
deepSlots  = min(cpuSlots, memSlots, BOT_DEEP_MAX)
```

- `comfort` defaults to 0.7. "Comfortable" means sized for bursts, such as every deep seat
  hitting a river at once, and not just for the average load.
- `BOT_DEEP_MAX` lets the operator set the number directly, for example `BOT_DEEP_MAX=5`.
  `0` disables deep agents entirely, which acts as a kill switch.
- **Weighted leases.** Kinds cost different amounts, so each lease has a weight of
  `ceil(cost(kind) / cost(cheapest kind))`. Capacity is counted in weight units: "5 units"
  might be 5 net seats, or 2 search seats plus 1 net seat. The operator-facing number is still
  "N deep agents" in cheapest-kind units.

The count is recomputed at startup and on every monitor event (§3.4). It is **not** measured
live on each move, because live CPU is too noisy to size from. Live readings decide whether
the static count currently holds (the monitor), not what that count is.

### 3.3 Leases: who holds a deep agent

`capacity.Pool` is a weighted pool of deep-agent leases.

- **Held per seat for the life of the match**, not per move. A deep agent's per-seat memory
  (belief state, opponent model) stays resident between moves, and per-move leasing would
  swap engines mid-game.
- **Acquire:**
  - at `addBot` / match start for a seat that asked for deep;
  - on an upgrade event for a seat that was waiting.
- **Release:**
  - when the match finishes;
  - when the match is evicted idle (`match/live.go`);
  - on handover to another node;
  - when the seat is removed;
  - on a revoke.
- **Leak guard:** a lease records its match and seat. A sweep every minute releases any lease
  whose match is no longer live. This is the same shape of bug as "bot loop never restarts on
  an active table", so it gets a test.
- **State on the seat:** `models.Player` gains `AIEngineWanted` (what the host asked for) and
  the runtime tracks `engineActual`. `botSeatFor` hands the bot `engineActual`.

### 3.4 Resource monitor: the notifications

`internal/capacity.Monitor` samples resources every 2 s and **publishes events** to
subscribers. It replaces "ask admission when something happens" with "be told when the
answer changes".

**Signals**
- PSI `cpu.pressure some avg10`.
- Think-time overrun: p95 of deep `Act` time as a fraction of the think window.
- Memory fraction of the limit.
- KDB zone.

On macOS and devices, where PSI is unavailable, the think-time overrun alone drives the
monitor.

**Levels, with hysteresis.** Steps down fast; steps up only after the lower threshold has held
for 30 s.

| Level | Enters when | Effective deep capacity |
|---|---|---|
| **Green** | all below Amber thresholds for 30 s | `deepSlots` |
| **Amber** | PSI ≥ 0.10, **or** deep p95 ≥ 50% of think window, **or** mem ≥ 0.75 | `floor(deepSlots / 2)` |
| **Red** | PSI ≥ 0.20, **or** deep p95 ≥ 100% of think window, **or** mem ≥ KDB high zone | `0` |

Admission refuses new matches at CPU 0.25 and memory 0.85, so these thresholds sit **below**
admission on purpose. The ladder reads: downgrade deep agents first, stop offering them next,
and refuse new tables last.

**Events**

```go
type Event struct {
    Level     Level      // Green / Amber / Red
    Capacity  int        // effective deep slots (weight units) at this level
    InUse     int
    Reason    string     // "cpu_pressure", "memory", "overrun", "recovered"
}
```

- Delivery is a fan-out over buffered channels. Subscribers never block the monitor; a slow
  subscriber gets the latest state, not a backlog.
- **Subscribers:**
  - the lease pool, which revokes or grants;
  - the lobby hub, which pushes `botCapacity` to connected lobby sockets;
  - metrics;
  - the log (one line per level change, not per sample).

**What the pool does on an event**
- **Capacity drops below InUse → revoke.** It revokes leases, newest first (LIFO). Long-running
  games keep their agent, and the seat that has had the deep agent for the least time loses
  it. Tables with no human seated are revoked before any table with a human: bot-only soak and
  demo tables go first.
- **Capacity rises above InUse → grant.** It grants to waiting seats first come, first served
  (FIFO), one per event tick. This keeps a burst of upgrades from pushing the level straight
  back to Amber.
- A revoke or grant only **marks** the seat. The bot loop applies it at the next turn
  boundary (§3.5) and only then releases or starts using the lease. A revoked seat's memory is
  therefore freed after its current turn, never in the middle of it.

### 3.5 Bot loop integration

In `botLoop`, at the point where the actor changes (`actor != lastActor`, which is already
where `turn` resets):

```
if seat.engineWanted == Deep:
    switch pool.State(match, seat):
    case revoked:  engineActual = Rule; pool.Release(...);  emit seat-engine change
```

At a round boundary (the round counter in the view moves on, or the awaited seats become
"ready" prompts):

```
    case granted:  if dp.Warm() ok { engineActual = Deep }; emit seat-engine change
```

A grant that arrives mid-round waits, and its lease stays reserved for the seat. The pool's
leak sweep treats a reserved-but-unused lease as held.

Then the move itself:

```
deadline := now + thinkFor
bot := ruleBot
if engineActual == Deep { bot = deepBot }
action := bot.Act(state, seat{Engine: engineActual}, offers)
    // backstop: deep Act past deadline+2s → count, log, use the rule bot's answer for this move
sleep(until deadline)
```

- The rule bot is always resolved alongside the deep one. The backstop is simply "ask the rule
  bot", which plays a legal move in microseconds.
- `Warm` failing (model missing, or memory refused) leaves the seat on the rule engine and
  releases its lease.
- **The backstop already exists.** `botAct` (`match/bots.go`) bounds every `Act` at 5 s
  (`botActBudgetDefault`) and falls back to the offer list. A timed-out `Act` keeps running on
  its abandoned goroutine, though, so it **still burns CPU**. Phase 0 counts these orphans. A
  deep agent must honour a cancel signal, because an orphaned deep search is exactly the load
  this plan exists to prevent.
- Every move records which engine played it (`engine` on the move log entry). Replay then
  reproduces the move, and stats can separate results against deep agents from results against
  rule engines.

### 3.6 What the players see

- **Lobby/table:**
  - The skill picker gains a deep tier, for example **"Expert (AI)"**, rendered from live
    capacity, not from the hard-coded `BOT_SKILLS`.
  - When no lease is free, the tier shows disabled with an explanation on press ("All expert
    AI seats are busy right now"), using `ExplainOnPress` and keeping `disabled` on the
    Pressable.
  - The lobby socket pushes `botCapacity {free, total, level}`, so the picker enables itself
    when a lease frees up without the player refreshing.
- **`addBot` with deep and no free lease:** the server seats the bot on the rule engine at
  Hard, marks it waiting for a lease, and replies `BOT_ENGINE_QUEUED`. The client shows "Seated
  as Hard. It will switch to Expert when capacity frees up." New message keys go in
  `serverKeys.json` and must be passed as static literals.
- **In match:** a seat-engine change is broadcast as a small match event. The seat badge
  changes (for example, a subtle "AI" marker comes and goes). Players are not interrupted and
  nothing pops up in the middle of a hand.

### 3.7 Hint endpoint

Hints always use the **rule engine** at Hard, unless a lease is free **and** the level is
Green. A hint borrows a lease for the duration of the request and returns it immediately. Add
a per-user token bucket (`internal/ratelimit`): 1 hint per 2 s, burst 3. This closes today's
unbounded synchronous-Act path whatever else ships.

### 3.8 On-device (offline / mobile build)

`app/mobile.go` runs the same loop on the phone. On the phone:
- `deepSlots` comes from a startup micro-benchmark (one net forward pass × N, 50 ms, cached
  per install) and device memory.
- The monitor runs on think-time overrun plus the OS thermal state and low-power mode, which
  are exposed from the native side.
- A hot phone emits Red, and the offline table quietly drops to rule engines, following the
  same turn-boundary rules as the server.

## 4. Phases

### Phase 0 — measure

Nothing here changes behaviour.

- **Metrics:**
  - `bot_act_seconds{module,engine,kind}` and allocations per `Act`;
  - gauges for bot loops and hint rate.
- **`cmd/aibench`:** report CPU-ms per decision and per-seat resident memory for each deep
  agent kind on `learn-core`. That fills in the cost table in §3.2.
- **Soak:** run `dev-stack.sh soak` at `cpus: 1` and `cpus: 2` with K deep seats, increasing K
  until WebSocket broadcast p95 degrades. The K where it bends, times `comfort`, has to agree
  with the formula. That checks the capacity model against reality.

### Phase 1 — monitor in observe mode, hints off the lock *(built 2026-10-04)*

What was built, against §5.3's reduced scope:
- **Hints off the match lock.** `Manager.Hint` validates and copies the position under the
  lock, then releases it before `Act`. `TestASlowHintDoesNotHoldTheTable` fails on the old
  code ("a move waited on a hint") and passes on the new.
- **Hint rate limit.** Five worked-out hints per player per ten seconds
  (`internal/ratelimit`). Refusals that run no bot (`NOT_YOUR_TURN`, `HINTS_OFF`) don't
  count. Over the limit: `429 HINT_TOO_SOON` with `Retry-After: 10`, worded in all 24 locales.
- **`GOMAXPROCS`: nothing to build.** The server is on Go 1.26. Since Go 1.25 the runtime sizes
  `GOMAXPROCS` from the container's CPU limit and follows changes. The monitor reports it
  (`/debug/capacity`, plus a startup log line), so the quota reader planned in §3.2 can read
  `runtime.GOMAXPROCS(0)`.
- **Resource monitor** (`internal/capacity`):
  - Samples every 2 s: admission's CPU stall and memory fraction, and the recent bot-decision
    p95 relative to the think window.
  - Green/amber/red with §3.4's hysteresis: worse applies at once; better needs 30 s of calm
    and steps one level at a time.
  - Subscribers get coalesced events.
  - Observe mode: the app logs each level change, and `/debug/capacity` shows the level, the
    last reading, the thresholds and `GOMAXPROCS`. `BOT_MONITOR=false` turns it off.
- **Sat-out seats** (drop-in tables, PR #242) are recorded as skill `sitout` in `/debug/bots`,
  so passive play doesn't make a real skill look cheaper.

Deferred to Phase 2, because they need something to act on: the cost table and `deepSlots`;
the `would_revoke` / `would_grant` counters; and `BOT_DEEP=off|observe|enforce`.

**Test trap:** `internal/match` tests that need a store **skip silently** unless Mongo is up
or `ZOLIK_TEST_DB_ENGINE=kdb` is set. Run them with the variable set.

### Phase 2 — the governor: leases and switching *(built 2026-10-04)*

`internal/botgov` replaces §3.1–3.3's deep-vs-rule design with what §5.4 measured:
- **Cost classes are (game, skill, engine).** `DefaultCosts` holds the measured mean CPU per
  decision. Only classes above a 2 ms floor need a lease: today Mariáš Hard (rule, 54 ms) and
  Žolíky's model (10 ms). Everything else plays freely.
- **Fallback is the next class down:** a model falls back to the rule bot at the same skill,
  and a rule bot to one skill lower. A seat without a lease walks down to the first class that
  needs none: Mariáš Hard → Medium, Žolíky net → Žolíky rule Hard.
- **Capacity:** cores (`GOMAXPROCS`) × 0.5 share × 0.7 comfort, halved at amber and zero at
  red. A lease weighs its cost per think window (mean CPU ÷ 1.35 s).
- **Speed:** every cost is scaled by `Calibrate()`, a few-millisecond integer workload timed
  against the machine that measured the table. A phone three times slower leases three times
  the CPU per seat.
- **When a seat changes:**
  - Decisions are made at a seat's turn start, judged by who made the match's last move, so it
    survives bot-loop restarts.
  - A revoked lease is given back at the next turn; an upgrade waits for the next round.
  - Revocation takes bot-only tables first, then the newest leases.
  - The bot chosen at turn start (including the operator's model switch) is held for the
    whole turn, so neither the governor nor the admin switch can hand half a plan to another
    engine.
- **Leases are returned** when a match stops, when a seat asks for a different class, and by
  a 30 s sweep for seats unseen for 2 minutes.
- **Hints** use `ForHint`: the asked-for class at green, the cheapest at amber or red.
- **Modes:** `BOT_GOVERNOR=off|observe|enforce`. The server defaults to `observe`: it decides
  and counts (`/debug/capacity`'s `reducedSeats`, `reducedTurnsTotal`) but changes no move.
  The mobile build enforces.
- **Testing aids:** `BOT_MONITOR_PIN=green|amber|red` and `POST /debug/capacity/pin?level=…`
  hold the monitor at a level. Both are debug-endpoint only.

Tests:
- `botgov` unit tests: lease limits, fallbacks, mid-turn safety, round-gated upgrades,
  revocation order, observe/off, lease return on a changed request, sweep, speed scaling,
  hints.
- `TestTheGovernorDowngradesModelSeatsAtTurnBoundaries`: a real Canasta engine whose Hard
  seats are a recording model-over-heuristic bot; red is set mid-match. It asserts the model
  played before, the rule bot after, and that no turn ever mixed engines. It fails when turn
  tracking is disabled.
- Live, on a local server: a Mariáš table with two Hard bots at red played Medium (20
  decisions), with seats marked simplified. Re-pinned green, both returned to Hard after a
  round boundary (36 Hard decisions), and the marks cleared.

### Phase 3 — what the players see *(built 2026-10-04)*

- **Seat badge.** `PlayerMsg.simplified` marks a bot playing below what it was seated with
  (enforce only), and `SeatStrip` shows a localized **SIMPLER** badge next to BOT. Its
  accessibility label says why. Badges no longer shrink in narrow tiles; the name truncates
  instead (checked at 390 px).
- **Table notice.** `GET /bots/capacity` (public, also on the offline host) returns
  `{level, simplifying, reducedSeats}`. The table screen asks every 30 s and, while
  simplifying, says under the bot buttons that bots may play simpler moves until the server
  has room.
- The skill buttons stay Easy/Medium/Hard. With downgrade-in-place there is nothing to
  disable: a Hard seat added under pressure plays the cheaper class and comes back by itself.
  The §3.6 "Expert (AI)" tier and the `BOT_ENGINE_QUEUED` response were dropped; the
  operator's model switch decides whether Hard means the model.
- Wording is in all 24 locales.
- Verified in the browser against a pinned-red server: the notice on the table screen, and
  **BOT · SIMPLER** on both Hard Mariáš seats, at desktop width and at 390 px.

### Phase 4 — on-device *(built 2026-10-04)*

- The mobile config runs the monitor and **enforces** the governor.
- `Calibrate()` scales costs to the phone's speed.
- Heat:
  - `Host.SetThermalPressure(0|1|2)` (gomobile) feeds a `Thermal` signal: 1 makes the level
    amber, 2 makes it red.
  - iOS (`NearbyThermal.swift`) maps `ProcessInfo.thermalState` (serious → 1, critical → 2)
    and Low Power Mode → 1.
  - Android (`NearbyThermal.kt`) maps `PowerManager` thermal status (severe → 1, critical and
    above → 2, API 29+) and Battery Saver → 1.
  - Both report when the host starts and on every change.
- Verified:
  - The Swift type-checks against a freshly built `Zolikcore.xcframework` (iOS simulator
    slice).
  - The Kotlin compiles against `android.jar` (API 36) with a stub of the gomobile binding.
  - **Not run on a device or simulator.**

## 5. Phase 0 results (2026-09-29)

These come from `go run ./cmd/botcost -matches 5 -decisions 1500` on one core (`-procs 1`),
on an Apple-silicon laptop. Every game, variation, smallest and largest table, and skill was
measured: about 280 000 decisions in total. Trained nets were measured with the same driver in
a scratch checkout of `claude/learn-core`, using `ml/runs/holdem-20m` and
`ml/runs/canasta-12m`. The live server now records the same numbers per decision
(`/debug/bots`, plus a ten-minute log line).

### 5.1 Cost per decision, 2026-09-29 (main @ `20bbc20`, wall time)

| Engine | mean | p95 | p99 | allocated per decision |
|---|---|---|---|---|
| **Hold'em rule** (post-flop Monte Carlo), heads-up | 6–15 ms | 31–49 ms | 36–73 ms | **1.8–3.8 MB** |
| **Hold'em rule**, 9 seats | 5.8–7.8 ms | 27–34 ms | 36–56 ms | 1.6–2.0 MB |
| Žolíky rule, continental | 0.9–1.1 ms | 2.7–3.4 ms | 4.9–6.8 ms | 510–680 KB |
| Canasta rule, samba 6 seats | 0.9–1.2 ms | 3.3–4.4 ms | 9–17 ms | 77–86 KB |
| Canasta rule, other variations | 0.2–0.4 ms | 0.4–1.2 ms | 1.3–6 ms | 25–40 KB |
| Gin, Tiles, Blackjack, Prší rule | < 0.2 ms | < 0.4 ms | < 1 ms | 3–47 KB |
| **Hold'em net** (holdem-20m) | 0.35–0.64 ms | 0.5–1.3 ms | 0.7–2.9 ms | 31–60 KB |
| **Canasta net** (canasta-12m) | 0.30–0.53 ms | 0.75–1.5 ms | 1.7–4.1 ms | 25–58 KB |

- Maximums reach 50–180 ms in several games. The single outliers line up with GC cycles on
  one core, and they are the reason a comfort factor exists.
- The model files are 0.8–0.9 MB. `NetBot` keeps no per-seat state, so one resident copy per
  kind serves every table.

### 5.2 What the numbers changed (2026-09-29, superseded by §5.3)

1. **The expensive engine today is a rule engine.** Hold'em's Monte Carlo costs 10–25 times
   what the Hold'em net does, and it churns about 2–4 MB of garbage per decision. At one
   decision per 1.35 s think window, 100 Hold'em bot seats allocate roughly 200 MB/s. That is
   the biggest CPU and GC load bots put on the server, and it comes from a bot that is not
   deep.
2. **"Downgrade to rule" is backwards for Hold'em once a net ships.** Moving a Hold'em net seat
   to the rule engine under pressure would multiply its CPU by more than 10. The fallback has to
   be the *cheapest adequate* engine for each game, and cost classes have to come from this
   table, not from whether an engine is deep. For Canasta, rule is still cheaper, by about 2x,
   and both engines are small.
3. **The trained nets as they stand need almost no gating.** At about 1 ms p95 and at most one
   decision per think window, half of one core carries hundreds of net seats before the
   comfort factor. The lease pool matters for:
   - Hold'em rule seats (about 30 per core under §3.2's formula);
   - future search agents (the `Searchable`/MCTS sketch);
   - hints (Hard, synchronous, and running under the match lock; see below).
4. **The hint path holds the match lock while it thinks.** `Manager.Hint` runs `Act` inside
   `lockMatch`. On Hold'em that is up to 70 ms at p99 during which the table cannot take a move.
   Phase 1 should move the `Act` outside the lock, the way the bot loop already does, as well as
   rate-limiting it.

### 5.3 Recheck, 2026-10-03 (main @ `9e21df8`)

**What changed on main since 2026-09-29:**
- learn-core merged (PR #221). It brought `holdem/score.go`, a seven-card ranker about 100x
  faster than `Best`, which equity rollouts now use. `TestEquityIsUnchanged` pins identical
  answers.
- Hard got smarter in Hold'em (#236, #238: claim reading, a jam-or-fold chart, river bluff
  pricing) and in Canasta (#234, #235, #237).
- The trained Hard agent is still not in live play.

`cmd/botcost` now records **CPU time** per decision alongside wall time. The first rerun ran
while the machine sat at a load average of 78 on 16 cores (other sessions' benches). That
showed a false 5x regression in Canasta Samba Hard, and nets apparently 3–5x slower. Under
contention wall time inflates and CPU time does not, so compare runs by the CPU columns. The
numbers below are CPU time from a run with load around 7.

| Engine (worst configuration) | CPU mean | CPU p95 | CPU p99 | KB/decision | vs 2026-09-29 |
|---|---|---|---|---|---|
| Hold'em rule, Hard | 0.08–0.28 ms | 0.23–0.55 ms | 0.39–0.75 ms | 22–77 | **30–75x cheaper** |
| Canasta rule (Samba 6, the worst) | 0.13–0.34 ms | 0.22–0.88 ms | 0.6–3.4 ms | 34–97 | unchanged; the Hard changes cost nothing |
| Žolíky rule (continental, the worst) | 0.2–0.73 ms | 0.37–1.6 ms | 0.65–2.3 ms | 117–407 | slightly cheaper |
| Gin, Tiles, Blackjack, Prší rule | < 0.3 ms | < 0.4 ms | < 3.4 ms | 3–47 | unchanged |
| **Hold'em net** (holdem-long) | 0.24–0.41 ms | 0.32–0.69 ms | 0.38–0.86 ms | 36–79 | about the same |
| **Canasta net** (canasta-long) | 0.40–1.04 ms | 0.9–5.0 ms | 2.5–11 ms | 62–247 | 2x, mostly Samba |
| **Žolíky net** (zolik-long, new) | 0.68–1.08 ms | 1.9–4.2 ms | 3.6–6.9 ms | 314–913 | new |

Model files: Hold'em 0.8 MB, Canasta 0.9 MB, Žolíky 2.0 MB. Each is resident once and shared
by every table.

**What this does to §5.2's conclusions:**
1. ~~The expensive engine is a rule engine.~~ **Reversed.** Every rule engine now averages
   under 0.75 ms. The nets are the most expensive engines, at 1.5–5x their game's rule bot.
2. ~~"Downgrade to rule" is backwards for Hold'em.~~ **Reversed.** Rule is now cheaper in every
   game, so the plan's original design stands: a deep seat falls back to the rule engine at
   Hard. The open question about the fallback is closed by measurement. Keep the guard,
   though: the cost table comes from `botcost`, and it gets rerun whenever an engine changes,
   since this table moved 75x in four days.
3. **The nets still need little gating, and the lease count is large.** By §3.2's formula
   (1.35 s think window ÷ CPU mean × 0.5 share × 0.7 comfort), one core carries about 440 seats
   on the worst net (Žolíky, 8 seats, 1.08 ms) and about 1 700 on Hold'em rule. The production
   box has no CPU limit, so the count is effectively unbounded until `cpus:` is set. For now,
   the operator cap (`BOT_DEEP_MAX`) and memory churn (Žolíky net, about 0.9 MB per decision
   at 8 seats) bind before CPU does. Revisit if a search agent (MCTS) is ever built; that is
   the class of engine this plan really protects against.
4. **Hints still compute under the match lock** (`match/hint.go`: `lockMatch` … `Act`). The
   cost is now sub-millisecond for rule Hard, but it rises with a net. Moving `Act` outside the
   lock and adding a rate limit remain the first Phase 1 items.

**Revised order of work:**
- Phase 1 is only: hint off-lock + rate limit; `GOMAXPROCS` from the quota; and the
  monitor/levels in observe mode.
- Leases and engine switching (Phase 2) start when a net is wired into live play.
- The scheduled task `redo-bot-cost-when-hard-agent-lands` reruns this table on that day.

### 5.4 The trained models are live, 2026-10-04 (main @ `4b908f6`)

PR #243 shipped three models, embedded in `internal/learn/models/` (Hold'em 0.84 MB, Canasta
1.1 MB, Žolíky 2.7 MB). An admin switch per game (`learn.SetHardModel`, the console's Bots
card, persisted in `botsettings`) puts **Hard seats** on the model. It is **off by default**,
it is read on every bot move, and Easy, Medium and hints always stay on the rule bot.

`/debug/bots` now keys every decision by **engine** (`rule` or `net`), so a game with its
switch on doesn't blend the two into one "hard" series. `cmd/botcost -learned` throws the
same switch and labels those rows `hard/net`. Mariáš (new since Phase 0) is now in the sweep.

CPU time per decision, 5 matches × up to 1 500 decisions per configuration (load average
13–21 on 16 cores, so read CPU, not wall):

| Engine | CPU mean | CPU p95 | CPU p99 | KB/decision | seats/core* |
|---|---|---|---|---|---|
| **Mariáš Hard (rule)** | **42–54 ms** | **336–361 ms** | **377–421 ms** | **26 000–33 000** | **≈ 9** |
| Mariáš Easy/Medium | 0.07–0.08 ms | 0.14–0.16 ms | 0.2 ms | 15 | > 5 000 |
| Žolíky net, classic 8 seats (reproduced exactly on a rerun) | 9.9–10.2 ms | 51 ms | 219–230 ms | 9 000 | ≈ 46 |
| Žolíky net, other configurations | 1.1–1.6 ms | 2.9–3.5 ms | 5–9.6 ms | 600–800 | ≈ 290–430 |
| Canasta net | 0.40–1.15 ms | 1.2–4.6 ms | 2.3–9.6 ms | 77–322 | ≈ 410–1 180 |
| Hold'em net | 0.27–0.41 ms | 0.44–0.69 ms | 0.48–0.87 ms | 17–53 | ≈ 1 150–1 750 |
| Žolíky rule Hard | 0.38–0.78 ms | 0.77–1.7 ms | 1.3–2.2 ms | 139–415 | ≈ 610–1 240 |
| Every other rule bot | < 0.37 ms | < 0.8 ms | < 3.5 ms | < 93 | > 1 290 |

\* (1.35 s think window ÷ CPU mean) × 0.5 CPU share × 0.7 comfort, per core.

Both outliers are **bimodal**. Mariáš Hard's median decision is 0.1–0.2 ms, and the Žolíky
net's at 8 seats is 0.9 ms. A minority of positions costs hundreds of times the median, and
Mariáš's worst (1.37 s wall) outlasts the 0.9 s think window.

**What this means for the plan:**
1. **The expensive engine is Mariáš Hard, which is a rule bot.** The deep-vs-rule framing
   misses it. The fallback has to be the *next-cheaper engine for that game and skill*:
   - for a model seat, the rule bot at Hard;
   - for Mariáš Hard, Mariáš Medium (about 700x cheaper).

   Phase 2 should key cost classes by (game, skill, engine) from this table, not by "deep".
2. **The models cost 1–5x their rule bot,** except Žolíky's 8-seat tail. At these numbers the
   models alone need no gating below hundreds of seats per core.
3. **Mariáš Hard is what would trip the monitor.** Its p95 is already 0.4 of the think window,
   against amber at 0.5. On the one-vCPU box admission was measured on, a handful of Mariáš
   tables would put the server at amber. Bounding its search (a node or time budget, as
   Žolíky's meld search has) is cheaper than gating around it. **Done differently (§5.5):**
   the search already had a node budget, and lowering it measured weaker. Making each node
   allocation-free cut CPU about 4x and p95 to about 0.1 of the window, with moves unchanged.
4. **Phase 2 can now start.** There is something to switch: the per-game model flag, and
   Mariáš's skill. The smallest useful Phase 2 is the monitor driving the existing switch:
   - at **red**, the model flag goes off for every game and Mariáš Hard plays as Medium;
   - both come back after green has held.

   One catch: the model flag is read per move, not per turn. Žolíky's net plays a meld plan
   one step per call, so a flip mid-turn hands the heuristic half a plan. The switch has to be
   applied at the turn boundary (§3.5) before the monitor may drive it.

### 5.5 Mariáš Hard made cheap, 2026-10-04

Mariáš Hard plays its cards by sampling the unseen hands about 20 times and solving each deal
open-handed (`marias/sampling.go` on `tricks/search`). It was by far the most expensive bot
decision. Its cost is bimodal, and the bimodality is the node budget: across a sweep of 1 345
card-play positions (`marias/testdata/search_golden.txt`), the median decision searches 67 k
nodes, but **20% of them hit the 2 M-node budget** (p90 2.1 M, max 2.4 M: the last sample may
run 400 k past it). These are the early tricks of a deal. Bidding, talon and doubling are rules
of thumb and cost nothing.

The solver allocated on every node: a sorted move list (`sort.Slice`), the trick slice, a
`defer` closure, and a Go map as transposition table. Garbage collection was most of the cost.
It now lists moves into stack arrays in a precomputed order, keeps the trick in fixed arrays,
and reuses an open-addressed table across solves. It is **the same search, node for node**:
`tricks/search/reference_test.go` keeps the old solver and checks values, node counts and
exhaustion against it, and the golden sweep matches unmodified main at every position. About
40 ns per node, down from 180.

`botcost -game marias -matches 5`, both binaries run side by side at load ~110 on 16 cores
(CPU columns; before / after):

| Hard | CPU mean | CPU p95 | CPU p99 | allocs/decision | KB/decision |
|---|---|---|---|---|---|
| volený | 73.4 → **15.7 ms** | 500 → **104 ms** | 562 → **119 ms** | 1 293 583 → 5 405 | 32 907 → **225** |
| licitovaný | 56.3 → **13.1 ms** | 457 → **101 ms** | 511 → **117 ms** | 1 028 078 → 3 739 | 26 160 → **173** |

At lower load the old binary measured 52 / 47 ms CPU mean. It suffers more from contention
because its collector does. The worst decision is now bounded at about 2.4 M nodes, roughly
0.1–0.15 s of CPU, which is well inside the 0.9 s think window. By §3.2's formula one core now carries
about 30 Mariáš Hard seats, up from about 9.

**The budget was not lowered, because that measured weaker.** On `gamebench -game marias`,
600 seeds of duplicate play:
- Hard beats Medium by +12.5 ± 0.9 units a match (100 seeds).
- A 1 M-node budget against the shipped 2 M loses −0.90 ± 0.22 (volený).
- Even making 2 M a hard ceiling (cutting the last sample off instead of letting it overrun) lost
  −0.35 ± 0.15 in licitovaný, against −0.04 ± 0.09 in volený.

So Hard ships with the old budget semantics, and its moves are unchanged at every position.
The bench styles `hard-capped` and `hard-<n>k` stay, to re-measure if a cheaper Hard is
ever wanted.

### 5.6 Still open in Phase 0

- **Soak at `cpus: 1` and `cpus: 2`.** Increase the number of concurrent Hold'em bot tables,
  the expensive engine that exists today, until WebSocket broadcast p95 bends. That checks the
  §3.2 formula against a real container. The recorder is in place, so the soak only has to read
  `/debug/bots`.
- **Production baseline.** Once deployed, the ten-minute `bot decisions` log line gives real
  traffic's mix and `botCpuShare`.

## 6. Open questions

1. **Revoke order.** The plan uses LIFO with humans last. The alternative is to revoke tables
   furthest from finishing first, which saves more compute per revoke but interrupts games
   people are invested in. Recommendation: LIFO, because it is predictable and easy to test.
2. ~~When does a queued seat upgrade?~~ **Decided:** at the next round or deal boundary.
   Downgrades stay at turn boundaries.
3. **Is `BOT_CPU_SHARE` = 0.5 the right share?** The Phase 0 soak will show how much CPU KDB
   commits actually take under load.
4. **Multi-node (`FEATURE_FLAG_SYNC`):** capacity and leases are per node. A lobby should only
   offer Expert for a table that will be hosted on a node with a free lease. This is deferred
   until tables are placed across nodes.
