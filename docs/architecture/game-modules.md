# Game modules

A game is a Go package that implements `module.GameModule`. The runtime
(`internal/match`) and every client are written against that contract and
nothing else. This page describes the contract as it is in
[`server/internal/module`](../../server/internal/module), the conventions the
modules share, and the seventeen games registered today.

History: the interface was extracted from the Žolíky engine and then
deliberately tested against games that broke it — Prší (parameters that are
not cards), Canasta (partnership winners), Hold'em (numbers that are not
cards, split pots), Blackjack (nobody wins). See
[`extensibility-plan.md`](../extensibility-plan.md),
[`canasta-plan.md`](../canasta-plan.md) and
[`one-architecture-plan.md`](../one-architecture-plan.md).

## The contract

[`protocol.go`](../../server/internal/module/protocol.go):

```go
type GameModule interface {
    Descriptor() ModuleDescriptor
    NewMatch(cfg MatchConfig, players []PlayerRef, seed int64) (State, error)
    Apply(s State, playerID string, a Action) (State, []Event, error)
    View(s State, viewerID string) (ViewModel, error)
    LegalActions(s State, playerID string) ([]ActionOffer, error)
    Finished(s State) (done bool, winners []string, err error)
}
```

- **`State`** is `json.RawMessage`: opaque to everything but the module. The
  runtime persists it, passes it back, and never looks inside.
- **`NewMatch`** must be reproducible from the seed. Rebuilds and replay rely
  on it.
- **`Apply`** is pure — no I/O, no clock beyond what the caller supplies — and
  deterministic. That is what lets the runtime keep the board in memory and
  rebuild it from a snapshot plus the move log.
- **`View`** is the only place hidden information is filtered: the module
  returns the board as one viewer may see it.
- **`LegalActions`** returns the *full* set of offers including disabled
  ones, each disabled offer carrying a reason.
- **`Finished`** returns a list of winners: a partnership has two, a split pot
  several, and a Blackjack table where everyone went broke has none.

### Actions and offers

`Action` is a move in vocabulary no game owns: `OfferID` (echoed from the
offer), `Verb`, `Cards`, `Target`, `Params map[string]string`.

`ActionOffer` is what a seat may do now:

