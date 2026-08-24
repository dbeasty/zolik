# Žolíky backend (v1 scaffold)

## Prerequisites
- MongoDB via Docker Compose (recommended)

## Run locally
From `server/`:
- `cp .env.example .env` (adjust secrets as needed)
- `docker compose up --build`

## Scaling (multiple app instances)

| Store | Purpose |
|-------|---------|
| **MongoDB** | Games, users, sessions, stats (source of truth) |
| **Redis pub/sub** | Fan-out WebSocket messages when players connect to *different* app pods |

Redis is **not** used for login, registration, or guest sessions — only for `zolik:ws:broadcast` envelopes after a game action is persisted.

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
- WebSocket: `ws://localhost:8090/ws/matches/:id?token=<JWT>` — carries
  `module.Action` in and a per-viewer `match_state` out
- REST:
  - Auth: `/auth/guest`, `/auth/register`, `/auth/login`, `/auth/refresh`, `/auth/logout`
  - Games hosted: `GET /modules` — every module's self-description, which is
    what a lobby renders its picker and its new-match form from
  - Matches: `POST /matches`, `GET /matches/:id`, `/matches/:id/join`,
    `/matches/:id/start`, `/matches/:id/add-bot`
  - Statistics: `/users/me/stats`, `/users/me/matches`, `/users/me/head-to-head`,
    `/leaderboard`, `GET /matches/:id/result`
  - Offline scoring: `/scoring-sessions/*` (a pen-and-paper scorepad, unrelated
    to hosted matches)

Dev-only, when `APP_ENV` is unset or `local`:

- `POST /matches/:id/debug-state` — replaces a match's state with the module's
  own state blob, so a test can start from the position it wants. It bypasses
  every rule by design.

