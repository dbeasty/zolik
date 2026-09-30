# Solitaire (Klondike) — implementation plan

- **Deliverable:** `server/internal/klondike`, implementing `module.GameModule` like the seven
  games before it; the small protocol and shell additions it needs, all game-agnostic; and a
  one-seat path through the lobby, presence and stats, which no game has needed until now.
- **Also:** a deal that never leaves the server and cannot be guessed or chosen (§4a), and
  replay, both of a finished game and of the same deal played again (§4b).
- **Non-goals, this plan:** FreeCell, Spider, Pyramid; a timer or time bonus; leaderboards for
  solitaire; offline play; a "winnable deals only" option. Each is noted in §9 as a follow-up.

"Solitaire" here means **Klondike**, the game people mean when they say the word. The module id
is `klondike` and its label is "Solitaire", so that a later FreeCell or Spider is a sibling
module (sharing a `internal/patience` helper package if one earns its place). It is not a
variation of this module. Those games share a deck and nothing else, and a variation that is a
different game on the table is what Canasta's Samba taught us to avoid.

---

## 0. Why this game is different from every one before it

Every module so far seats at least two players; even blackjack declares `MinPlayers: 2`. Klondike
is the first game with **one seat, no opponent and no bot**. That makes it the next honest test of
the claim the runtime makes: that it hosts a game without knowing what the game is. The audit
behind this plan says the claim mostly holds:

| Question | Answer today |
|---|---|
| Can a one-seat table be created and started? | Yes. `startLocked` only checks `< min` (`match/manager.go:480`); join and add-bot correctly return `MATCH_FULL`. |
| Does the bot loop cope with no bots? | Yes. `firstBot` returns "" and the goroutine exits (`match/bots.go:333`). |
| Does the contract test accept it? | **No.** `allmodules_test.go:156` requires `MinPlayers >= 2`, and `:576` fails a finished match with no winner. |
| Does presence cope? | It works but lies. Every closed solitaire session is suspended, then after two minutes it is **abandoned** and counted in `MatchesAbandoned` (`match/reaper.go:74-94,188`). |
| Do stats cope? | They record it, but the result misleads. Solo wins feed the **Overall** leaderboard (`stats/repository.go:262-352`), and average rank is always 1. |
| Can the protocol say "three hidden cards under these face-up ones"? | **No.** `Group.Cards` is `[]string`. `Zone.Count` backs render on the *wrong* end (`ZoneView.tsx:167-193`). |
| Can the client move cards from one table column to another? | **No.** Drags start only from the viewer's hand (`HandZone.tsx:832`, `[matchId].tsx:541-571`). |
| Can the player learn or choose the deal? | **Yes, three ways:** a time-based seed (`match/manager.go:358`), device sync of the full state (`sync/auth.go:201-219`), and offline hand-up with a seed the phone chose (`match/offline.go`). See §4a. |
| Can a finished game be watched back? | Mostly. `match/replay.go` rebuilds it from the seed and the move log on the server; Klondike adds an open view and chapters. See §4b. |
| Do the lobbies offer a zero-bot game? | **No.** Both RN (`app/lobby/games.tsx:625-631`) and the TUI (`client-tui/ui/lobby.go:263-280`) clamp the bot count to at least 1. |

So the work splits cleanly into four parts: one-seat plumbing with a server-only deal, the
module itself, replay, and table interaction in the shell. Each can land as its own PR.

## 1. Rules implemented

Standard Klondike, one 52-card deck:

- **Deal:** seven tableau columns of 1–7 cards, with only the top card of each face up. The
  remaining 24 cards form the stock.
- **Stock:** turning it moves 1 or 3 cards to the waste (option). When the stock is empty, the
  waste is turned back over as a new stock, up to the redeal limit (option).
- **Tableau:** build down in alternating colours. Any face-up run may move as a unit. Only a King,
  or a run headed by one, may fill an empty column. The newly exposed face-down card turns up
  **automatically**. That is the modern convention, and it means no offer ever has to name a
  hidden card.
- **Foundations:** four, one per suit, built up from Ace to King. A card may come back from a
  foundation to the tableau (option; allowed by default, with a score penalty).
- **Won:** all 52 cards are on the foundations. **Lost:** the player gives up, or the game ends
  itself because no progress is possible (see §3).

