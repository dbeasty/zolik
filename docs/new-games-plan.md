# New games: Poker (Five-Card Draw, Omaha), Hearts, Spades, Schnapsen

Order of work: **Poker rename → Five-Card Draw → Omaha → Omaha Hi-Lo →
(trick-taking groundwork) → Hearts → Spades → Schnapsen.**
Each slice lands as its own PR. Every slice ships playable,
with Easy/Medium bots, written rules, refusal explanations, and all 25 locales.
A slice is not finished until it has been played in the browser.

---

## 0. Per-module checklist (applies to every new game)

Nothing in the client is per-game: lobby, tiles, table, trick area and rules
screen are all driven by the descriptor and zones. The hand-maintained
touch points are:

| Where | What |
|---|---|
| `server/internal/app/app.go:639` | add to `module.NewRegistry(...)` |
| `server/cmd/dump-keys/main.go:21-37` | registry **and** the `internal/<game>` source dir |
| `server/internal/gamemcp/games.go:19-33` | MCP registry |
| `server/cmd/botcost/main.go:90` | registry, then add an entry to `botgov/costs.go` if a decision costs > 1 ms |
| `server/cmd/repair-partnership-draws/main.go` | registry (partnership games especially) |
| `server/internal/match/addbot_test.go:162`, `replay_test.go:63-82` | registry and a replay row per variation |
| `server/internal/module/allmodules_test.go:61-159` | a `hosted{...}` row per variation |
| `server/cmd/gamebench` + `<game>/learn.go` | a `learn.Benchable` adapter, so the bot strength can be benched |
| `client-react-native/src/lib/gameOrder.ts` + test | popularity position |
| `client-react-native/src/lib/locales/*.ts` (25) | every key; `check-server-labels.js` `PROPER` set for the game name |
| `serverKeys.json` | `go run ./cmd/dump-keys > ../client-react-native/src/lib/serverKeys.json` |
| `gamemcp/messages_en.json` | `go generate ./internal/gamemcp` |
| `e2e/tests/<game>.spec.ts` | new spec; also the id lists in `generic-shell.spec.ts:170,493`, `intro.spec.ts:16`, `untranslated-sweep.spec.ts:170-194` (Mariáš is missing from these three as well; add it in the same pass) |
| `docs/<game>-plan.md`, `README.md:6-11`, `cmd/game-mcp/README.md` | README game counts are already stale |

Module files, using the ginrummy/marias layout: `descriptor.go state.go engine.go
offers.go view.go rules.go ruleindex.go remedy.go rounds.go events.go
bot.go learn.go`. The tests are `conformance_test engine_test view_test bot_test`
(including `TestBotDoesNotPeek`), `ruleindex_test` (`module.RuleIndexCheck`) and
`events_test`.

Standing rules from earlier work:
- Message keys, and their params maps, must be static literals. Rule text is
  branched with literal `Fact`s, never `Sprintf`.
- House rules are declared options that default to the standard rule, never
  constants.
- Refusals point at rule IDs in the module's own `Rules()`.
- No turn may end in a dead position.
- A bot must never read hidden cards. A golden test pins each bot's moves.

---

## 1. Poker: one game, the poker games as its variations

### Decision (David, 2026-10-07): one "Poker" tile
The Hold'em module becomes **Poker**. Its variations are the poker games: Texas
Hold'em, Five-Card Draw, Pot-Limit Omaha and Omaha Hi-Lo.
- **The module ID stays `holdem`.** It is stored on every match, stat row and
  shipped model (`learn/models/holdem.bin`) and in the MCP tools; only the label
  a player reads changes. Stats already record the variation, so results per
  poker game stay separable.
- **How a match ends is a table option, not a variation.** The old `freezeout`
  and `timed` variations differed only in `OptHandLimit`, which is already an
  option.
- **Betting structure becomes an option too** (`betting`: no-limit, pot-limit,
  and later fixed-limit). Each variation sets a default: Hold'em NL, Draw NL,
  Omaha PL.
