# AI agent seats over MCP

Status: Phase 1 implemented and verified against a running server on `claude/mcp-agent-seats`.

## Goal

An AI client (Claude, or any MCP-capable agent) can connect to the server as an
MCP client and either

- **join one specific table** by match id or join code, or
- **register as generally available**, so a host can pick it from a list and
  seat it with one action — the way "Add a bot" works today.

An agent seat has its own icon (an `✦ AGENT` badge, machine-pool face) so
nobody mistakes it for a person or for a server-driven bot.

In games where everyone does not have to stay connected — poker, blackjack —
an agent (or a person) with a connectivity problem is **paused out**: the table
plays on and the seat is played passively until its player returns. This is not
tournament play, so nobody is eliminated for a dropped connection.

## Design

### Agents are seats, not bots

`models.Player` gains `IsAgent` and `AgentLabel`. `IsAI` keeps meaning "the
server drives this seat". An agent plays from outside, can be slow, and can
vanish, so every presence rule that applies to a person applies to it, and the
bot loop does not touch it while it is present.

### Transport: MCP Streamable HTTP, hand-rolled

`POST /mcp` speaks JSON-RPC 2.0 (`initialize`, `ping`, `tools/list`,
`tools/call`, notifications → 202). `GET /mcp` is 405: there are no
server-initiated messages. No SDK dependency; the surface is small and the
repo has none.

Auth is the existing bearer token. `POST /auth/guest` is enough to obtain one,
so an agent needs no account; a signed-in token seats the agent under that
account's identity (still flagged as an agent).

### Tools

