# Game picker plan

Replace the home screen with a screen for choosing a game. Each game gets its own page, and table settings move to a separate screen. Mockups were agreed in the 2026-09-29 session.

## The flow

```
/ (pick a game) ──tap row──▶ /lobby/games?moduleId=X (game page)
   │  Resume ─▶ /match/… or /lobby/table…       ├─ Make me available (waiting for game X)
   │  Table code ─▶ /join/<code>                 ├─ Open a table ─▶ /lobby/setup?moduleId=X&mode=table ─▶ /lobby/table
   │  footer ─▶ /about                           └─ Play against bots ─▶ /lobby/setup?moduleId=X&mode=bots ─▶ /match/…
   avatar menu: Your games · … · Sign out · About (versions)
```

1. **Pick a game (`/`).** Each game is one row, and its status shows inside that row:
   - "your turn" plus a Resume button, "in progress", or "set aside", taken from the player's own unfinished tables;
   - "N waiting", taken from the per-game waiting list.
   
   Below the rows is a table-code box with a Join button. At the bottom sits the version footer, and tapping it opens About. The rows stay in a fixed order; the badges are what draw attention.
2. **Game page (`/lobby/games?moduleId=X`).** It shows who is waiting for *this* game, with a "Make me available" button. Under that are "Open a table" and "Play against bots", and the Rules link is in the header.
3. **Table settings (`/lobby/setup?moduleId=X&mode=table|bots`).** This is the setup screen from `games.tsx` with every section open. The button at the bottom stays visible while the screen scrolls, and reads "Open table" or "Deal me in".
4. **Table lobby or match.** Both screens are unchanged. "Deal me in" works the way today's "Play against a bot" button does: create the match, add the bots, start, then go to `/match/…` (`games.tsx:186-231`), with no lobby in between.

## Phase 1: server

### 1a. Availability per game — done

Today there is a single global pool: one Redis hash, `zolik:lobby:waiting`, and room `__lobby__` (`server/internal/lobby/store.go:30,137`).

- `lobby.Entry` gets `ModuleIDs []string`. **An empty list means "any game"**. Old clients connect without a module, so they keep working and appear in every game's list.
- Keep the one hash and filter it when reading. That way `Pickup`, `Leave` and `Heartbeat` stay keyed by player. Change `List(ctx)` to `List(ctx, moduleID)`, where `""` means everyone.
- `handleWS` reads `moduleId` from the query string, the same way it reads `avatar` (`lobby/handler.go:129`). `broadcastWaitingList` sends each recipient the list for their own game.
- `GET /lobby/waiting?moduleId=X` returns one game's list. With no module it returns everyone, each row including its `moduleIds`, so the picker can count per game with a single poll.
- `Store.IsWaiting` and `match.WaitingLookup.IsWaiting` take the match's module. `Manager.Invite` passes `match.ModuleID`, so a player waiting only for other games is refused with the existing `NO_LONGER_WAITING` code. From the host's point of view, that player has stopped waiting for this table. No new message key is needed, and `serverKeys.json` is unchanged.
- Tests: `lobby/bygame_test.go` (store, filtering, Redis mirror), `lobby/bygame_ws_test.go` (socket broadcast and `GET /lobby/waiting`), and two new invite tests in `match/invite_test.go`.

### 1b. "Your turn" on my tables — done

`storedTable` had no turn information, `Match.State` is never persisted (`bson:"-"`), and since PR #159 an ordinary move only appends to the move log without rewriting the envelope. Storing whose turn it is on the envelope would bring back a write on every move, so the answer is worked out when it's asked for:

- `GET /users/me/tables?turns=1` adds `yourTurn: true|false` to each row. Without `turns=1` the field is absent, so `/lobby/mine` and older clients pay nothing.
- `Manager.Awaits(matchID, playerID)` (`match/presence.go`) answers from the match's board through `module.AwaitedSeats`. That loads the match into memory, where the Resume tap will want it anyway. A suspended match already names its player, so it answers without loading anything. Lobby and completed rows are never loaded.
- Only active, suspended and abandoned rows are asked, and the list is at most 50 rows (20 by default).
- Test: `TestMyTablesSaysWhoseTurnItIsWhenAsked` in `match/stored_test.go`.

## Phase 2: client (`client-react-native`) — done

Built as described below, with these differences found while testing:

- Guest sign-in with no pending link returns to the existing main menu (`router.dismissTo('/')`) rather than pushing a second copy of it on top.
- A game's page and the main menu stay mounted under the screens they lead to, so both stop polling the waiting room while not focused (`useIsFocused`).
- The settings screen has its own Rules link (`setup-rules-<id>`), carrying the chosen ruleset and options, as the old setup cards did.
- Signed-out players still see "Sign in or continue as guest to play online." above the two buttons.
- Being available is held in memory. A full page reload ends it, just as it did when the card was on the home screen.
- The table-code box reuses the existing "Join code or invite link" string. Seven strings nothing uses any more were removed from every locale.


