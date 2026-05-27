# Žolíky Defold client

Playable GUI client for the Continental Rummy server.

## Setup

1. Open **`client/`** in [Defold Editor](https://defold.com/).
2. Fetch dependencies (**Project → Fetch Libraries**) so `extension-websocket` is installed (listed in `game.project`).
3. Start the server (`server/docker compose up --build`).
4. **Project → Build** and run (desktop or HTML5).

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
