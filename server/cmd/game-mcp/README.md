# game-mcp

An MCP server (and a command line) that lets Claude, or any MCP client, **play,
inspect and coach** games on the real zolik rules engine: against the
heuristic bots at any strength, or against a trained network, with a coach that
scores every legal move with a model.

Every move goes through the module's own `Apply`; every legal move comes from
the module's offers (through the learn adapter's `Candidates` for Žolíky,
Canasta and Hold'em, so composite melds and pile-pickup-plus-opening moves are
listed whole); every board a seat is shown is the module's own `View` for that
seat, the one place hidden information is filtered. Nothing in
`internal/gamemcp` knows a rule of any game.

All eight games the server hosts can be played. The three with a learn adapter
(`zolik`, `canasta`, `holdem`) also take `net:` opponents and `coach` scores.

## Setup

**Claude Code (MCP).** `.mcp.json` at the repo root registers the server as
`zolik-games`; it runs `go run -C server ./cmd/game-mcp`, so the first start
compiles for a few seconds. Approve the project server when Claude Code asks. The
`ZOLIK_LEARNED_MODEL_<GAME>` variables are passed through when set; `coach` uses
them as its default model. For a faster start, build once and point `command`
at the binary:

```sh
cd server && go build -o ~/bin/game-mcp ./cmd/game-mcp   # then "command": "/Users/you/bin/game-mcp", "args": []
```

**Any shell (no MCP).** The same tools, from separate commands:

```sh
cd server && go build -o /tmp/game-mcp ./cmd/game-mcp
export GAME_MCP_STATE=/tmp/zolik-game.json    # where tables live between calls
/tmp/game-mcp call new_table '{"game":"zolik","seats":2,"claude_seats":[0],"opponents":["hard"],"seed":7}'
/tmp/game-mcp call play '{"table":"t1","seat":0,"move":3}'
/tmp/game-mcp call coach '{"table":"t1","seat":0,"model":"/path/to/final.bin"}'
```

`call` loads the tables from the state file, runs one tool, writes them back and
prints the result's text; `-json` prints the whole reply. Each call is its own
process, so this works from separate Bash tool calls with nothing left running,
and a crashed call leaves the previous file intact. This is the recipe for a
Claude Code session that has not loaded the MCP server.

For batch use, `game-mcp cli` reads one JSON command per line and writes one
JSON reply per line, keeping tables for the whole run; `--script FILE` reads the
commands from a file (blank lines and `#` comments are skipped):

```sh
printf '%s\n' '{"id":1,"tool":"new_table","args":{"game":"holdem","seats":3,"claude_seats":[0]}}' \
              '{"id":2,"tool":"play","args":{"table":"t1","seat":0,"move":1}}' | /tmp/game-mcp cli
# {"id":1,"tool":"new_table","ok":true,"result":{...},"text":"..."}
```

## Seats and opponents

Seats are 0-based. `claude_seats` are the seats you drive with `play`; every
other seat is a bot:

