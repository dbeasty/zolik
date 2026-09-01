# Gin Rummy and Rummy Tiles — implementation plan

Fifth and sixth game modules for the runtime finished in [`one-architecture-plan.md`](./one-architecture-plan.md).

- **Baseline:** `main` @ `c09c37d`. The working tree carries in-flight AI-skill work
  (`docs/ai-skill-plan.md`, `module/skill.go`, the widened `Bot.Act(s, BotSeat, offers)`); both
  modules below are written against the *new* bot signature and declare
  `module.BotSkillOption()`, so this plan lands after that one rather than beside it.
- **Deliverable:** `server/internal/ginrummy` and `server/internal/rummytiles`, both implementing
  `module.GameModule`, registered alongside the four existing games, each with the full test
  pyramid ending in a browser-run e2e that plays a whole match over real HTTP + WebSocket + Mongo.

---

## 0. Two games, two entirely different risks

They are in one plan because they are one request, and separated into two phases because they
test opposite things.

| | Gin Rummy | Rummy Tiles |
|---|---|---|
| What it is | 52 cards, two players, draw-and-discard | 106 tiles, 2–4 players, table manipulation |
| Protocol changes needed | **none predicted** | two, named in §3.4 |
| Client changes needed | **none predicted** | a tile face, and selection from the table |
| Bot | a real one (offers decide nothing) | a solver (offers cannot be enumerated) |
| Risk | low — it is the routine case | high — it is the first game that is not made of cards |

Phase A is therefore also a measurement: if adding Gin Rummy touches nothing outside its own
package and one line of `app.go`, the claim `one-architecture-plan.md` closes on — *one way to
add a game* — is true for a game nobody had in mind when the seam was built. If it does not,
that is the more interesting result and it comes cheap.

Phase B is where the money is. Rummikub is the first candidate that breaks the assumption every
module so far has shared: that a move takes cards **out of a hand** and puts them somewhere. Its
whole character is moving things that are already on the table, and no existing offer can say so.

---

## 1. Naming

The tile game ships as **Rummy Tiles** — `id: "rummytiles"`, `Label: "Rummy Tiles"` — and the
trademarked name appears nowhere a player, a URL, a message key, an asset name or a database
document can see. It is used in this document and in package comments to say which rules are being
implemented, which is a citation, not a brand.

Gin Rummy is a generic name for a public-domain game and ships as itself: `id: "ginrummy"`,
`Label: "Gin Rummy"`.

---

# Phase A — Gin Rummy

## 2.1 Rules implemented

Standard two-handed Gin Rummy, the ruleset in Hoyle and on pagat.com.

**Deck and deal.** One 52-card deck, no jokers. Ten cards each. One card turned face up as the
first discard; the rest is the stock. Non-dealer may take the upcard, and if they decline, the
dealer may; if both decline, non-dealer draws from stock and normal play begins. The dealer
alternates each hand.

**Turn.** Draw one — from the stock or the top of the discard pile — then discard one. Exactly two
actions, always in that order. There is no melding during play: melds are declared only at the end
of the hand, which is the structural difference from every rummy already in this server.

**Melds.** Sets of 3–4 of a rank, runs of 3+ in a suit. A card belongs to at most one meld. Ace is
low; there is no round-the-corner run.

**Card values.** A 1 · 2–9 face · T J Q K 10.

**Deadwood** is the total value of the cards left over after the *best* arrangement of the hand
into melds. Best, not any: computing it is the engine's job and §2.2 turns on that.

**Knocking.** After drawing, a player whose deadwood would be ≤ 10 after discarding may knock,
laying the discard face down. **Gin** is knocking with zero deadwood. **Big gin** — eleven cards
melding with no discard — is a declared option, off by default.

**The lay-off.** After a knock that is not gin, the defender may lay their own deadwood onto the
knocker's melds, then the hands are compared. After gin, no lay-off.

**Scoring a hand.** Knocker's deadwood below defender's → the difference. Defender level or lower
→ an **undercut**: the defender scores the difference plus the undercut bonus (25). Gin scores the
defender's deadwood plus the gin bonus (25).

**Dead hand.** If the player to move faces a stock of two cards and does not knock, the hand is
dead: nobody scores, and the same dealer redeals.

