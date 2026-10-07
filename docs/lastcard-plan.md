# Last Card — plan

A shedding game with coloured number cards and action cards, played to the
same mechanics as the well-known commercial game, but under our own name, our
own card design and our own wording. "Last Card" is the traditional folk name
for this family in Australia and New Zealand. The family descends from Crazy
Eights and Mau-Mau, as Prší does, so it is a generic name and not anyone's
brand.

Module id: `lastcard`. Display name: "Last Card", translated per locale through
`module.lastcard` (Czech: *Poslední karta*).

---

## 0. Legal guard-rails (read before writing any string or art)

Game mechanics are not protected by copyright. The brand is. These rules apply
to every file this work touches: server keys, `en.ts` and the other locales,
`Rules()`, docs, commit messages, store listing and art.

| Don't | Do |
|---|---|
| The word "UNO" anywhere: UI, rules, code identifiers, tests, store keywords, "UNO-like" in marketing | "Last Card", "shedding game", "Crazy Eights family" |
| Calling out "UNO!" | Calling out **"Last card!"** |
| Their card look: the white oval tilted across a full-colour face, a black border, their logo, the red back with the logo | Our own face design (§3) and our own back |
| Their trademarked variant names ("Flip", "Show 'Em No Mercy", "Attack") | Plain descriptive option names (§2.4) |
| Copying or closely paraphrasing their rule leaflet | `Rules()` written from scratch in our own Facts style |

Generic card names are fine: *Skip*, *Reverse*, *Draw Two*, *Wild*,
*Wild Draw Four*. Add a CI guard: a Go test (or a step in
`scripts/check-server-labels.js`) fails if `\buno\b` (case-insensitive) appears
in `server/internal/lastcard/**`, any locale file, or `serverKeys.json`.

Before the first public release, have an IP lawyer look over the final name and
card art. It costs little and is the last line of defence.

---

## 1. The deck: 108 cards

| Kind | Per colour | Total |
|---|---|---|
| 0 | 1 | 4 |
| 1–9 | 2 each | 72 |
| Skip | 2 | 8 |
| Reverse | 2 | 8 |
| Draw Two | 2 | 8 |
| Wild | — | 4 |
| Wild Draw Four | — | 4 |

**Card codes.** These follow the Rummy Tiles precedent: a hyphen, so a code can
never be mistaken for a French `"7H"`. Duplicates share a code, as Canasta's
double deck already does.

```
<colour>-<face>      colour ∈ R B G Y     face ∈ 0..9 | S (skip) | V (reverse) | D (draw two)
W                    wild
W4                   wild draw four
```

Examples: `"R-7"`, `"B-S"`, `"G-V"`, `"Y-D"`, `"W"`, `"W4"`. The colour named on
a wild lives in state (`DeclaredColour`) and in `Action.Params["colour"]`,
exactly like Prší's `DeclaredSuit` and `Params["suit"]`.

`module.DeckLastCard = "lastcard"` sits next to `DeckGerman` in
`module/descriptor.go`. Unlike `german`, a client that ignores it cannot draw the
cards, so §3 is part of v1 and not an optional extra.

---

## 2. Rules

### 2.1 Core (v1, default)

- 2–10 players, 7 cards each. The rest is the draw pile; turn up one card to
  start the discard pile.
- **Opening card:**
  - Wild Draw Four: bury it in the pile and turn up another.
  - Wild: the first player names the colour.
  - Skip, Reverse or Draw Two: it takes effect against the first player.
- **On your turn,** play one card that matches the top card's colour, number or
  symbol, or play a Wild. A Wild Draw Four may only be played when you hold no
  card of the current colour (but see the challenge rule in §2.2).
- **If you don't play,** draw one card. If it is playable, you may play it
  immediately; otherwise your turn passes. This means `draw` followed by
  `play_card` (that card only) or `pass`. It is the same shape as Prší's
  `OpenDiscard`-free draw flow.
- **Effects:**
  - Skip: the next player loses their turn.
  - Reverse: flips the direction of play. With two players it acts as a Skip.
  - Draw Two: the next player draws 2 and loses their turn.
  - Wild: name the next colour.
  - Wild Draw Four: name the colour; the next player draws 4 and loses their turn.
- **Going out:** the first player to empty their hand wins the deal. If the last
  card is a Draw card, its effect still applies; this matters for scoring.
- **Empty draw pile:** reshuffle everything except the top discard. Reuse
  Prší's `drawOne`.

**State changes compared with Prší.**
- `Direction int` (+1 or −1), and a `nextPlayer(from, steps)` that respects it.
- `DeclaredColour`.
- `DrawnCard string`: the card just drawn that may still be played this turn.

### 2.2 Calling "Last card!" and the Draw Four challenge (v2)

Both of these interrupt the strict turn order, so they arrive after the core
game is stable.

