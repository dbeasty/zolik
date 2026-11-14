# Developer guide

How to build, run and test Zolik (Jokerless), and how to make the two most
common changes: adding a game and adding a message key. For how the system is
put together, read [../architecture/README.md](../architecture/README.md)
first.

## Prerequisites

| Tool | Why | Notes |
|---|---|---|
| Go 1.26 | Server, terminal client, tools | `server/go.mod` says `go 1.26.0` |
| The **kdb** repository, checked out as a sibling: `../kdb` next to this repo | The server links kdb | `server/go.mod` has `replace github.com/limidus/kdb/go => ../../kdb/go`; `go build`, `go test` and the Docker build all need it. A zolik branch can depend on a kdb branch: merge kdb first |
| Node 20+ (22 in the Docker build), **arm64** on Apple silicon | Expo client, e2e | On a Mac put `/opt/homebrew/bin` first on `PATH`. An x64 `/usr/local/bin/node` makes Expo fail (HTTP 500 from lightningcss) and the shared `node_modules` are arm64-only |
| Docker with buildx | `scripts/dev-stack.sh`, `scripts/deploy.sh` | |
| Playwright (installed by the scripts) | e2e | Run from `e2e/` |
| uv (Python 3.11+) | Only for training models in `ml/` | |
| gomobile, Xcode, Android NDK | Only for the embedded core and native builds | `scripts/build-mobile-core.sh` |

```sh
git clone <zolik> zolik && git clone <kdb> kdb   # side by side
cd zolik/client-react-native && npm install
cd ../e2e && npm install
```

## Running it

### The dev stack (Docker)

[`scripts/dev-stack.sh`](../../scripts/dev-stack.sh) brings up the same shape
as production — one kdb container — plus the Expo web client:

```sh
./scripts/dev-stack.sh up      # server http://127.0.0.1:8090, web http://127.0.0.1:8114
./scripts/dev-stack.sh logs    # tail the server container
./scripts/dev-stack.sh test    # Go, terminal client, tsc + jest, then Playwright
./scripts/dev-stack.sh soak    # e2e/longevity: browsers playing for hours
./scripts/dev-stack.sh down
```

Useful variables: `ZOLIK_DEV_COMPOSE=mongo` (Mongo + Redis + two app
instances, `server/docker-compose.yml`), `ZOLIK_DEV_DATA=clean|prodcopy`,
`ZOLIK_DEV_PORT` / `ZOLIK_DEV_WEB_PORT`, `ZOLIK_KDB_HISTORY_MODE`. The stack
turns on test and debug endpoints. `server/.env` is created from
`.env.example` on first run.

Before trusting that "the stack is up", check that the process answering on
the port is yours (`lsof -nP -iTCP:8090 -sTCP:LISTEN`): another session's
server may be holding it.

### A local stack without Docker

```sh
cd server
go build -o /tmp/zolik-server ./cmd/server
FEATURE_FLAG_DB_ENGINE=kdb KDB_PATH=/tmp/zolik-kdb REDIS_URL= PORT=8090 /tmp/zolik-server
```

Unset `APP_ENV` means local: the dev JWT secret is accepted, SMTP is optional
(sign-in codes are logged; `GET /auth/dev/last-code` returns them), test and
debug endpoints are on, and the SSH client listens on 2222
(`ssh -p 2222 guest@localhost`). With `FEATURE_FLAG_DB_ENGINE` unset the server
tries Mongo at `mongodb://localhost:27017`.

Then the web client, pointed at it:

```sh
cd client-react-native
PATH=/opt/homebrew/bin:$PATH EXPO_PUBLIC_ZOLIK_BASE_URL=http://127.0.0.1:8090 \
  npx expo start --web --port 8114
```

Tips:

- Do not set `CI=1` for a dev Metro: it stops file watching and your edits
  never reach the page.
- Running Metro from a git worktree: give it its own `TMPDIR`, or a shared
  Metro cache can serve the main checkout's routes.
- To exercise a seat as a specific player, use the bearer token; the match view
  follows the token (no token = spectator view, every hand empty).

## Tests

| Suite | Command | Notes |
|---|---|---|
| Server | `cd server && go test ./...` | Store-backed suites (`internal/match`, `internal/auth`, ...) **skip and still say `ok`** unless a store is configured. Use `ZOLIK_TEST_DB_ENGINE=kdb go test ./...`. The Canasta suite is long. |
| Module contract | `go test ./internal/module` | `allmodules_test.go`: ~30 contract tests over every module. `ZOLIK_FUZZ_SEEDS=500` for a long random-play soak |
| Terminal client | `cd client-tui && go test ./...` | |
| Client | `cd client-react-native && ./node_modules/.bin/tsc --noEmit && npm test` | Jest |
| Locales | `npm run i18n` (and `npm run i18n:labels` with a server running) | |
| End-to-end | `cd e2e && ZOLIK_E2E_API_BASE=http://127.0.0.1:8090 ZOLIK_E2E_WEB_BASE=http://127.0.0.1:8114 npx playwright test` | Must run from `e2e/`. `ZOLIK_E2E_PHONE=1 ... --project=phone` for touch. See [`e2e/README.md`](../../e2e/README.md) |
| Longevity | `./scripts/dev-stack.sh soak` | [`e2e/longevity`](../../e2e/longevity) |
| Benchmarks | `go test ./internal/dbperf -bench '/kdb'` | Mongo rows need the Mongo stack; run kdb rows isolated |
| Bots | `go run ./cmd/aibench`, `./cmd/gamebench`, `./cmd/botcost` | Strength and cost |
| Trainer | `cd ml && uv sync && uv run pytest` | Builds `gameenv` itself |