**Match.** First to the target (100) after a hand ends. Then the line bonuses: 25 per hand won
("box"), and 100 for the game — doubled if the loser scored nothing (a shutout).

**Deliberately out of scope**, stated so it is a decision rather than an oversight: Hollywood
(three simultaneous games) and Round-the-Corner runs. Both are additive and neither changes a type.

## 2.2 The design decision that makes this game worth building: knock is *pressable*

`extensibility-plan.md` §1.1 records the offer-explosion limit, and `canasta-plan.md` §7 records
where it bites: a Žolíky lay-down needs a meld *shape* a human composes, so the offer protocol
declines to enumerate it and `module.PlayWithOffers` cannot go out at Žolíky to this day.

Gin Rummy does not have that problem, and for a reason worth writing down: **the best arrangement
of a ten-card hand is computable, so the player never chooses it.** A knock is one decision — which
card to discard — and the melds that follow are arithmetic. So `knock:<card>` and `gin:<card>` ship
as concrete, enumerated offers, one per legal discard, each carrying the resulting deadwood on
`ActionOffer.Facts` so the button reads "Knock — 7 deadwood" rather than asking the player to count.

That makes Gin Rummy the **second** module (after Canasta) that a driver knowing nothing about the
game can play to a *result*, which is the property the conformance test in §2.6 asserts.

The engine computes it with the standard exact search: partition into candidate melds, take the
minimum-deadwood cover. A ten-card hand has few enough candidate melds that exhaustive search over
compatible subsets is microseconds, and the plan does not pretend otherwise — `BenchmarkDeadwood`
guards it rather than a claim in a comment.

## 2.3 The lay-off phase, and whose turn it is

A knock hands the turn to the *defender* for a bounded sub-phase: lay off, or decline. This is the
first module where the seat on turn changes for a reason that is not "the previous player
finished", and it costs nothing — `Seat.Active` says so, `AwaitedSeats` reads it, and the runtime's
bot loop drives the defender's lay-off exactly like any other turn.

The alternative — resolving lay-offs automatically because the engine can compute them — is
rejected. A defender chooses *whether* to lay off (they may prefer not to reveal a holding when it
does not change the score), and folding a player's decision into the engine because it is usually
obvious is the same mistake as deriving a rule on the client.

## 2.4 Verbs and offers

| Offer ID | Verb | Input | Notes |
|---|---|---|---|
| `draw:stock` | `draw` | — | |
| `draw:discard` | `draw` | — | `LabelKey` distinguishes it from the above, per the contract test |
| `pass_upcard` | `pass` | — | first turn only |
| `discard:<card>` | `discard` | one card from hand | |
| `knock:<card>` | `knock` | one card from hand | `Facts`: resulting deadwood |
| `gin:<card>` | `knock` | one card from hand | separate id so a client can style it |
| `lay_off:<meldId>` | `lay_off` | eligible deadwood cards | defender only, post-knock |
| `finish_layoff` | `finish_layoff` | — | defender declines or is done |
| `continue` | `continue` | — | the shared between-rounds pause |

Every enabled/disabled decision is produced by **probing the real engine**, as `prsi/offers.go` and
`canasta/offers.go` do. The offer list is `Apply`'s own answer asked in advance, so it cannot drift.

## 2.5 Options and variations

Per the house rule that a house rule is a declared option and never a constant:

| Option | Choices | Default | Why an option |
|---|---|---|---|
| `targetScore` | 100 · 150 · 250 · 500 | 100 | the single most-varied house number |
| `knockLimit` | 10 · Oklahoma (the upcard sets it) | 10 | the one real ruleset fork |
| `bigGin` | off · on (+25) | off | commonly played, not universal |
| `lineBonuses` | off · on | on | box and game bonuses; off makes short matches readable |
| `module.PauseOption()` | | **on** | a hand ends in a settlement worth reading |
| `module.BotSkillOption()` | | module default `medium` | per `ai-skill-plan.md` §2.1 |

Variations: `standard` and `oklahoma`, differing only in the `knockLimit` default — a variation is
a named bundle of these, which is exactly what `VariationSpec.Defaults` is for.

## 2.6 The bot

