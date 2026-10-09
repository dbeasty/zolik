# Players who drop, players who leave, and hybrid tables

Status: plan, nothing built. Written 2026-10-08 against main at `1962f4f`.

## Why

A table today handles a missing player in one of two ways:

- **Drop-in games** (Hold'em/Draw/Omaha, Blackjack implement `module.DropIn`). After `SitOutGrace` (20 s) the seat is *sat out*: the bot loop plays it with the module's most passive verbs (check, fold, stand) until the player is heard from again (`internal/match/agents.go`).
- **Every other game** is *suspended* when the seat it is waiting on drops (`SuspendOnDisconnect`, `internal/match/presence.go`). The reaper abandons it once `AbandonWindow` (2 min) passes.

So for most games one dropped phone ends the game for everyone after two minutes. Hybrid tables (a phone as the server, remote players coming in through the cloud) make drops more common and add a new failure: the *server itself* going away.

What David asked for:

1. **The default is to wait, then a bot stands in.** The table pauses for a grace period, then a bot plays the seat until its player comes back.
2. **Poker that is not a tournament lets you leave your seat.** You cash out, and the table plays on without you. In a tournament you stay in, and your stack is blinded off as it would be at a real table.
3. **The UI makes it obvious which device is the server.** If the server goes down, the whole game is paused, and everyone can see that this is what happened.

## Shape of the work

| Part | What | Depends on | PR size |
|---|---|---|---|
| A | Stand-in bots and the "when a player drops" table option, for every game | nothing | M |
| B | Poker and Blackjack: cash vs tournament, leaving the table | A (shares the presence plumbing) | M |
| C | Clear pause/drop UI: three banners, seat badges, host prompt | A | S–M |
| D | Hybrid tables: cloud relay, server-is-a-phone UI, host-down rules | A, C; mobile Phases 2–3 | L |
| E (later) | Sit out the next deal in round-based games; a person takes over a seat; late joining at cash tables | A, B | — |

A, B and C help cloud tables today, so they ship first and on their own. D is Phase 2d of the mobile plan (`~/.claude/plans/do-a-plan-for-tender-spark.md`).

---

## Part A: stand-in bots

### The option

The runtime declares one table option for every multi-seat module, following the house-rules-are-options rule:

| Option | Values | Default |
|---|---|---|
| `standInAfter` (seconds) | 30, 60, 120, 300, 0 = never | **60** for games that are not drop-in; not offered for drop-in games, which use Part B |

- `0` (never) is today's behaviour: pause, then abandon.
- One-seat games (solitaire) never get the option. They are saved games already (`SavedGameWindow`).
- The option is `OptionEnumInt`, like every other option, so `ValidateOptions`, the lobby form and the rules screen all handle it with no new kind.
- **Where it is declared** (as built): `module.StandInOption()` in `internal/module/standin.go`, dropped into each pausing module's option list, with `standInAfter: 60` in every variation's `Defaults`. This is the same pattern `PauseOption` and `BotSkillOption` use. A contract test in `allmodules_test.go` checks that exactly the non-drop-in, multi-seat modules declare it, with the default in every variation.
- **Rules screen** (as built): the `/modules/{id}/rules` handler appends `module.StandInRules(cfg)`, a "Players who drop" section that reads the chosen wait. It is not added inside each module's `Rules()`, so the bot simulator and MCP tables, where nobody can drop, don't state it.

### Model

`models.Player` gains:

```go
// StandIn is set while the server plays this seat for a person who is away.
// The seat stays theirs: IsAI stays false, and the name, account and avatar
// are untouched. Empty when they are playing it themselves.
StandIn *StandIn `bson:"standIn,omitempty" json:"standIn,omitempty"`

type StandIn struct {
    Since      time.Time `bson:"since" json:"since"`
    Skill      string    `bson:"skill" json:"skill"`
    By         string    `bson:"by" json:"by"` // "timeout" | "host"
}
```

`Match` gains `StoodIn []string`, the seat ids a stand-in has ever played for. It is kept after the stand-in ends, because results need it.

### Runtime behaviour (`internal/match/agents.go`, `presence.go`, `bots.go`)

1. **Drop:** `SuspendOnDisconnect` keeps suspending as it does today. The table pauses while it waits, which is the "wait" part. When `standInAfter > 0` it also notes when the seat went away and schedules a check for each seat. A seat that drops while the table is not waiting on it gets its check scheduled by the bot loop, once the turn reaches it.
2. **Timer fires:** the seat is still away, the match is still suspended on it, and **at least one person at the table is present** → set `StandIn{By: "timeout"}`, add the seat to `StoodIn`, un-suspend the match (status `active`, clear `SuspendedAt`/`AbandonAt`), broadcast, and call `RunBotsIfNeeded`.
   - If nobody is present, the table stays paused and the reaper abandons it as today. Bots never play to an empty room.
3. **`drivenSeat`** gains a case before the sat-out check. A seat with `StandIn != nil` is played by the module's **bot** (`passive=false`), at `StandIn.Skill`, through `botSeatFor`.
4. **Strength:** the strength of the table's bots when it has any (the highest one), otherwise `medium`. The host changes it with `skill` on the stand-in endpoint below. The bot-strength endpoint stays for bot seats only.
5. **Return** (as built): arrival (`ResumeIfReturning`) clears `StandIn` at once, and forgets the bot's turn for that seat. Every stand-in move is checked under the match lock (`handleAction`, standIn=true), so a bot move that loses the race with the return is refused (`STAND_IN_ENDED`) instead of landing. The player makes the next decision, even in the middle of a turn the bot began. There is no "back after this move" state.
6. **Host actions** (new `POST /matches/{id}/seats/{playerId}/stand-in` with `{on: bool, difficulty?}`, host only):
   - Put a bot in for an away seat right away, without waiting out the timer.
   - Take the bot out again; the table then pauses on that seat if it is awaited.
   - Refused for a seat whose player is present (`ruleId` → the "Players who drop" section).
7. **Several people away at once** is handled independently per seat. The "someone is present" rule counts only people with no stand-in.
8. **Every place that reads `IsAI` for "the server plays this"** has to treat `StandIn` the same way: `firstBot`, governor/compute gating, `botMaxSteps`, offer bots, hint suppression. List them with `grep -n "IsAI" internal/match/*.go` and go through each; a stand-in seat must never be skipped *and* never be double-driven.

### Results honesty

- A seat in `StoodIn` gets **no win or loss** for that match. There is no rating system: `stats.ApplyMatch` adds it to the history with `standIn: true`, and to no tally, split, head-to-head or streak. Opponents keep their result.
  - Otherwise someone could drop on purpose and let a Hard bot win for them.
  - It also keeps the human's record honest when a bot lost for them.
- Results screen: "Jana (a bot played 3 of 9 rounds)", or simply "finished by a bot" when the stand-in was there at the end.
- **Offline / Phase 2c upload:** the bundle carries `StoodIn`. The importer applies the same rule.

### Tests (Go)

- Per-module conformance (`internal/module/conformance.go` drives it): for each multi-seat module and variation, take a seat away at a random point, put a stand-in in, and play to the end. Five seeds, and both Canasta variations (canasta-bot-bugs-hide-in-samba).
- Returning at awkward moments: mid-trick (Mariáš), mid-meld (Žolíky, Continental), mid-bid (Mariáš licitovaný), between rounds (the interstitial).
- Nobody present → no stand-in, and abandoned after the window.
- Host on/off, and refusing a present seat.
- Stats: a flagged result is excluded from rating and counted in games played.
- Remember `ZOLIK_TEST_DB_ENGINE=kdb`, or the repo tests skip and still report ok (match-tests-skip-without-a-store).

---

## Part B: poker and Blackjack: cash tables and tournaments

### The option

Hold'em/Draw/Omaha get `format`. Blackjack is always a cash-style table, since each seat plays against the dealer, so it gets the cash rules without the option.

| `format` | Meaning | End of match |
|---|---|---|
| `tournament` (default when `handLimit = 0`) | Freeze-out: everyone starts with the same stack, last player standing wins | today's `matchOverAfterHand` |
| `cash` | Come and go: your result is the chips you leave with | the host ends the table, the hand limit is reached, or fewer than 2 are seated |

`handLimit > 0` today ("most chips after N hands") behaves like a cash table with a fixed end, so it maps to `cash` with that limit. Migration rule: a stored match with no `format` reads `handLimit == 0 ? tournament : cash`, so no data migration is needed.

### Leaving (cash format, and Blackjack)

- **A new verb, `leave`,** offered to a seated, non-out player at a cash table at any time.
  - **Between hands, or folded:** the seat leaves now.
  - **Mid-hand and still live:** the seat leaves at the end of the hand. The player can also fold first. The button says "Leave after this hand".
  - Blackjack: the player leaves after the current round settles; a bet already placed plays out.
- Engine (`internal/holdem/engine.go`): `Seat.Left bool` plus `Seat.CashOut int`, locked when they leave. `liveSeats()` excludes `Left`. The button and blinds skip the seat. `matchOverAfterHand` ends the match when fewer than 2 seats are live and not left.
- The runtime keeps the `Player` in `match.Players` (history, replay and results need them), and the client shows the seat as empty with "Jana left with 1 340".
- **Results:** ranked by chips, using `CashOut` for those who left and the stack for those still seated. A cash table shows net (+/−) against the buy-in.
- **Leaving is not dropping.** A player who leaves does not get a stand-in, and reconnecting does not reseat them. Late joining is Part E.
- `Rules()` gets a "Leaving the table" fact for cash and a matching one for tournaments. A refused `leave` in a tournament points at it.

### Away at a poker or Blackjack table

- **Cash:** sat out as today (check/fold, stand, minimum bet in Blackjack, per `SitOut()`). After **5 minutes** sat out, the seat **leaves automatically** with its chips, so an abandoned phone doesn't pay blinds for ever. The time is another option, `leaveAfterAway`: 2, 5 or 10 minutes, or 0 = never.
- **Tournament:** sat out and blinded off, which is the real-table rule. They are never removed and never get a stand-in. When the blinds take their last chip, they are out like anyone else.
- **Blinds while sat out at a cash table:** today a sat-out seat still posts blinds, because the forced verbs aren't in `SitOut()`. At a cash table, skip a sat-out seat's blinds the way a left seat's are skipped. They post again when they come back. This is the usual "sitting out" rule and goes into `Rules()`.

### The poker bot as a stand-in

Not used. In a tournament, being blinded off is the expected rule. At a cash table you can leave, so a bot playing your chips adds risk without adding anything. The `standInAfter` option is not offered for drop-in modules.

### Tests

- Engine:
  - Leaving between hands, folded, and mid-hand; the button and blinds skip a left seat; the match ends at fewer than 2.
  - Heads-up leave: the match ends and the result is right.
  - Results by `CashOut`.
  - Tournament refuses `leave`.
  - Sat out at cash → no blinds, automatic leave after the timeout.
  - Sat out in a tournament → blinded off, eventually out.
- Blackjack: leaving with a live bet, and leaving between rounds.
- Old stored states (no `format`) resolve as above.
- Bots never pick `leave`. Add it to the bot's excluded verbs and check `Botted`.

---

## Part C: making pauses and drops clear (cloud tables first)

Server additions to `MatchStateMsg` (`internal/match/state_msg.go`):
- Each player: `standIn` (difficulty, since) and `left`/`cashOut` (from module views, already per-seat).
- `standInAt`: when the timer fires for a suspended seat, so clients can show a countdown without guessing the option.
- `server`: `{kind: "cloud" | "phone", name}`. It is `cloud` on jokerless.com; Part D fills in `phone`.

Client (`client-react-native`):
- **`app/match/[matchId].tsx`, the status line** (`match.pausedFor` today) becomes **three different banners**, each with its own key and colour:
  - **"You're offline. Reconnecting…"**: this client's own socket is down (`useMatchSocket` state). This is checked first, because when you are offline you cannot know anything else.
  - **"The server is offline. The game is paused and continues where it left off."**: only in Part D, from the relay's `hostGone`.
  - **"Waiting for Jana. A bot plays for them in 0:42."**: suspended on a seat with `standInAt`. With `standInAfter = 0`: "Waiting for Jana…" and the abandon countdown.
- **`SeatStrip.tsx`:**
  - A stand-in seat keeps the person's avatar and name, with a small 🤖 corner badge and the line "a bot is playing for Jana".
  - A left seat (cash) is dimmed: "left with 1 340".
  - The existing `satOut` mark stays for poker.
- **Host seat menu:** "Let a bot play now" / "Take the bot out" and the strength picker, reusing the existing bot-strength control.
- **Returning player:** a one-time toast, "A bot played 2 moves for you", so their hand state isn't a surprise.
- **Lobby/setup:** the `standInAfter` control (or `format` and `leaveAfterAway` for poker), driven from the descriptor like every other option.
- **Poker controls:** a **Leave table** button (cash only) with a confirm: "Leave with 1 340 chips?"
- Keys: every new message key is a static literal (message-keys-must-be-static-literals). Then regenerate `serverKeys.json` (server-keys-manifest-locks-wording) and run `go generate ./internal/gamemcp` after the en.ts additions (en-keys-feed-gamemcp). All locales get the keys; non-English can start as the English text if that's the existing practice.

Playwright:
- A two-person Žolíky game. Close one browser context → the other sees "Waiting for … 0:5x" → after the (test-shortened) grace, a bot moves for them → reopen → the seat is taken back and the toast shows.
- A cash Hold'em game with 3 players: one leaves, and the other two finish.
- Assert what is on screen (banner text, badge), not only state fields (assert-what-the-user-sees).
- Keep the bot think-time defaults (bot-think-time-breaks-e2e).

---

## Part D: hybrid tables (phone is the server, the cloud relays remote players)

This is the detail behind Phase 2d of the mobile plan.

### Transport

- **Cloud relay** (new `internal/relay` on the cloud server only, not in `RegisterMobileRoutes`):
  - `GET /relay/host` (WSS) is authenticated with the node credential from node mode. It returns a relay code, valid while the socket lives plus 10 min for reconnects.
  - `GET /relay/join/{code}` (WSS) is for each remote guest.
  - Frames are `connId(4) | payload` and are opaque. The relay never opens them.
  - Control frames: `guestOpen/guestClose(connId)` to the host, and `hostGone`/`hostBack` to every guest.
  - Limits: 8 guests per relay, 64 KiB per frame, a frame-rate cap, and enrolled nodes only. Relays are counted in `/debug/memory`-style metrics.
  - Single-process is fine: relays live in the process the host's socket reached. Multi-instance needs sticky routing by code, which is out of scope while there is one deployment (no-staging-environment-exists).
- **Go core** (`server/mobile/zolikcore/relay.go`): `Host.OpenRelay(cloudURL) (code string, err)`, `CloseRelay()`, `RelayStatus()`. Each `guestOpen` creates a `Tunnel` (`tunnel.go`, unchanged) whose `TunnelSink` writes to the relay socket. Reconnects with backoff and keeps the same code.
- **TS** (`src/net/relay/transport.ts`): `RelayTransport` reuses `src/net/ble/crypto.ts` and the tunnel message format, without the chunking. The `transport.ts` abstraction already exists.
- **Web guest:** jokerless.com serves `/r/{code}`, which opens the normal web app with a relay endpoint. It is a secure context, so push and clipboard work.
- **Version gate:** the first tunnel request is `GET /version`. Outside the compatibility window, show "The host's app is older/newer — ask them to update." Define the window in one constant, next to `tunnelVersion`.

### Identity and results
- Signed-in remote guests present a pass (node mode already verifies passes against the cloud JWKS) and sit as their account.
- Guests who aren't signed in sit as guests.
- Uploading the finished match goes through Phase 2c consent, unchanged. There is no new upload path.

### Drops on a hybrid table
- **A remote or local player drops:** Part A/B rules, exactly as on a cloud table.
- **The host phone loses internet but keeps running:** every remote seat goes at once. The core knows the relay is down (`RelayStatus`), so **stand-in timers for remote seats are frozen while the relay is down**, and for 60 s after it comes back. Local seats are unaffected and play on until the game waits on a remote seat.
- **The host phone itself goes away** (backgrounded/suspended, app closed, dead battery): **the whole game is paused**. The phone *is* the server.
  - The relay sends `hostGone` to remote guests. Local guests lose their LAN/BLE connection and see the same banner, because their client can't reach the server.
  - **On return:** the core resumes from its kdb file, and before the reaper or any timer runs, `Manager.HostResumed()` resets every seat's away clock, every pending stand-in timer and every `AbandonAt`. Absence caused by the host is never charged to the guests.
  - No stand-in ever starts because of time the host was away.
- **The host leaves for good** (the host stops hosting): the table is closed. Remote guests get "The host ended the table". The match is `abandoned` with the usual rules, and Phase 2c consent applies to anything worth keeping.

### UI: "this phone is the server"
- **Where the table lives:** the same three names everywhere, at setup, in the table header and in the lobby list.
  - **Nearby**: the server is this phone; only people here can join.
  - **Nearby + Online**: the server is this phone; anyone with the link can join.
  - **Online**: the server is jokerless.com.
- **Table header chip:** `Server: David's phone` (phone icon) or `Server: jokerless.com` (cloud icon). Tapping it explains: "Everything runs on David's phone. If it goes offline, the game pauses for everyone and continues when it's back."
- **The host's own screen:**
  - A server badge on their seat.
  - A permanent strip: "Your phone is the server. Keep the app open."
  - A relay pill: Online ✓ / reconnecting… / off.
  - The toggle "Let people join over the internet".
  - keep-awake stays on while hosting.
  - Backgrounding the app while remote guests are seated triggers a local notification: "Your game is paused for 3 players. Come back to continue."
- **Invite sheet:** *In this room* (LAN QR code / Bluetooth) | *Anywhere* (a `jokerless.com/r/ABC123` link and Share). The Anywhere tab explains why it is unavailable when the host is offline or not signed in.
- **Per-seat connection badge:** Wi-Fi · Bluetooth · Internet · server (the host) · bot. It shows who disappears if the internet goes.
- **Guests' banners** (Part C, plus the server case):
  - "You're offline"
  - "The server (David's phone) is offline. Paused, and nothing is lost."
  - "Waiting for Jana. A bot plays in 0:42"

### Tests
- Go:
  - A full match through an in-process relay.
  - Relay down → remote stand-in timers frozen, and local play continues until it waits on a remote seat.
  - Host stop/restart → nothing abandoned, timers reset, guests reattach to their seats.
  - Version gate refuses outside the window.
  - Relay limits.
- Playwright (private stack): a browser joins a core-hosted table through the relay. Kill the core → the guest sees the server-offline banner. Restart → play continues.
- Devices: an iPhone host on mobile data, a Bluetooth guest and a remote browser guest.
  - The remote guest closes the tab → the bot takes over after 60 s → they return and take the seat back.
  - The host backgrounds for 2 min → everyone sees "server offline" → it resumes.

---

## Part E: later, not planned in detail

- **Sit out the next deal** in round-based games (Žolíky, Continental, Last Card): the stand-in finishes the current round, and the seat is dealt out until its player returns. This needs a per-module capability (`module.DealsAround`) and a written scoring rule for a skipped round, so each module opts in.
- **A person takes over a seat** (a spectator or a link joiner inherits it). It needs the original seat holder's consent when they have an account, and a rule that they can't see the hand before taking it.
- **Late joining at cash tables:** take a free seat between hands with the starting stack, posting the big blind on the first hand.

## Order of work

1. **Part A server**, with conformance tests. Ship it behind `standInAfter` with a default of 60. That changes current behaviour, so it goes in the release notes.
2. **Part C client** for Part A, then the e2e test.
3. **Part B** engine and UI (Hold'em family, then Blackjack).
4. **Part D** after mobile Phases 2–3 are merged.

Every step is its own PR from its own worktree (concurrent-sessions-use-worktrees). Each has the Go tests with kdb set, the Playwright spec, and screenshots of the new banners in light and dark at phone width.

## Open questions

- Should the stand-in strength follow the table's bots (proposed) or be fixed at Medium?
- At a cash table, should automatic leave also apply when the player's *own* connection is fine but they are idle? Proposed: no, only when away.
- For a hybrid table with no remote guests, should the "Nearby + Online" label and the relay stay up anyway, or only while the toggle is on? Proposed: only while the toggle is on.
