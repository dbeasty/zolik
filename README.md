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
./scripts/dev-stack.sh up     # Docker server (:8090) + web client (:8114)
./scripts/dev-stack.sh test   # every suite: Go, terminal client, RN, e2e
```

See [`docs/testing-this-branch.md`](docs/testing-this-branch.md) for what to
look at, and each directory's README for setup details.

## Deploy (play.limidus.com)

Production runs the KDB single-container stack on **limi-mini** (`192.168.13.13`)
behind nginx at **https://play.limidus.com/** — same Docker shape as local dev,
with the Expo web client exported as a static bundle. Requires the **kdb**
repo as a sibling checkout (`../kdb`) and SSH access as `davja@192.168.13.13`:

```sh
./scripts/deploy.sh
```

The script bootstraps a `zolik` user on the server, rsyncs source, builds the
web client locally, runs `docker compose -f docker-compose.kdb.yml`, and
installs the nginx vhost.

`APP_ENV=production` is the deployed setting and turns off the SSH terminal
client, its admit-any-key mode, and both development hatches. It also makes
`SMTP_*` **required** — the server refuses to start without a mail host rather
than swallowing sign-in codes — so set those in the server `.env` before
deploying. The script checks for that combination before it builds anything,
because the alternative is a container that fatals on startup and is restarted
forever. Guest-only play with no mail server is `APP_ENV=local`.

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

## Versioning

Every running piece — the server and both clients — reports a build like
`1.1.1.2+7feb025`: a number plus the short commit it was built from, so
"is the fix in?" is answerable by looking at the screen instead of reading
logs.

The `VERSION` file at the repo root holds the three-part number (`1.1.1`),
bumped by hand for a real release. The fourth part is generated automatically
— commits since `VERSION` itself last changed, so a bump resets it to `0`. A
build made from an uncommitted tree gets `-dirty` appended to its hash.
`scripts/version.sh` is the one place this is computed; `dev-stack.sh`, the
RN npm scripts and the Docker build all read it from there so the number and
hash can never disagree between pieces for any reason other than actually
being different builds.

The server exposes its build at `GET /version`; the RN client shows both
halves in a footer on the main menu, and the terminal client shows its own in
the header (naming the server's too, only when it differs — see
`client-tui/ui/menu.go`).

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

## Licence

Licensed under the [GNU Affero General Public License v3.0](LICENSE).
Copyright © 2026 Limidus Corp — see [COPYRIGHT](COPYRIGHT).

The server links [kdb](https://github.com/dbeasty/kdb) directly
(`internal/db/kdb.go` imports its embed, storage, codec, schema, auth, server
and document packages, statically linked into the binary), and kdb is
AGPL-3.0. The combined work is therefore distributed under the same terms —
which also matches how zolik is used: served over a network at
play.limidus.com, the situation AGPL section 13 addresses.