Gin Rummy is the game where the offer list *decides nothing*: every legal discard is offered, and
which one to throw is the entire game. So `Botted`, not `OfferBot`, and it reads `BotSeat.Skill`:

- **draw** — take the upcard when it reduces deadwood or completes a meld; otherwise stock.
- **discard** — minimise own deadwood, then subtract a danger score for cards the opponent has
  shown interest in (upcards they took, ranks adjacent to them), from the module's own discard log.
- **knock** — knock when deadwood ≤ limit *and* the expected undercut risk is below a
  skill-dependent threshold. `easy` knocks the moment it may; `expert` holds for gin when the
  count and the stock justify it.
- **lay off** — always, when it lowers the count. There is no bluff to protect at that point.

Reproducible from `BotSeat.Seed` mixed with the turn, per `module.BotSeat`'s contract.

`internal/ai` is not reused: it is a Žolíky player behind a Žolíky interface. What *is* reused is
the shape — a scored decision, a skill dial, and a monotonic strength gate (§5.2).

## 2.7 Files

```
server/internal/ginrummy/
  cards.go      deck, card values, rank/suit helpers
  meld.go       candidate melds and the minimum-deadwood cover (the heart of it)
  state.go      GameState, phases, error codes, verbs
  engine.go     NewMatch, Apply and the seven verbs, hand rollover, match end
  scoring.go    knock/gin/undercut, line bonuses, match end
  offers.go     LegalActions, by probing the engine
  view.go       Descriptor, View, Seats, Standings, Rounds
  rules.go      Rules() and ExplainRefusal() — the written rules and the refusal index
  bot.go        the player described above
```

Registered in `internal/app/app.go`. Nothing else outside the package changes.

## 2.8 Tests

| Level | What it proves | Where |
|---|---|---|
| Unit — deadwood | the minimum cover is minimal, on hands with overlapping melds where a greedy pick is wrong | `meld_test.go` |
| Unit — scoring | knock, gin, undercut, big gin, line bonuses, shutout doubling | `scoring_test.go` |
| Unit — engine | the first-turn upcard dance, knock below/above the limit, dead hand at two cards, lay-off legality, dealer alternation | `engine_test.go` |
| Unit — agreement | over states × players × candidate actions, `LegalActions` and `Apply` never disagree in either direction | `agreement_test.go` |
| Unit — hidden info | `View` never shows the opponent's hand or the stock; after a knock it shows exactly what the rules make public | `view_test.go` |
| Unit — rule index | every code the package emits (`module.EmittedCodes`) resolves to a rule `Rules()` actually states, per the house rule that a refusal points at a written rule | `ruleindex_test.go` |
| Conformance | `PlayWithOffers` plays whole matches to a winner, across seeds, both variations — **including knocking**, which no Žolíky run can do | `conformance_test.go` |
| Shared contract | the eight terms in `module/allmodules_test.go`, which the new module is added to | existing file |
| Bench | `BenchmarkDeadwood`, `BenchmarkLegalActions` | `bench_test.go` |
| Bot strength | win rate rises monotonically along `module.Skills`, over N seeded matches | `bot_test.go` |
| E2E | two real players on two sockets play a match to a winner from the offer list alone; Mongo agrees; a viewer never receives the other hand | `e2e/tests/ginrummy.spec.ts` |

## 2.9 Exit criteria

1. `go test ./...` green in `server/`, the four existing modules included and unmodified.
2. `PlayWithOffers` finishes Gin Rummy matches — knocks and gins included — from offers alone, on
   every seed tried, under both variations.
3. `/modules` lists five games; the generic shell plays it in a browser with **no client change
   beyond i18n keys**, proven by the e2e spec. If that turns out to be false, the reason is
   recorded here as a finding, because it falsifies §7 of `one-architecture-plan.md`.
4. No file outside `internal/ginrummy` learns a Gin Rummy rule.

---

# Phase B — Rummy Tiles

## 3.1 Why this one is not a Canasta-shaped module

Everything the runtime hosts today shares an assumption nobody had to state, because until now it
was always true: **a move takes cards out of a hand.** Draw, meld, lay off, discard, bet, fold —
every verb in four games either adds to a hand or takes from one, and the table only ever grows.