| tool | purpose |
|---|---|
| `register_agent{name,label?,available?}` | first call; `available` offers it to hosts |
| `set_available{available}` | start/stop being offered |
| `list_tables` | unfinished tables it sits at, with `yourTurn` — seats a host gave it show up here |
| `join_table{table}` | take a seat in a lobby by id or code |
| `get_state{matchId}` | the viewer-filtered state + `legalActions` + `playable` (ready-to-send actions at the offer's own minimum values) |
| `wait_for_turn{matchId,timeoutSeconds≤25}` | long-poll until the table waits on it or ends |
| `act{matchId,offerId,verb,cards?,target?,params?}` | play, through `Manager.HandleAction` — the same validated path as a human |

State is the same `MatchStateMsg` a client gets, so hidden information is
filtered by the module exactly as for any viewer. Refusals come back as
`isError` results carrying the stable code and `ruleIds`.

### Connecting from the app: "Connect an AI agent"

The host's table screen has a **Connect an AI agent** button. It calls
`POST /agents/invite {matchId}`, which mints a fresh agent identity and a token
with `scope: "agent"` bound to that table, and returns the endpoint plus a
ready-to-paste `claude mcp add --transport http zolik <url> --header
"Authorization: Bearer <token>"`, an `mcpServers` JSON block, and the sentence
to tell the agent. When the agent calls `register_agent` it is seated at that
table (idempotently). The token is rejected by every other route, so pasting it
into a tool does not hand over the host's account; it lasts 30 days.

### OAuth: connecting claude.ai and other clients that sign in themselves

`/mcp` is an OAuth-protected resource and this server is its authorization
server, per the MCP authorization profile. A client given only the URL
`https://<host>/mcp` finds everything else itself:

1. `POST /mcp` without a token → `401` with
   `WWW-Authenticate: Bearer resource_metadata=".../.well-known/oauth-protected-resource"`.
2. `/.well-known/oauth-protected-resource` (RFC 9728) and
   `/.well-known/oauth-authorization-server` (RFC 8414) describe the endpoints.
3. `POST /oauth/register` — dynamic client registration (RFC 7591). Public
   clients only; redirect URIs must be https, or http on a loopback host.
4. `GET /oauth/authorize` — authorization code with PKCE (S256 mandatory).
   Sends the person to the app's `/oauth/consent` screen.
5. The person is signed in as themselves, sees who is asking, and presses
   **Allow** or **Deny** (`POST /oauth/approve`, ordinary login required). The
   server answers with the redirect back to the client.
6. `POST /oauth/token` — `authorization_code` and `refresh_token` grants.
   Access tokens last 1 h; refresh tokens 30 days and rotate.

Nothing is stored. A registered client, an authorization request, a code and a
refresh token are each a token signed with their own key derived from the
access secret (`auth.DeriveKey`), so none can be presented as another or as an
access token, and any instance can verify what another issued. Codes are
single-use (in-memory, one-minute lifetime). Cost: a refresh token cannot be
revoked before it expires.

The agent that results has its own identity — one per (person, client name), so
a reconnect keeps its seats — and an `agent`-scoped token that every route but
`/mcp` rejects. It cannot see the person's account.

To add it in claude.ai: *Settings → Connectors → Add custom connector*, URL
`https://<your host>/mcp`. `PUBLIC_BASE_URL` must be the public https origin
(it is the OAuth issuer); set `OAUTH_CONSENT_BASE_URL` only if the app is served
from a different origin than the API, as in development.

### Host side

- `GET /agents/available` — agents registered as available and heard from
  recently.
- `POST /matches/{id}/add-agent {agentId}` — host-only; seats that agent.

### Presence without a socket

Agents have no WebSocket. `match.AgentPresence` (implemented by the MCP
registry) answers "heard from within 90 s"; `Manager.seatHere` is now the one
question the resume/attended rules ask, so they cover both. Any tool call
counts as presence. `StartAgentSweep` (15 s) catches agents that went quiet.

### Paused out (`module.DropIn`)

A module opts in by implementing `SitOut() []string` — the verbs a sat-out seat
plays, least committal first.

- Poker: `check`, `fold`.
- Blackjack: `decline_insurance`, `stand`, `bet` (the table minimum to stay in).

At a drop-in table `suspendLocked` no longer pauses the whole match. Instead a
seat that has been away `SitOutGrace` (20 s) is **sat out**: the bot loop plays
it with `OfferBot(SitOut()…)` — same validated path, no privileged entry.
Coming back (any socket or any agent call) ends it immediately and broadcasts
so the mark comes off. A timer wakes the loop at the end of the grace period,
since a seat going quiet writes nothing to react to. `PlayerMsg.satOut` drives a
`PAUSED` badge.

Every other game keeps today's behaviour: pause for the missing seat, abandon
after two minutes.

## Known limits (deliberate for Phase 1)

- **Blinds and antes still bleed.** A sat-out hold'em seat still posts blinds
  and folds; a sat-out blackjack seat still bets the minimum. A true sit-out
  (skip the deal, keep the stack) needs an engine-level seat state in each
  module. Phase 2.
- **Agent registry and tracking are in memory.** A server restart forgets who
  is available; agents re-register on their next call. Seats persist normally.
- **Single instance.** Presence is this process's own, as the socket registry
  already is under Redis fan-out.
- **No client UI to pick from the available-agent list yet.** The
  `GET /agents/available` and `add-agent` endpoints exist; the invite button
  above is the supported way to connect from the app.
- Refresh tokens are not revocable before expiry (stateless by design).
- Not tried against claude.ai itself — it needs a public https deployment.
  The flow was run end to end with the official MCP SDK's OAuth client.
- Rate limiting for `/mcp` and `/oauth/register` is not added; it sits behind the same bearer auth as
  every other route.

## Phases

1. **Done** — seats, MCP server, presence, sit-out for poker and blackjack,
   host endpoints, seat badge, tests (`internal/mcp`).
2. Table-screen picker for already-available agents; a revocable grant list in account settings; engine-level sit-out (no blinds/ante
   while away); persisted agent registry; per-agent rate limits.
3. Agent profiles on stats (an `agent:` persona key like bots' `AIPersona`),
   optional per-table "no agents" option, and a skill hint so agents play at a
   chosen strength.

## Verification

`go test ./internal/mcp` covers the protocol, joining by code, acting, seat
isolation, host seating, and — for both poker and blackjack — a silent agent and
a never-connected host being played past while a third seat keeps acting and the
table never leaves `active`.

### Checked against a real server

A built `cmd/server` binary, the official `@modelcontextprotocol/sdk` client
(and the MCP inspector CLI) as the external client, and the Expo web app as the
human: the SDK client connected over Streamable HTTP, listed tools, registered,
was auto-seated by the table-bound token, and traded moves with a human at
hold'em. Closing the human's browser tab turned their seat `satOut` after the
grace period while the agent kept playing and the table stayed `active`;
reopening the tab cleared it. The `claude` CLI itself could not be used as the
client in that environment (not logged in), so it has not been tried.

OAuth was checked the same way: the SDK client (`StreamableHTTPClientTransport`
with an `OAuthClientProvider`) hit the 401, discovered the metadata, registered
itself, ran PKCE, and a human approved on the app's consent screen in the
browser; the client then received tokens, connected, and called tools.
`TestOAuthFlow` pins each step, including refusal of unregistered redirects,
plain PKCE, replayed codes, wrong verifiers, and a code or refresh token being
used as an access token.
