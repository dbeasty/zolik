# Žolíky backend (v1 scaffold)

## Prerequisites
- MongoDB via Docker Compose (recommended)

## Run locally
From `server/`:
- `cp .env.example .env` (adjust secrets as needed)
- `docker compose up --build`

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

### How `REDIS_URL` resolves

Resolution turns on whether the variable is *set*, not on whether it is empty:

| `REDIS_URL` | Result |
| --- | --- |
| unset, locally | tries `redis://localhost:6379/0`; **falls back to local-only if nothing answers** |
| unset, `APP_ENV` set | no Redis — a deployment has to say where its Redis is |
| set to a value | required: if it cannot be reached, the server **refuses to start** |
| set to empty | no Redis — the explicit opt-out |

So `go run ./cmd/server` picks up a running Redis on its own and still starts on a machine without
one. The asymmetry is deliberate: a URL somebody configured must never be downgraded silently, because
a deployment serving as a lone instance while believing it is clustered fails far worse than one that
refuses to boot.

## Terminal client (SSH)

`SSH_ENABLED=true` embeds **[client-tui](../client-tui/)** on port **2222**. It is **off by default** —
it binds a second port, and a second server started on one machine would otherwise fail to bind it
for a feature that server was not being used for.

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

## Admin console

`/admin` is an operator's console served by the server itself (`internal/admin`) — a single embedded
page, no build step and no second artifact to keep in step with the API. It lists accounts, resets a
password, revokes sessions, deletes an account, and shows usage: accounts and activity, matches per
day and per game, open sessions, and this instance's live connections.

Who gets in is an allow-list of addresses in the environment, not a flag on the user document:

```sh
ADMIN_EMAILS=you@example.com,someone@example.com
```

**Unset means nobody gets in.** That default is deliberate — an admin API that opens up when
unconfigured is a far worse failure than one that is unreachable. A caller must also be signed in
with a *verified* address on the list, so claiming an administrator's address at sign-up gains
nothing. Because membership lives in configuration, there is no bootstrap problem, removing an
address takes effect on the very next request, and nothing the running system exposes can grant it.

Two things the console deliberately does not do. Deleting an account removes it along with its
identities and sessions, but **keeps its match records** — those are the immutable history every
other player's statistics are derived from, so deleting them would quietly rewrite opponents'
records too. And a generated password is shown exactly once and stored only as a bcrypt hash;
there is no way to read it back afterwards.

Sign-in uses the ordinary emailed-code flow, so locally (no `SMTP_HOST`) the code is written to the
server log rather than mailed.

### Feedback

`POST /feedback` takes a bug report or suggestion from a client (`internal/feedback`); the console's
Feedback section is where they are read and triaged. Reports move `new → open → resolved`, carry an
operator-only note, and can be deleted outright — that last one is for spam, since resolving is what
you want for a report you have actually dealt with.

The endpoint takes **optional** auth rather than requiring it. Most players never sign in, so
demanding an account first would lose exactly the reports most worth reading; whatever session *is*
presented gets recorded, so a signed-in report can be traced back. Being open is also why it is
throttled, at 10 reports per reporter per hour — a stuck client retrying in a loop would otherwise
fill the collection with no malice involved.

A report denormalises its author (username plus the account or guest id) rather than referencing a
user document, for two reasons: guests have no document to reference, and a report has to outlive its
author being deleted. `contactEmail` is supplied by the reporter and is **not** verified — it is
somewhere to write back, and nothing may treat it as proof of identity.

