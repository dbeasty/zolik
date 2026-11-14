# Defect review — 2026-11-13

A whole-codebase review at `origin/main` 73bc9a16 (1.19.0), split into seven areas. Each finding was traced through the code end to end; findings marked **[repro]** were also reproduced with a throwaway test (run with `go test -overlay`, nothing added to the tree) or a live run. Items marked **[re-checked]** were independently re-read during consolidation.

Severity: **H** high, **M** medium, **L** low. Line numbers are at 73bc9a16.

## Fix first

| # | Sev | Area | Defect |
|---|-----|------|--------|
| 1 | H | auth | Any account can take over any guest's identity via a self-enrolled node + `claim-offline` |
| 2 | H | auth | Email sign-in code attempt cap is bypassed by parallel guesses; no rate limit on verify |
| 3 | H | server | Game / lobby / `/ws/me` sockets have no read limit — one guest can OOM the process |
| 4 | H | server | One stalled socket write freezes broadcasts for every game in the process |
| 5 | H | deploy | `deploy.sh` overwrites the production `.env` if one SSH check fails |
| 6 | H | holdem | Chips destroyed when the biggest bettor leaves a cash table mid-hand |
| 7 | H | okobere | An oko payout can push the bank negative (default options) |
| 8 | H | prsi | Dead position: no legal action when stock and discard are exhausted |
| 9 | H | prsi | `NewMatch` panics with 6 players × 6 cards |
| 10 | H | client | Match-screen keyboard shortcuts fire from screens pushed on top (web) |
| 11 | H | client | Lobby socket: two reconnect chains displace each other forever |
| 12 | H | macOS | A Bluetooth drop can freeze a Mac guest table permanently |

## Server platform core

1. **H — No WebSocket read limit.** [re-checked] `match/handlers.go:1601`, `lobby/handler.go:172`, `notify/handlers.go:412` never call `SetReadLimit` (only `relay` does). gorilla treats 0 as unlimited, so a guest token plus one multi-GB frame OOMs the server; kdb mode is single-process, so everything goes down. *Fix:* `conn.SetReadLimit(64<<10)` after every `Upgrade`.
2. **H — Registry-wide lock held across a blocking write.** [repro] [re-checked] `ws/registry.go:128-140`: `WriteJSON` writes under `r.mu.RLock()`, match sockets have no write deadline or keepalive. A dead phone fills its TCP buffer; the next `Add`/`RemoveIfCurrent` queues for the write lock and then every reader in every game blocks. *Fix:* copy the conn under the lock, write outside it; add write deadlines and ping/read deadlines to match sockets.
3. **M-H — Stale suspend pauses a table on a connected player.** `match/presence.go:68-136`, caller `handlers.go:1512-1534`. The old socket's deferred `SuspendOnDisconnect` can run after the new socket's `ResumeIfReturning`; the table is then suspended with the player present and every action is refused as `MATCH_NOT_ACTIVE`. *Fix:* under the match lock, return early if `Registry().Has(matchID, playerID)`.
4. **M — Out-of-order state broadcasts.** `presence.go:335-346`, `manager.go:767-773`: snapshot under lock, broadcast after unlock, no seq in `MatchStateMsg`. A stale board can arrive last and strand the player on turn. *Fix:* add `seq` and have clients drop older states, or broadcast under the lock.
5. **M — Relay guests left on dead tunnels after the host re-attaches.** `relay/relay.go:209-230`: `attachHost` keeps `t.guests`, and `hostGone(t, old)` returns early, so guests never get `CloseHostGone`. *Fix:* close existing guests when replacing a host.
6. **L-M (Mongo only) — Failed persist leaves the cached match loaded.** `match/repository.go:303-316`, `manager.go:888-895`: a non-conflict error after the move is inserted keeps `e.loaded`; a finishing move can strand the match "active". *Fix:* `e.loaded=false` on any persist error.
7. **L — `awaySince` leaks per finished match.** `match/standin.go:91-145`, `agents.go:80-94`. *Fix:* drop `matchID|` entries in `matchStopped`.

## Identity, auth and operations