Rummikub's whole character is the opposite. Once you are on the table, your turn is a *rearrangement*
of tiles that are already there, and the rule is not "is this move legal" but "is the table valid
when you let go". A player may break a run of five into two runs, steal its middle tile for a group,
take a joker out of a set by replacing it, and play one tile from hand — one turn, six moves, and
the only thing that matters is the state it ends in.

That is not a rule this codebase can add to an existing verb. It is a different shape of turn, and
§3.3 is how it fits.

## 3.2 Rules implemented

**Tiles.** 106: numbers 1–13 in four colours, two of each, plus two jokers.

**Deal.** 14 tiles each, **2–4 players** — `MinPlayers: 2, MaxPlayers: 4`, decided in §6.2. The
rest is the pool.

**Sets.** A *group* is 3–4 tiles of one number in distinct colours. A *run* is 3+ consecutive
numbers in one colour. 13 does not wrap to 1. A joker stands for any tile.

**The initial meld.** Until a player has laid 30+ points in a single turn, from tiles in their own
hand only, they may not touch anything already on the table. A joker in the initial meld counts as
the tile it represents.

**A turn.** Play at least one tile from hand, rearranging the table freely, and end with every set
on the table valid. If you cannot, draw one tile from the pool and the turn passes. There is no
discard.

**Jokers.** A joker on the table may be taken by a player who replaces it with the tile it stands
for, from hand; the joker must be used in a set that same turn. A joker may not be taken from a
group of three that would be left as a pair.

**Ending a round.** First player out of tiles wins it. Everyone else scores the negated value of
their remaining tiles (joker 30, others face value); the winner scores the sum of what everyone
else lost. If the pool runs dry and nobody can play, the round ends with the lowest hand value
winning — a declared option, since the physical rules disagree with each other here.

**Match.** Rounds until a player passes the target, or a fixed number of rounds — an option.

**Deliberately out of scope:** the turn timer and the associated penalty for a failed manipulation.
Both exist because a physical table cannot be rolled back; this one can, and a refusal is a better
answer than a punishment. Recorded so it reads as a decision.

## 3.3 The staged turn — the load-bearing design

A turn is a **workspace**: a scratch copy of the table the player edits with small moves, which may
be invalid in between, and which is validated as a whole when they commit.

Manipulation is **on from the first build that plays** (§6.1). There is no simplified turn to grow
out of and no option to switch it off, so the workspace and its validator are the engine's centre
rather than a later addition to it — which is also why they are cheaper to build now than to
retrofit: a rearranging turn is a different data structure from a hand-to-table one, not an extra
branch in it.

| Offer ID | Verb | What it does |
|---|---|---|
| `place` | `place` | hand tiles → a new set on the workspace |
| `add:<setId>` | `add` | tiles → an existing set (source may be the hand **or** another set) |
| `take:<setId>` | `take` | tiles out of a set, into the loose tray |
| `split:<setId>` | `split` | a run into two, at a chosen position |
| `swap_joker:<setId>` | `swap_joker` | replace a joker with the tile it stands for |
| `reset_turn` | `reset_turn` | throw the workspace away and start the turn again |
| `commit` | `commit` | end the turn — enabled only when the workspace is legal |
| `draw` | `draw` | take one from the pool; ends the turn |

**Commit is where the rules live.** `commit` is offered enabled when, and only when: every set on
the workspace is valid, the loose tray is empty, at least one tile came from the player's hand,
and — if this is their first lay — the tiles they laid from hand reach 30. Otherwise it is offered
*disabled*, with the code that says which of those failed, and the shell already renders that
reason. This is the same anti-drift construction as everywhere else: the offer is `Apply`'s answer
asked early, and the workspace validator is the single implementation of "is this table legal".

**Nothing can strand a player.** The house rule this codebase already enforces — never end a turn
in a dead position — is what `reset_turn` is for, and it is offered enabled on every step of a
turn in progress. Canasta needed `reachableValue` to look ahead because cards on the table could
not be taken back; here they can, so the guarantee is structural rather than predictive. The
conformance test asserts it: at every state, the player on turn has at least one enabled offer.