### Options (house rules are options, not constants)

| Option | Choices | Default |
|---|---|---|
| `draw` | 1, 3 | 1 |
| `redeals` | unlimited, 2, 0 | unlimited |
| `scoring` | standard, Vegas, none | standard |
| `foundationTakeBack` | on, off | on |
| `autoFinish` | on, off | on |

`module.OptBotSkill` is **not** declared; there is nothing for it to configure.
`OptPauseBetweenRounds` is not declared either, since one deal is one match.

### Variations

- **Classic:** draw 1, unlimited redeals, standard scoring.
- **Draw Three:** draw 3, unlimited redeals, standard scoring.
- **Vegas:** draw 3, no redeals, Vegas scoring. The score starts at −52 and each foundation card
  is worth +5.

Standard scoring follows the familiar Windows table:

- waste to tableau: +5
- to a foundation: +10
- turning a card up: +5
- foundation back to tableau: −15
- recycling the waste: −100 when drawing one, −20 when drawing three

There is no time bonus. `Apply` is pure and has no clock, and adding one is a runtime change this
game does not justify.

## 2. How the board is drawn

| Area | Zone | Notes |
|---|---|---|
| Stock | `stack`, `Shared`, Count only | Tap to draw. |
| Waste | `pile`, `Shared` | Sends the top 1 (draw-1) or up to 3 (draw-3) cards, with `Count` = the whole pile. |
| Foundations | `spread`, `Shared`, 4 groups `f-S f-H f-D f-C` | An empty group is still drawn, because every offer's target must be (contract term 4). |
| Tableau | `spread`, `Shared`, 7 groups `t1…t7` | Cascades vertically, which `ZoneView` already does (`ZoneView.tsx:399-432`). |
| Seat | one `Seat`, always `Active` | Facts: score, moves, redeals left. |

The zones are marked `Shared` and are not owned by the seat. If they were owned, the layout would
title them "<player> (you)" (`BoardLayout.tsx:134`), and in any case the cards are "the table's".

## 3. What the protocol has to grow

These are predictions, as in the Rummy Tiles plan. Anything that turns up beyond them goes in the
outcome section.

| Need | Change |
|---|---|
| **Face-down cards under a group's face-up cards.** This is the tableau's only secret. | Add **`Group.Hidden int`**: the number of cards beneath `Cards` that are drawn back-up and never sent. It is the per-group counterpart of `Zone.Count` and works the same way: a secret stays secret because it is not sent. `CardView.FaceDown` stays ceremonial and is not reused, because its documented meaning is "not a secret" and reusing it would weaken that. |
| **A fanned waste in draw-3.** | Add **`Zone.Fan int`** on a `pile`: how many top cards to show overlapped instead of folding to one. It is presentational: a client that ignores it shows the top card and loses nothing it needs to play. |

Nothing else changes. Moving a run between columns is `Source{Zone: from_meld, MeldID: "t3",
Submit: [run…]}` → `Target{Zone: meld, MeldID: "t5"}`, which is the shape Rummy Tiles added and the
RN client never consumed (§5).

### Verbs and offers

| Verb | Source → Target | Notes |
|---|---|---|
| `draw` | stock → waste | One-tap. Disabled with a remedy pointing at `recycle` when the stock is empty. |
| `recycle` | waste → stock | Disabled once the redeal limit is spent; the refusal cites the written redeal rule. |
| `move` | waste / `t*` / `f-*` → `t*` / `f-*` | One offer per legal (source run, target) pair. At most about 90 exist, and most deals have under 15. Each has a distinct `LabelKey` and destination `Fact`, so offers shown together can be told apart (contract term 3). |
| `autofinish` | — | Offered when the stock and waste are empty and every card is face up. |
| `undo` | — | Flagged `Undo`. It may never take back a turn-up, because that would let the player peek at the card and hide it again. Undo is one-way past a reveal, the same rule "an undo is not progress for a bot" asked for. |
| `giveup` | — | Always enabled. Ends the match as lost. |

**Never end in a dead position.** When no `move` is legal, the stock and waste are empty, and no
redeal is left, the module ends the match itself as lost. With unlimited redeals the same happens
after a full pass through the stock with no other move since the last recycle; otherwise the only
offer left would be an endless `recycle`. This also bounds the conformance driver.