Things that make e2e lie or flake:

- Keep the default bot think time. `BOT_THINK_MIN_MS=1` breaks gameplay specs;
  `dev-stack.sh test` sets 400–1300 ms.
- Do not edit client source while Playwright is running: Metro rebuilds and
  `goto` timeouts look like regressions. Re-run before investigating.
- Assert what the user sees. Several suites once passed while watching the
  wrong element (`e2e/README.md`, "Two ways these tests have passed while
  proving nothing").
- There is no CI and no auto-merge on this repository: run the suites locally
  before merging.

## Repository conventions

- **Rules live in modules.** A client, the runtime or a tool that computes a
  rule is a bug; ship a fact or an offer instead.
- **Keys, never sentences**, from the server. Keys and their params maps must be
  static literals so `dump-keys` can see them.
- **Refusals point at written rules**: a new error code gets a rule id in the
  module's `ExplainRefusal`, never a per-code explanation.
- **House rules are options**: a variant rule is a declared `OptionSpec`
  defaulting to the current behaviour.
- **Never end a turn in a dead position**: if a move would leave a player with
  no legal way to finish, refuse the move that ends the turn rather than
  strand them for the deal.
- **Skins change colours and decoration only**, never sizes.
- **Live boards come from `Manager.Current`**, not from the repository.
- **Lock order**: never wait for `KDB.mu` while holding a namespace lock; never
  hold a match's entry lock while notifying observers or broadcasting.
- **Versioning**: bump [`VERSION`](../../VERSION) only for a release; everything
  else is computed by `scripts/version.sh`.
- Git: work on a branch; commits end with the attribution line your tooling
  requires; PRs are merged with a merge commit.

## Adding a game module

The smallest complete modules are good templates:
[`internal/sedma`](../../server/internal/sedma) and
[`internal/lora`](../../server/internal/lora) (trick-taking),
[`internal/ginrummy`](../../server/internal/ginrummy) (rummy),
[`internal/okobere`](../../server/internal/okobere) (chips). Read
[game-modules.md](../architecture/game-modules.md) first.

1. **Create the package** `server/internal/<game>` with a `New()` returning a
   type that implements `module.GameModule`.
2. **Descriptor** (`descriptor.go` or the module file): a stable `ID` (it is
   stored on every match and used as a message-key prefix), `Label`, seat range,
   `Variations` (each with its own seat range and default options) and
   `Options`. Use the shared option helpers where they apply:
   `module.PauseOption()`, `OpenDiscardPileOption()`,
   `BotSkillOption()`, `StandInOption()`, `StepOutOption()`,
   `LeaveAfterAwayOption()`, `DealAroundOption()`, `HintsOption()`. Every
   house rule is an option whose default is the usual rule. Set `Deck` for a
   non-French deck and `SimulatedGambling` for chip games.
3. **State and engine**: a JSON-serialisable state. `NewMatch` must be a pure
   function of config, players and seed. `Apply` must be pure and
   deterministic, return `module.Error{Code: ...}` for refusals, and emit
   events (types are message keys).
4. **View**: return only what that viewer may see. Spectators get the onlooker
   view. Fill `Seats` (`Active` is how the runtime knows whose turn it is),
   `Side` for partnerships, and facts as label keys with params. If an event
   names a private card, implement `EventProjector`; implement `EventNarrator`
   for the move log.
5. **Offers**: return every offer, disabled ones with `WhyNot` codes,
   `Source`/`Target` selectors so cards can be dragged, `ParamSpec`s for
   non-card choices, `Facts` for amounts, `LabelKey` where two offers share a
   verb, `Undo` / `Composite` / `Manual` where they apply. Use
   `module.PauseOption` + the intermission helpers for "continue" between
   rounds rather than inventing a runtime pause.
6. **Finish and score**: `Finished` returns all winners (or none). Implement
   `Ranked` (`module.RankByScore(s)`) and, if the match has rounds, `Rounded`.
7. **Written rules**: implement `Rules(cfg)` with `module.Section` /
   `module.Rule`, resolved for the variation and options, and
   `ExplainRefusal(cfg, code)` mapping every code you emit to rule ids.
8. **Bot**: implement `Botted`. Respect `BotSeat.Skill` (empty = your default)
   and use `BotSeat.Seed` for any randomness. Never peek at hidden state beyond
   the seat's own view. Easy < Medium < Hard should measure as ordered.
9. **Optional**: `Seated` (teams), `DropIn`, `LateSeater`, `DealsAround`,
   `LessonProvider`, `OpenViewer` (replay), `ServerDealt` (single-player games
   whose deal must stay secret).
10. **Tests** in the package: a conformance test with
    `module.PlayWithOffers` over several seeds and variations, a
    `module.RuleIndexCheck{Module: New(), Dirs: []string{"."}}.Run()` test,
    bot strength tests, and engine tests. Add the module (and its variations)
    to the table in
    [`internal/module/allmodules_test.go`](../../server/internal/module/allmodules_test.go).
11. **Register it** — all five lists, they are separate:
    - `hostedModules` in [`internal/app/app.go`](../../server/internal/app/app.go)
    - [`cmd/dump-keys/main.go`](../../server/cmd/dump-keys/main.go): the
      registry **and** the list of package directories it scans for codes
    - [`cmd/dump-lessons/main.go`](../../server/cmd/dump-lessons/main.go)
    - [`cmd/botcost/main.go`](../../server/cmd/botcost/main.go) (then rerun it
      and update `botgov.DefaultCosts` if the bot is expensive)
    - [`internal/gamemcp/games.go`](../../server/internal/gamemcp/games.go)
12. **Message keys**: regenerate `serverKeys.json` (and `lessonKeys.json` if
    the game ships lessons) and translate every new key (see below). Add the game's name to the "names that stay as they are" list
    in [`scripts/check-server-labels.js`](../../scripts/check-server-labels.js)
    if it is a proper name.
13. **Client** (optional): place the id in `DEFAULT_POPULARITY_ORDER` in
    [`src/lib/gameOrder.ts`](../../client-react-native/src/lib/gameOrder.ts)
    (unlisted ids sort last); a new deck needs card art.
14. **Verify** in a browser: `every-game-ui.spec.ts` and `every-game.spec.ts`
    pick up the new module automatically; play a few hands yourself.

Nothing else in the runtime or the clients should need to change. If it does,
the module contract is missing something — fix it there, generically.

## Adding a message key

### A key the server sends (error code, option label, fact, rule sentence, event)

1. Use the key as a **static string literal** in Go (a constant is fine). A key
   or a params map passed through a variable is invisible to the scanner and
   will reach players as `SCREAMING_SNAKE`.
2. Regenerate the golden file:

   ```sh
   cd server
   go run ./cmd/dump-keys > ../client-react-native/src/lib/serverKeys.json
   go test ./cmd/dump-keys          # fails if the file is stale
   ```
3. Add English wording to
   [`client-react-native/src/lib/locales/en.ts`](../../client-react-native/src/lib/locales/en.ts)
   and a translation to **each of the other 23 locale files** in the same
   directory. Option labels from `/modules` are re-keyed in
   [`gameLabels.ts`](../../client-react-native/src/lib/gameLabels.ts); a
   label two modules word differently needs a module-scoped key.
4. Because `en.ts` changed, refresh the game MCP server's copy:

   ```sh
   cd server && go generate ./internal/gamemcp   # rewrites messages_en.json
   go test ./internal/gamemcp                     # TestMessagesMatchClientBundle
   ```
5. Run the client checks: `npm test` (`serverKeys.test.ts`, `i18n.test.ts`) and
   `npm run i18n`.

### A client-only key

Add it to `en.ts` and all 23 other bundles, then run
`go generate ./internal/gamemcp` (any `en.ts` change breaks
`TestMessagesMatchClientBundle` until you do) and `npm test`.

### Lesson and notification text

Lesson keys are a second golden file:
`go run ./cmd/dump-lessons > ../client-react-native/src/guide/lessonKeys.json`
(checked by `go test ./cmd/dump-lessons` and a client test that every key is
worded in every language; lesson wording lives under `locales/guide`);
push texts are generated by
[`scripts/gen-notify-texts.js`](../../scripts/gen-notify-texts.js) into
`server/internal/notify/texts.json`.

## Other common tasks

| Task | How |
|---|---|
| Hash an admin password | `cd server && printf '%s' 'passphrase' \| go run ./cmd/adminpass` |
| Web push keys | `go run ./cmd/vapid-keys` |
| Play a game from the shell | `go run ./cmd/game-mcp cli` (or the MCP server over stdio) |
| Seed a position in a dev match | `POST /matches/{id}/debug-state` with the module's own state JSON (test endpoints only; you must be seated) |
| Test a local model | `ZOLIK_LEARNED_MODEL_<GAME>=/path/model.bin` |
| Build the phone core | `scripts/build-mobile-core.sh` |
| Run the phone server on a desktop | `go run ./cmd/phonehost` |
| Deploy | See [deployment-and-operations.md](../architecture/deployment-and-operations.md) |
