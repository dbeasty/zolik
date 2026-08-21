# E2E tests

Playwright specs that drive the real web build (`expo start --web`) against a
real Go server + Mongo, and seed each test straight into the mid-round UI
state it actually wants to exercise via a dev-only REST endpoint —
`POST /games/{id}/debug-state` — instead of playing a full deal turn-by-turn
over WebSocket every time. See `server/internal/game/rest_handlers.go`'s
`debugState` and `helpers/seed.ts`.

## One-time setup

```sh
cd e2e
npm install
npx playwright install chromium
```

## Running

You need three things running:

1. **Mongo + Redis** (already provided by `server/docker-compose.yml` in
   dev — `docker compose up -d mongo redis` from `server/`).
2. **The Go server**, with test endpoints enabled (on by default when
   `APP_ENV` is unset/`local`) and pointed at whatever Mongo/Redis you're
   using — pick a port and DB name that won't collide with a real dev
   server you might also have running:

   ```sh
   cd server
   APP_ENV=local PORT=8095 MONGO_URI=mongodb://localhost:27017 \
     MONGO_DB=zolik_e2e REDIS_URL=redis://localhost:6379/0 \
     go run ./cmd/server
   ```

3. **The web client**, pointed at that same server:

   ```sh
   cd client-react-native
   EXPO_PUBLIC_ZOLIK_BASE_URL=http://127.0.0.1:8095 npx expo start --web --port 8100
   ```

Then, from `e2e/`:

```sh
ZOLIK_E2E_API_BASE=http://127.0.0.1:8095 \
ZOLIK_E2E_WEB_BASE=http://127.0.0.1:8100 \
npx playwright test
```

(Those two env vars default to exactly those values — see
`helpers/env.ts` — so if you used the ports above you can drop them.)

## Why the drags look the way they do

`helpers/drag.ts` has the load-bearing details, empirically discovered:

- **A drag needs a pause right after `mouse.down()`, before any
  movement.** Without it, react-native-gesture-handler's web pointer
  manager never recognizes the pan gesture at all — the card just silently
  snaps back on `mouse.up()`, no error anywhere.
- **Raw `page.mouse` coordinates don't auto-scroll** the way locator
  actions (`.click()`, etc.) do. Staging a card grows the staging box,
  which can push the hand row below the fold — always
  `scrollIntoViewIfNeeded()` the drag source right before reading its
  `boundingBox()`.
- **The quick-swipe-up-to-discard shortcut is checked last**, only once
  every real drop zone (discard pile, table melds, staging area) has
  already come up empty — see the comment on `tryDropOnStaging` in
  `client-react-native/src/components/HandRow.tsx`. This *used* to be
  checked first, which meant any fast drag upward toward the staging
  area or a table meld (both of which sit above the hand row) got
  silently rerouted into a discard instead. Caught by this suite.

## Debug-state and card-supply conservation

`debug-state` bypasses `rules` validation entirely and writes hands/melds
directly, so it doesn't care whether a card you put in the human's hand is
also sitting in an AI's meld — real 2-deck games have duplicates anyway
(see `rules.DeckCountForPlayers`). It does still run `rules.ValidateMeld`
on any melds you seed, so a nonsense meld (e.g. three different ranks)
gets rejected with a 400 rather than silently corrupting state.
