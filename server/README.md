# Žolíky backend (v1 scaffold)

## Prerequisites
- MongoDB via Docker Compose (recommended), **or** nothing at all — see
  [Storage engines](#storage-engines-mongo-or-embedded-kdb) for the embedded
  KDB mode.

## Run locally
From `server/`:
- `cp .env.example .env` (adjust secrets as needed)
- `docker compose up --build`

## Storage engines (Mongo or embedded KDB)

The server has two storage backends behind one repository layer, selected at
runtime by a feature flag — the same binary serves either, so a deployment
can switch engines by changing an environment variable:

| Variable | Values | Meaning |
|---|---|---|
| `FEATURE_FLAG_DB_ENGINE` | `mongo` (default) / `kdb` | Which engine this process runs on |
| `FEATURE_FLAG_KDB` | `true` / `false` | Boolean spelling of the same flag (`true` = kdb) |
| `KDB_PATH` | path, default `data/kdb` | Where the embedded engine keeps its data |

**Mongo** is what every existing deployment runs: the compose stack above
(app, app2, MongoDB, Redis, mongo-express).

**KDB** ([sibling repo](../../kdb), `github.com/limidus/kdb/go`) runs
*embedded in the server process* — no database server, no Redis (the ws hub
and lobby fall back to their in-process modes), one binary plus a data
directory. That is the low-memory single-instance deployment shape:

```bash
# bare metal / dev — no Docker, no Mongo, nothing to start first
FEATURE_FLAG_DB_ENGINE=kdb go run ./cmd/server

# or the one-container stack
docker compose -f docker-compose.kdb.yml up --build
```

Building the image (either compose file) needs the `kdb` repo checked out as
a sibling of this one (`../../kdb` from `server/`), wired in as a BuildKit
named context; `go build`/`go test` need the same sibling via the go.mod
`replace`.

What differs behind the flag, deliberately:

- **Durability**: KDB fsyncs every commit before acking; Mongo acks first and
  journals on an interval. KDB writes cost a few ms each — fine for a card
  game, and the honest price of not losing an acked write on power loss.
- **Uniqueness and CAS**: Mongo enforces unique indexes and filtered replaces
  server-side. The KDB layer enforces the same contracts in-process under
  per-namespace critical sections (`internal/db/kdb.go`), which is correct
  precisely because the KDB shape is single-instance. Do not run two server
  processes against one `KDB_PATH` — the engine's directory lock refuses it
  anyway.
- **TTLs**: Mongo expires sessions/codes/flows/abandoned matches with TTL
  indexes; the KDB layer filters expired documents on read and sweeps them
  once a minute.
- **Scaling**: the Redis-backed multi-instance story below is Mongo-only.
  KDB mode is one instance, by design.

`go test ./...` exercises the KDB layer with no services running. The same
integration suites that run against Mongo run against KDB with
`ZOLIK_TEST_DB_ENGINE=kdb go test ./internal/auth ./internal/match`.
Side-by-side performance numbers: `go test ./internal/dbperf -bench . -benchmem`
(Mongo rows skip unless the dev stack is up) — see
[`docs/kdb-port.md`](../docs/kdb-port.md#insert-and-read-head-to-head) for
the headline insert/read comparison (KDB reads ~525x faster, in-process;
Mongo writes ~21x faster, no fsync-per-commit).

## Full stack with the web client

Docker only runs the backend (`app`, `app2`, Mongo, Redis, mongo-express) — there
is no web page on any of those ports. To actually play in a browser, start the
[client-react-native](../client-react-native/) web build separately, pointed at
the Docker API.

**Start**

```bash
cd server
docker compose up -d --build   # app :8090, app2 :8092, mongo :27018, redis :6379, mongo-express :8081

cd ../client-react-native
EXPO_PUBLIC_ZOLIK_BASE_URL=http://localhost:8090 npx expo start --web --port 8114
```

Open **http://localhost:8114** and press **Play**.

**Stop**

```bash
# Ctrl-C the expo process, then:
cd server
docker compose down            # add -v to also wipe the Mongo/Redis volumes
```

### Troubleshooting

- **`address already in use` on 8090/8092`** — something else is already
  listening, often a stray `go run` server left over from an earlier session.
  Find it with `lsof -nP -iTCP:8090 -sTCP:LISTEN` and stop it before
  `docker compose up`.
- **`app`/`app2` crash-loop with `IndexKeySpecsConflict` or
  `DuplicateKey ... matchId_1`** — an existing `mongo_data` volume predates the
  unique (non-sparse) `matchId` index on `match_results`. A fresh volume is
  unaffected; an older one needs the stale index dropped and any leftover
  `matchId: null` rows cleared, then the app containers restarted so they
  rebuild the index:
  ```bash
  docker compose exec mongo mongosh --quiet zolik --eval 'db.match_results.dropIndex("matchId_1")'
  docker compose exec mongo mongosh --quiet zolik --eval 'db.match_results.deleteMany({matchId: null})'
  docker compose restart app app2
  ```
  Or, if that volume's data doesn't matter, `docker compose down -v` wipes it
  and starts clean instead.

## Scaling (multiple app instances)

| Store | Purpose |
|-------|---------|
| **MongoDB** | Games, users, sessions, stats (source of truth) |
| **Redis pub/sub** | Fan-out WebSocket messages when players connect to *different* app pods |
| **Redis hash** | Cross-instance presence for the waiting room (`internal/lobby`) — who's currently waiting, mirrored so a host's "invite" lands correctly regardless of which pod they or the waiting player are connected to |

Redis is **not** used for login, registration, or guest sessions — only for `zolik:ws:broadcast` envelopes after a game action is persisted, and for waiting-room presence. Without `REDIS_URL` the waiting room still works, but only sees players connected to the same instance — fine for local dev and single-instance deployments.

- Set `REDIS_URL` (see `.env.example`). Compose includes `redis` plus a second app on port **8092** for smoke tests.
- Use a load balancer with **sticky sessions** for WebSocket upgrades, *or* accept that clients may reconnect to any node (state is always loaded from MongoDB).
- Optional `INSTANCE_ID` labels each pod in logs.

Without `REDIS_URL`, the hub runs in **local-only** mode (fine for development).

## Terminal client (SSH)

When `SSH_ENABLED=true` (default in local), the server embeds **[client-tui](../client-tui/)** on port **2222**:

```bash
ssh -p 2222 guest@localhost
```

See [client-tui/README.md](../client-tui/README.md) for key bindings.

## GUI clients

**React Native (primary GUI):** **[client-react-native](../client-react-native/)** — Expo app for web/iOS/Android. See [client-react-native/README.md](../client-react-native/README.md).

## Endpoints

There is one gameplay path, and it does not name a game. `/games/*` and
`ws://…/ws/games/:id` are gone, along with the 24-field `GameStateMsg` they
carried; `cmd/migrate-games` moves any documents left behind.

- `GET /healthz`
- `GET /version` — `{"version": "1.1.1.2", "commit": "7feb025"}`, this binary's
  build. See [Versioning](../README.md#versioning) at the repo root.
- WebSocket: `ws://localhost:8090/ws/matches/:id?token=<JWT>` — carries
  `module.Action` in and a per-viewer `match_state` out
- REST:
  - Auth: `/auth/providers`, `/auth/guest`, `/auth/email/start`, `/auth/email/verify`,
    `/auth/oauth/:provider/start`, `/auth/oauth/:provider/callback`, `/auth/oauth/exchange`,
    `/auth/oauth/:provider/token`, `/auth/identities`, `/auth/claim-guest`, `/auth/guest-summary`,
    `/auth/refresh`, `/auth/logout` — see [User management](#user-management) below.
    Legacy: `/auth/register`, `/auth/login` (username/password, kept for the SSH/TUI client).
  - Games hosted: `GET /modules` — every module's self-description, which is what a lobby
    renders its picker and its new-match form from
  - Matches: `POST /matches`, `GET /matches/:id`, `/matches/:id/join`, `/matches/:id/start`,
    `/matches/:id/add-bot`, `/matches/:id/invite` — see [Waiting room](#waiting-room) below
    for `/invite`
  - Waiting room: `/lobby/waiting`, and WebSocket `ws://localhost:8090/ws/lobby?token=<JWT>`
  - Statistics: `/users/me/stats`, `/users/me/matches`, `/users/me/head-to-head`,
    `/leaderboard`, `GET /matches/:id/result`
  - Offline scoring: `/scoring-sessions/*` (a pen-and-paper scorepad, unrelated to hosted
    matches)

Dev-only, when `APP_ENV` is unset or `local`:

- `POST /matches/:id/debug-state` — replaces a match's state with the module's own state blob,
  so a test can start from the position it wants. It bypasses every rule by design.

## Waiting room

A second, lighter lobby concept sits above a specific game's: the pool of human players who are
simply available to play, visible to any host looking to pick someone up directly instead of
sharing a join code (`internal/lobby`). Connecting to `/ws/lobby` *is* the request to wait — there
is nothing else to negotiate — and the connection doubles as how a picked-up player is notified
(`{"type":"lobby_invited",...}`) the moment a host seats them via `POST /matches/:id/invite`.

It rides the same Hub and ConnRegistry the per-match WebSocket rooms use (a reserved room id,
`lobby.RoomID`), so it gets the existing local-write-plus-Redis-fanout broadcast path for free
rather than needing a transport of its own. Both it and the runtime take that transport from
`internal/ws`, so neither imports the other: `/invite` reaches the waiting room through a
narrow, primitive-typed `match.WaitingLookup` interface that `*lobby.Store` happens to satisfy,
wired together in `app.go`. The runtime never learns what a waiting room is.

## User management

Every player can play as a guest with no account. A guest's device gets a durable, random guest id
(`models.Session.GuestID`), and every match they play is recorded against it — not thrown away —
so that when they later sign in, that history can be claimed onto the new account
(`stats.Claimer.ClaimGuestHistory`, `Handlers.ClaimGuest`). A guest is never durable on its own: it
earns no lifetime aggregate and never appears on a leaderboard (`stats.Subject.Durable`).

Registered accounts sign in through one or more identities (`internal/identity`, `models.Identity`),
each a `(provider, subject)` pair with a database-enforced unique index — the same external account
can never end up attached to two players here. Two providers ship built in:

- **Email** — passwordless: a six-digit code is mailed and redeemed once (`internal/auth/email.go`).
  No password exists anywhere in this path, so there is nothing to reset or leak on reuse.
- **Google** — OIDC, both the browser redirect flow and native ID-token verification.

Apple and Microsoft are implemented as the same generic OIDC provider (`internal/identity/oidc.go`,
`providers.go`) with their specific quirks (Apple's minted client-secret JWT, Microsoft's per-tenant
issuer) expressed as configuration — enabling either is setting that provider's environment
variables (see `.env.example`), not writing code. `/auth/providers` reports whichever are configured,
so a client's sign-in screen is built from what the server actually offers.

Signing in resolves in a fixed order (`auth.Accounts.SignIn`): a known identity wins outright;
otherwise a *verified* email match attaches the new identity to an existing account; otherwise a new
account is created. Linking a second provider to an already-signed-in account, and unlinking one
(never the last), are handled the same way regardless of which provider is involved.