**Why not one big "submit the whole table" action.** It is the obvious alternative and it is worse
in two ways. `module.Action` carries cards, a target and string params — a proposed table would
have to travel as an encoded blob, which is a private protocol inside the shared one. And it moves
the composition entirely into the client, so a generic shell could not offer it and a bot could not
be checked against the same offers a person sees. Small moves keep one vocabulary.

## 3.4 What the protocol has to grow — two things, and they are small

| What Rummy Tiles needs | Why nothing before asked | Change |
|---|---|---|
| A source selector that is **a set on the table** | Every existing verb's cards come from a hand, a deck or a discard pile. `SelectorZone` has `ToMeld`/`ToTable` and no `From` counterpart, because until now nothing took cards *off* the table. | `FromMeld`, `FromTable` in `module/protocol.go` — mirroring the two that exist |
| A way to say **this set is not valid yet** | Every group a module has ever drawn was already legal; a mid-turn workspace is legal only at the end. | *No new field.* `Group.BadgeKeys` already carries per-group marks, and an `invalid` badge is exactly what it is for. Worth recording that the protocol absorbed this one without changing. |

That is the whole predicted protocol delta. It is deliberately in the plan as a prediction: if a
third thing turns up, it goes in the outcome section, because the last four games each bent the
interface in a place their plan did not foresee and pretending otherwise would be the only real
failure here.

## 3.5 What the client shell has to grow

Two things, and both are shell-generic rather than tile-specific.

**A tile face at exactly card size.** Tiles need their own face — a number and a colour, four
colours rather than red-and-black. It renders inside the existing `CardView` footprint, unchanged
in width, height and margin, so `flights.ts`, `drops.ts`, `hand.ts`, `layout.ts` and every skin are
untouched. This follows the standing constraint that presentation may change colour and decoration
and never a size, and it is what keeps the drag, drop and animation machinery out of this phase.

Notation is where it starts: tiles ship as `7-R`, `13-B`, `1-O`, `12-K` — number, hyphen, colour
(red, blue, orange, black) — with jokers keeping the existing shared `JOKER1`/`JOKER2` spelling.
The hyphen is not decoration: `isCardCode` is deliberately strict two-character matching, so a tile
can never be mistaken for a card and vice versa, and both parsers branch cleanly. Knowing that
`7-R` is the red seven is a fact about a *deck*, not about a game — the same standing that keeps
`parseCard` and `internal/render` in the clients while every rule lives on the server.

**Selection from a table group.** The shell's selection model is a set of *hand* slots
(`heldSlots` in `app/match/[matchId].tsx`). Rummy Tiles is the first game whose offers name a
source that is not the hand, so selection widens to board slots: a card inside a rendered group
becomes selectable when a live offer's `source` names that group. `ZoneView` already renders group
cards with an `onPress` and already has the group ids; what changes is where a selected slot may
come from, and it changes once, generically, for any game that later wants it.

The acceptance test for both is unchanged from Phase 7 of `one-architecture-plan.md`: **the shell
still contains no game's vocabulary.** No file mentions a tile, a joker swap or a run.

## 3.6 The bot

`OfferBot` cannot play this game at all: the offers describe single moves and the game is about
combinations, so a bot taking whichever offer comes first would shuffle the table forever. So
`Botted`, with a real solver, phased by skill:

- **easy / medium** — greedy: lay every set formable from the hand alone, then extend existing sets
  tile by tile, then draw. It plays legally and unimaginatively, and it is enough for a first
  playable opponent.
- **hard / expert** — the maximum-tiles-played rearrangement, computed over the combined multiset of
  table plus hand. This is the known Rummikub optimisation problem (Den Hertog & Hulshof's ILP
  formulation is the reference); implemented here as a dynamic program over numbers 1–13 carrying
  the count of runs open per colour, with jokers handled by a bounded branch over what each stands
  for, and a node budget so a pathological position degrades to the greedy answer rather than
  hanging a turn.

The solver is the largest single piece of work in this plan, and it is deliberately *last*: the
game is complete and playable against the greedy bot before a line of the DP is written.

## 3.7 Files

```
server/internal/rummytiles/
  tiles.go       the 106-tile deck, notation, values, colour/number helpers
  sets.go        group/run validity, joker resolution, the workspace validator
  state.go       GameState, the workspace, error codes, verbs
  engine.go      NewMatch, Apply and the eight verbs, round rollover, match end
  scoring.go     round settlement, match end
  offers.go      LegalActions, by probing the engine
  view.go        Descriptor, View, Seats, Standings, Rounds
  rules.go       Rules() and ExplainRefusal()
  solve.go       the rearrangement solver
  bot.go         greedy and solver policies, by skill
```