**`Finished`** returns `done, [self]` on a win and `done, []` on a loss. That is the "nobody won"
case blackjack already established. `Ranked` sets `Won` itself, because `RankByScores` would hand a
lone player rank 1 and `Won: true` on a lost game (`module/protocol.go:678-716`).

## 4. One-seat plumbing (server)

1. **Contract test.**
   - Relax `MinPlayers >= 2` to `>= 1`.
   - Add a `solo` flag to the `hosted` table that allows an empty winner list in
     `NamesItsWinners`.
   - Add `klondike` to `hosted` with `players: refs("p1")`.
   - `ExplainsItsRefusals` still requires a `NOT_YOUR_TURN` explanation. Klondike emits it for a
     non-seated actor and indexes it to a written rule.
2. **Presence.** A table with one human seat is a *saved game*, not an abandoned one. It is
   suspended when the socket drops and resumed when the player returns. The reaper skips it, so it
   neither increments `MatchesAbandoned` nor loses the game. Old saved games need a retention
   bound: use the existing stranded-table sweep with a long window (e.g. 30 days) instead of
   15 minutes.
3. **Stats.**
   - A match whose module has `MaxPlayers == 1` updates `ByModule`, the streak and
     `RecentMatches`.
   - It does **not** update `Overall` or the leaderboard.
   - Record the solitaire facts that matter: games won, win rate, best score and fewest moves.
     These go in `ByModule` facts, not in a new schema.

## 4a. The deal stays on the server

Solitaire is played against the deck, so the deck is the only opponent there is. A player who
knows the deal, or chooses it, has beaten the game before the first move. Every card is dealt on
the server today (`NewMatch` in `match/manager.go:489`). The deal still gets out, or can be chosen,
in three ways, and a two-player game never noticed because the other seat was the defence.

| Leak | Where | Fix |
|---|---|---|
| **Predictable seed.** It is `time.Now().UnixNano()`. The creation time is roughly known and the seven face-up cards are a checksum, so searching a nanosecond window recovers the whole deal. | `match/manager.go:358` | Draw every new match's seed from `crypto/rand`, not only solitaire's. There is no reason for any game's shuffle to be guessable. |
| **Device sync.** A seated player's device may pull the match namespace, which holds the full `State` (every face-down card) and the `Seed`. | `sync/auth.go:201-219` | Refuse pull and push of the match namespace to non-server identities for a server-dealt module. The home never moves to a device. |
| **Offline hand-up.** A phone may host the match and hand the finished game up. The importer checks every move but **accepts the seed the phone chose**, so a phone could deal itself easy games. | `match/offline.go:75-100,165-230` | The importer refuses bundles of a server-dealt module, and the mobile client does not offer the game offline. |

The switch is one optional interface, `module.ServerDealt` (a method returning `true`). It is
declared by the module, so sync, offline and import ask the registry, not a module id, and a
future FreeCell or Spider opts in the same way. Solitaire therefore needs a connection. That is
the honest cost, and it is stated in the lobby.

Tests:
- A device identity is refused the namespace of a `klondike` match.
- A handed-up `klondike` bundle is refused, with its reason kept.
- Two matches created in the same millisecond deal differently.

## 4b. Replay

Two different things, and both are wanted.

**Watching a finished game.** This comes almost free. `match/replay.go` rebuilds any match by
dealing it again from the seed on the server and folding the move log, so the seed never has to
leave the server. What Klondike adds:
- **`OpenViewer`**, so a *completed* game replays with every face-down card shown. `openable`
  already limits this to completed matches (`replay.go:332`).
- **Chapters** at each recycle of the waste, via `chaptersOf`, so a long game can be skipped
  through.
- An **abandoned or given-up game is still completed** and still replayable. That is one more
  reason §4 makes "give up" a real ending, not a timeout.

While a game is in play, its replay goes through the ordinary `View`: rewinding your own game
never shows a card you have not turned up.

**Playing the same deal again.** "Play this deal again" and "send this deal to a friend" create a
new match that **copies the seed on the server** from a finished match the requester sat at, or
was sent. Neither the seed nor a deal number is ever shown.
- Add `Create(..., dealFrom: matchID)`. It is allowed only when the source is completed and the
  requester may see it.
