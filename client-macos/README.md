# Jokerless for Mac

A native Mac app (Swift, AppKit) around the same web client jokerless.com
serves, with the game core (`server/mobile/zolikcore`) linked into the app
itself. One process: the window, the web view's page and the core that hosts
offline tables. There is no helper program.

```
Web view (WKWebView): the React screens, display and JavaScript only
   │  bridge.js turns fetch, WebSocket and the zolik-nearby module into messages
   ▼
Swift (Bridge.swift, NearbyService.swift, Shared/*)
   │  Bonjour and Bluetooth: the iPhone module's own files, linked from
   │  client-react-native/modules/zolik-nearby/ios
   ▼
Go core (Zolikcore.xcframework): hosts tables, and makes every network request
   the page asks for: jokerless.com, this Mac, the local network, nothing else
```

## Windows and menus

- **One main window, a window per game** (`docs/macos-window-model-plan.md`).
  The main window holds everything but a game: home and the game picker,
  setup, the waiting room, My games, the account, rules, settings. It has
  Back and Forward in the title bar, and closing it hides it (the Dock icon or
  ⌘1 brings it back; the app and any table it hosts keep running). Each match
  or replay opens in a window of its own, titled "Game · vs opponents" with
  the status as its subtitle; resuming a game already open brings its window
  forward, closing one keeps the match under In progress, and the open games
  come back at the next launch. Rules and other screens asked for from a
  game go to the main window. **File › New Game** (⌘N) shows the game picker.
- **The app owns the connections.** The table this Mac hosts, the seat at an
  offline table (`seat` in `AppDelegate`) and a guest's tunnel to a table
  across the internet or over Bluetooth (`zolikcore/guest.go`, served on
  loopback) all belong to the app, so any window can show any game. The home
  screen and My games show a game with a window open as being played, with a
  button to show it, not as something to resume.
- The Dock badge counts invites waiting while the main window is not in
  front, and games waiting on you in windows that are not; their titles get a
  dot.
- **Account** holds what the phones keep behind the face in the header (who is
  playing, My games, Game circle, Sign in / Account, Sign out), worded by the
  page in the player's language (`src/desktop/DesktopMenuBridge.tsx`). The
  page does not show that face in the app.
- **View**: Hide/Show Hand, Table and Log (⌥⌘1–3) for the game in front
  (`src/desktop/useDesktopMatchView.ts`); Back (⌘[) and Forward (⌘]), which
  always undo each other; Home (⇧⌘H); Actual Size / Zoom In / Zoom Out;
  Reload; Full Screen.
- **Help › Rules** lists every game, the one in front first (⌥⌘/). Help also
  holds Terms, Privacy, Accessibility and Source, and **About** shows this
  build's version and commit and the server's: what the footer shows on the
  phones, which the app does not show.
- The page draws no header in the app. Back and Forward are in the main
  window's title bar and walk its history, the same one View › Back and
  Forward do; a game window has neither.
- Every menu bar title follows the player's language (`desktop.menu.*`).
 **Table**: Start an offline table (⇧⌘N), Back to online play.
  **Jokerless › About** shows the app's and the server's versions.
- The page sits below the title bar, which takes the header's colour.

## The web view has no network

- Pages load from `app://jokerless/`, answered in process by the core
  (`zolikcore.WebAsset`), with the same routing the server uses.
- Every page carries a content security policy whose `connect-src`,
  `img-src` and the rest allow only the app itself. The engine refuses
  anything else, whatever the page tries.
- `bridge.js` replaces `fetch` and `WebSocket` for http(s)/ws(s) addresses;
  the requests go to `zolikcore.NetFetch` / `NetSocketOpen`, which refuse any
  host that is not the configured server, loopback or a private address.
- Leaving the app (legal pages, source, sign-in providers) opens the
  person's browser.

## Build and run

```bash
scripts/build-macos.sh
```

```bash
open client-macos/build/Jokerless.app
```

The script exports the web client, builds the core with gomobile
(`-target=macos`, macOS 13 floor), compiles the Swift package and assembles an
ad hoc signed `.app`. `ZOLIK_MACOS_SKIP_WEB=1` / `ZOLIK_MACOS_SKIP_CORE=1`
reuse the previous web export or core.

A debug build can be pointed at a local server with `ZOLIK_BASE_URL`, and
keeps its state in `ZOLIK_DATA_DIR` if set.

## End-to-end tests

```bash
node e2e/macos/run.mjs
```

Builds and starts a local server, launches copies of the built app, and
drives them through a debug-only control folder (`E2EControl.swift`); a
Chromium plays a browser in the room. It covers: pages served from inside the
app; the web view refused the network directly; guest sign-in and an online
game against a bot through the core; hosting an offline table from the menu
bar; a browser joining by the room link; a second Mac finding the table over
Bonjour and joining; the deal reaching all three; a second window playing a
second game; zoom and the Account menu's sign-out; quitting ending the table.

The copies run as `com.jokerless.mac.e2e`, so the Local Network permission
macOS keeps for the real app is never consulted or changed by a test run.
Use the arm64 node (`PATH=/opt/homebrew/bin:$PATH`) on this Mac.

Set `ZOLIK_E2E_BLUETOOTH=1` to add a Bluetooth step: a copy started through
LaunchServices hosts a table over Bluetooth. The first run asks to allow
Bluetooth for Jokerless. (Started straight from a shell, macOS holds the
shell's app responsible for the request and kills Jokerless, because that app
declares no Bluetooth use; a Jokerless opened from Finder is unaffected.)

## Not done yet

- Bluetooth between two devices is untested: a Mac cannot hear its own
  advertisement, so it needs a phone or a second Mac. The host side (permission,
  radio on, advertising) is covered by the opt-in step above.
- Signing in with an outside provider (Google and the like) opens the browser
  but does not come back to the app: it needs a `jokerless://` callback.
- Mac App Store packaging: sandbox, team signing, notarization, Sparkle for
  the direct download.
