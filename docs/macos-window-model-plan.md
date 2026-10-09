# Mac app: one main window, a window per game

Status: implemented 2026-10-09 on `claude/macos-client` (PR #305), except the
Bluetooth half of phase 3 (see below). `node e2e/macos/run.mjs`: 21/21.

Differences from the plan as written:

- **Bluetooth guests stay in the page.** The guest tunnel is in the core
  (`server/mobile/zolikcore/guest.go`, with a gateway on loopback) and the
  internet relay uses it. The Bluetooth link's bytes are not yet handed to it
  from `NearbyBleGuest`, and a Mac cannot be tested against its own radio. A
  Bluetooth seat is not shareable (`seatIsShareable`), so that game stays in
  the main window instead of opening a game window.
- **New Game (⌘N)** goes to the home screen, which is the game picker;
  `/lobby/games` is a per-game screen.
- "Your turn" alerts: badge and title dot only (decision 3, as suggested).
  Invite notifications use `UNUserNotificationCenter` and are not exercised
  by the e2e run (they would ask for permission); the Dock badge is.

## Why

Today every window is a whole copy of the app (File › New Window), and new
windows open as tabs of the first. Playing two games means two copies of the
home screen, the lobby and the account, which is not how the app is built: a
window holds one stack of screens, and a game is one place in that stack.

Mac apps that hold documents work differently: one window that is always
there (Mail's viewer, Xcode's welcome window) and one window per document.
Here the document is a game.

## The principle: the app owns the connections, windows are views

Everything that talks to a network, and the server that hosts games, lives in
the app (the Go core and its Swift around it), not in any window's page:

- **The game server** (the core hosting this Mac's table, the room listener,
  Bonjour, Bluetooth advertising, the internet relay for remote guests) is
  already in the app process. It starts and stops with the app, not with a
  window: closing every window leaves the table running; quitting ends it.
- **Every connection a player makes**: online requests and match sockets
  already go page → app → core. The two that still live in a page are moved
  into the core (phase 3): a guest's **Bluetooth** tunnel and an **internet**
  guest's relay link.
- **Which offline table the player sits at** (the seat) is app state, not a
  page's memory, so every window sees the same seat.

A window's page then does one thing: draw a screen and send requests to an
address the app gives it. Any window can show any game, whichever way it is
reached.

## The model

| | Main window | Game window |
|---|---|---|
| Holds | Home, game picker, setup, waiting room, In progress, Play offline, account screens, rules, settings | One match (or one replay) |
| How many | Exactly one | One per open match |
| Connections | None of its own: requests go through the app | The same |
| Opens | At launch | When a match starts or is resumed |
| Closing it | Hides it; the app keeps running (Dock icon, ⌘1 or Window › Jokerless brings it back) | Closes the view only. The player stays seated; the match is under In progress |
| Back / Forward | Yes, both ways, in the title bar and View | None. A game window is one game: closing it is how you leave |
| Invites and banners | Shown here | Not shown |
| Title | The screen's name | The game and table, e.g. "Last Card · vs 2 bots"; the match status (active, the server, the code) as the window's subtitle |

Rules opened from inside a game, and any other screen a game window asks for
("Back to games", the rules link on a refusal), go to the main window, which
comes forward. Rematch and the next deal stay in the same game window.

## Phases

### 1. Windows (native)

- `MainWindowController` (one) and `GameWindowController` (keyed by match id),
  replacing today's interchangeable `GameWindowController`.
- `openGame(matchId, path)`: brings an existing window for that match forward
  instead of opening a second one, which would also displace the first
  window's match socket.
- No tabs (`tabbingMode = .disallowed`).
- Closing the main window hides it; clicking the Dock icon and ⌘1 show it.
- Open game windows are remembered (match ids in user defaults) and reopened
  at launch, skipping matches that have finished.
- Signing out closes every game window.

### 2. Routing (page, desktop only)

- The boot config says which window the page is in:
  `__ZOLIK_DESKTOP__.window = { role: 'main' | 'game', matchId? }`.
- Main window: one route watcher (next to `NavHistoryTracker`) catches every
  arrival at `/match/:id` or `/replay/:id` (13 call sites today: setup, table,
  join, deal links, seat links, resume buttons, my games, table rows). It asks
  the app to open the game window and puts the main window back where it was:
  `router.back()` when it came from a screen, or Home when it was a `replace`
  from the waiting room. The call sites themselves stay unchanged, and so do
  the phones and the website.
- Game window: navigation that leaves the match (`/match/:id` → anything else
  that is not another match id) is sent to the main window instead, and the
  game window closes when the move means the player left the match ("Back to
  games" after the end) or stays open otherwise (rules).
- The waiting room (`/lobby/table`, `/lobby/join`) stays in the main window;
  the game window opens when the host starts.

### 3. Connections in the app (the part that needs care)

Today an offline seat is a live object in the main page's memory
(`OfflineState` in `SessionContext`). For Bluetooth and internet guests it
also holds the connection itself: `BleTransport` (`src/net/ble/transport.ts`,
`crypto.ts`), the tunnel's handshake and sealed messages, run over a
Bluetooth link (`src/net/ble/link.ts`) or the cloud relay
(`src/net/relay/link.ts`). Both links carry the same tunnel, so this is one
port, not two.

**3a. The seat moves into the app.** A native `SeatStore` holds the table the
player sits at (the serializable `OfflineTable`: instance id, address, role,
how it is reached, check code, the server's name). Pages read it from their
boot config and hear when it changes; on desktop `SessionContext` restores
its offline state from it and the credentials already stored under
`zolik_offline_<instanceId>`. One seat per app, as on a phone: sitting down at
another table in any window moves every window.

**3b. The guest tunnel moves into the core.** `zolikcore` gains the guest
side of the tunnel it already serves as host (`tunnel.go`): handshake, keys,
check code, framing, and requests and sockets multiplexed over it, ported
from `transport.ts` and `crypto.ts`.
- Over Bluetooth: Swift's `NearbyBleGuest` hands the link's bytes to the
  core's guest tunnel instead of to the page.
- Over the internet: the core dials the cloud relay itself (a WebSocket, like
  the rest of its outgoing connections).
- Tested in Go against the existing crypto vectors (`vectors.json`) and
  against the host side in process: a guest tunnel and a host tunnel wired
  back to back, carrying requests, a match socket and a close.

**3c. A gateway per tunnel.** The core serves each guest tunnel on loopback
(`http://127.0.0.1:<port>`), forwarding every request and socket through it.
To a page, a table over Bluetooth or the internet is then an ordinary
address, exactly like a table on Wi-Fi: it goes in the seat's `baseUrl`, and
any window can use it. `via` still says Bluetooth or internet, for the
labels and the check code.

**3d. The page asks the app to connect.** On desktop, joining over Bluetooth
or by an internet code calls new bridge operations (`bleJoin(peripheralId)`,
`relayJoin(code)`) that answer with the gateway address, the host's info and
the check code, instead of building `BleTransport` in the page. Online and
Wi-Fi joins do not change.

**Lifetime.** A guest tunnel lasts as long as the seat: closing windows keeps
it, Back to online play or sitting down elsewhere closes it, quitting closes
it. If the link drops, the core reconnects as the page does today (find the
table again by instance id, `connectToTable` in `link.ts`), and the gateway's
address stays the same, so open windows carry on.

**The phones** keep their page-side tunnel for now. They run the same core
(gomobile), so a follow-up can move them onto the core's guest tunnel too and
delete the page copy; one implementation is the goal. Until then, the
tunnel's crypto vectors keep the two in step.

### 4. Window chrome

- The page's own header is not drawn in the app (`headerShown: false` on
  desktop). The window title is the screen's name, and the main window gets
  ← → as title-bar buttons (`NSToolbar`), so "Jokerless" appears once in the
  title bar and once as the home screen's heading.
- The match screen sends its status line (active, the server, the table code)
  as the game window's subtitle.
- View › Hide/Show Hand, Table and Log act on the front game window; Back,
  Forward and Home on the main window.

### 5. Notifications

- Invite banners only in the main window.
- When the main window is not in front: a macOS notification for an invite
  (asks permission once), and the Dock badge counts waiting invites.
- Game windows report "your turn": a background game window bounces nothing
  but adds to the Dock badge, and its title gets a dot (•), the way unsaved
  documents do. Clicking a notification brings the right window forward.

### 6. Menus

- File: **New Game** (⌘N) brings the main window forward at the game picker,
  replacing New Window. Close Window (⌘W) closes a game window or hides the
  main window.
- Window: **Jokerless** (⌘1) for the main window, then each game window by
  title (macOS lists them).
- Account and Table act on the main window wherever the focus is.

### 7. Tests (e2e/macos/run.mjs)

Replace "a second game in a second window" with:
- starting a bot game opens a game window, and the main window is back home;
- a second game opens a second game window; both play;
- resuming a game that is already open brings its window forward, with no
  duplicate;
- closing a game window keeps the match under In progress, and Resume reopens
  it;
- an offline game hosted on this Mac opens in its own window (the seat in
  the app), and the browser and second-Mac steps still pass;
- an internet guest: a second Mac joins a table hosted by `cmd/phonehost`
  through the local server's relay, by code; the game opens in its own window
  and keeps playing after the window that joined is closed (the tunnel is the
  app's, not the page's);
- closing every window while hosting keeps the table: the browser guest
  carries on;
- quit and relaunch reopens the open game windows;
- the Dock badge counts an invite while the main window is in the background;
- a Wi-Fi guest's game opens in a game window.
- Bluetooth between two devices still needs a phone or a second Mac (a Mac
  cannot hear its own advertisement). The guest tunnel itself is covered by
  the Go tests in 3b, which run the same code with the radio replaced by a
  pipe.

The website and phone suites must stay untouched: everything above is behind
`IS_DESKTOP` or in Swift.

## Decisions to confirm

1. **Home heading:** keep the "Jokerless" heading on the home screen as the
   brand, with the title bar showing it once more? Suggested: keep it.
2. **Waiting room:** stays in the main window, with the game window opening on
   Start. The alternative opens the game window already at the waiting room.
3. **"Your turn" alerts:** Dock badge and title dot only, or also a macOS
   notification? Suggested: badge and dot only, so playing a game in one window
   doesn't make the other window send notifications.

## Size

About five to seven days: two to three for the window model (phases 1, 2, 4
to 7), and three to four for phase 3, most of it the guest tunnel in Go and
its tests. Moving the phones onto the core's tunnel is a separate follow-up.
