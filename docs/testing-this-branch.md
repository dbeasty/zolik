# Testing this branch

Everything below is on `worktree-canasta`, merged with `main`.

## The short version

```sh
./scripts/dev-stack.sh up      # Mongo/Redis if needed, server, web client
./scripts/dev-stack.sh test    # every suite: Go, terminal client, RN, e2e
./scripts/dev-stack.sh down    # stop
./scripts/dev-stack.sh logs    # tail the server
```

`up` runs on **:8096** (API) and **:8114** (web), against Mongo on **:27018** and
Redis db **2** — deliberately beside the usual dev ports, so it does not disturb
a server you already have running. Override with `ZOLIK_DEV_PORT`,
`ZOLIK_DEV_WEB_PORT`, `ZOLIK_DEV_MONGO_URI`, `ZOLIK_DEV_REDIS_URL`,
`ZOLIK_DEV_MONGO_DB`.

Then open **http://127.0.0.1:8114** and press **Play**.

## What to look at, in about five minutes

**One picker, four games.** *Play* → the list is rendered from `GET /modules`.
Žolíky, Prší, Canasta and Texas Hold'em are all there with their own variations
and options; nothing in the client names any of them.

**One screen plays all four.** Pick any game → *Play against bots*. The same
screen renders a rummy table, a shedding pile, a canasta partnership's melds and
a poker board — laid out from zones, seats and offers the server pushes.

- Every control is one the server offered. Disabled ones stay on screen with the
  engine's own reason next to them.
- Hold'em shows a **stepper** on *raise*, between the minimum legal raise and
  your stack. No card game shows one, because no card game declares a numeric
  input.
- The bots move on their own — that is the runtime driving them from the same
  offer list, not per-game AI.

**Playing a card by dragging it.** Pick a card up and the places it may go
light up — the discard pile, a meld it extends, the table. Drop it on one and
the move is sent. Drop it on nothing and it goes back where it came from.

The whole of that is read off the offer list: an offer says which cards it
takes, where it lands, and — for a card that could go at either end of a run —
which ends are legal. So it works in every game, and a game added tomorrow gets
it without the client being touched. Two things worth trying, because they are
the ones that look like they must be hard-coded and are not:

- In Canasta, drag a card onto one meld of several. It joins that one. The
  spread and the meld inside it are both drop targets; the smaller wins.
- Anywhere, pick up a card the engine will not take right now. Nothing lights
  up, and letting go does nothing — the same refusal the greyed-out button
  gives, shown rather than stated.

Select several cards first and drag one of them, and the whole selection goes
together. Drop cards on a target that wants more than you are holding — a rummy
meld needs three — and they stay picked instead, until the offer's own button
has enough to light up.

**Rearranging your hand.** Drag a card along the fan and the cards part: a
card-sized dashed gap opens where it will land, and follows the pointer for the
length of the drag. Drop it in the empty space to the right of the last card to
put it at the end. Then wait for a bot to move — the arrangement stays put,
which is the part that needed building, since the server re-pushes the whole
board after every move and does so in its own order.

Worth trying in more than one game: no module knows the feature exists. How a
player likes their cards laid out is not a fact about the game, so it never
reaches the server — `GET /matches/{id}?as={you}` returns the same hand in the
same order before and after a drag.

Two decks are in play in Žolíky and Canasta, so a hand often holds the same card
twice. Tap one of a pair: it lights up and its twin does not, and both can be
picked for one meld. Selecting by card name could do neither, which is why
selection is by position now.

**The waiting room, which needs two browsers.** In window A press *Find
players*. In window B: *Play* → *Open a table* → window A's player appears in
the panel → *Invite* → window A jumps straight into the table with no code
exchanged. Then *Start*.

**Sign-in.** Guest, passwordless email (the code is printed to the server log —
`./scripts/dev-stack.sh logs`), and legacy username/password. Play a match as a
guest, then sign in: the guest history is claimed onto the new account.

**The terminal client.** It defaults to `:8090`, so point it at the stack:

```sh
cd client-tui && ZOLIK_BASE_URL=http://127.0.0.1:8096 go run ./cmd/play
```

The same four games, drawn as text: `↑/↓` chooses an offer, `1-9` picks cards
out of your hand, `←/→` adjusts a numeric input, `enter` plays.

## Running suites individually

```sh
cd server            && go test ./...      # 15 packages
cd client-tui        && go test ./...
cd client-react-native && npx tsc --noEmit && npm test
cd e2e               && npx playwright test        # needs the stack up
```

**Run Playwright from `e2e/`.** It resolves its config from the working
directory; invoked from the repo root it silently runs with none and fails with
`did not expect test.describe() to be called here`.

**Start the server as a built binary, not a backgrounded `go run`.** A reaped
`go run` child produces a wall of `ECONNREFUSED` that looks exactly like a code
regression. `dev-stack.sh` does this for you; it is the reason the script exists.

## Two environment traps worth knowing

**A dev server on the same Redis shares the waiting room.** That is by design —
one pool across instances — so if you have another server running on
`redis://…/0`, its waiting players show up in yours. The lobby tests were
asserting the pool held *exactly* one entry, which made them pass or fail on
what else happened to be running; that is fixed, but the sharing is real.

**Mongo lives on 27018 here**, not 27017 — 27017 on this machine belongs to a
different project.

## What changed that you may want to poke at

| | |
|---|---|
| `/games/*`, `ws://…/ws/games/:id`, `GameStateMsg` | gone — `curl -i localhost:8096/games/x` returns 404 |
| `/matches/*`, `ws://…/ws/matches/:id`, `match_state` | the only gameplay path |
| `POST /matches/:id/invite` | the waiting-room pickup, ported from `/games/:id/invite` |
| `GET /matches/:id/result` | a recorded result, for any game |
| `cmd/migrate-games` | one-shot `games` → `matches`; `-dry` counts without writing |

Statistics now work for every game, not just rummy — play a poker match to the
end and `GET /matches/{id}/result` returns a record with the same shape a rummy
one has.
