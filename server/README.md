# Žolíky backend (v1 scaffold)

## Prerequisites
- MongoDB via Docker Compose (recommended)

## Run locally
From `server/`:
- `cp .env.example .env` (adjust secrets as needed)
- `docker compose up --build`

## Endpoints
- `GET /healthz`
- WebSocket: `ws://localhost:8080/ws/games/:id?token=<JWT>`
- REST:
  - Auth: `/auth/guest`, `/auth/register`, `/auth/login`, `/auth/refresh`, `/auth/logout`
  - Games: `/games`, `/games/:id`, `/games/:id/join`, `/games/:id/start`, `/games/:id/add-ai`, `/games/:id/replay`
  - Offline scoring: `/scoring-sessions/*`

