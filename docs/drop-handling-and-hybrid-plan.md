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

**As built.** This differs from the first draft of this section in four places, each noted below.

### The option

Hold'em/Draw/Omaha get `format`: **Tournament** (the default) or **Cash table**. Blackjack is always a cash-style table, since each seat plays the dealer, so it gets the leaving rules without the option.

| `format` | Meaning | End of match |
|---|---|---|
| Tournament (0) | Freeze-out: everyone starts level and plays to the last chip. Nobody leaves with theirs. | `matchOverAfterHand`, as before |
| Cash table (1) | Come and go: what you leave with is your result | The hand limit, fewer than 2 seated, or no person still seated |

*Changed:* a stored match with no `format` reads as a tournament, whatever its hand limit. That is what every table was, so nothing changes under an existing game.

### Leaving

- **`module.VerbLeave`,** offered by `module.LeaveOffer`, last in the list, to every seated player who is not out.
  - The offer is **`Manual`**: no bot, sat-out seat or driver ever picks it (`module.ChooseActions`, the bot loop's candidates).
  - `ActionOffer.Live()` (enabled and not manual) is now what every "is it this seat's turn" check reads, on the server and in the client (`isLive`). An always-open "leave" says nothing about whose turn it is.
- **Poker** (*changed*: leave at once, never "after this hand"). A seat still holding cards folds as it goes, so its chips this hand stay in the pot. It is `Left` and `Out` from then on, with its stack frozen as its result; no separate `CashOut` field is needed. At a showdown, leaving counts as going on.
- **Blackjack.** Before your stake is dealt to, you leave at once, and an undealt stake is handed back. With cards out, the seat is `Leaving`: it plays the round out (sat out, if its player has gone) and gets up when the round settles.
- **Both games:**
  - A table where nobody who is a person is still seated ends, rather than leaving bots to play to nobody. Seats record `Bot` at the deal.
  - Standings rank by stack, which for a leaver is what they left with.
  - Seats show `seat.left` ("Left the table") or `seat.leaving`.
- **Client:** the generic controls render the offer ("Leave the table"), and `LeaveSheet` asks before sending it.

### Away at a poker or Blackjack table

- **Option `leaveAfterAway`:** 2, 5 (default) or 10 minutes, or never. It is declared by both games.
  - The runtime (`match/leave.go`) starts the clock when a person's connection goes, or when the bot loop finds the seat sat out after a restart.
  - When the wait is over it submits `leave` on the player's behalf. This is the one move the runtime makes for a person, and only the one the rules state.
  - The module decides whether the move is allowed: a tournament refuses it, and the seat stays sat out.
- **Tournament:** sat out and blinded off, never removed, no stand-in.
- *Changed:* **blinds while sat out at a cash table are still posted.** The wait before an absent player is cashed out bounds what that costs, and skipping blinds would need the engine to know about presence. Left for later if it matters.
- *Changed:* there is **no "net against the buy-in"** column on results. Standings stay chips.

### Tests (as built)

- `holdem/leave_test.go`:
  - A tournament refuses leaving and offers no leave.
  - Leaving mid-hand, off turn.
  - Leaving heads-up ends the match.
  - A table with only bots left ends.
  - Leaving at the showdown doesn't hold the table up.
- `blackjack/leave_test.go`:
  - Leaving before the deal, with a stake handed back.
  - Leaving mid-round waits for the round to settle.
  - The last person leaving ends the table.
  - The offer is last and manual.
- `match/leave_test.go`:
  - An away player is cashed out of a cash table.
  - A tournament never cashes anybody out.
  - Coming back in time keeps the seat.
- `e2e/tests/cash-table.spec.ts`:
  - Leave, "stay" changes nothing, then leave for real; every board shows it.
  - A tournament has no leave control.
- `e2e/tests/blackjack.spec.ts`'s socket driver now skips manual offers, as every driver must.

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

**As built.** This is the detail behind Phase 2d of the mobile plan.

### Transport
- **Cloud relay, `internal/relay`.** Mounted on the full server only, never in `RegisterMobileRoutes`.
  - `GET /relay/host` (WSS) is for the phone. It needs `Authorization: Bearer <node credential>`, checked with `auth.VerifyNodeCredential` against this server's own keys, so only an enrolled phone can open a table to the internet. The first frame back is `{"t":"code"}`.
  - `GET /relay/join/{code}` (WSS) is for each guest. It answers 404, 503 (`RELAY_HOST_AWAY`) or 409 (`RELAY_FULL`) *before* upgrading, so a browser gets a status rather than a socket that opens and closes.
  - `GET /relay/info/{code}` returns online, name, instanceId, guests and protocol.
  - Frames: binary `id(4) ‖ payload` to and from the host, raw payload to the guest. Text `open`, `close` and `end` frames are for the host only.
  - Guests are closed with **4002** when the phone drops and **4003** when it ends the table. A dropped phone's code is kept for 10 minutes and given back to the same node, and only that node.
  - Limits: 8 guests per table, 256 KiB per frame, a per-guest token bucket, and pings every 25 s.
  - Tests: `internal/relay/relay_test.go` covers both directions, the host leaving, the same code reclaimed, a different node refused, non-nodes refused, and the guest cap.
- **Go core, `zolikcore/relay.go`.**
  - `OpenRelay(name)`, `CloseRelay()`, `RelayStatus()`, `RelayCode()`, `RelayURL()`, `RelayGuests()` and `Resumed()`.
  - Each `open` creates a `Tunnel`; `tunnel.go` is unchanged apart from a `tunnelCloser` hook, so the relay can hang up a guest whose handshake fails.
  - It redials with backoff and asks for the same code.
  - `Host` now keeps the node credential and the cloud URL from `StartNode`.
  - Test: `relay_test.go` signs in a remote guest through a real relay, and closing ends the table.
- **Native module.** `openRelay`, `closeRelay`, `relayStatus` and `hostResumed` in `index.ts`, Swift and Kotlin. The rebuilt iOS framework's header names every method as the Swift calls it. *Not verified:* a device build and run.
- **TS:**
  - `src/net/relay/link.ts` gives `BleTransport` a WebSocket link (`relayLink`). It asks `/relay/info` first, so "server offline" and "you're offline" can be told apart.
  - `SessionContext.joinRelay` sits the player down `via: 'internet'` with the table's `serverName`, and sets `serverGone` from the link.
- **Version gate:** the `protocol` in `/relay/info` is compared before joining, and a mismatch reuses the nearby "update the app" message.

### Identity and results
Unchanged from the plan. A signed-in guest presents their offline pass (`sitAt`); everyone else sits as a guest. The Phase 2c consent flow applies.

### Drops on a hybrid table
- **A player drops:** the Part A/B rules apply.
- **The host's relay link drops** (`HoldStandIns(true)` from the core): no stand-in and no cash-out starts. When it is back (`HoldStandIns(false)`), every away clock starts again from zero.
- **The host app returns to the foreground** (`AppState` → `nearby.hostResumed` → `Manager.HostResumed`): every away clock restarts, and the reaper abandons nothing for one abandon window. Tests: `match/hostpause_test.go`.
- **The phone dies or is put away:** guests see "The server (Ada's phone) is offline". Coming back under the same code restores the game.
- **The host stops the table:** the relay closes guests with 4003, and the code is gone.

### UI
- **Guest join page `app/r/[code]`:** "This table runs on Ada's phone", what happens if that phone goes offline, a name field, and "Join the table". It then goes to the guest's offline screen, which names the table's phone.
- **Hosting screen:** an "Anywhere" card.
  - Needs sign-in.
  - "Let people join over the internet".
  - Status (open, N joined / reconnecting).
  - "Your phone is the server…"
  - Link, QR code, Share and Close.
- **Match header:** "Server: this phone" or "Server: Ada's phone". The status explainer says what that means.
- **Banner:** "The server (Ada's phone) is offline. The game is paused and nothing is lost." This is checked *before* "you're offline", because the server going takes this player's socket down too.

### Tests
- `e2e/tests/relay.spec.ts`, with `ZOLIK_E2E_PHONEHOST` set to a built `cmd/phonehost`; it is skipped otherwise:
  - A real account enrols a desktop "phone" with the cloud. The cloud needs `JWT_SIGNING_KEY_FILE` to issue node credentials.
  - The phone opens its relay.
  - A browser on the cloud's web client joins by link, takes a seat, and plays.
  - The phone is killed: the server-offline banner shows.
  - The phone restarts: the same code comes back, and the game with it.
  - Passed 4 times out of 4.

### Not done
- Per-seat connection badges (Wi-Fi / Bluetooth / Internet). The runtime does not know which transport a socket came over, and the relay status shows the remote count instead.
- A device build of the new native calls.

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