- **The trained network plays Hold'em only.** `Bot()` checks the ruleset and
  hands Draw and Omaha to the rule bot, because the encoder reads exactly 2 hole
  cards (`learn.go:237-255`) and would silently drop the rest. An `omaha` or
  `draw` learn game can come later.

**Implementation:** copy Canasta's `ruleset` pattern (`canasta/ruleset.go`).
- `holdem` gets `ruleset{holeCards, board, mustUseHole, draws, betting, hiLo}`,
  stored on `GameState` and read via `s.rules()`.
- Old states, which have no ruleset, resolve to Hold'em.
- Every rule difference reads the ruleset, never the variation ID.

### P0 — Rename to Poker (done on `claude/poker-rename`)
- **Descriptor:** the label is now "Poker", with one variation:
  `holdem` / "Texas Hold'em", whose default hand limit is 0 (until one seat is
  left).
- **`module.VariationSpec.Formerly`:** retired IDs a variation still answers to,
  never sent on the wire. `descriptor.Variation(id)` resolves them, so
  `Manager.Create`, rematch, `/modules/{id}/rules` and the MCP all accept an old
  `freezeout`/`timed` match.
- `resolveVariation` keeps both legacy IDs with their original defaults. A match
  stored as `timed` therefore still plays 10 hands, and Hold'em's bot goldens are
  unchanged.
- **Lobby:** with a single variation the picker hides itself (`setup.tsx:189`).
  A saved setup naming `timed` falls back to the first variation and keeps its
  saved options.
- **Locales:** none needed. "Poker" joins `PROPER` in
  `scripts/check-server-labels.js` ("Texas Hold'em" is already there). The
  `variation.holdem.freezeout/timed` keys stay so stored "my games" rows still
  read.
- **Tests:** `TestRetiredVariationsStillDeal`,
  `TestVariationAnswersToItsFormerIDs`, and a new `holdem` conformance case. The
  generic-shell e2e now expects no variation picker and a hand-count option.

### P1 — Five-Card Draw (done on `claude/five-card-draw`)
- **Ruleset (`holdem/ruleset.go`):**
  - 5 hole cards, no board, one draw of up to 3, at most 6 seats. Six players
    hold 30 cards and draw at most 18, which fits a 52-card deck with no
    reshuffle rule. For the same reason there is no "four when keeping an ace"
    option.
  - The variation narrows `MaxPlayers` the way Samba does.
  - Every rule difference reads `s.rules()`, derived from the stored variation,
    never the variation ID.
- **Streets:** `predraw`, then `draw`, then `postdraw`, then the showdown.
  - The draw starts left of the button and visits every seat still in the hand
    once. All-in seats draw too.
  - The verbs are `discard` (1–3 cards, a composite hand selection) and `stand`.
    Stand is listed first, so a bot or retry with no opinion stands pat.
  - A bet during the draw is `DRAW_PENDING`, a discard outside it is
    `DRAW_NOT_NOW`. The discard is bounded by `DRAW_TOO_MANY`, `DRAW_EMPTY`, and
    the shared `CARD_NOT_IN_HAND`, which also catches a card named twice.
- **What is public:** the count. Each seat carries `Drawn`/`Drew` and shows
  "Drew N" or "Stood pat". The `drew` event carries the count only. `Discarded`
  is kept on the seat for that seat's own bot and is never sent.
- **Evaluator and showdown:** unchanged. Every `Best(hole++board)` now goes
  through `s.handOf(seat)`.
- **View:** there is no board zone. The header names Draw's street as its own
  literal fact ("Before the draw" / "The draw" / "After the draw"), because the
  key manifest only finds keys in a `LabelKey`.
  - Correction to the earlier note: opponents' hands are not drawn as zones on
    the match screen at all, so `MAX_BACKS` needed no change.
- **Rules:** `holdem.rules.draw.{deal,streets,draw,public}`, stated only at a
  Draw table, with the refusals mapped to them.
