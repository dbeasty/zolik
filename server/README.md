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
- `GET /healthz`
- WebSocket: `ws://localhost:8090/ws/games/:id?token=<JWT>`
- REST:
  - Auth: `/auth/guest`, `/auth/register`, `/auth/login`, `/auth/refresh`, `/auth/logout`
  - Games: `/games`, `/games/:id`, `/games/:id/join`, `/games/:id/start`, `/games/:id/add-ai`, `/games/:id/replay`
  - Offline scoring: `/scoring-sessions/*`