## 3.8 Tests

| Level | What it proves | Where |
|---|---|---|
| Unit — sets | group and run validity, jokers standing in, 13 not wrapping, duplicate-colour groups refused | `sets_test.go` |
| Unit — workspace | the validator accepts exactly the legal end states: split runs, stolen middles, a joker swapped out, a tray left loose | `sets_test.go` |
| Unit — engine | the 30-point initial meld from hand only, the table untouchable before it, joker retrieval limits, drawing when stuck, pool exhaustion — the last written against the **two-handed** deal, where it is a routine ending rather than a rare one | `engine_test.go` |
| Unit — no dead ends | over a corpus of mid-turn workspaces at 2, 3 and 4 seats, the player on turn always has an enabled offer, and `reset_turn` always returns the table to what it was | `deadend_test.go` |
| Unit — agreement | `LegalActions` and `Apply` never disagree | `agreement_test.go` |
| Unit — hidden info | `View` never shows a hand or the pool to the wrong viewer, and never shows one player's workspace to another before commit | `view_test.go` |
| Unit — rule index | every emitted code resolves to a written rule | `ruleindex_test.go` |
| Conformance | `PlayWithOffers` takes real turns and never sticks, at 2, 3 and 4 seats. **It is not expected to finish a match** — combinations are the game and a driver has no taste — so the test asserts progress and the absence of dead ends, honestly, the way the Continental case is asserted in `extensibility-plan.md` §1.6 | `conformance_test.go` |
| Shared contract | the eight terms, with the new module added | `module/allmodules_test.go` |
| Solver | the DP's answer is never worse than greedy, and matches a brute-force optimum on small positions | `solve_test.go` |
| Bench | `BenchmarkSolve` on a full table; `BenchmarkLegalActions` mid-turn | `bench_test.go` |
| Client | tile parsing, the tile face's size equalling a card's, board-slot selection | `cards.test.ts`, `board.test.ts`, a shell test |
| E2E | three players — one human, two bots — play a round to a winner in a browser; a manipulation is performed through the real UI; the shell still contains no game vocabulary | `e2e/tests/rummytiles.spec.ts` |

## 3.9 Exit criteria

1. `go test ./...` green in `server/`; the five other modules untouched.
2. A whole match is played in a browser through the generic shell, manipulation included: a turn
   that breaks a run, moves a tile between two sets and swaps a joker is performed through the
   real UI and accepted by the real engine. A build that plays without that is not done (§6.1).
3. Matches complete at 2, 3 and 4 seats (§6.2), the two-handed pool-exhaustion ending included.
4. `reset_turn` is enabled at every point of every turn in the dead-end corpus — no position exists
   from which a player cannot proceed.
5. The solver beats greedy on tiles-played over N seeded positions, and never loses to it.
6. No file outside `internal/rummytiles` learns a Rummy Tiles rule, and no client file learns one at all.

---

## 4. Cross-cutting work

| | Where |
|---|---|
| Registration | one line in `internal/app/app.go`; both modules added to `module/allmodules_test.go` |
| Message keys | every string ships as a key with params, never a sentence; keys are **static literals** at the construction site, since a key routed through a variable drops silently out of the manifest |
| `serverKeys.json` | regenerated via `cmd/dump-keys` after each module's keys land, or CI fails on the wording lock |
| i18n | English and Czech bundle entries for both games' rules, offers, refusals and round facts |
| Stats | free: `module.Standing` is what the recorder reads, so both games get lifetime records, head-to-head and leaderboards on registration day |
| Bots, reconnection, pause-between-rounds | free, for the same reason — they are runtime properties |

---

## 5. Sequencing

1. **A1** Gin Rummy engine, offers, view, rules — server-only, driven by the conformance test.
2. **A2** Gin Rummy bot and its strength gate.
3. **A3** Gin Rummy i18n and e2e. *Ship.* Five games.
4. **B1** Tile notation and the tile face, in both clients, with the size test. Shippable alone,
   and it de-risks the rest by proving the rendering claim before any rules depend on it.