- **Bots (`holdem/drawbot.go`):**
  - **Draws:** the textbook — keep anything made, draw 1 to a four-flush or an
    open-ended straight, else keep the pair or the two highest. Easy keeps a
    kicker with a pair.
  - **Bets:** equity by rollout over the unseen cards (own hand and discards
    excluded), with this seat's and each opponent's draw played out.
  - **Hard** also reads the table. Half of each opponent's range is dealt
    consistent with its draw count and, after the draw, with a bet: a bet
    claims at least two pair.
  - **Measured (300 heads-up matches):** Hard beats Medium by +5.6 big blinds
    a match. On the fixed-seed ladder test, Medium beats Easy, and all three
    beat a calling station.
  - **Cost:** at most about 5 ms per Hard decision at six seats.
  - The trained Hold'em network refuses Draw positions (`errNotHoldem`), so
    every AI seat falls back to this rule bot. The style opponents (maniac,
    rock, …) do the same through `holdemOnly`.
- **Tests:**
  - `draw_test.go`: deal, turn order, discard, refusals, all-in draws, offers,
    the textbook, bot legality at 2–6 seats, no peeking at any street or skill,
    and rules text.
  - A `holdem/draw` row in `allmodules_test` and `replay_test`.
  - `e2e/tests/draw.spec.ts`: a person swaps two cards in the browser.
  - A Five-Card Draw row in the one-shell e2e.
  - Hold'em goldens unchanged.

### O1 — Pot-Limit Omaha (PLO) as a variation
**Engine**
- Deal `ruleset.holeCards` (4) at `engine.go:105`. Nine seats need 9×4+5 = 41
  cards, which fits.
- **Evaluator:** `BestOmaha(hole, board)`, the best of exactly 2 hole + 3 board
  cards (6×10 = 60 five-card sets through `evaluateFive`). Route every
  `Best(hole++board)` call through one `s.bestHand(hole)`:
  - `engine.go:539, 589, 640`
  - `history.go:173`
  - `learn.go:425`

  The `applyShow` guard at `:589` becomes "board ≥ 3".
- **Pot-limit:** the max raise-to is `CurrentBet + pot + Σ live bets + toCall`,
  capped at `seat.Bet + seat.Stack`. Blinds count as live bets.
  - Put it in one function used by `applyRaise` (`engine.go:291`), `raiseRange`
    (`offers.go:144`), `raiseQuickChoices` (`offers.go:172`; the choices are ½
    pot and pot, and all-in only when it is within the cap) and `remedy.go:39`.
  - New refusal `ErrOverPotLimit`, mapped to a new `holdem.rules.potLimit`. Don't
    reuse `ErrNotEnoughChips`.
  - The existing `potIfCalled` is not the pot-limit pot with 3+ players. Write a
    separate function and leave `potIfCalled` for the No-Limit quick choices.
- **Rules:** literal-key branches for the 4-card deal, "exactly two from your
  hand and three from the board", pot-limit betting, and the end section.
  `holdem.rules.noLimit` moves from "end" to "betting" for both games. Fix
  `configOf` (`remedy.go:103`) so it carries `OptShowdownReveal` at the same
  time; it is a known bug.
- `weakAt` / `readShown` (`history.go:122,162`): replace the `hole[0]==hole[1]`
  pocket-pair test with a ruleset-aware "made hand from the hole" test.

**Fast evaluator (bots only)**
- `score.go` ORs hole and board into one set, so it cannot express 2+3. Add
  `scoreOmaha`: precompute the 10 board triples once per runout, then score each
  of the 6 hole pairs against each triple as an exact 5-card set.
- Pin it to `BestOmaha` with a twin of `TestScoreAgreesWithBest`.
- Measure with `botcost`. A rollout costs about 60× a Hold'em one, so cut the
  trial counts (`bot.go:955`) until a Hard decision is within the existing
  governor cost class.

**Rule bot (Easy/Medium/Hard)**
Most of the current bot is Hold'em-specific: Chen, the 169-class push/fold
charts, 2-card equity and outs.
- **Preflop:** a generated 4-card strength table (equity against a random
  hand, over suit-isomorphic classes), written by a `cmd/omahachart` generator
  the way `cmd/pushfold` writes its charts. Thresholds are tuned for 2-9 seats.
  The table is a coarse proxy for later tuning, and a Hutchison-style formula is
  the fallback if generating the table is too slow.