| Field | Meaning |
|---|---|
| `ID`, `Verb`, `Enabled` | Identity and whether it can be taken |
| `WhyNot` | Stable error code when disabled, never a sentence (`NOTHING_FITS_HERE` is the conventional weakest refusal) |
| `RuleIDs` | Ids of written rules that justify `WhyNot` at this table |
| `Remedy`, `RemedyOfferID` | The next legal move in words, and the offer that performs it |
| `LabelKey` | A label when the verb alone is ambiguous (two "draw" offers) |
| `Source`, `Target` | `Selector`s: which cards, from where (`hand`, `deck`, `discard_pile`, `from_meld`, ...), to where (`meld`, `table`), with positional `Placement`s |
| `Params` | `ParamSpec`s for choices that are not cards (Prší's suit, a bet amount) |
| `Facts` | Numbers shown on the control (the amount to call) |
| `Undo`, `Composite`, `Manual` | Takes a move back / must be composed by a person (a Žolíky meld) / only a person may submit it (leaving a table) |

Because an offer says which cards it takes and where they land,
**drag-and-drop is a filter over the offer list**: the client never knows a
rule about where a card may go.

### The view

`ViewModel` is zones, seats and facts:

- **Zones** (`ZoneKind`): `hand` (fanned, owned), `stack` (face-down, count
  only), `pile` (face-up, top card last), `spread` (groups side by side: melds,
  tricks). A game with no melds simply never emits a meld zone.
- **Seats**: `Active` (who the module is waiting on — the runtime reads this to
  know whether a bot must act), `LabelKeys` (`seat.dealer`, `seat.folded`),
  `Facts` (chips, score), `Side` (partnership id).
- **Facts**: label key + value/params, pre-resolved by the module and
  interpreted by nobody.

### Optional interfaces

A module opts into runtime features by implementing extra interfaces. Each is
discovered with a type assertion, so a module that does not need one is
unaffected.

| Interface | File | What it enables |
|---|---|---|
| `Botted` | [`bot.go`](../../server/internal/module/bot.go) | Its own bot; otherwise `OfferBot` (first enabled offer, optionally by preferred verbs) |
| `RulesProvider` | [`rules.go`](../../server/internal/module/rules.go) | Written rules for a config (`/modules/{id}/rules`, the rules screen) |
| `RuleIndexProvider` | `rules.go` | Maps a refusal code to rule ids |
| `Ranked` | `protocol.go` | Standings for the scoreboard |
| `Rounded` | [`rounds.go`](../../server/internal/module/rounds.go) | A round log (scores per round) |
| `Projector` / `ApplyProjector` | `protocol.go` | Decode state once per move and share it between broadcast, bot loop and finish check (Žolíky only today) |
| `OpenViewer` | `protocol.go` | A nothing-hidden board for replay |
| `EventProjector`, `EventNarrator` | [`events.go`](../../server/internal/module/events.go) | Per-viewer filtering of events; narrated move log |
| `Seated` | [`descriptor.go`](../../server/internal/module/descriptor.go) | Sides (teams) from seat order |
| `DropIn` | [`dropin.go`](../../server/internal/module/dropin.go) | Table does not pause for an absent seat; it is sat out after a grace period |
| `LateSeater`, `DealsAround` | [`partE.go`](../../server/internal/module/partE.go) | Seat a player at a game under way (cash tables); deal around an away seat |
| `LessonProvider` | [`lesson.go`](../../server/internal/module/lesson.go) | Guided lessons with fixed seeds |
| `ServerDealt` | [`dealt.go`](../../server/internal/module/dealt.go) | The deal must never leave the server: not synced to devices, not hosted offline, not accepted back as a bundle (Solitaire) |

## Descriptor, variations and options

`ModuleDescriptor` ([`descriptor.go`](../../server/internal/module/descriptor.go))
is what `GET /modules` returns and what the lobby renders its game picker and
setup form from: `ID`, `Label`, seat range and default, `Variations`,
`Options`, `Deck` (`german`, `spanish`, `lastcard`, or empty for French), and
`SimulatedGambling` (chip games, which clients treat specially).

- A **variation** (`VariationSpec`) is a named ruleset: its own seat range,
  default option values and summary facts. `Formerly` lets a renamed variation
  keep matching old stored matches. Seat limits are per variation
  (`SeatRange`), so a Pinochle partnership table is exactly four.
- An **option** (`OptionSpec`) is an integer setting with an enumerated set of
  choices. `MatchConfig.Options` is `map[string]int`; the runtime validates
  every value against the descriptor at creation, so an undeclared value is
  refused at the door.

**House rules are options.** A rule that tables play differently is exposed as
a declared option whose default reproduces the existing behaviour (Žolíky's
`initialMeldMinimum`, `discardDrawMinRound`, `requireCleanRun`,
`jokerReclaimMustPlay`; Canasta's `dirtySequences`, `poach`,
`topOnlyCapture`; Last Card's `stacking`, `sevenZero`, ...). It is never a
constant changed in place.

Shared options are written once in the `module` package and appear across
games: `pauseBetweenRounds` (`PauseOption`, implemented by
[`intermission.go`](../../server/internal/module/intermission.go) as an
ordinary `continue` offer, not a runtime primitive), `openDiscardPile`,
`botSkill` (`BotSkillOption` / `BotSkillOptionWithAI`, see
[skill.go](../../server/internal/module/skill.go)), `standInAfter` and
`stepOut` ([`standin.go`](../../server/internal/module/standin.go)),
`leaveAfterAway` for cash tables ([`leave.go`](../../server/internal/module/leave.go)),
`dealAroundAway`, and `hints` ([`hints.go`](../../server/internal/module/hints.go)).

## Written rules and refusals

Every module now writes its rules (`RulesProvider`) and explains its refusals
(`RuleIndexProvider`).

- `Rules(cfg)` returns `[]RuleSection`, each a title key and a list of
  `RuleItem`s. A rule item is a `Fact` (label key + params) whose **id is its
  own label key**. Because the rules are built for the actual variation and
  option values, the sentence a player reads carries that table's numbers.
- When an action is refused, the runtime asks
  `ExplainRefusal(cfg, code)` for the rule ids and puts them in the error frame
  (`ruleIds`); offers carry `RuleIDs` the same way. The client links to the
  sentence on the rules screen. There is deliberately **no per-code
  explanation text** anywhere: the explanation is the written rule.
- `StatedRuleIDs` caches the set of ids a config states, so offers can drop any
  pointer to a sentence the table does not state. The cache key is the module's
  *type* plus the config, not its address (zero-sized modules share an address;
  that bug stripped every rule id from three games until a test caught it).
- Guard rails: [`ruleindexcheck.go`](../../server/internal/module/ruleindexcheck.go)
  checks both ends of each index, and
  [`emittable.go`](../../server/internal/module/emittable.go) reads every error
  code a package constructs *out of its source*, so a new refusal without a
  rule mapping fails the module's tests.

## Hidden information

- Only `View(s, viewer)` decides what a viewer sees. A spectator (no token, or
  a token for nobody at the table) gets the onlooker view.
- Events that name a private card are filtered per viewer through
  `EventProjector`; the narrated move log is built from the *projected* event,
  so it cannot say more than the viewer may know.
- Bots receive only the offer list plus the state through their own module's
  code; trained models see only the adapter's encoding, which each adapter's
  "no peek" test pins.
- `allmodules_test.go` enforces this for every module:
  `TestEveryModuleHidesOtherPlayersCards`, `TestAnEventNeverNamesAHiddenCard`,
  `TestAnOnlookerNeverKnowsMoreThanAPlayer`, `TestARoundLogNeverNamesACard`.

## Scoring and results

- `Ranked.Standings` returns `[]Standing` (`PlayerID`, 1-based `Rank` with ties
  sharing a rank, `Score`, `LabelKey` naming the unit). Helpers `RankByScore`
  and `RankByScores` cover most games.
- `Rounded` gives a round log that the tests require to be arithmetic and to
  agree with the scoreboard (`TestARoundLogIsArithmetic`,
  `TestTheLastRoundAgreesWithTheScoreboard`).
- On completion the runtime hands `module.Outcome` to the stats recorder
  ([`internal/stats`](../../server/internal/stats)), which writes a permanent
  `match_results` record and lifetime aggregates. Guests are recorded but are
  not durable subjects (no leaderboard) until claimed by an account.

## The conformance harness

[`conformance.go`](../../server/internal/module/conformance.go) is a driver that
plays any module using only its offers and views — it never decodes state or
names a card. If it can finish a game, a UI shell can play it.
[`allmodules_test.go`](../../server/internal/module/allmodules_test.go) runs
every module (and its main variations) through ~30 contract tests, including
`TestRandomPlayNeverBreaksTheContract` (`ZOLIK_FUZZ_SEEDS=500` for a long run).
Each module also has its own `conformance_test.go`.

## The games

From the descriptors in this tree (`go run` over `hostedModules`):

| ID | Label | Package | Seats | Variations | Deck | Notes |
|---|---|---|---|---|---|---|
| `zolik` | Žolíky | [`zolikmod`](../../server/internal/zolikmod) (+ engine [`rules`](../../server/internal/rules), bot [`ai`](../../server/internal/ai)) | 2–8 | `zolik_classic`, `continental` | French + jokers | Trained model; `Projector` |
| `prsi` | Prší | [`prsi`](../../server/internal/prsi) | 2–6 | `classic` | German | No round log |
| `canasta` | Canasta | [`canasta`](../../server/internal/canasta) | 2–6 | `classic`, `modern_american`, `samba`, `canastax` | French | Trained model; partnerships |
| `holdem` | Poker | [`holdem`](../../server/internal/holdem) | 2–9 | `holdem`, `omaha`, `draw` | French | Trained model; chips; cash or tournament format |
| `ginrummy` | Gin Rummy | [`ginrummy`](../../server/internal/ginrummy) | 2 | `standard`, `oklahoma` | French | |
| `rummytiles` | Rummy Tiles | [`rummytiles`](../../server/internal/rummytiles) | 2–4 | `standard` | Tiles | Takes cards back off the table (`from_meld`) |
| `blackjack` | Blackjack | [`blackjack`](../../server/internal/blackjack) | 2–7 | `vegas`, `atlantic`, `single` | French | Chips; may have no winner |
| `marias` | Mariáš | [`marias`](../../server/internal/marias) | 3–4 | `voleny`, `licitovany` | German | Trick engine in [`tricks`](../../server/internal/tricks) with a search solver |
| `klondike` | Solitaire | [`klondike`](../../server/internal/klondike) | 1 | `classic`, `draw3`, `vegas` | French | `ServerDealt`; no bot |
| `lastcard` | Last Card | [`lastcard`](../../server/internal/lastcard) | 2–10 | `classic` | Own deck | UNO-style mechanics, own names and art |
| `snaps` | Šnaps | [`snaps`](../../server/internal/snaps) | 2 | `snaps`, `sedesatSest` | German | |
| `sedma` | Sedma | [`sedma`](../../server/internal/sedma) | 2–4 | `classic` | German | |
| `okobere` | Oko bere | [`okobere`](../../server/internal/okobere) | 2–6 | `classic` | German | Chips |
| `ferbl` | Ferbl | [`ferbl`](../../server/internal/ferbl) | 2–6 | `classic` | German | Chips |
| `lora` | Lóra | [`lora`](../../server/internal/lora) | 4 | `classic` | German | |
| `sietemedio` | Siete y Medio | [`sietemedio`](../../server/internal/sietemedio) | 2–8 | `mexicana`, `cantina`, `abierta` | Spanish | Chips |
| `pinochle` | Pinochle | [`pinochle`](../../server/internal/pinochle) | 2–4 | `partnership`, `cutthroat`, `twohanded`, `doubledeck` | Pinochle | |

"Chips" = `SimulatedGambling` in the descriptor. All but Solitaire implement
`Botted`; every module implements `RulesProvider` and `RuleIndexProvider`.
Rules write-ups for several games are in `docs/*-rules.md` (for example
[`marias-rules.md`](../marias-rules.md), [`pinochle-rules.md`](../pinochle-rules.md),
[`sietemedio-rules.md`](../sietemedio-rules.md)).

Shared helpers used by several games: [`cardinfer`](../../server/internal/cardinfer)
(public-information card inference for rummy bots),
[`tricks`](../../server/internal/tricks) (trick-taking rules and search),
`module.DiscardPile` and the intermission and skill helpers.

How to add a game: [developer guide](../developer/README.md#adding-a-game-module).
