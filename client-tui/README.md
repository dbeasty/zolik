# Žolíky — text-only client (client-tui)

SSH terminal UI for playing Žolíky. Connects to the same REST + WebSocket API as the Defold client.

## How it runs

The TUI is **embedded in the game server** (`server/cmd/server`). When `SSH_ENABLED=true`, the server listens on `SSH_PORT` (default `2222`) alongside HTTP.

```bash
cd server
docker compose up --build
# or: go run ./cmd/server
```

Connect:

```bash
ssh -p 2222 guest@localhost
# password: anything (guest user)
```

Registered users:

```bash
ssh -p 2222 myusername@localhost
```

## Local development

This module (`zolik/client-tui`) is imported by `zolik/server` via a `replace` directive in `server/go.mod`.

```bash
cd client-tui && go test ./...
cd ../server && go build ./cmd/server
```

## Layout

- `ssh/` — Wish SSH server (imported by server)
- `ui/` — Bubbletea screens
- `api/` — HTTP + WebSocket client
- `internal/render/` — ASCII card rendering