- **No push/fold** under pot-limit. Gate `pushFold` (`bot.go:320`) off with
  `ruleset.betting != noLimit`.
- **Postflop:** `equityBoth` with 4 cards per opponent and `scoreOmaha`, plus
  `drawOuts` rewritten for Omaha (a flush needs 2 hole cards of the suit, and
  nut awareness matters more). Bet sizing against the pot-limit cap: the
  "shove" helper becomes "pot".
- **Styles:** the `rock` and `jamCaller` bars read Chen. Switch them to the new
  strength score; the hand-agnostic styles carry over unchanged.
- The Hard network comes later (an `omaha` learn game). Until then an AI seat
  plays the Hard rule bot. That fallback already happens in `match/bots.go:455`
  for modules with no model.

**Client:** no code change. The raise slider reads min and max from the offer.
Look at the 9-seat layout on phone width.

**Tests:** extend `cards_test`, `score_test` and `engine_test` with
"must use two" cases (the board's four-flush with one heart in hand does not
make a flush; a board straight is unusable), the pot-limit cap with side pots,
conformance, and a new golden file for the Omaha bot. Hold'em's own golden file
must not change: that is the regression guard for the ruleset refactor.

### O2 — Omaha Hi-Lo (8-or-better) as a variation
- Low evaluator: 5 distinct ranks ≤ 8, aces low, 2 hole + 3 board, wheel counts
  for both halves.
- `distributePots`: for each side-pot level, split hi/lo when a qualifying low
  exists. Quarter the halves, scoop when one hand wins both, and give the odd
  chip to the high half.
- `PotResult` gains a `Low *...` half with its own `LabelKey` and `Cards`. Add
  view status keys `holdem.status.potHiLo` and `potScoop`. `RoundResults`
  already shows the result rows, so check how it looks rather than assuming.
- Bots: low draws in the strength table and in equity, which means scoring
  scoops and halves.
- Optional later: 5-card PLO as a `holeCards: 5` variation.

---

## 2. Trick-taking groundwork (T0, before Hearts)

What exists: `tricks.Legal`/`Winning`, and an exact double-dummy solver
(`tricks/search`) that handles 4 seats, 64 cards and partnerships, with a
`TrickValue` hook. Mariáš's PIMC (`marias/sampling.go`) is a good pattern but is
bound to Mariáš.

- **`tricks`:**
  - Add the 52-card order `"AKQJT98765432"` and a `Deck52()` helper.
  - Add a `LeadFilter`, the game's restriction on which cards may lead a trick.
    It is passed into `Legal` and mirrored in the bitmask move generator in
    `search.go:285-326`.
  - The filter covers hearts-broken, spades-broken and no-points-on-the-first-
    trick.
  - Tests mirror `reference_test` (fast and reference must agree node for node)
    with 4 seats, which no current test uses.
- **`tricks/pimc` (new, generic):** the Mariáš pattern made generic.
  - `Knowledge`: seen cards, voids learned from failing to follow, passed cards
    known.
  - `Sample(knowledge, rng)` returns deals consistent with that knowledge.
  - `Choose(problemFor, samples, nodeLimit)` returns averaged card values.
  - Node-counted budgets keep replays deterministic.
  - Mariáš stays on its own code; porting it is optional, later, and behind its
    golden test.
- **Measure a full 52-card double-dummy solve** with 4 seats before relying on
  it. The expectation is that Hard uses PIMC only in the last N tricks (start at
  ~7 and tune it from `botcost`), with heuristics before that.

---

## 3. Hearts

**Standard rules:**
- 4 players and 52 cards; each player is dealt 13.
- Before play, each player passes 3 cards. The direction cycles left, right,
  across, then no pass.
- The 2♣ leads the first trick. You must follow suit if you can.
- No point cards may be played on the first trick.
- Hearts may not be led until they are broken.
- Each heart is worth 1 point and the Q♠ 13.
- Shooting the moon gives every opponent 26 points.
- The game ends when someone reaches 100, and the low score wins.

