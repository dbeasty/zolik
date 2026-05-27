# Žolíky Defold build notes (v1)

Repo path: **`client-defold/`** (formerly `client/`). Headless builds use that directory, e.g. `bob.jar build client-defold --platform wasm-web --output client-defold/build`.

**Important:** In `game.project`, resource paths must end with **`c`** (compiled-resource convention). Bob strips the last character when resolving, so `/main/main.collection` becomes `/main/main.collectio` and fails. Use e.g. `main_collection = /main/main.collectionc` and `game_binding = /input/game.input_bindingc`.

## Prerequisites

| Piece | Required? | Notes |
|-------|-----------|--------|
| **Docker Desktop** (or Docker Engine + Compose v2) | Yes (backend) | Runs MongoDB, Redis, and the Go API via `server/docker-compose.yml`. You do **not** need Go, Mongo, or Redis on the host. |
| **Defold Editor** | Yes (client) | [defold.com/download](https://defold.com/download/) — desktop build/play is included. |
| **Network** | Yes (once) | **Project → Fetch Libraries** pulls `extension-websocket` from GitHub (`game.project` dependency). |
| **Java JDK** | CI / bob only | Headless builds via `bob.jar`; not needed for normal Editor workflow. |
| **Android SDK + JDK** | Android bundles only | Configure in Defold per their Android docs. |
| **Xcode** (macOS) | iOS bundles only | Required for iOS device/simulator builds. |

**Not required for default local play:** Go toolchain, local MongoDB/Redis installs. A single `app` container is enough; Redis in Compose is for multi-instance smoke tests.

## First-time setup

1. **Server env** (first time only): `cd server && cp .env.example .env` — defaults are fine for local dev.
2. **Start backend** (from `server/`): `docker compose up --build`
   - API: `http://127.0.0.1:8090` — health: `GET /healthz`
   - Optional: mongo-express at `http://127.0.0.1:8081` (login `dev` / `dev`)
   - Second app on port `8092` is only for scale testing (see README).
3. **Verify server** before building the client:
   ```bash
   curl -s http://127.0.0.1:8090/healthz
   ```
4. **Defold**: open `client-defold/` → **Project → Fetch Libraries** → **Project → Build** (desktop) or **Bundle** (HTML5 / mobile).
   - Entry: `/main/main.collection` → GUI `/gui/app.gui`.
   - Client default host: `http://127.0.0.1:8090` (`zolik.base_url` in `game.project`).

## Local server

- Start Mongo + server via Docker Compose (from `server/`):
  - `docker compose up --build`

## Open the project (macOS)

Do **not** open `game.project` as a loose file. Defold needs the **folder** that contains `game.project` as the project root.

**Easiest:** in Finder, double‑click [`open-project.command`](open-project.command) inside `client-defold/` (first time: right‑click → Open if macOS blocks it).

**Or from Defold’s start screen:** **Open From Disk** → go to the parent folder → select the **`client-defold` folder** (one click on the folder name) → **Open**. Do not double‑click into the folder and then open `game.project`.

**Or terminal:**

```bash
open -a Defold /Users/davidj/devel/zolik/client-defold
```

**You opened it correctly when** the **Assets** pane lists `main/`, `gui/`, `game/`, `input/`, plus a library entry for `extension-websocket`. The window title should show **Žolíky** (from `game.project`), not only “game.project”.

## Defold editor

- Project root must be `client-defold/` (see above).
- **Project → Fetch Libraries** (installs `extension-websocket` from `game.project`).
- Main collection: `/main/main.collection` → GUI `/gui/app.gui`.
- Build targets:
  - Desktop: Build in Defold Editor
  - HTML5: Bundle → HTML5
  - Android/iOS: Bundle for the respective platform

## CI (optional)

- Use Defold’s headless builder (bob) for automated builds — requires a **JDK** and `bob.jar` from [Defold releases](https://github.com/defold/defold/releases).
- Typical command pattern (from repo root):
  - `bob.jar build client-defold --platform <platform> --output client-defold/build`

## Troubleshooting

### Fetch Libraries finishes instantly

That is usually **cache hit**, not a failed download. Defold stores zips under `client-defold/.internal/lib/`.

From the repo root:

```bash
ls -lh client-defold/.internal/lib/*.zip
unzip -l client-defold/.internal/lib/*.zip | grep -E 'websocket/ext.manifest|refs/tags'
```

You want a multi‑MB zip whose listing includes `websocket/ext.manifest`. After changing the URL in `game.project`, quit Defold, delete `client-defold/.internal/` and `client-defold/build/`, reopen the project, then **Fetch Libraries** again (first fetch should take a few seconds).

In the editor **Assets** pane, expand the library dependency tree — you should see an **`extension-websocket`** (or `websocket`) folder from the dependency, not only your local `main/`, `gui/`, `game/` folders.

### “Extension might be missing” / “unrecognizable data” (GUI, collection, or Build)

Defold’s full message is usually: *“contains unrecognizable data. Could the project be missing a required extension?”* That almost always means the **project root is wrong** (e.g. only `game.project` was opened, or the repo root `zolik/` was opened), not that Fetch Libraries failed.

1. Quit Defold.
2. Reopen using [Open the project](#open-the-project-macos) above.
3. Confirm **Assets** shows `gui/app.gui` and `main/main.collection`.
4. **Project → Fetch Libraries**, then **Project → Build**.

If it still fails: **Help → Show Logs** and search for `Unable to read resource` or `ParseException` on `/gui/app.gui`.

### `the Lua module '/websocket.lua' can't be found` (macOS / desktop Build)

You do **not** need to run an HTML5 **Bundle** first. Desktop **Build** and HTML5 **Bundle** are separate targets.

This error usually means Lua used `require("websocket")`. **extension-websocket** is a native extension — it registers a global `websocket` table, not a `websocket.lua` file. Bob’s compiler then fails even if **Fetch Libraries** succeeded.

1. **Project → Fetch Libraries** (dependency must appear under **Assets**).
2. Use the global API (as in the [extension docs](https://defold.com/extension-websocket/)), not `require("websocket")`. This repo’s `game/scripts/ws_client.lua` follows that pattern.
3. **Project → Build** again for macOS.

### GUI looks black, clipped, or wrong size (desktop / Retina / Vulkan)

The GUI **canvas** (`game.project` `[display]` + **`gui/app.gui`** `root`) is **900×800** so the default window is card-sized instead of spanning a multi-monitor logical width. Layout uses reference **960×640**: **`px(n) = n × min(900/960, 800/640)`**. **`FONT_BOOST`** is **`1.0`**. **`apply_gui_root_scale()`** compares **`window.get_size()`** to the design after dividing by **`window.get_display_scale()`** (backing-store vs logical pixels on Retina), only shrinks the root when **`s < 1`** (never scales up inside the scene — that clips).

1. **`gui/app.gui`** — **`adjust_reference: ADJUST_REFERENCE_DISABLED`**, root **900×800** centered at **(450, 400)**. Do **not** use **`ADJUST_REFERENCE_PARENT`** with **`ADJUST_FIT`** on the root — on Retina, the parent size is ~2× the scene and the lobby clips / looks like the old tiny UI.
2. **`gui/app.gui_script`** — **`gui.set_adjust_mode(ui.root, gui.ADJUST_NONE)`** in **`init`**; **`window.set_size(900, 800)`** via **`pcall`** on startup (desktop). There is **no** **`window.set_listener(WINDOW_EVENT_RESIZED)`** — on macOS it can spam resize events while dragging and fight the engine.
3. **`[display] high_dpi`**: default **`0`**; try **`1`** after the layout looks correct.
4. Logs: **`[zolik_gui] apply_gui_root_scale`** includes **`scene`** from **`gui.get_width()` / `gui.get_height()`** (expect **900×800**). **`render.*`** may stay **`nil`** in GUI scripts; use **`window.get_size`** (and **`display_scale`** when interpreting width).
5. To silence logs: **`[zolik] debug_gui = 0`** in `game.project`.

- **Edit screens:** double‑click **`gui/app.gui`** after the project is loaded correctly.
- **Run the game:** **Project → Build** — you do not need to open collections in the scene view.

### `/main/main.collection` can't be found

1. **Open the correct project root** — **File → Open Project** → `client-defold/` (contains `game.project` and `main/`). Not the repo root `zolik/`.
2. **Confirm the asset exists** — **Assets** should list `main/main.collection`. If `main/` is missing, `git pull` the `client-defold/` tree (old top-level `client/` was renamed).
3. **Check `game.project`** — `[bootstrap]` → `main_collection = /main/main.collectionc` (note the trailing **`c`**).
4. **Reload** — **Project → Reload Project**, or quit Defold and reopen `client-defold/`.

## Reconnect/orientation testing checklist

- Verify WS reconnect preserves `myHand` and hides other players’ hands.
- Force suspend/resume flows by disconnecting during an AI/human turn.
- Rotate device mid-round and confirm HUD/offer panel remains usable.