### Pick a game — `app/index.tsx`

Rewrite the home screen as the picker, keeping these parts:
- `useIntroGate`, `useFollowPendingDestination`, and `WaitingInvitesCard` (invites are still worth showing first);
- the offline button and the sign-in buttons for someone with no session.

Remove `MyTablesCard` and the "Play" / "Join a table" buttons, and move `WaitingStatusCard` to the game page.

- **Rows:** extend `GameButtons` (or write a new `GameRow`) to take a badge. The data comes from three sources:
  - `client.modules()`;
  - `listMyTables('unfinished')` with `turns=1`, grouped by module;
  - the waiting list, grouped by module (every 5 s through `useWaitingLobbyStatus`).
- **Resume:** picks the table where it is the player's turn, or failing that the most recently updated one, and goes through `routeForMatch`. Tapping anywhere else on the row opens the game page.
- **Table code:** the box accepts a code or a full invite link (`codeFromInviteInput`) and sends it to `/join/<code>`.
- **Footer:** `BuildFooter` stays at the bottom and now opens `/about` when tapped.

### Game page — `app/lobby/games.tsx`

- With no `moduleId`, redirect to `/`. `app/auth/guest.tsx` sends new guests to `/lobby/games` today; change that to `/`.
- With a `moduleId`, show:
  - the waiting card (today's `WaitingStatusCard`, filtered to this game);
  - the two buttons.
- **Staying available after leaving the page:** being available means holding a socket open. Today that socket belongs to a card on the home screen, so leaving the game page would drop the player from the list. Move `useLobbySocket` into a provider next to `InviteProvider`, holding `{ available, moduleId }`, so the player stays on the list while they browse. Show a small "Waiting for Hold'em · Stop" strip on the picker while they're on it.

### Table settings — new `app/lobby/setup.tsx`

- Move `GameSetupScreen` out of `games.tsx`.
- **Every section open by default.** This drops `openSetup`, `arrivedAt` and the scroll-to-section measuring, and the closed-by-default comment goes with it.
- **The bots row appears only in bots mode**, where it starts at the saved choice or 1. In table mode the host adds bots from the table lobby, as they do today.
- The start button is pinned to the bottom, with a different label for each mode. Choices are still saved through `saveGameSetup`.
- Register the screen in `app/_layout.tsx`.

### Your games and About — `src/components/AccountMenu.tsx`

- Add **Your games** as the first menu item, linking to `/lobby/mine`. It's offered to guests too, because guests have stored games. Remove it from `app/more.tsx`.
- Move **About** to the last item, below Sign out, with the versions as its hint: `app 1.3.5.3 · server 1.3.5.3`, from `CLIENT_VERSION` and `useServerBuild`. The old comment that kept About above Sign out is removed.

### Strings

Add new keys for the picker badges, the game-page headings, the "Deal me in" and "Open table" button labels, and the availability strip. They must be in every locale, and `untranslated-sweep.spec.ts` checks this.

## Tests

- **Unit tests:** `src/lib/__tests__/shell.test.ts` and `pendingDestination.test.ts` (both reference `/lobby/games`), plus tests for the resume choice (your turn first, then most recent) and the per-module grouping.
- **e2e tests to update:**
  - `home-waiting-status.spec.ts`: move it to the game page and cover the per-game filter;
  - `build-version.spec.ts`: the footer and the About hint;
  - `generic-shell.spec.ts`, `rematch.spec.ts`, `localisation.spec.ts`;
  - `e2e/helpers/lobby.ts` and `e2e/longevity/lib/ui.ts`, whose navigation paths change.
- **New e2e tests:**
  - picker badges: Resume opens the match; "N waiting" appears when a second player makes themselves available for that game only;
  - the bots flow goes straight to `/match`;
  - the table flow lands on `/lobby/table`.
- **In the browser:** walk both flows at phone width, and check a waiting player shows up under Hold'em but not Canasta.

## Order and branches

1. Server phase 1a and 1b on `claude/game-picker-server`. This merges first, and old clients keep working because an empty module list means "any game".
2. Client phase 2 on `claude/game-picker`. Develop it in a worktree, because the main checkout's tree is stale.

## Left out on purpose

- **"Open tables" on the game page.** There is no endpoint for it, and a match has no public or private flag, because a table's code is its access key. Listing tables would need an opt-in setting such as "Anyone can join" at creation, a `FindOpen(moduleID)` in both repositories, and a route. The game page leaves room for this section, but it ships without it.
