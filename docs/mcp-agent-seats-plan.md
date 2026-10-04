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

Not supported: claude.ai's *custom connector* UI, which authenticates MCP
servers with OAuth rather than a header token. That would need an OAuth
authorization server on this endpoint.

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
- Rate limiting for `/mcp` is not added; it sits behind the same bearer auth as
  every other route.

## Phases

1. **Done** — seats, MCP server, presence, sit-out for poker and blackjack,
   host endpoints, seat badge, tests (`internal/mcp`).
2. Table-screen picker for already-available agents; OAuth for claude.ai connectors; engine-level sit-out (no blinds/ante
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
