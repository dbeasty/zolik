# zolik

Card game experiments — Go backend and three clients.

## Layout

- `server/` — Žolíky game API (REST, WebSocket, MongoDB, embedded SSH TUI host)
- `client-react-native/` — Expo mobile GUI client (primary GUI for iOS/Android)
- `client-defold/` — Defold GUI client (renamed from `client/`; open this folder in Defold, not the repo root)
- `client-tui/` — Text-only terminal client (Bubbletea + SSH)

See each directory's README for setup and run instructions.

Defold asset paths inside the project (e.g. `/main/main.collection` in `game.project`) are unchanged — only the repo directory name moved to `client-defold/`.