**Options** (house rules, defaulting to the standard rules):
- target score (50/100)
- passing (on/off)
- moon scoring: add 26 to the others, or subtract 26 from yourself
- J♦ = −10 (Omnibus)
- points allowed on the first trick
- Q♠ breaks hearts

A later variation could support 3 or 5-6 players with a reduced deck. v1 is
4 players only.

**Engine:**
- Phases: pass, then play, then hand end.
- Pass offer: hand selection with `MinCards = MaxCards = 3` (the protocol
  already supports this, `protocol.go:433-453`). Passes are simultaneous: a
  pass is hidden until all four are in, then every player sees what they
  received.
- Play: `tricks.Legal` with `FollowOnly` and the Hearts `LeadFilter`.
- **Dead-position check:** a player holding only hearts before hearts are
  broken may lead one. A player whose whole hand is point cards on the first
  trick may play one. Both are covered by a test.
- `Rounds` gives per-hand points plus running totals; there is a moon-shot
  badge.

**View:** the hand, and the trick zone as `ZoneSpread, Shared, ArrangeBySeat`
(4 seats draw as a cross with no client work). A received-cards zone is shown
during the first trick so the player sees what was passed to them. Seat labels
show running scores, and "hearts broken" is a status line.

**Bots:**
- **Easy:** a legal card that avoids taking points, with some randomness.
- **Medium:** pass the Q♠, K♠ and A♠ and high hearts, and make voids. Duck,
  dump the Q♠ when void, lead low, and track whether the Q♠ is out.
- **Hard:** PIMC in the last N tricks with a paranoid model (the seat to move
  against the other three; `search` is two-sided). The `TrickValue` is points
  taken. Detect moon shots and block them: if one opponent holds every point
  card so far, switch to stopping them. Passing and early tricks stay heuristic.

**Tests:**
- conformance and a golden bot file
- `TestBotDoesNotPeek`, including cards passed to the bot
- the passing direction cycles
- shooting the moon scores under both moon options
- the first-trick rule with every option combination

---

## 4. Spades

**Standard rules:**
- 4 players in two fixed partnerships (opposite seats).
- Spades are always trump, and spades may not be led until broken.
- Each player bids 0-13; a team's contract is the sum of its two bids.
- Making the contract scores 10 × bid, plus 1 per overtrick ("bag").
- Every 10 bags costs 100 points.
- Failing the contract scores −10 × bid.
- Nil (bid 0): +100 if the bidder takes no tricks, −100 otherwise. The partner's
  tricks don't count toward the nil.
- The game is to 500.

**Options:**
- target (250/500)
- blind nil (on/off, only when the team is behind by ≥ 100)
- bag penalty on/off
- minimum team bid
- partner nil tricks count toward the contract or not

v1 is partnership only. A Cutthroat (individual) variation can come later.

**Engine:**
- Phases: bid (clockwise, a `ParamKindChoice` 0-13 plus Nil) and play.
- Implement `module.Seated.Sides` the way `canasta/engine.go:98` does.
- `FollowOnly` with trump ♠ and a spades-broken `LeadFilter`. The dead-position
  case: a player holding only spades may lead them.
- Team scoring and bags go in `Rounds`, with one row per team.

**View:** the trick cross. Seat labels show each seat's bid and tricks taken
("3/4"). Team totals and bags are a status. Because the client has no team
colouring, carry the partnership in seat `LabelKeys` ("partner") rather than
building a new client concept in v1.

**Bots:**
- **Bidding (Medium):** count sure tricks (A♠, K♠ with a guard, side aces,
  ruffing chances from short suits). Bid nil on a hand with no high cards and
  low spades.
- **Bidding (Hard):** average the double-dummy trick count over ~20 sampled
  deals, discounted. The solve is two-sided, which is exactly Spades. Watch the
  cost: the auction is a full 13-card solve.
