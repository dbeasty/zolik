# Žolíky Defold build notes (v1)

## Local server
- Start Mongo + server via Docker Compose (from `server/`):
  - `docker compose up --build`

## Defold editor
- Open the Defold project at `client/`.
- **Project → Fetch Libraries** (installs `extension-websocket` from `game.project`).
- Main collection: `/main/main.collection` → GUI `/gui/app.gui`.
- Build targets:
  - Desktop: Build in Defold Editor
  - HTML5: Bundle → HTML5
  - Android/iOS: Bundle for the respective platform

## CI (optional)
- Use Defold’s headless builder (bob) for automated builds.
- Typical command pattern:
  - `bob.jar build <path-to-project> --platform <platform> --output <dir>`

## Reconnect/orientation testing checklist
- Verify WS reconnect preserves `myHand` and hides other players’ hands.
- Force suspend/resume flows by disconnecting during an AI/human turn.
- Rotate device mid-round and confirm HUD/offer panel remains usable.

