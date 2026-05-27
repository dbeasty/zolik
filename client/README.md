# Žolíky Defold client (v1 scaffold)

This folder contains the Defold client-side scripts for the Continental Rummy (Žolíky) project.

## Server endpoints (expected)
- WebSocket: `ws://<host>/ws/games/:id?token=<JWT>`
- Auth + lobby:
  - `POST /auth/guest`
  - `POST /games`
  - `POST /games/:id/join`
  - `POST /games/:id/start`
  - `POST /games/:id/add-ai`
- Offline scoring:
  - `POST /scoring-sessions`
  - `GET /scoring-sessions/:id`
  - `PATCH /scoring-sessions/:id`

## Script entry points
- `game/scripts/network.script` – WebSocket client + message send helpers
- `game/scripts/game_state.script` – authoritative local store from server messages
- `game/scripts/hud.script` – offer panel + error/suspension placeholders
- `game/scripts/hand.script`, `table.script`, `card.script` – thin rendering placeholders
- `game/scripts/lobby.script` – REST bootstrap + create/join/start/add-ai helpers
- `game/scripts/scoring_table.script` – REST scoring session helpers

## Notes
This repo does not include fully authored `.gui` / `.collection` assets due to Defold Editor format complexity.
You can wire the scripts into scenes and GUI nodes via Defold Editor, using the message types documented in the backend spec.