- The new match records `dealFrom`. Stats mark it as a repeat, so a best score on a deal you have
  already seen open is not presented as a first attempt.
- Two players who get the same deal can compare scores and move counts on the finished-match
  screen. That is the only "multiplayer" solitaire needs, and it needs no seat sharing.

Tests:
- A replay of a completed game shows the whole tableau.
- A replay of a live game hides exactly what the live view hides.
- A `dealFrom` match deals card for card the same as its source.
- `dealFrom` a live match, or one the requester cannot see, is refused.

## 5. What the client shell has to grow

This is generic, as before: **no file in the shell learns the word "foundation"**.

1. **`Group.Hidden`.** Draw N backs at the top of a cascade, using the stacked overlap. This needs
   a stacked face-down path in `CardView.tsx:314`, which today ignores `stacked`.
2. **Pick up from a table group.**
   - A card in a group becomes draggable when a live offer's `source.meldId` names that group and
     its `submit` starts at that card.
   - The drag carries the rest of `submit` with it, which is the run.
   - Drop targets reuse `dropSpotsFor` unchanged. Drops on empty groups have to register.
   - This widens `beginDrag` beyond `myHands`, which is the same widening the Rummy Tiles plan
     §3.5 described and never shipped in RN.
3. **Tap a table card to play it.** If exactly one enabled offer's source starts at the tapped
   card, take it. If there are several, light up their targets using the existing
   `pendingGroupKey` path. That path currently lights only `meldId` targets, so it must be
   extended to `zoneId` ones too.
4. **Tap a pile or stack whose offer does not land in the viewer's hand.** Widen `sourceSpotsFor`
   (`drops.ts:370-394`) so tapping the stock draws.
5. **`Zone.Fan`.** Show an overlapped fan of the top N cards of a pile.
6. **Seven columns on a phone.**
   - A `Shared` spread with many groups gets its own full-width line and never wraps.
   - At 375 px, seven card-width columns do not fit at today's size. The layout, not the skin,
     picks a narrower column overlap or card scale, because skins must not change sizes.
   - **Measure it in the browser** at 375 px and in the desktop pane; don't infer it from the code.
7. **Lobby.**
   - When `maxPlayers == 1`, `games.tsx` shows a single **Play** button: create, start, no bots.
     It hides the bot-count picker and "Open table".
   - `lobby/table.tsx` hides "Add bot" for a full table.
   - A server-dealt game is marked online-only. The mobile client does not offer it offline, and
     says why instead of failing at the first move.
   - The finished-match screen gets **Play this deal again** and **Send this deal** (§4b), and
     shows the other players' results on the same deal.
   - The TUI lobby stops clamping the bot count to at least 1.
8. **TUI.** No new rendering. It plays through one-tap offers, since every `move` carries
   `submit` and a target, and prints `Hidden` as a count.

## 6. Files

`server/internal/klondike/`, mirroring blackjack:

| File | Contents |
|---|---|
| `cards.go` | Card codes and colour/rank helpers. |
| `state.go` | Tableau with a face-down split per column, foundations, stock, waste, score, redeals, undo history (which stops at reveals), and the pass-without-progress marker. |
| `table.go` | `tableOf`: options → table, resolved in one place for `NewMatch`, `Rules` and `ExplainRefusal`. |
| `engine.go` | `NewMatch`, `Apply`, `Finished`, auto-flip, dead-position end. |
| `offers.go`, `remedy.go` | `LegalActions`, `ruleIDsFor`, remedies. |
| `view.go` | `Descriptor`, `View` (the only place `Hidden` is computed), `Standings`. |
| `rules.go`, `zones.go` | Written rules, including rules that are *off*; zone and group ids. |