5. **B2** Rummy Tiles engine and workspace validator — **manipulation included** — plus the two
   protocol additions, which `add` and `take` need on the day they first work.
6. **B3** Offers, the greedy bot, and the dead-end corpus, at 2, 3 and 4 seats. Playable end to
   end here, rearranging the table, against an opponent that plays legally.
7. **B4** Board-slot selection in the shell, and the e2e.
8. **B5** The solver, and the skill ladder. *Ship.* Six games.

Every step is committable on its own and leaves the tree better than it found it. The only step
that touches shared code paths is B4, and it lands after the game already works, so a regression
there cannot be confused for a rules bug.

The two decisions in §6 move work earlier rather than adding any: manipulation is B2–B3 instead of
a phase that never gets scheduled, and the seat range is a descriptor line whose cost is in the
tests — every conformance and dead-end run is parameterised over 2, 3 and 4 seats from B3 onward.

---

## 6. Decisions taken

Two questions this plan opened have been answered, and they are recorded here as settled rather
than left as recommendations, because both shape what B2–B3 must deliver.

1. **Table manipulation is in from the first playable build.** Not a later phase, not an option,
   not a flag: `commit` validates a rearranged table from the moment the engine exists, and B3 is
   not done until a turn can break a run, steal a middle tile and swap a joker. Manipulation is
   the game — a version without it is a worse Žolíky — and the staging design in §3.3 is what
   makes shipping it that early tractable. Consequences carried through this plan:
   - The workspace validator (§3.3) is written in B2, alongside the engine, not after it.
   - The two protocol additions (§3.4) land in B2 for the same reason: `add` and `take` need a
     source selector that names a set on the table on the day they first work.
   - The dead-end corpus (§3.8) is a B3 exit condition, not a hardening pass. A turn that can
     rearrange the table is a turn that can be left mid-rearrangement, and `reset_turn` being
     enabled at every point is what makes that safe.
   - The solver (§3.6) stays last regardless. Manipulation being available to a *player* from B3
     is independent of a *bot* being clever about it; the greedy policy plays legally against it.

2. **Rummy Tiles seats 2–4.** `MinPlayers: 2, MaxPlayers: 4` in the descriptor. Two-handed is a
   played form of the game, not a degenerate case, and it is the cheaper table to test against:
   the conformance run and the dead-end corpus both cover 2, 3 and 4 seats, and the e2e spec in
   §3.8 plays a three-seat table (one human, two bots) because that is the configuration where
   turn rotation and a contested table can both go wrong.

   One rule genuinely reads differently at two seats and is called out so it is implemented rather
   than discovered: pool exhaustion. With four players the pool empties late and rarely; with two
   it is a routine ending, so the lowest-hand-wins option in §3.2 is exercised far more often and
   its test cases are written against the two-handed deal.

### Still open

**Gin Rummy's Oklahoma variation now or later?** *Now*, on the recommendation above: it is one
number resolved from the upcard, and it costs a `VariationSpec` entry — cheaper to build than to
retrofit around a shipped `knockLimit` option. Say so if you would rather ship `standard` alone.

---

## 7. Outcome

*Written after the work, per the convention every plan in this directory follows: what shipped,
what the interface refused to absorb, what it cost the games already running, and the measurements
that replace the claims above.*

**Phase A shipped; Phase B has not started.** `server/internal/ginrummy` implements the whole
file list in §2.7 (`cards.go`, `meld.go`, `state.go`, `engine.go`, `scoring.go`, `offers.go`,
`view.go`, `rounds.go`, `rules.go`, `bot.go`), plus the full test pyramid in §2.8, registered in
`app.go` and `cmd/dump-keys` alongside the four existing games. `go test ./...` is green across the
whole server, `internal/module`'s shared contract passes for `ginrummy` on every one of its 19
tests, and `internal/module/allmodules_test.go` needed exactly one addition — a `hosted{}` entry —
to absorb it. No file outside `internal/ginrummy` learned a Gin Rummy rule.

