# zolik

A card-game server that does not know which card game it is running, and two
clients that do not either.

Four games are hosted today — Žolíky (Continental rummy), Prší (Czech Mau-Mau),
Canasta and No-Limit Texas Hold'em — and they share one runtime, one wire
protocol, one screen and one terminal client. Adding a fifth is a server-only
change: register a module and it appears in the lobby, gets an opponent, gets a
scoreboard, and is playable in a browser and over SSH without either client
being edited.

## Layout

- `server/` — the runtime and the game modules (REST, WebSocket, MongoDB,
  embedded SSH host)
- `client-react-native/` — Expo GUI client for web/iOS/Android
- `client-tui/` — text-only terminal client (Bubbletea + SSH)
- `e2e/` — Playwright specs that drive the real server and the real web build

```sh
./scripts/dev-stack.sh up     # server + web client, on ports beside your dev ones
./scripts/dev-stack.sh test   # every suite: Go, terminal client, RN, e2e
```

See [`docs/testing-this-branch.md`](docs/testing-this-branch.md) for what to
look at, and each directory's README for setup details.

## How a game is added

A game implements `module.GameModule` (`server/internal/module/protocol.go`):
it deals a match, applies an action, describes the board as *zones* a viewer
may see, and lists the *offers* that viewer may take. It is registered in
`internal/app/app.go`, and that is the whole integration.

Everything else belongs to the runtime and is written once: persistence, the
socket, turn order, bots, reconnection, statistics and both user interfaces.

Drag-and-drop comes with it, and is the sharpest test of the idea: an offer
already says which cards it takes and where they land, so the places a held
card may be dropped are a filter over the offer list rather than anything a
client knows. Say where a move lands and it can be dragged.

The one rule the design enforces:

> Every rule-derived fact is computed once, on the server, inside the module —
> and shipped. A client that has to *derive* a rule is a bug.

## Documents

- [`docs/architecture.md`](docs/architecture.md) — the original review, and
  what had to change
- [`docs/extensibility-plan.md`](docs/extensibility-plan.md) — phases 0–4: the
  offer protocol, the module contract, and the second game that falsified it
- [`docs/canasta-plan.md`](docs/canasta-plan.md) — the third module, and why a
  second rummy was not a configuration problem
- [`docs/one-architecture-plan.md`](docs/one-architecture-plan.md) — phases
  5–8: poker, bots for every game, one client shell, and the deletion of the
  Žolíky-specific path