- **"Last card!" call** (option `lastCardCall`: `off` | `on`, default `on` once
  shipped).
  - Play the second-to-last card with `Params["announce"]=true`. The client
    shows this as a toggle on the play button, rendered from a `ParamSpec`.
  - If you don't announce, any opponent gets a `catch` offer. It stays open until
    the next player acts. A successful catch makes you draw 2.
  - Bots always announce. They catch at Medium and above.
- **Draw Four challenge** (option `drawFourChallenge`: `off` | `on`, default `on`).
  - When `on`, the engine lets you play a Wild Draw Four at any time, which
    allows bluffing.
  - The victim gets a short phase with two offers: `challenge` or `accept`.
    - If the challenge succeeds (the player held the current colour), that
      player draws 4 instead.
    - If it fails, the challenger draws 6.
    - The `View` reveals the challenged hand to the challenger only, in an event.
  - When `off`, the engine refuses a Wild Draw Four while you hold the current
    colour, with a WhyNot that points at the rule.

### 2.3 Scoring across deals (v2)

- The winner of a deal scores the cards left in everyone else's hands:
  - Numbers: face value.
  - Skip, Reverse, Draw Two: 20 each.
  - Wild, Wild Draw Four: 50 each.
- Option `targetScore`: `single deal` | `200` | `500`.
  - Default `500`, which matches the familiar game.
  - `single deal` plays like Prší, with no `Rounded`.
- Implement `module.Rounded`, `RoundLog`/`ScoreLine` and `Intermission`. Copy
  the pattern from ginrummy.

### 2.4 House-rule options (v3)

Per house-rules-as-options, every option defaults to the standard rule:

- `stacking`: `off` | `drawTwoOnDrawTwo` | `anyDraw`. A Draw card may be passed
  on by playing another, and the pile accumulates. Prší's `PendingDraw` already
  does this, so reuse it.
- `drawUntilPlayable`: `off` | `on`.
- `sevenZero`: `off` | `on`. Playing a 7 swaps hands with a chosen player;
  playing a 0 passes every hand in the direction of play.
- `handSize`: 5 | 7 | 10.
- Shared options: `module.BotSkillOption()`, `HintsOption()`.

"Jump-in" (playing an identical card out of turn) is left out. It needs
real-time races, and the turn engine doesn't have them.

---

## 3. Card design: original, not a look-alike

The goal is a deck that reads as instantly as the familiar one but that nobody
would mistake for it.

**Principles**
- **Light card stock with a coloured frame**, not a full-colour face with a
  tilted oval. The skin owns the stock colour, as it does for the French and
  German decks.
- **Every colour also has a shape.** This sets the deck apart and works for
  colour-blind players. Each colour's shape appears in the corners and behind
  the central figure:

  | Code | Colour | Shape |
  |---|---|---|
  | R | Red (coral-red, not their red) | ● circle |
  | B | Blue | ◆ diamond |
  | G | Green | ▲ triangle |
  | Y | Gold | ■ square |