**Elsewhere:**
- `module/protocol.go`: `Group.Hidden`, `Zone.Fan`.
- `app/app.go:584` and `cmd/dump-keys/main.go:33-37`: registry and scan dirs.
- `allmodules_test.go`: the `hosted` entry and the `solo` flag.
- `match/presence.go` and `reaper.go`, `stats/aggregate.go` and `repository.go`.
- `module/protocol.go` (or a new `module/dealt.go`): the optional `ServerDealt` interface.
- `match/manager.go:358`: the seed from `crypto/rand`; `Create` gains `dealFrom`.
- `sync/auth.go`: refuse device access to a server-dealt match's namespace.
- `match/offline.go`: the importer refuses server-dealt bundles.
- `match/replay.go`: chapters for Klondike come from the module; no replay logic learns the game.
- `stats/`: the `dealFrom` repeat mark and same-deal comparison.
- `client-react-native/src/api/matchTypes.ts`, `lib/board.ts`, `lib/drops.ts`,
  `components/CardView.tsx`, `components/match/ZoneView.tsx`, `app/match/[matchId].tsx`,
  `app/lobby/games.tsx`, `app/lobby/table.tsx`, `lib/gameOrder.ts`.
- Regenerate `serverKeys.json` and add wording in every locale bundle. Keys must be static
  literals, including params maps.

## 7. Tests

| Level | What it proves | Where |
|---|---|---|
| Unit | Every clause: alternating build, King-only empties, run moves, auto-flip, both draw counts, redeal limits, each scoring scheme and its penalties, foundation take-back on and off, autofinish. | `klondike/engine_test.go` |
| Unit | **Secrecy:** no `View` and no offer ever names a face-down card, across many seeds and at every step. Undo never re-hides a revealed card. | `klondike/view_test.go` |
| Unit | Offers never disagree with `Apply`; every refusal's rule id resolves for every option value. | `klondike/agreement_test.go`, `ruleindex_test.go` |
| Unit | Whole games played from offers alone across three variations. Every match terminates, with no infinite recycle, and 52 cards are conserved at every step. | `klondike/conformance_test.go` |
| Contract | Every hosted-module term, with the `solo` relaxations and nothing else. | `module/allmodules_test.go` |
| Unit | A one-seat table survives a dropped socket and the reaper; a lost game is recorded without touching the leaderboard. | `match/presence_test.go`, `stats/*_test.go` |
| Unit | **Server-only deal:** a device identity is refused a `klondike` namespace; a handed-up `klondike` bundle is refused with its reason kept; two matches created in the same millisecond deal differently. | `sync/auth_test.go`, `match/offline_test.go`, `match/manager_test.go` |
| Unit | **Replay:** a completed game replays with the whole tableau open; a live game's replay hides exactly what its view hides; a `dealFrom` match deals card for card like its source; `dealFrom` a live or unseen match is refused. | `match/replay_test.go`, `klondike/view_test.go` |
| Client | `Group.Hidden` backs, a run drag, tap-to-play, the stock tap, the fan, and the one-button lobby. | jest beside each lib |
| E2E | Real HTTP and sockets: the lobby's Play button starts a game with no bot; a drag of a three-card run lands; the stock deals; a hidden card's code never reaches the socket; giving up ends the match as lost; the finished game replays open; "Play this deal again" deals
the same tableau. **Assert what the user sees**, not the socket alone. | `e2e/tests/klondike.spec.ts` |

## 8. Sequencing

1. **PR 1: one-seat plumbing and a server-only deal** (§4, §4a, plus the lobby part of §5.7).
   Mergeable on its own. The `crypto/rand` seed is the one change current games see, and it is a
   fix for them too.
2. **PR 2: protocol and module** (§3, §6 server). After this PR the game is playable end to end
   through the generic shell's buttons and the TUI. The cascade looks wrong until PR 4.
3. **PR 3: replay** (§4b). Adds `OpenViewer` and chapters, plus `dealFrom` with its
   "Play this deal again" and "Send this deal" buttons.
4. **PR 4: shell interaction** (§5.1–5.6). This is where the game becomes pleasant. It is verified
   in the browser at phone and desktop widths.
5. **PR 5: release.** A VERSION bump, then deploy.

## 9. Open questions and follow-ups

- **The seed on the device.** This is resolved by §4a: the deal never leaves the server. That
  also removes the blocker to a solitaire leaderboard, which can be a follow-up (best score per
  deal, first attempts only).
- **Winnable deals only.** This needs a solver that is fast enough to run at deal time. It is a
  follow-up, and it is also the natural basis for a `hint` offer.
- **FreeCell and Spider.** Separate modules. Spider needs `Group.Hidden` too, and FreeCell needs
  nothing new, so the protocol cost is paid once here.
