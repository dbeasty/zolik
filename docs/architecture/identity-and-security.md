# Identity and security

Code: [`server/internal/auth`](../../server/internal/auth),
[`server/internal/identity`](../../server/internal/identity),
[`server/internal/admin`](../../server/internal/admin),
[`server/internal/ratelimit`](../../server/internal/ratelimit),
[`server/internal/admission`](../../server/internal/admission).

## Who a player is

Every request that acts as somebody carries a JWT access token whose subject
is a **seat subject**: either an account's id (ObjectID hex) or a guest id.
That subject is what appears in `players[].id` at a table.

### Guests

- `POST /auth/guest` creates (or resumes) a guest: a durable random guest id,
  a refresh session, and an access token.
- Everything a guest plays is recorded against the guest id. A guest is not a
  durable statistics subject (no lifetime aggregate, no leaderboard) until the
  history is claimed by an account (`POST /auth/claim-guest`,
  `stats.Claimer.ClaimGuestHistory`; the guest's circle moves with it).
- **The guest key** ([`auth/guestkey.go`](../../server/internal/auth/guestkey.go))
  is what proves a device *is* a guest. The guest id cannot be the proof — it is
  broadcast to everyone at every table. The key is the id plus a MAC under the
  server's access secret: stateless, never rotated, small enough for a link
  (`/guest/<key>` is the guest's own link). It grants nothing a guest session
  does not, and is worthless once the guest is claimed. Rotating
  `JWT_ACCESS_SECRET` invalidates every guest key, turning returning guests into
  new ones. Resuming with a bare guest id mints a *new* guest.
- Guest names are cleaned (`auth.CleanDisplayName`, `MaxGuestNameRunes`).

### Accounts

- Accounts sign in through one or more **identities**, each a
  `(provider, subject)` pair with a unique index
  ([`models/identity.go`](../../server/internal/models/identity.go)).
- Providers ([`identity/providers.go`](../../server/internal/identity/providers.go)):
  - **Email**, passwordless: a six-digit code, valid 10 minutes, redeemed once;
    at most 5 codes per address per 15 minutes
    ([`auth/email.go`](../../server/internal/auth/email.go)). Outside a local
    `APP_ENV` the server refuses to start without SMTP.
  - **Google**, OIDC: browser redirect flow and native ID-token verification.
  - **Apple** and **Microsoft**: the same generic OIDC provider
    ([`identity/oidc.go`](../../server/internal/identity/oidc.go)) with their
    quirks as configuration; enabled by setting `OAUTH_<PROVIDER>_*`.
    `/auth/providers` lists what is configured.
- Sign-in resolution (`auth.Accounts.SignIn`,
  [`auth/accounts.go`](../../server/internal/auth/accounts.go)): a known
  identity wins; otherwise a *verified* email match attaches the new identity to
  the existing account; otherwise a new account is created. Linking and
  unlinking (never the last identity) work the same for every provider.
- Legacy username/password (`/auth/register`, `/auth/login`, bcrypt) remains
  for the SSH client.
- **Account deletion** (`DELETE /auth/account`) runs registered forgetters —
  matches, stats, circle, scorepads and, on kdb, the user's namespaces — then
  the account's own identities, sessions and record
  ([`app.go` `wireAccountDeletion`](../../server/internal/app/app.go)).
- Only the identity authority mints accounts: a self-hosted sync spoke verifies
  cloud-signed tokens but refuses to create accounts
  (`SetIdentityAuthority`).

## Tokens

| Token | Lifetime | Form | Notes |
|---|---|---|---|
| Access, account | 15 min | JWT | `accessTokenTTL` in [`auth/handlers.go`](../../server/internal/auth/handlers.go) |
| Access, guest | 7 days | JWT | `guestAccessTokenTTL`; a guest has no other way back in |
| Refresh | 30 days | Opaque random string, stored as a session row | Rotated on every refresh; the old token still answers for 1 min (`refreshReuseGrace`) for requests already in flight |
| Match-scoped (seat link) | 12 h | JWT with a match scope | Admitted only by routes a seated player needs at their own table (`MatchAuthMiddleware`) |
| Agent (MCP) | 30 days (invite), 1 h (OAuth flow) | JWT | Play-only |
| Offline pass | 30 days | Signed pass | Lets a device seat a signed-in account offline ([`auth/offlinepass.go`](../../server/internal/auth/offlinepass.go)) |
| Node credential | — | Signed by the node key | Enrolled devices/servers for sync and relay ([`auth/node.go`](../../server/internal/auth/node.go)) |
| Guest key | No expiry | id + MAC | See above |

Signing ([`auth/jwt.go`](../../server/internal/auth/jwt.go)):

- With `JWT_SIGNING_KEY` / `JWT_SIGNING_KEY_FILE` set, the server signs EdDSA
  (Ed25519) tokens with a `kid`, an issuer and an audience, and publishes the
  public key at `/.well-known/jwks.json`, so other nodes can verify without
  being able to mint. Without it, tokens are HS256 under `JWT_ACCESS_SECRET`.
  Both are accepted during a migration window; nothing else is.
- `CheckAccessSecret` refuses to start outside a local `APP_ENV` when
  `JWT_ACCESS_SECRET` is unset, the built-in default, or the
  `REPLACE_ON_FIRST_DEPLOY` placeholder.
- `JWT_REFRESH_SECRET` is read into config but unused (refresh tokens are not
  signed).

### Browser cookies

[`auth/cookies.go`](../../server/internal/auth/cookies.go): a web client may
ask (header `auth.SessionCookieHeader`) to keep its refresh token and guest key
in HttpOnly cookies scoped to the `/auth` path, instead of `localStorage`.
Reasons: Safari erases script-written storage after seven days without a visit,
and HttpOnly cookies are out of reach of page scripts. The guest-key cookie
lives 400 days. Credentialed CORS is allowed only for `/auth/*` from the same
host (`auth.SameHostOrigin`); every other route answers CORS `*` without
credentials ([`cmd/server/main.go`](../../server/cmd/server/main.go)).

### WebSocket authentication

Browsers cannot set headers on an upgrade, so the gameplay, lobby and personal
sockets take `?token=`. `auth.SubjectForMatch` accepts an ordinary token or one
scoped to that match. A seat that has been handed to someone else refuses the
original player's ordinary token after the upgrade (`errSeatHandedOver`).
`GET /matches/{id}` with no token returns the spectator view: every hand
hidden.

## Seats, links and table authorisation

- **Join code / invite link**: seats whoever opens it as themselves.
- **Seat link** (`/seat/<match>/<secret>`,
  [`match/seatlink.go`](../../server/internal/match/seatlink.go)): brings one
  person back to one seat on a new device; anybody at the table may mint one for
  anybody at it. Claiming it returns a match-scoped token.
- **Host-only actions** (start, add bot, seat order, delete) check
  `requireHost`. Debug state injection requires being seated at the table even
  in development.
- **Replay** is seated-only and requires both the operator flag
  (`FEATURE_FLAG_MATCH_REPLAY`) and a store keeping full history.
- e2e coverage: [`e2e/tests/authz.spec.ts`](../../e2e/tests/authz.spec.ts) and
  `auth-refusals.spec.ts`.

## Operator console (admin)

- Served on a **second listener** (`ADMIN_BIND:ADMIN_PORT`, default
  `127.0.0.1:8091`) from a separate router
  ([`app/admin.go`](../../server/internal/app/admin.go)); the public router does
  not register it at all, and production publishes the port to the host's
  loopback only, reached through an SSH tunnel.
- Starts only when `ADMIN_USERNAME` and `ADMIN_PASSWORD_HASH` (bcrypt, from
  `go run ./cmd/adminpass`) are both set. The console token signing key is
  derived from the hash, so changing the password invalidates every console
  token.
- Logins are limited to 10 per 15 minutes
  ([`admin/login.go`](../../server/internal/admin/login.go)).
- `ADMIN_EMAILS` optionally admits allow-listed accounts that sign in by
  verified email.
- Every bot-switch change is logged with who made it.

## Rate limiting and abuse controls

| Control | Where | Limit |
|---|---|---|
| Request body cap | [`app/limits.go`](../../server/internal/app/limits.go) | 8 MiB on every route (scorepad 16 KiB, MCP 1 MiB) |
| Email codes | `auth/email.go` (counted in the store) | 5 per address per 15 min |
| Admin login | `admin/guard.go` | 10 per 15 min |
| Hints | `match/hint.go` | 5 per 10 s per player |
| Relay frames | [`relay/relay.go`](../../server/internal/relay/relay.go) | Burst 100, refill 20/s per guest; 256 KiB frames; 8 guests |
| Capacity | `internal/admission` | 503 `SERVER_BUSY` with `Retry-After` past the connection, memory or CPU watermark |

[`ratelimit`](../../server/internal/ratelimit/ratelimit.go) is in-memory and
per-process by design, with a bounded key map; it stops stuck clients and
casual guessing, not a distributed attacker.

## Hardening switches

| Variable | Default | Production | Effect |
|---|---|---|---|
| `APP_ENV` | unset (= local) | `production` | Local turns on the switches below by default |
| `ENABLE_TEST_ENDPOINTS` | on when local | `false` | `/matches/{id}/debug-state` (writes state past the rules), `/auth/dev/last-code` (returns mailed codes) |
| `ENABLE_DEBUG_ENDPOINTS` | on when local | `false` | `/debug/memory`, `/debug/pprof`, `/debug/bots`, `/debug/capacity` |
| `SSH_ENABLED` | on when local | `false` | SSH terminal client; user `guest` with any password |
| `SSH_ALLOW_ALL_KEYS` | on when local | `false` | Admit any SSH public key |
| `AUTH_ALLOWED_RETURN_URLS` | app scheme + `PUBLIC_BASE_URL` | all served domains | Where OAuth may return |

Production states each of these as a literal in
[`deploy/compose/zolik.yml`](../../deploy/compose/zolik.yml), so a host's
`.env` cannot open them.

## Transport security for local play

Bluetooth and relay traffic between a host phone and its guests is end-to-end
sealed (X25519 + ChaCha20-Poly1305, host key pinned per table, a check code on
both screens); the relay sees which phone, how many guests and how many bytes,
and nothing else. See [clients.md](clients.md#embedded-core-offline-and-hybrid-tables).

## Known weak spots

- `/scoring-sessions` is unauthenticated, has no owner and no version check
  (see the [index](README.md#known-gaps-and-stale-documents)).
- Rate limits are per process; a multi-instance Mongo deployment multiplies
  them.
- The legacy password endpoints remain reachable for the SSH client.
