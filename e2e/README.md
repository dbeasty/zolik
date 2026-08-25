# E2E tests

Playwright specs that drive the real web build (`expo start --web`) against a
real Go server and Mongo.

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
2. **The Go server**, pointed at whatever Mongo/Redis you're using — pick a
   port and DB name that won't collide with a real dev server:

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

(Those two env vars default to exactly those values — see `helpers/env.ts` — so
if you used the ports above you can drop them.)

> **Run it from `e2e/`.** Playwright resolves its config from the working
> directory, and invoking it from the repository root silently runs without
> this project's config — the specs then fail with "did not expect
> test.describe() to be called here" rather than anything useful.

> **Start the server detached, not with a foreground `go run` you background.**
> A `go run` child that gets reaped mid-suite produces a wall of
> `ECONNREFUSED` failures that look exactly like a code regression and are not.
> `go build -o /tmp/zolik-server ./cmd/server && nohup /tmp/zolik-server &` is
> the reliable shape.

## What the suites cover

There are four, and none of them names a rule of any game.

- **`generic-shell.spec.ts`** — the load-bearing one. It drives
  `app/match/[matchId].tsx` through Prší, Canasta, Hold'em and Žolíky, clicking
  the controls a person would and never the same control twice by name. It also
  proves the numeric control poker added appears only where a game asks for a
  number, that a disabled control shows the engine's own reason, that the lobby
  renders from `/modules`, that joining by code works, and that the retired
  `/games/*` endpoints really are gone.
- **`bots.spec.ts`** — that `add-bot` produces an opponent that *plays*, in all
  four games. Its assertion is that the turn **comes back** after the human uses
  theirs; checking that a human has moves at the start proves nothing, because
  the deal usually puts them first.
- **`canasta.spec.ts`** — a whole Canasta match played to a winner over real
  WebSockets, plus partnerships and hidden information through the real
  serialisation.
- **`match-runtime.spec.ts`** — the runtime itself: persistence, per-viewer
  projection, refusals, and unknown modules.

### Two ways these tests have passed while proving nothing

Both were found and fixed; both are worth knowing about before writing another.

**Counting clicks instead of progress.** The shell spec used to assert that it
had pressed something. A click on a dead control counts just as well, so it now
reads the match back over HTTP and requires the *server's* board to have moved.

**A selector that matched the container.** `[data-testid^="offer-"]` also
matched the scrolling bar the offer buttons live in, so every "move" was a click
on a div. The bar is `action-bar` now. Fixing it turned three green tests red,
correctly.

## What is no longer here

Twelve specs drove the bespoke Žolíky screen — drag-to-meld, staging, joker
swaps, undo, auto-scroll — and seeded themselves through a rummy-shaped
`POST /games/{id}/debug-state`. Both the screen and the endpoint are gone. The
generic equivalent, `POST /matches/{id}/debug-state`, takes the module's own
state blob and is available for a spec that needs to start from a specific
position; none currently does, because playing against a bot is cheap.