- **Play:** Medium uses heuristics (cover partner, avoid bags when set, protect a
  partner's nil). Hard uses PIMC with the true two-sided model and a
  `TrickValue` that weighs the contract, bags and nil.

**Tests:**
- bag rollover
- nil and blind-nil scoring
- set scoring
- the partner-nil option
- the spades-broken refusal and its remedy text
- a partnership replay row in `replay_test`

---

## 5. Schnapsen (with Sixty-six as a variation)

**Standard rules (Austrian Schnapsen):**
- **Players and deck:** 2 players and 20 cards (A T K Q J in four suits).
  Card points are A 11, T 10, K 4, Q 3, J 2.
- **Deal:** each player is dealt 5. The next card is turned up as trump, and the
  rest form the stock.
- **While the stock is open:**
  - You need not follow suit.
  - Draw after each trick, winner first.
  - Marriages: K+Q of a suit are worth 20, or 40 in trumps. Announce one when
    leading.
  - The trump jack may be exchanged for the face-up trump.
  - A player may **close** the stock.
- **After the stock is closed or runs out,** you must follow suit and must win
  the trick if you can, else trump. This is `tricks.FollowBeatTrump`, already
  built for Mariáš.
- **Winning a deal:** the first player to claim 66 wins it. They score 1, 2 or
  3 game points depending on the opponent's points, and the closing rules
  penalise a failed close.
- **The match:** the first to 7 game points wins the bummerl.

**Variation: Sixty-six.** 24 cards (adds the 9s) and 6-card hands. The scoring
differs slightly.

**Options:**
- deck art: German (default for the Czech/Austrian audience; the deck
  presentation already exists) or French
- the last trick is worth 10 (Sixty-six) or not
- the match target

**Engine:**
- Phases: lead, which can carry marriage, exchange, close or claim ("I have
  66"), and follow.
- Card points and the marriage reuse Mariáš's shape: `search.Problem.Pairs`
  already models the marriage bonus.
- A false claim of 66 loses the deal. Make that option-free and state it in
  `Rules()`.
- **Dead-position check:** claiming and closing must never strand the next
  player without a move.

**Bots:**
- **Medium:** count points, take the face-up trump with the trump jack, meld
  marriages early, and close when the bot holds trump control and is ≥ 50 points.
- **Hard:** once the stock is closed or empty, all 10 cards are known (the
  unseen cards are the opponent's), so solve **exactly** with `tricks/search` and
  no sampling. While the stock is open, use PIMC over the stock order and the
  opponent's hand. Deciding whether to close is part of the search: compare
  "close now" against "play on" over the samples.
- Hard should be near-optimal here. Check it with a gamebench run against
  Medium.

**Tests:**
- the closing rules (a failed close scores against the closer)
- the 66 claim, true and false
- the jack exchange is offered only while the stock has more than 2 cards
- the follow rules switch on close and on exhaustion
- the German deck renders
- a golden endgame solve

---

## 6. Sequencing and estimates

| Slice | Main risk | Rough size |
|---|---|---|
| P0 Poker rename | old match IDs and rematch | small (done) |
| P1 Five-Card Draw | the draw phase, and a new bot without Hold'em's charts | medium (done) |
| O1 PLO | evaluator and bot cost; the ruleset refactor must leave Hold'em goldens identical | large: about Blackjack-sized, plus bot work |
| O2 Hi-Lo | splitting pots per side-pot level, and two-half result views | medium |
| T0 trick groundwork | 52-card, 4-seat solve cost | small to medium |
| Hearts | simultaneous pass, the moon model in a two-sided search | medium |
| Spades | partnership presentation, Hard bid cost | medium |
| Schnapsen | the closing and claim rules state machine | medium; the bot is the easy part |

After each slice:
- run `botcost`
- update `botgov/costs.go`
- run a Playwright spec
- play a hand in the browser at desktop and phone width

## 7. Open questions for David
1. ~~Omaha as its own tile?~~ Decided: one Poker tile with the poker games
   as variations.
2. Hi-Lo in the first Omaha PR, or as a follow-up? The plan assumes a follow-up.
3. Schnapsen default deck: German (recommended) or French?
4. Should Hearts and Spades get trained networks in the learn-core track
   (`trained-bots-learn-core`), or stay rule and PIMC bots for now?
