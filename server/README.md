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
  - Auth: `/auth/providers`, `/auth/guest`, `/auth/email/start`, `/auth/email/verify`,
    `/auth/oauth/:provider/start`, `/auth/oauth/:provider/callback`, `/auth/oauth/exchange`,
    `/auth/oauth/:provider/token`, `/auth/identities`, `/auth/claim-guest`, `/auth/guest-summary`,
    `/auth/refresh`, `/auth/logout` — see [User management](#user-management) below.
    Legacy: `/auth/register`, `/auth/login` (username/password, kept for the SSH/TUI client).
  - Games: `/games`, `/games/:id`, `/games/:id/join`, `/games/:id/start`, `/games/:id/add-ai`, `/games/:id/replay`
  - Offline scoring: `/scoring-sessions/*`

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

