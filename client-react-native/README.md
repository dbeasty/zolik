# Žolíky — React Native client (Expo)

Mobile and desktop GUI client for the Žolíky server. Uses the same REST + WebSocket API as [client-tui](../client-tui/).

## Prerequisites

- Node.js 20+
- [Expo CLI](https://docs.expo.dev/) (`npx expo`)
- Running backend: from `server/`, `docker compose up` (default `http://127.0.0.1:8090`)
- Note: The Expo web dev server runs on port 8091 (port 8090 is occupied by Docker)

## Setup

```bash
cd client-react-native
cp .env.example .env
# Edit EXPO_PUBLIC_ZOLIK_BASE_URL for your platform (see below)
npm install
EXPO_PUBLIC_ZOLIK_BASE_URL=http://localhost:8090 npx expo start --web
```

The Expo web dev server will run on port 8091. Press `w` to open in browser, `i` for iOS Simulator, `a` for Android Emulator, or scan the QR code for Expo Go on a physical device.

### API URL by platform

| Platform | `EXPO_PUBLIC_ZOLIK_BASE_URL` |
|----------|------------------------------|
| iOS Simulator | `http://127.0.0.1:8090` |
| Android Emulator | `http://10.0.2.2:8090` |
| Physical device | `http://<your-computer-lan-ip>:8090` |

Ensure the server binds on `0.0.0.0` or your LAN interface when testing on a phone.

## Features

- Guest, register, and login (tokens stored in SecureStore)
- Create / join lobby, add AI, start game (4+ players)
- Live game: draw, meld, lay-off, discard, accept/decline offers
- Round and game end screens
- Offline scoring sessions (create, patch rounds, export)
- Stats (registered users) and public leaderboard

## Manual smoke test

1. Start server: `cd server && docker compose up`
2. Guest login → New game → Add AI ×3 → Start
3. Play: draw from deck, lay meld if legal, discard
4. Register → Sign in → Stats & leaderboard
5. Offline score table → new session → save a round → export

## Project layout

- `app/` — Expo Router screens
- `src/api/` — HTTP client and types
- `src/hooks/useGameSocket.ts` — WebSocket + `game_state`
- `src/components/` — cards, hand, melds, actions
- `src/context/` — session and game-flow state

## Production builds

For App Store / Play Store builds, use [EAS Build](https://docs.expo.dev/build/introduction/) and point `EXPO_PUBLIC_ZOLIK_BASE_URL` at your HTTPS API (`wss` for WebSocket).
