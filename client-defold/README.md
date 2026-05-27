# Žolíky Defold client

Playable GUI client for the Continental Rummy server.

## Project location

This Defold project lives in **`client-defold/`** (formerly **`client/`** at the repo root).

**Open the folder, not the file:** use **Open From Disk** and choose the **`client-defold`** directory, or double‑click **`open-project.command`** in this folder (macOS). Opening `game.project` alone in a file dialog does not set the project root and causes “extension might be missing” on every asset.

Paths inside the game (bootstrap `/main/main.collection`, `/gui/app.gui`, etc.) are **project-relative** and did not change when the repo folder was renamed.

**Edit UI in Defold:** open **`gui/app.gui`**, not the collection scene view. **Fetch Libraries** may return quickly when the zip is already cached — see [BUILD.md](BUILD.md#fetch-libraries-finishes-instantly).

## Prerequisites

- **Docker Desktop** (or Docker + Compose v2) — runs the game server locally (no Go/Mongo on the host).
- **Defold Editor** — [defold.com/download](https://defold.com/download/)
- **Network** (once) — **Project → Fetch Libraries** installs `extension-websocket`.

Optional: JDK + `bob.jar` (CI), Android SDK / Xcode (mobile bundles). See [BUILD.md](BUILD.md) for CI commands, platform SDK notes, and the reconnect/orientation testing checklist.

## Setup

1. **Server env** (first time): `cd server && cp .env.example .env`
2. **Start backend**: `cd server && docker compose up --build` — API at `http://127.0.0.1:8090`
3. **Verify** (optional): `curl -s http://127.0.0.1:8090/healthz`
4. Open **`client-defold/`** in [Defold Editor](https://defold.com/).
5. **Project → Fetch Libraries** — installs `extension-websocket` (listed in `game.project`).
6. **Project → Build** and run (desktop or HTML5), or **Bundle** for mobile.

## Configuration

In **game.project** add custom properties (optional):

| Property | Default | Purpose |
|----------|---------|---------|
| `zolik.base_url` | `http://127.0.0.1:8090` | REST + WebSocket host |

For a second server instance (scale test), point HTML5/desktop builds at `http://127.0.0.1:8092` while Redis syncs game broadcasts.

## Flow

1. **Guest login** — obtains JWT (`userId` = token subject = your player id).
2. **Create game** / **Join** (tap join code row to cycle demo codes; paste real join code in `game.project` config or extend UI).
3. **Add AI** + **Start** (or **Connect WS** if already started).
4. In-game: tap cards to select, **Draw**, **Lay meld**, **Discard**, **Accept/Decline** offer.

## Architecture

- `gui/app.gui` + `app.gui_script` — full UI (lobby + table).
- `game/scripts/api.lua` — REST (`/auth/guest`, `/games`, …).
- `game/scripts/ws_client.lua` — WebSocket actions.
- Legacy scripts under `game/scripts/*.script` remain as reference hooks.

## Multi-server note

User accounts and JWTs stay in **MongoDB**. **Redis pub/sub** only mirrors WebSocket messages between app instances so players on different nodes still see updates. You do **not** need Redis for a single local server (leave `REDIS_URL` unset).