**§7's "one way to add a game" claim held.** `TestConformance_PlaysWholeMatchesToAWinner` plays
whole matches — knocks and gins included — from `LegalActions` alone, across 20 seeds and both
variations. `e2e/tests/ginrummy.spec.ts` plays a whole match to a winner over real WebSockets with
real Mongo persistence. And `e2e/tests/generic-shell.spec.ts`'s `GAMES` table — the spec that already
proved one screen plays Prší, Canasta and Hold'em with no game-specific code — took a fifth entry,
`{ moduleId: 'ginrummy', ... }`, and passed with no client change at all: not a new component, not a
new zone kind, not a new selection rule. Exit criterion 3 is true, measured rather than assumed.

**The bot's knock timing needed a finding, not just an implementation.** §2.6's first draft held a
harder skill back from knocking, waiting for a bigger margin or gin outright. Self-play
(`TestBot_StrengthRisesMonotonicallyAlongSkill`) found that backfires: in a two-player race, whoever
ends a hand first keeps the tempo, so patience mostly hands the *easier* skill more turns to reach
its own knock — an early build lost 55 of 60 self-played matches with hard skill playing patient
against eager easy. Every skill now knocks the moment it may, and the skill dial moved to the
decision that does not have that failure mode: which card to discard when it cannot yet knock.
`Interest`, a new state field, is the module's own discard log per §2.6's own text — every card a
player has taken from the discard pile this hand — and a bot skill above `easy` uses it to prefer
discarding away from what the opponent has shown interest in, with `hard` tolerating a point of
extra deadwood for a meaningfully safer discard. This is the actual content of `bot.go` where the
plan predicted a threshold; it is a smaller, more defensible claim than "expert holds for gin", and
it is the one the self-play measurement supports.

**The protocol absorbed the draw split without changing.** `draw:stock` and `draw:discard` share the
verb `draw`, told apart by `LabelKey` on the offer and by `Action.OfferID` in `Apply` — the exact
mechanism zolikmod's own upcard-vs-stock draw already uses (`internal/zolikmod/module.go:230`), not
a new one. No protocol field was added; §3.4's prediction that Gin Rummy needs none held.

**One thing this plan did not foresee: static `LabelKey`s are easy to lose to a local variable.**
Two places in the first draft built an offer or a fact's `LabelKey` through a local variable chosen
at runtime (`id, labelKey := "knock:"+card, "..."; if deadwood == 0 { ... }`) — legal Go, and
invisible to `cmd/dump-keys`'s static AST scan, which only resolves a `LabelKey` that is a literal
or a package-level `const` at its construction site. The keys were still sent to real players; they
were just never in `serverKeys.json` for the i18n coverage test to require wording for. Caught only
by an agent cross-referencing the emitted keys against what the code actually builds, not by any
test — rewritten as one literal `ActionOffer`/`Fact` per branch instead of one shared literal with a
variable swapped in. This is exactly the shape of bug the "message keys must be static literals"
rule exists to prevent, and it is worth restating here because the failure mode is silent: nothing
breaks, a key just never gets translated. A second, framework-level version of the same gap survived
this PR rather than being fixed in it: `internal/module/emittable.go`'s `labelKeysIn` resolves an
assignment to `Seat.LabelKey`/`TitleKey` but not to `Seat.LabelKeys` (plural) — `seat.LabelKeys =
append(...)` is invisible to the scanner regardless of whether the appended value is a literal.
Canasta's `canasta.seat.notOpened` has the identical gap already; `ginrummy.seat.dealer` has it too,
worded in the bundle by hand since the test cannot require it. Fixing the scanner is shared-module
work affecting every game at once and was flagged as a follow-up rather than folded into this PR.

**Options landed as declared, per the house rule.** `targetScore`, `knockLimit` (with the Oklahoma
sentinel), `bigGin` and `lineBonuses` are all `OptionSpec`s with real defaults per variation, not
constants — `oklahoma` differs from `standard` in exactly one default, as §5's "Still open" note
recommended, so that question is answered: Oklahoma shipped now, in Phase A, for the cost the note
predicted.

**Left for Phase B, or for whoever reads this next:** `ExplainRefusal` (`RuleIndexProvider`) was not
implemented, matching prsi and canasta's own precedent of shipping without it; `RuleIDs` on offers is
consequently empty. Rummy Tiles (§§3–6) has not been started.
