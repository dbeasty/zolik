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
Bonjour and joining; the deal reaching all three; quitting ending the table.

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