- **Faces:**
  - Number: a large central numeral in the colour, on a soft tinted version of
    the shape. Small numeral and shape in two corners. The 6 and 9 are
    underlined.
  - Skip: our own glyph, an arrow hopping over a dot. Not the circle-slash.
  - Reverse: a single U-turn arrow. Not two curved arrows chasing each other.
  - Draw Two: a bold "+2" with two small fanned card outlines.
  - Wild: a 2×2 grid of the four shapes in their four colours, on a dark
    neutral frame (the skin's ink).
  - Wild Draw Four: the same four-shape grid with a bold "+4".
- **Back:** our own pattern, a repeat of the four shapes in one muted tone, with
  a small "Last Card" wordmark. Not red, no oval, no black field.
- **Fixed colours:** the four card colours are fixed, like the German suit
  colours (`GermanSuit.tsx:14-17`). Skins change only the stock, ink, bevel and
  shadow (skins must not change sizes).

**How it's made.** Like `germanArt.ts`, the art is path data in code: the shapes
are simple geometry, and the action glyphs are a handful of hand-drawn paths.
No images and no vendored assets, so there is nothing to license. Generate a
contact sheet of all 54 distinct faces at three sizes, check it in light and
dark and on every skin, then iterate.

---

## 4. Server work

New package `server/internal/lastcard/`. It mirrors `prsi/` file for file:

| File | Contents |
|---|---|
| `state.go` | codes, `colourOf`/`faceOf`, `GameState` (+ `Direction`, `DeclaredColour`, `DrawnCard`, later `Phase`, `Scores`), error codes, `playable()` |
| `engine.go` | `buildDeck` (108), `shuffle`, `NewMatch` (opening-card rules), `Apply` (`play_card`, `draw`, `pass`; later `catch`, `challenge`, `accept`), `drawOne`, `Finished` |
| `offers.go` | `LegalActions` by probing `Apply`, as in Prší; a colour `ParamSpec` only when a wild is playable |
| `view.go` | `Descriptor()` (`Deck: module.DeckLastCard`, variation `classic`), `View`/`OpenView`, `Standings`, `Bot()`. Header Facts show the current colour and the direction, because the shell may not contain game words. |
| `rules.go`, `ruleindex.go`, `remedy.go` | written rules; every refusal points at a stated rule id |
| `bot.go` | heuristic bot with skill profiles (§5) |
| tests | `lastcard_test.go` (every rule, including reverse with 2 players, the opening card, reshuffle, a Draw card as the last card), `conformance_test.go`, `ruleindex_test.go`, `standings_test.go`, `bot_test.go` (`TestBotDoesNotPeek`, `TestBotLadderIsOrdered`), and the no-"uno" guard |

**Wiring checklist.** The registry has no auto-discovery, so each of these is a
manual edit:
1. `internal/app/app.go` registry
2. `internal/gamemcp/games.go` registry, plus `gamemcp/render.go` and
   `table.go` text for the new codes ("red 7", "wild draw four")
3. `cmd/botcost/main.go`, then rerun botcost and add the result to
   `botgov/costs.go` if the bot costs more than about 1 ms per decision
4. `cmd/dump-keys/main.go`, then regenerate `serverKeys.json`
5. `cmd/repair-partnership-draws/main.go`
6. Test registries:
   - `module/allmodules_test.go`
   - `match/replay_test.go` (×2)
   - `match/addbot_test.go`
   - `match/bots_test.go`
7. Regenerate `gamemcp/messages_en.json` with `go generate ./internal/gamemcp`

---

## 5. Bot

A heuristic first, in the same shape as Prší's `bot.go`. It chooses only among
legal offers and never reads hidden cards.

| Skill | Behaviour |
|---|---|
| Easy | Plays the first playable card (sorted), names a random colour it holds, never challenges. |
| Medium | Plays numbers before actions and keeps its wilds. Names the colour it holds most of. Plays Draw Two, Skip or Reverse when the next player is low. Announces and catches. |
| Hard | Medium, plus: keeps a Wild Draw Four as an escape and changes colour away from what an opponent is short of (from public draw history). Challenges based on the victim's hand size and public play history. Bluffs a Wild Draw Four only when the opponent is at 1–2 cards. |

The ladder test must show Easy < Medium < Hard. A bench-only `learn.go`, like
Mariáš's, can come later. A trained network is out of scope.

---

## 6. Client work (`client-react-native`)

1. `CardDeck` in `src/api/matchTypes.ts` becomes `'german' | 'lastcard'`.
2. `src/lib/cards.ts`:
   - `isLastCardCode`, `parseLastCard` (colour, face, isWild).
   - `lastCardText` for sentences ("Red 7", "Wild Draw Four").
   - Route `cardText` through them when the deck is `lastcard`.
3. Art and faces:
   - `src/components/cards/lastCardArt.ts`: the shapes and glyph path data.
   - `LastCardFace.tsx`: full face, stacked corner, plain index.
   - A `deck === 'lastcard'` branch first in `CardView.tsx`'s face selection.
4. Deck-aware paths in `CardIndex.tsx`, `CardGlance.tsx` and `src/lib/hand.ts`
   (auto-arrange by colour, then by face).
5. Colour picker: the generic `ParamSpec` picker shows four swatches with their
   shapes. The labels come from server keys.
6. `src/lib/gameOrder.ts`: add `lastcard`, and update `gameOrder.test.ts`.
7. Strings:
   - `en.ts` plus the other 23 locales: `module.lastcard`,
     `variation.lastcard.classic`, `lastcard.rules.*`, `lastcard.remedy.*`,
     `lastcard.unit.*`, colours, card names, `err.*`, option labels.
   - `shell.test.ts` must stay green: no game words in shell files.
8. TUI: `client-tui/internal/render/cards.go` draws a coloured `[R7]` / `[W+4]`.

---

## 7. Delivery phases

| Phase | Scope | Done when |
|---|---|---|
| **1 — Playable deal** | §1, §2.1, server module and wiring, heuristic bot, card faces (§3), client parsing and labels, en + all locales, no-"uno" guard | A human plus 3 bots finish a deal in the browser on every skin; conformance and allmodules pass; contact sheet reviewed |
| **2 — The real game** | §2.2 call/catch and challenge, §2.3 scoring to 500 with intermissions | Full match to 500 in the browser; challenge reveal shown only to the challenger |
| **3 — Table options and polish** | §2.4 house rules, Hard bot, `e2e/tests/lastcard.spec.ts` plus id lists in `intro`/`untranslated-sweep`/`generic-shell`, `docs/lastcard-rules.md`, bench `learn.go` | Ladder test ordered; e2e green against the private stack |

---

## 8. Decisions to confirm

1. **Colour palette.** The four classic colours with our own hues and shapes
   (recommended), or a clearly different palette such as coral, teal, violet and
   amber for extra distance.
2. **Default ruleset.** The official rules by default, with stacking and the
   rest as opt-in house rules (recommended). The other choice is to turn
   stacking on by default, since many people play that way.
3. **Default length.** To 500 (recommended once phase 2 lands); phase 1 ships
   single-deal only.