| spec | plays |
| --- | --- |
| `easy`, `medium`, `hard` | the hand-written heuristic at that skill |
| a style (Hold'em: see `list_games`) | an extra heuristic opponent |
| `net:<path>[@temperature]` | a trained model; temperature 0 (its favourite move, every time) unless given |

`opponents` is one spec per bot seat in seat order, or a single spec for all of
them; the default is `hard`. Bots move automatically until one of your seats has
a decision. A seat of yours with exactly one legal move (a draw from the stock
when nothing on the pile is worth taking, the ready-up between deals) plays it
automatically and the log says so; pass `"auto_play": false` to be asked
anyway. The same seed and seating deal and play the same match.

## Tools

| tool | input | returns |
| --- | --- | --- |
| `list_games` | — | games, variations, options with their allowed values, seat ranges, accepted opponents, whether a model can play/coach it |
| `new_table` | `game`, `variation?`, `options?`, `seats`, `claude_seats`, `opponents?`, `seed?`, `auto_play?` | table id, seed, seats, the log of what the bots did first, and your first seat's observation |
| `observe` | `table`, `seat` | text rendering, `toAct`, `yourTurn`, numbered `moves`, and the structured `view` |
| `play` | `table`, `seat`, `move` (index) or `action` (explicit engine action) | what you played, the public log of everything since, and the new observation |
| `coach` | `table`, `seat`, `model?` | each listed move's probability under the model (temperature 1), the model's value estimate, and what the Hard heuristic would play |
| `replay_log` | `table`, `reveal?` | the public history; with `reveal` (finished matches only) every action as sent, every event unfiltered, each net bot decision's probabilities, and all hands |
| `rules` | `game`, `variation?`, `options?` | the written rules (`Rules()`), resolved for that table, in English |
| `close_table` | `table` | — |

Example observation (Žolíky, 2 seats, against a trained model):

```
Žolíky (Žolík Classic) · table t1 · you are seat 0 (claude)
Deal 1 · Round 3 · Deck 78 · Needs a joker-free run

Seats:
  seat 0 claude (you): TO ACT · Cards 13
  seat 1 bot1 (net:.../final.bin@0): Contract met · Cards 7

Table:
  Your hand (13): 10♦ 3♠ 10♠ 6♦ 7♠ 4♠ 7♦ 2♣ 9♥ 10♥ 6♥ 2♥ Q♥
    by suit: ♠ 3 4 7 10 | ♥ 2 6 9 10 Q | ♦ 6 7 10 | ♣ 2
  bot1: Their hand: 7 cards
  Stock: 78 cards
  Discard pile (4, bottom to top): 8♠ K♠ Q♠ A♠ — top A♠
  Melds: none
  bot1: Melds:
    [meld_1] run: 2♦ 3♦ 4♦ (Clean run)
    [meld_2] run: J♦ JK K♦

Your move. Legal moves (play one by its number):
  [0] draw from the stock
  [1] take K♠ from the pile (and the 2 cards above it), then lay meld Q♠ K♠ A♠
```

and `coach` on it:

```
Model .../final.bin (value estimate -0.024):
  [1] 100.0%  take K♠ from the pile (and the 2 cards above it), then lay meld Q♠ K♠ A♠
  [0]   0.0%  draw from the stock
Hard heuristic would play: draw from the stock (move [0])
```

`play` returns the log since your move, public information only:

```
Played: discard 9♥
Since then:
  claude discarded 9♥
  bot1 took 9♥ from the discard pile
  bot1 laid down 9♥ 9♣ 9♦
  bot1 added 5♠ to claude's meld
  bot1 discarded 4♠
  claude drew from the stock (only legal move, played automatically)
```

## What a seat can and cannot see

The observation is built from `module.View(state, seat)` and nothing else; the
tests redeal every hidden card (other hands, the stock) and require the text,
the moves and the view to be identical. The log between your moves is the
module's own narration of its events as a spectator receives them
(`ProjectEvent` drops the card a bot drew blind), and any card a narration names
that no spectator's board shows face up is written as "a face-down card" (Žolíky
narrates the card a player goes out on, which the rules lay face down). A game
that does not narrate (Hold'em) is logged from the action's verb and parameters
and its projected events. `coach` reads only what the seat may know: the
adapter's encoding, which each adapter's own no-peek test pins.

## Example prompts

- "Play a game of Žolíky against the hard model and explain your moves." (with
  `ZOLIK_LEARNED_MODEL_ZOLIK` set, use `net:$ZOLIK_LEARNED_MODEL_ZOLIK` as the
  opponent)
- "Read the Canasta rules, then play seat 0 of a 4-seat classic table with the
  hard bots; at every discard, ask coach first and tell me where you disagree."
- "Deal Žolíky with `initialMeldMinimum` 35 and a clean run required, you
  against `net:ml/runs/zolik-long/final.bin@0`. Play one deal and report every
  decision where the model and the Hard heuristic disagree."
- "Play 10 hands of Hold'em against the maniac style and summarise how it
  exploits a tight player."
- After a match: "replay_log with reveal, and list the model's decisions where
  it played a discard it gave less than 20%."

## Files

- `server/internal/gamemcp`: the tables, rendering, tools, MCP registration
  (github.com/modelcontextprotocol/go-sdk), the JSON-lines CLI and the state
  file.
- `server/internal/gamemcp/messages_en.json`: a copy of the client's English
  bundle, so message keys render as words. Regenerate with
  `go generate ./internal/gamemcp` when `client-react-native/src/lib/locales/en.ts`
  changes; `TestMessagesMatchClientBundle` fails until you do.