1. **H — Guest identity takeover via a self-enrolled node.** [repro] [re-checked] `auth/nodes.go:264`, `auth/claimoffline.go:75-96`. Any account can enrol a node with its own key, sign a `SeatReceiptClaims{GuestID: victim}` for a guest id read from any table, and `claim-offline` moves the victim's history, friend profile and future imports to the attacker. The existing test `TestAGuestSeatBecomesTheirsWhenTheySignIn` passes with exactly this shape. *Fix:* require the guest key (or a live guest refresh token), and honour receipts only from nodes the guest actually sat at.
2. **H — Email code attempt cap is a check-then-act race.** [repro] [re-checked] `auth/email.go:144-156`: attempts are read before the bcrypt compare and incremented after; `ConsumeLoginCode` ignores attempts. 60 parallel guesses → success. There is no per-IP limit on `/auth/email/*`. *Fix:* atomic conditional increment before comparing, same condition in consume, plus a limiter.
3. **M-H — Unverified email squatting locks out the real owner.** [repro] `auth/handlers.go:660`, `auth/accounts.go:231,256`. Register with someone else's address; their verified email or OAuth sign-in then 500s ("could not resolve identity … after 9 attempts"). *Fix:* don't reserve unverified addresses, or let a verified sign-in take the address over.
4. **M — OAuth link CSRF.** `auth/oauth_handlers.go:86,142`: `state` is not bound to the browser, so an attacker's link-flow URL links the victim's Google identity to the attacker's account. *Fix:* HttpOnly state cookie checked in the callback.
5. **M-L — No limiter on `/auth/login` and `/auth/register`.** `auth/handlers.go:232-233`: unbounded online guessing and ~250 ms bcrypt CPU per unauthenticated request.
6. **M — Every IP rate limit is bypassable.** [re-checked] `deploy/nginx/zolik-proxy.conf:32,69` uses `$proxy_add_x_forwarded_for` (appends to the client's header) and `ratelimit/ratelimit.go:96-103` takes the leftmost entry. IPv6 `RemoteAddr` is split on the first `:`, collapsing every IPv6 client to `[`. *Fix:* `proxy_set_header X-Forwarded-For $remote_addr;` and `net.SplitHostPort`.
7. **L-M — Web Push endpoint is an unvalidated URL (blind SSRF).** `notify/service.go:995`, `notify/webpush.go:123`. *Fix:* require https, refuse private addresses, no redirects.
8. **L — Tokens in WebSocket query strings land in nginx access logs; empty `APP_ENV` means local** (test endpoints, `SSH_ALLOW_ALL_KEYS`, public dev secret). `app/config.go:200`. *Fix:* empty = production.
9. **M — Scorepad sessions have no auth or owner.** [re-checked] `scoring/handlers.go:70-75`, mounted bare at `app/app.go:866`: anyone with a (time-ordered, guessable) id can read or overwrite a scorepad; `patch` has no version check; `create` writes `OwnerUser: ""`, so account deletion's `DeleteForOwner` never matches.
10. **L — `JWT_REFRESH_SECRET` is configured and generated but never used**; sign-in emails still say "Žolíky" (`AppName` in `app.New`, `SMTP_FROM_NAME` default).

## Game modules

1. **H — Hold'em destroys chips.** [repro] `holdem/engine.go:789` (`distributePots`) skips a side-pot level with no eligible seat; reachable when the biggest bettor leaves (`applyLeave`, :322). Probe: table total 2130 → 2010. The uncontested branch (:593) has the same root. *Fix:* refund unwinnable levels to their contributors.
2. **M — Hold'em short all-in reopens betting.** [repro] `holdem/engine.go:390-435`: `applyRaise` never checks `Acted`. *Fix:* refuse raise when `seat.Acted`.
3. **L — Hold'em odd chip goes to lowest seat index**, not first clockwise from the button. `engine.go:784,809`.
4. **M-L — Two-handed Pinochle scores one dix twice.** [repro] `pinochle/engine.go:494` (`canExchange`) doesn't check the nine was already shown. *Fix:* reuse `canDeclareDix`'s unshown-dix test.
5. **H — Oko bere bank goes negative.** [repro] `okobere/engine.go:237`: `min(OkoPays*Bet, Bank)` ignores other punters' stakes; with default 3× oko the banker ends at −30 in the probe. Breaks `okobere.rules.bank`. *Fix:* cap at `Bank - (committed - Bet)`.
6. **H — Prší dead position.** [repro] `prsi/engine.go:196`: empty stock, discard down to its top card, nothing playable → draw, play and pass all refused. 11/200 seeds stuck in a 2-player sim. *Fix:* allow a blank draw that ends the turn (as Last Card does); end the game if all seats are stuck.
7. **H — Prší panic.** [repro] [re-checked] `prsi/engine.go:55-59`: 6 players × 6 cards = 36 > 32 cards; `deck[0]` panics. *Fix:* validate `players*handSize < 32`.
8. **M — Gin Rummy big gin pays plain gin.** [repro] `ginrummy/engine.go:265` → `scoring.go:29`: the "further 25" in `bigGinBonus` is never added.
9. **L — Gin Rummy dead hand one draw late** (`engine.go:240`, `<= 1` vs rules' "last two"). Pinned by a test; fix code or wording.
10. **L — Rummy Tiles ends the round on one empty-pool draw** (`engine.go:376`), though the rules say "and nobody can play".

Checked and found sound: Blackjack, Siete y Medio, Ferbl, Lóra, Canasta (guards checked, arithmetic not deep-audited), Žolíky view filtering, Mariáš, Last Card, Snaps, Sedma, Klondike.

## Bots and AI

1. **M — A silent remote agent strands a table.** `match/agents.go:200-205`: the sweep drops tracking for `lobby` matches and nothing re-tracks at start, so a dead agent seat is never suspended. Server restarts do the same. *Fix:* keep lobby entries; track every `IsAgent` player on start/load.
2. **M (fairness) — game-mcp `observe` reveals bot hands.** [repro] `gamemcp/service.go:236-242`: no `s.Claude` check on the seat (Play has one). *Fix:* refuse non-Claude seats (and `coach`) until the match ends.
3. **L-M — OAuth agent identity keyed on client-chosen `client_name`.** `mcp/oauth.go:347-351`: a second client with the same name inherits the first's tables. *Fix:* mix in `client_id` / redirect host.
4. **L — MCP agent registry never evicts** (`mcp/registry.go:62-81`).
5. **L — `LoadNet` trusts header layer sizes** (`learn/mlp.go:153-155`): corrupt model → panic or huge allocation.

Checked and found sound: hidden-information isolation of server bots, MCP `act`/`get_state` seat checks, undo unwinding, model fallback, governor accounting, learnexport anonymisation, ml/ train/test separation.

## React Native client

1. **H — Keyboard shortcuts fire from other screens.** `app/match/[matchId].tsx:1004-1009, 1805-1859`: `document` keydown listener with no focus gate; Enter on the pushed Rules screen sends the lead move, D draws, U undoes. *Fix:* `useIsFocused()` ref guard.
2. **H — Lobby socket reconnect fight.** `src/hooks/useLobbySocket.ts:44,73-115,128-133`: shared `closingRef` (the bug `useMatchSocket` already fixed); switching games leaves two chains displacing each other every ~2 s. *Fix:* per-run `let closing`; close before `retryNow` reconnects.
3. **M — Join/table polls run under other screens and `router.replace` over them.** `app/lobby/join.tsx:53-73`, `app/lobby/table.tsx:76-111`. *Fix:* focus-gate.
4. **M — Bad guest link replaces this device's guest key.** `src/context/SessionContext.tsx:501-526`. *Fix:* only persist when the returned id matches the key.
5. **M — Offline hosting and node mode share one embedded server lifecycle.** `SessionContext.tsx:376-385, 898-911`, `src/net/nodeSession.ts:78-81`: leaving a hosted table kills node mode; signing out kills a hosted table.
6. **L-M — Web OAuth callback skips `adopt`.** `app/auth/callback.tsx:48-52`: stale claim prompt, retired guest key re-sent.
7. **L — Match socket reconnects on every token refresh** (`[matchId].tsx:220-225` memo depends on `accessToken`).
8. **L — A failed poll's error is never cleared** (`join.tsx:64`, `table.tsx:91`).

## Other clients, scripts, deploy, tests

1. **H — `deploy.sh` regenerates production `.env` on an SSH hiccup.** [re-checked] `scripts/deploy.sh:175`: `! ssh_admin "sudo test -f …"` treats exit 255 as "missing" → fresh JWT secrets, SMTP/OAuth/VAPID/admin hash gone. *Fix:* distinguish exit 1 from 255; never regenerate without `--init-env` on SSH error.
2. **H — Mac BLE drop freezes a guest table.** `NearbyBle.swift:314,346-354,425` (shared with the iOS module): pending `sendDone`/`pendingInfo` callbacks never fire on disconnect; `radioSend`/`connectBlocking` wait forever while Go holds `sendMu`/`ensureMu`. *Fix:* fail pending callbacks on disconnect; time out semaphore waits.
3. **M — Mac main-thread blocking up to 15 s** on `openRelay`, `startNode`/`stopHost` (`NearbyService.swift:366-371`); and up to 20 s on a stalled `NetSocketSend` (`zolikcore/desktop.go:312-323`).
4. **M — Foreground revive silently closes the phone's room and relay.** `server/mobile/zolikcore/core.go:178-183`.
5. **M — TUI reads one websocket from two goroutines.** `client-tui/ui/match.go:56-58,113,121`.
6. **M-L — TUI leaks sockets on quit and never refreshes tokens** (`ui/model.go:118-120`, `api/client.go`).
7. **L-M — `dev-stack.sh` can expose a prod-copy to the LAN with dev sign-in endpoints** (`:110-118`).
8. **L — `phone-e2e-stack.sh` swallows setup failures (no pipefail, no trap) → silently skipped tests and leaked servers.**
9. **L — e2e leaks processes on failure** (`relay.spec.ts:41,82-85,127`, `e2e/macos/run.mjs:60-91`); `hand-order.spec.ts:304` can pass without a bot push.
10. **L — Compose files publish Mongo/Redis/mongo-express/SSH on all interfaces; production compose has no healthcheck.**
11. **L — SSH server: a guest row per anonymous connection, no password rate limit** (SSH is off in production).
12. **L — `dev-stack.sh test` runs `go test ./...` without `ZOLIK_TEST_DB_ENGINE=kdb`**, so store-backed tests skip yet report `ok`.

## Structural notes (not bugs)

- The module list is duplicated in `app.hostedModules`, `cmd/dump-keys`, `cmd/dump-lessons`, `cmd/botcost`, `internal/gamemcp/games.go` and `allmodules_test.go`; forgetting one silently drops a game from that tool.
- `internal/match` (game-agnostic) imports status constants from `internal/rules` (Žolíky).
- Much state is per-process (board cache, bot-loop flags, MCP registry, relay hub, rate limiters, stand-in timers), and the stranded-table sweep is off under Redis — the multi-instance Mongo+Redis story is weaker than `server/README.md` suggests.
- Stale docs are listed in [architecture/README.md](architecture/README.md).

## Written rules vs code vs UI (found while writing the user guide)

Confirmed in code:
- **Rummy Tiles can have no end**: Target score Off + Round limit Off leaves no end condition (`rummytiles/scoring.go:46-53`); help says "pass" but code ends at ≥.
- **Žolíky tie-break** picks the player with *fewer* deals won (`rules/winner.go:84`, commented as deliberate) — the written rules state no tie-break at all; confirm the intent and write it down.
- **Privacy Notice contradicts the server for guests**: it says nothing is kept beyond the table, but guest identity, match history, circle and My games are stored.
- **Canasta rules always say "Play in partnerships"**, though 2/3/5 seats play alone (`canasta/engine.go` `seatsToTeams`); canasta bonuses and red-three values aren't in the written rules.
- **Prší written rules omit** ace-on-ace answers, seven stacking, svršek on anything and the first turned-up card's effect; the move log contradicts the "next player is skipped" wording.
- **Gin Rummy rules never state ordinary knock scoring**; **Žolíky Classic rules never say lowest total wins**, nor the pack count.
- **Hard-coded English** in `client-react-native/src/lib/auth.ts` (sign-in buttons, "You have N games recorded…").

Wording: "forhont" typo in Mariáš Auction rules; Czech terms left in Mariáš English help (stakes, doubling names); card names inconsistent across games and screen-reader labels (Under/Over vs Unter/Ober); Canasta "Poach wilds" (server) vs "Swap for wilds" (app); Žolíky "Discard pickup" help grammar.

Tooling: the registered `zolik-games` MCP server is an old build (no Pinochle, stale option defaults).
