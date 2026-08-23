# Zolik Engine Blueprint

Architecture review and forward plan for the Žolíky server, its rules core, its AI, and its
clients — and what has to change for the same engine to run a different card game behind a
different UI.

- **Baseline:** `main` @ `6152d9a`
- **Scope read:** `server/internal`, `client-react-native/src`, `client-tui` (~16k LOC)
- **Test state:** `go test ./...` green (89 rules, 16 AI, 24 game, 4 TUI, plus auth/scoring); RN Jest green
- **Status:** every defect in §10 has been fixed; Phases 0–4 of the migration plan are complete.
  Two games now run behind one runtime — see [`extensibility-plan.md`](./extensibility-plan.md)
- **Detailed build order:** [`extensibility-plan.md`](./extensibility-plan.md)

---

## 1. Why this document

Zolik is a working multiplayer implementation of Žolíky (Continental Rummy / Žolík Classic): a
Go server holding an authoritative rules engine, MongoDB persistence, WebSocket fan-out and
heuristic AI opponents, with an Expo/React Native GUI client, an SSH terminal client, and a
third (Defold) client currently an empty scaffold.

Two things are wanted from it:

1. The UI can be **swapped or restyled** for a different card game.
2. **Rules and UI business are actually separated** — which today they are not, despite a rules
   package that looks like they are.

### The two kinds of "different game"

These need very different answers, and conflating them is the main risk to the plan.

**Slightly different** — another rummy: Canasta, Gin, Burraco, Kalooki, a house variant. Same
nouns (hand, draw pile, discard pile, melds, lay-offs, going out), different numbers and
clauses. This is a *configuration* problem, and the codebase is already most of the way there
via `RulesConfig` and named profiles.

**Completely different** — trick-taking (Mariáš, Whist), shedding (Prší, Crazy Eights),
climbing (Big Two). Different nouns entirely: tricks, trumps, bids, no melds, no discard pile.
This is an *interface* problem. `rules.GameState`, `rules.Action` and the three-phase
`draw → meld → discard` turn model are rummy vocabulary baked into the engine's type system,
and no amount of config makes them describe a trick.

Section 7 proposes one structure that serves both: keep the current engine as the first *game
module*, and put a game-agnostic contract in front of it.

---

## 2. The system today

Four deployables, one authoritative server, no client-side rules authority.

```
 CLIENTS                    GO SERVER (single binary)                 STORES
 ┌────────────────┐        ┌───────────────────────────────┐
 │ Expo / RN      │──┐     │ auth · chi router · ws upgrade│
 │ web/iOS/Android│  │     ├───────────────────────────────┤
 └────────────────┘  │     │ internal/game — orchestration │◀──▶ Redis (optional)
 ┌────────────────┐  ├REST─▶│  Manager · Hub · ConnRegistry │      pub/sub fan-out
 │ Bubbletea TUI  │──┤ +WS │  state_builder · action log   │
 │ over SSH :2222 │  │     ├───────────────┬───────────────┤
 └────────────────┘  │     │ internal/rules│ internal/ai   │
 ┌ ─ ─ ─ ─ ─ ─ ─ ┐   │     │ PURE, no I/O  │ HeuristicAgent│
   Defold (empty) ───┘     │ ApplyAction() │ VisibleState  │
 └ ─ ─ ─ ─ ─ ─ ─ ┘         ├───────────────┴───────────────┤
                           │ internal/models — bson shapes │◀──▶ MongoDB
                           └───────────────────────────────┘      one doc = one match
```

`internal/rules` is the only package that decides what is legal, and it is reachable only
through `internal/game`.

| Package / dir | Responsibility | Game-specific? |
|---|---|---|
| `server/internal/rules` | Deck, meld validation, action application, scoring, contracts, profiles | Rummy-only |
| `server/internal/game` | Manager (apply→persist→broadcast), Hub, ConnRegistry, REST, replay, action log | Mostly generic |
| `server/internal/ai` | `Agent` interface + `HeuristicAgent` | Rummy-only |
| `server/internal/models` | Mongo document shapes | Rummy-only |
| `server/internal/{auth,user,db,app,tuiauth}` | JWT, guest sessions, repositories, wiring, SSH bridge | Generic |
| `server/internal/scoring` | Standalone pen-and-paper scorepad (unrelated to live games) | 7-round assumption |
| `server/internal/stats` | Match records, lifetime aggregates, scoreboards, leaderboards | Generic |
| `client-react-native` | Expo Router screens, drag-and-drop table, meld staging | Rummy-only |
| `client-tui` | Bubbletea screens, ASCII card renderer, SSH server | Rummy-only |

---

## 3. The rules core

The entire rule surface is one entry point:

```go
// server/internal/rules/engine.go
func ApplyAction(state GameState, playerID string, action Action) (ApplyOutcome, error)
// ApplyOutcome = { State GameState; Events []StateEvent }
```

No I/O, no clock beyond a defaulted `Created`. It returns a *new* state plus semantic events;
persistence and broadcasting are the caller's job. That shape is correct and should survive
every change proposed here.

### Vocabulary the engine hardcodes

Everything below is a Go type, not a config value — the boundary a second game family cannot
cross.

| Concept | Type | Values |
|---|---|---|
| Turn phases | `rules.Phase` | draw · meld · discard · suspended |
| Action verbs | `rules.ActionType` | draw_card · lay_meld · lay_off · swap_joker · discard · 3 undo variants |
| Meld kinds | `rules.MeldType` | set · run |
| Card encoding | `string` | `"7H"`, `"TS"`, `"JOKER1"` — rank char + suit char |
| Zones | fields on `GameState` | DrawPile · DiscardPile · Hands · Melds · MeldMeta |
| Error codes | `rules.RulesErrorCode` | 23 constants, ~20 rummy-specific (ACE_BRIDGE, BREAKS_CLEAN_RUN…) |

### What *is* already parameterised

`RulesConfig` (`rules/config.go`) pulls 13 knobs out of the code. Two profiles ship:

| Knob | continental | zolik_classic (default) |
|---|---|---|
| DealSize | 12 | 13 |
| MinSetSize / MinRunSize | 3 / 4 | 3 / 3 |
| InitialMeldMinimum | 35 | 0 (off) |
| DiscardDrawMinRound | 3 | 0 (open) |
| DiscardPickupMode | top_only | any_from_pile |
| Contract | rotating, 7 deals | static: ≥1 joker-free run |
| MatchEndMode | after 7 deals | at 200 points |

This is a genuinely good design and it proves the "slightly different game" case is largely
solved: a Kalooki or Burraco profile is mostly new constants plus a couple of new knobs. Since
the split-brain fix below, `RulesConfig` is also the *only* home for a rule value — nothing in
the engine reads a rule from anywhere else, and a knob added here is automatically persisted
(guarded by `TestRulesConfigRoundTrip`).

> **The config used to have a split brain — now fixed.** `RulesConfig.InitialMeldMinimum` and
> `RulesConfig.DiscardDrawMinRound` were never read by the engine; it read duplicate fields on
> `GameState`, populated from separate Mongo columns. Meanwhile `toRulesState` re-derived the
> whole `RulesConfig` from the profile *name* on every load, contradicting `config.go`'s own
> comment that a game "always carries a fully-resolved RulesConfig … never re-derived from a
> profile name mid-game."
>
> As fixed: the duplicate `GameState` fields are gone, so `Rules` is the only home for a rule;
> the resolved config is frozen onto the document (`models.Game.Rules`) at creation and read
> back by `game.GameRules`, never re-derived. Documents written before the field existed are
> migrated on load from the profile name plus the two legacy scalar columns, and written back
> on their next action. See §10 for the regression tests.

### Mechanics worth preserving

- **Ace duality.** In a *run* an ace is natural at rank 1 or 14 of its own suit (worth 1);
  anywhere else a wild worth 25 in hand. `validateRun` enumerates every candidate rank window
  and prefers the placement spending the fewest wilds, so `J-Q-K-A` beats `10-J-Q-K` + wild ace.
  Toward the initial-meld floor it is worth `rules.AceMeldValue` (15) wherever it is played as
  the real, highest card — in a set (A-A-A), or at the *top* of a run (Q-K-A) — and only
  `rules.AceRunLowValue` (1) at the *bottom* of a run (A-2-3), the low-ace convention. Which end
  a run's ace occupies is known solely to the resolved rank window, since the card string is
  identical either way — which is why `runValue` reads a run's worth off that window rather than
  off the cards. Scoring every natural ace 1 made three aces worth 3 points — less than three
  2s — and Q-K-A worth 21 instead of 35, putting every floor the lobby offers (35/50/70) out of
  reach for a hand built on aces.
- **A wild is worth the card it replaces.** A joker melded as the 8 of a 7-8-9 run counts 8;
  behind a king, where it is the ace, it counts `AceMeldValue`. So a meld's worth is a property
  of its *shape*, not of which slots happen to hold real cards: `runRankValue` prices a run slot
  by its resolved rank and never asks what is sitting in it, and a set prices every card — jokers
  included — at its single shared rank. Wilds used to count 0, which made a joker the one card
  whose position on the table changed nothing, and put `Q♠-K♠`+joker at 20 against a 35 floor
  when it is plainly the 35-point ace-high run the player laid.
- **A joker may be an ace.** The rank-1 and rank-14 slots used to demand a *real* suited ace, so
  any window reaching them was discarded before the wild accounting saw it — the ace was the one
  rank a joker could not stand in for, and `Q♠-K♠` + two jokers was rejected outright rather than
  read as `J-Q-K-A`. Those slots now take a wild like any other gap.
- **Window tie-breaks, in order.** Fewest wilds first, so a flex ace resolves to its natural
  endpoint over standing in for an unrelated rank. Then the drop position, when a caller named
  one — `Action.Position` is the player stating which end they meant, and an engine that
  resolved the other end because it scored better would be overruling them. Then the higher
  value, which is what finally puts a lone joker *behind* the king (`Q-K-A`, 35) rather than in
  front of the queen (`J-Q-K`, 30). Then the lowest window, as always.
- **Same-turn undo windows.** `DiscardDrawnCards`, `LastLayOff`, `LastMeldLaid` let a player
  reverse the last pickup / lay-off / meld, each cleared as soon as anything else happens that
  turn. Derived flags are restored too, not just the cards.
- **Drop-position honesty.** `Action.Position` (`"front"`/`"end"`) makes a run lay-off extend
  the end the player physically dropped on, rejecting with `WRONG_RUN_END` rather than silently
  resolving to the other end. UX intent expressed as a rule input — a good pattern for the
  generic protocol.
- **Contract obligations.** Taking from the discard pile before going down obliges melding that
  exact card this turn; starting a rotating contract obliges finishing it before discarding.
  Both encoded as state, not client etiquette.

---

## 4. Orchestration & transport

`Manager.HandleAction` is the single write path:

```
client ──▶ HandleAction ──▶ ApplyAction ──▶ persist ──▶ broadcast ──▶ every client
 WSIncoming  load +           PURE          CAS on       per-player     game_state
             toRulesState     rejects here  version      projection
                  ▲               │              │
                  │               └─ RulesError ─┴─▶ error frame, state untouched
                  │                              └─▶ actionLog (append-only)
                  └──────── next actor is AI? re-enter ────────┘
```

The AI has no privileged write path — it produces the same `WSIncoming` a human would send.

### Properties worth keeping

- **Optimistic concurrency.** The whole match is one Mongo document with a `version` field;
  `UpdateWithVersion` does a filtered `ReplaceOne` and returns `version conflict` if someone
  else wrote first. Safe load→apply→store without transactions.
- **Per-viewer projection.** `BuildGameStateMsg` sends each player their own hand plus
  opponents' *card counts*. Hidden information is filtered at the serialisation boundary, not by
  client discipline. `broadcast.go` and `replay.go` do the same for events and the action log.
- **Write serialisation.** `syncConn` wraps each socket in a mutex because gorilla allows one
  writer; a broadcast and a direct error reply used to race and silently drop a frame.
- **Reconnect semantics.** Disconnect suspends the game only if the leaver held the turn, and
  only if that connection was still the registered one (`RemoveIfCurrent`); reconnecting the
  same player resumes it.

### Where it leaks game knowledge

`models.Game` is not a generic match document — `drawPile`, `discardPile`, `melds`, `meldMeta`,
`roundReqMet`, `lastLayOff` are first-class Mongo columns, with `toRulesState`/`fromRulesState`
hand-mapping 30-odd fields both ways. Adding one rules *state* field still means editing several
files. (Rule *config* no longer does: it is one opaque sub-document, `models.Game.Rules`, added
to in one place. That is a small preview of the §7.6 split.)

Two smaller wins landed with the defect fixes: `startGame` no longer hand-rolls the opening deal
— both it and `StartNextGame` go through `rules.dealNewGame`, so "shuffle, deal, turn the first
discard, reset per-deal state" has exactly one implementation — and `fromRulesState` is now the
single writer of rules state onto the document, rather than being shadowed by 25 hand-written
field assignments in the REST handler.

---

## 5. How the AI works

No learning, no search tree, no opponent model beyond discard history. Roughly 500 lines of
deterministic heuristics in `internal/ai`; the interesting decisions are structural.

### 5.1 Lifecycle

An AI is a `models.Player` with `IsAI: true` and an id like `ai:medium:1724…:0`, added via
`POST /games/{id}/add-ai` with difficulty `easy` | `medium` | `hard`. Names come from a
per-difficulty table (`Rookie Rita`, `Clever Karel`, `Shark Soňa`).

After every applied action — and once at game start — `Manager.RunAIIfNeeded` starts at most one
goroutine per game (guarded by an `aiRunning` map). That loop re-reads the game from Mongo each
iteration, checks the current actor is an AI, sleeps a human-plausible 500–2000 ms, asks the
agent for one action, and feeds it back through `Manager.HandleAction` — the same function a
WebSocket frame hits.

**Why that matters:** the AI cannot cheat by construction. It never mutates state directly, and
the only state it sees is `ai.VisibleState` plus its own hand, assembled by `aiVisibleFromGame`.
Other players' hands and the draw pile are simply not in the struct. Any anti-cheat argument
reduces to auditing that one 40-line function.

Two safety nets bound the loop:

- `maxSteps = 200` across the whole chain of consecutive AI turns, and `maxActorStall = 20`
  actions by the same actor without the turn moving on.
- A rejected `lay_meld`/`lay_off` is **not** retried — the loop falls back to `PickWorstDiscard`
  so the turn always ends.

### 5.2 What the agent can see

| Field | Contents | Used for |
|---|---|---|
| hand (separate arg) | own cards | everything |
| `DiscardPile` | full public pile | draw decision |
| `PlayerDiscards` | per-player discard history, rebuilt from the action log | hard-mode discard tie-break |
| `Melds` / `MeldMeta` | everything on the table, all owners | lay-off search, discard safety |
| `RoundReqMet` | who is down | obligation logic |
| `Rules`, `GameNumber`, `Round` | resolved config + deal/lap counters | contract targets, discard lock |
| `TotalScores` | match standings | **currently unused** |

### 5.3 The decision function

`HeuristicAgent.ChooseAction` returns exactly one action per call and is re-invoked until the
turn passes, so a turn like *draw → meld → meld → discard* is four independent calls, each
re-deriving its plan from scratch.

```
PHASE = DRAW
  discard pile usable? (non-empty, not round-locked)
    ├─ yes ─▶ already down? ─▶ take it (pickup is unrestricted)
    │         └─ not down ─▶ does a plan exist that USES that card?
    │                        (taking it obliges melding it this turn)
    │                        ├─ yes ─▶ take discard
    │                        └─ no  ─▶ fall through
    └─ no / declined ─▶ draw_card from deck

PHASE = MELD
  am I down yet?
    ├─ no  ─▶ findInitialMeldPlan (DFS): full contract + value floor, or nothing.
    │         Lays combo[0]; replans on the next call.
    └─ yes ─▶ findLayOff on ANY table meld (shed first — else the hand never empties)
              └─ none ─▶ findAnyValidMeld (only if ≥1 card would remain)
  no move available ─▶ pickSmartDiscard   ← difficulty applies HERE only
```

### 5.4 The plan-first melding rule

The single most important behaviour: **the AI never lays a meld unless it can complete its
entire contract this turn.** This is forced, not cautious — under a rotating contract the server
refuses to let a player discard once they have started but not finished their initial
combination (`ErrIncompleteInitialMeld`). An agent that laid one set of a required two would be
stuck, burn its stall budget, and hand the turn back only via the fallback path.

`findInitialMeldPlanRequiring` computes the *remaining* contract (subtracting sets/runs already
laid, allowing for a clean-run requirement), then runs `searchMeldCombo`: a depth-first search
that enumerates candidate melds, validates each through the real `rules.ValidateMeld`, recurses
on the remainder, and accepts only when every count is satisfied **and** the accumulated natural
value clears the floor **and** at least one card remains to discard. A 200,000-node budget caps
the blow-up.

> **Dominant weakness: minimum-size-only enumeration.** `combinations(hand, k)` is only ever
> called with `k = MinSetSize` or `k = MinRunSize`. The AI literally cannot conceive of a 5-card
> run or a 4-card set as part of its opening. Under `zolik_classic` (min run 3, no value floor)
> that is mostly harmless; under `continental`'s 35-point floor it is expensive, because the
> cheapest way to clear the floor is often one longer run — a shape the search never generates.
> Widening enumeration to `k = min … len(hand)` under the existing budget guard is the
> highest-value single AI improvement available.

### 5.5 Difficulty

Difficulty affects *only* the discard choice. Drawing, planning and lay-off logic are
byte-identical across all three tiers.

| Tier | Discard policy | Behavioural tell |
|---|---|---|
| easy | highest penalty points, blind (`pickWorstDiscard`) | feeds opponents' melds constantly |
| medium | drop any card that extends a live table meld, then highest points | stops gifting lay-offs |
| hard | as medium, plus on an exact points tie prefer a rank an opponent already discarded | reads the discard history |

Both smarter tiers respect the joker rule: a joker is the highest-penalty card in the deck (50)
but is unplayable as a discard unless it ends the hand, so it is filtered out rather than picked
and rejected.

### 5.6 Honest limitations

| Limitation | Effect | Fix cost |
|---|---|---|
| Single ply, no lookahead | no notion of racing an opponent one card from out | high |
| Min-size melds only | fails Continental's value floor far too often | **low** |
| Never uses `swap_joker` | leaves reclaimable jokers on the table all game | low |
| Never uses the undo actions | harmless, but a human-parity gap | low |
| ~~First-match lay-off over Go map iteration~~ | **Fixed.** Owners are now visited in sorted order; which lay-off is *best* is still unevaluated | — |
| ~~`HeuristicAgent.rnd` is dead~~ | **Fixed.** The unread RNG is gone and the determinism is now documented and tested, not accidental | — |
| Ignores `TotalScores` | no endgame risk adjustment when a rival nears the target score | medium |
| Reloads the match doc per sub-action | a 4-action AI turn is 4 Mongo reads + 4 CAS writes | medium |

The determinism is a *feature* for testing (16 table-driven tests pin the behaviour, including
`TestChooseAction_IsDeterministic`, which asserts it directly) and a *liability* for play feel —
three difficulty levels that draw and meld identically read as one opponent with three names.
Note that determinism is now a deliberate, tested property: if variety is wanted later, it has to
be introduced as a seeded, reproducible choice rather than by reinstating a wall-clock RNG.

### 5.7 What the AI needs from a generic engine

`HeuristicAgent` imports `rules` directly and calls `ValidateMeld`, `PlayerMeldCounts`,
`LayOffBreaksCleanRun`, `PenaltyPoints`. It is a rummy player, not an agent framework. In the
target architecture the `Agent` interface becomes module-scoped: each game module ships its own
policy, and the runtime only knows how to *drive* an agent. The generic contract a module must
expose for any agent — including a future search-based one — is:

```go
type Searchable interface {
    LegalActions(state State, playerID string) []Action  // enumerate
    Apply(state State, playerID string, a Action) (State, []Event, error)
    Clone(state State) State                             // cheap copy for rollout
    Determinize(state State, viewerID string, seed int64) State // resample hidden info
    Utility(state State, playerID string) float64        // terminal or heuristic value
}
```

Those five methods are enough for MCTS or a determinized rollout agent, and the engine already
satisfies four of them today. `Determinize` is the only genuinely new work, and the deterministic
`DeckSeed` plus append-only action log make it tractable.

---

## 6. Separation audit

Authority is clean: no client can make an illegal move stick, and every client re-renders from
server state. What is *not* clean is that each client independently re-implements enough of the
rules to decide what to *show* — which button is enabled, what a card is worth, what the contract
is called, whether the discard pile is locked. Every one of those is a second implementation of
a rule that can drift from the first.

### Rule knowledge living outside `internal/rules`

| Location | Duplicated rule knowledge | Class |
|---|---|---|
| ~~`client-tui/ui/helpers.go` `roundRequirementLabel()`~~ | **Fixed.** Deleted — the header reads the `contract` the server resolved for this deal | — |
| `client-tui/ui/helpers.go` `approximateNaturalValue()` | a second card-scoring table, for the live "natural value" readout | duplicated scoring — prices an unsent selection, so it needs Phase 2.3's preview, not an offer |
| ~~`client-tui/ui/game.go`~~ | **Fixed.** `dealHeaderLabel` now takes the resolved ruleset, not a profile name: a match with no fixed deal count no longer claims one, and a five-deal profile labels correctly | — |
| ~~`client-react-native/src/lib/cards.ts` `rulesSummaryLines()`~~ | **Fixed.** Reads `state.rules` field by field; a test renders a profile no client code has heard of | — |
| `…/cards.ts` `parseCard`, `rankOrder`, `autoOrganizeHand` | card string encoding, rank ordering, what "set material" and "consecutive run" mean | presentation-adjacent — §7.1 |
| ~~`app/game/[gameId].tsx` `canLayOff`~~ | **Fixed.** Reads the per-meld `lay_off:<id>` offers — which are also *stricter*, since the server disables a meld nothing in hand fits | — |
| ~~`app/game/[gameId].tsx` `discardLocked`~~ | **Fixed.** Reads `draw:discard`'s `enabled`/`whyNot`; the reason is rendered from the engine's own code | — |
| ~~`MeldTable.tsx`~~ | **Fixed.** Reads the per-meld `swap_joker:<id>` offer, so the control appears only where a card in hand takes that joker's place | — |
| `app/lobby/create.tsx` | `MELD_MINS = [0,35,50,70]`, `DISCARD_LOCK_ROUNDS = [0,1,2,3]` — the option space of a rule, as a client constant | config schema in UI — §7.5 / Phase 2.1 |
| `client-tui/internal/render/cards.go` | card parsing and suit symbols — a third implementation of the encoding | presentation-adjacent |

### The root cause is the wire format, not the clients

`GameStateMsg` ships *facts* — hand, piles, melds, counters, flags — and no *affordances*. It
says `phase: "meld"` and `roundReqMet: {…}` and leaves every client to work out that those two
imply "the lay-off button should be enabled". The three special-cased booleans that *are*
shipped — `canUndoDiscardDraw`, `canUndoLayOff`, `canUndoLayMeld` — prove the point: someone hit
the drift problem three times and patched it three times, one flag at a time, instead of
generalising.

The generalisation is the core proposal of §7: **ship the legal action list.** Then "which
button is enabled" stops being a rules question the client answers and becomes a lookup.

> **Shipped in Phase 1.** `GameStateMsg.legalActions` now carries the full offer set — every
> affordance, enabled or not, each disabled one with the engine's own reason. The three
> `canUndo*` booleans survive on the wire for older client builds but are *read back out of the
> offer list* rather than recomputed, so they are a second spelling rather than a second
> implementation. Both clients gained a pure-lookup module (`src/lib/offers.ts`,
> `client-tui/ui/offers.go`) whose defining property — it holds no rule knowledge at all — is
> asserted by a test that greps its own source. That test is what caught the error-message
> wording sitting in the wrong file.

### Two more structural blockers

- **The wire vocabulary is rummy.** `WSIncoming` has fixed fields `from`, `cards`, `meldId`,
  `card`, `position`. A trick-taking game has no place to put "I bid 2 hearts". Actions need a
  typed-but-open payload.
- **No i18n boundary.** Rule text, contract names and error messages are English string literals
  scattered across Go and TypeScript. A game called Žolíky with Czech AI names has no path to a
  Czech UI today. Module-supplied message keys fix this in the same move as the duplication.

---

## 7. Target architecture

One new interface, one new message shape. Everything else is deletion.

```
TODAY                                    PROPOSED
┌────────────────────────────────┐       ┌────────────────────────────────┐
│ client screen                  │       │ client shell                   │
│ renders + re-derives legality  │       │ renderer + dispatcher          │
│ + owns rule text tables        │       │ knows zones/cards/gestures,    │
└──────────────┬─────────────────┘       │ knows NO rules                 │
   raw facts, no affordances             └──────────────┬─────────────────┘
┌──────────────▼─────────────────┐        ViewModel + LegalActions[]
│ GameStateMsg — 24 rummy fields │       ┌──────────────▼─────────────────┐
│ + 3 ad-hoc canUndo flags       │       │ protocol: zones · groups ·     │
└──────────────┬─────────────────┘       │ actions · prompts (agnostic)   │
┌──────────────▼─────────────────┐       └──────────────┬─────────────────┘
│ game pkg — hand-mapped to      │       ┌──────────────▼─────────────────┐
│ models.Game (30-field mappers) │       │ match runtime — generic,       │
└──────────────┬─────────────────┘       │ opaque state blob              │
┌──────────────▼─────────────────┐       └──────────────┬─────────────────┘
│ rules pkg — ONE game,          │            GameModule interface
│ config-tunable                 │       ┌───────────────┬────────────────┐
└────────────────────────────────┘       │ module: zolik │ module: prší   │
3 clients × N rules = 3N drift sites     │ (today's      │ (new game,     │
A new game family = a new everything     │  rules pkg)   │  zero UI work) │
                                         └───────────────┴────────────────┘
```

### 7.1 Layer 0 — card primitives

A tiny `cards` package with `Card`, `Rank`, `Suit`, deck builders and a seeded shuffle. Card
identity becomes a struct (or a typed string with one parser), so `"TS"` is parsed in exactly one
place instead of four (`rules/meld.go`, `tui/render/cards.go`, `tui/ui/game.go`,
`rn/src/lib/cards.ts`). Games with different decks — 32-card Mariáš, tarot, Uno — need only a
different deck builder.

### 7.2 Layer 1 — the `GameModule` interface

This is the whole proposal. Every game implements it; the runtime knows nothing else.

```go
type GameModule interface {
    // identity + what the lobby may configure
    Descriptor() ModuleDescriptor          // id, display keys, player range, option schema

    // lifecycle — State is opaque to the runtime (json.RawMessage or any)
    NewMatch(opts Options, players []PlayerRef, seed int64) (State, error)
    Apply(s State, playerID string, a Action) (State, []Event, error)

    // the two projections the runtime fans out
    View(s State, viewerID string) ViewModel
    LegalActions(s State, playerID string) []ActionOffer

    // optional capability: an agent that can play this module
    NewAgent(difficulty string) Agent
}
```

Today's `rules` package satisfies `NewMatch` and `Apply` already — `ApplyAction` *is* `Apply`.
The new work is `View`, `LegalActions` and `Descriptor`, all pure functions over state that the
engine has everything it needs to compute.

### 7.3 The view model

A card game's board is, universally, *zones holding cards* plus *groups within zones*. That is
enough vocabulary for rummy melds, trick piles, bidding boxes and fanned hands alike.

```go
type ViewModel struct {
    Zones   []Zone     // hand, draw pile, discard pile, a player's melds, a trick
    Prompts []Prompt   // "your pickup must go into your initial meld" — key + params
    Status  []StatRow  // scoreboard rows the client tabulates without interpreting
    Header  []Fact     // deal 3 of 7 · round 2 · deck 41 — labelled, pre-formatted
}

type Zone struct {
    ID       string
    Kind     ZoneKind   // hand | stack | pile | spread | slot
    OwnerID  string     // "" = table
    LabelKey string     // i18n key, never a rendered English string
    Cards    []CardView // face-up entries; hidden zones send Count instead
    Count    int
    Groups   []Group    // melds/tricks: id, kind, badge keys, card index ranges
}
```

`ViewModel` is where hidden-information filtering happens — it replaces `BuildGameStateMsg`'s
hand-written per-viewer logic with a module responsibility, which is the only place that knows
what is secret in *that* game.

### 7.4 The action offer — the key idea

Instead of the client asking "may I?", the server answers before it is asked:

```go
type ActionOffer struct {
    ID       string      // opaque; the client echoes it back verbatim
    Verb     string      // "draw" | "play_group" | "extend_group" | "discard" | "bid"
    LabelKey string      // "action.lay_off"
    Enabled  bool
    WhyNot   string      // message key when disabled — the UI shows a real reason

    // what the gesture layer needs to bind this to the board
    Source   Selector    // zone + how many cards + which are eligible
    Target   Selector    // zone/group + drop hints ("front"/"end")
    Params   []ParamSpec // for non-card inputs: bid value, trump choice
}
```

What each client then implements, once, forever:

- Draw zones and cards from `ViewModel`.
- A card is draggable if some enabled offer's `Source` matches it; a zone is a valid drop target
  if that offer's `Target` matches. Highlighting, hover feedback and the "wrong end of the run"
  affordance all fall out of matching, not out of rules.
- On drop, send `{offerId, cards, target}`. On tap of a listed offer, send it.
- Render `WhyNot` when the player pokes a disabled offer.

What disappears: `canLayOff`, `discardLocked`, the three `canUndo*` flags,
`roundRequirementLabel`, `rulesSummaryLines`, `approximateNaturalValue`, the
`startsWith('JOKER')` checks, and the `MELD_MINS` constant list. That is most of §6's table,
deleted rather than fixed.

### 7.5 Lobby options from the descriptor

`ModuleDescriptor` carries the option schema — name, type, allowed values, default, label key —
so the lobby screen renders whatever knobs the module declares. Adding `TargetScore` to a profile
becomes a server-only change; the create-lobby screen picks it up with no edit.

### 7.6 Persistence

`models.Game` splits in two: a generic `Match` envelope (id, module id, options, players, turn
order, status, version, action log, timestamps) and an opaque `state` sub-document the module
owns. That deletes the `toRulesState`/`fromRulesState` pair and its whole class of
forgot-to-map-a-field bug. Existing Žolíky documents migrate by moving today's rummy columns
under `state` — a one-shot script, since the runtime never reads inside that blob.

`internal/stats` is already on the far side of that split. It reads `models.Game` only through
`BuildScoreboard`, which touches nothing but turn order, per-deal scores, the roster and the
action log — all envelope fields, none of them rummy-specific. When the envelope arrives, the
statistics subsystem follows it rather than the module.

### 7.7 What each client becomes

| Client | Keeps | Loses |
|---|---|---|
| React Native | drag/drop engine, card art, staging pane, animations, layout, theme | rule tables, legality expressions, profile text, contract labels |
| TUI | ASCII card renderer, keybindings, screen flow | contract table, natural-value calculator, "of 7" header |
| Defold | — (greenfield: implements the shell only) | never learns any rules at all |

This is what makes "swap the UI for a different game" real: the RN client's meld staging area
becomes a *group-building zone*, which a trick-taking module simply never offers, and a climbing
game offers with different validity. Same component, different offers.

---

## 8. Migration plan

Five phases, each shippable, none requiring a big-bang rewrite. Žolíky stays playable throughout.

### Phase 0 — Repair the config split brain ✅ **done**

Delivered, along with the rest of §10:

- `models.Game.Rules` persists the resolved `RulesConfig`; `game.GameRules` reads it back and
  `ResolveProfile` is no longer called on the load path. Pre-existing documents migrate on load
  and are written back on their next action.
- `GameState.InitialMeldMinimum` / `.DiscardDrawMinRound` are deleted — the engine reads
  `cfg`. The same duplicates are gone from `ai.VisibleState`.
- `engine.go`'s `GameNumber == 7` go-out check is now `cfg.IsFinalDeal(...)`.
- `rules.StartMatch` gives the opening deal and every later deal one shared implementation.
- `draw_card` carries its target card, so deep discard-pile pickup works from the wire.
- The AI's meld search is order-deterministic and its unread RNG is gone.
- The TUI header no longer claims a seven-deal match for every profile.

This unblocks adding a third profile: a house-rule override now survives a reload, and editing a
shipped profile constant can no longer change a game already in progress.

### Phase 1 — Ship `legalActions` alongside today's state ✅ **done**

`rules.LegalActions(state, player) []ActionOffer` now ships on every `GameStateMsg`, along with
the resolved ruleset and the current deal's contract. Both clients read it; the local derivations
are deleted, not synced.

The construction is what matters: every enabled/disabled decision is produced by *probing the real
validator* against a cloned state and reading back its `RulesError`, so `LegalActions` cannot
disagree with `ApplyAction` — it is not a second opinion, it is `ApplyAction`'s own answer asked in
advance. `rules/offers_agreement_test.go` cross-checks the two across a corpus of states, both
players and every concrete action, and a companion test asserts the corpus actually reaches every
verb and every `whyNot` code so the agreement cannot pass vacuously.

See [`extensibility-plan.md`](./extensibility-plan.md) §1 for the full task breakdown, what was
found along the way, and what was deliberately left for Phase 2.

### Phase 2 — Ship `ViewModel` alongside `GameStateMsg` ✅ **done** (2.4 folded into Phase 3)

Emit both from the same state; keep the old message for the TUI while the RN client migrates zone
by zone. Move the profile rule text and contract labels into `ModuleDescriptor` as message keys,
and add a locale bundle — the point at which a Czech UI becomes possible.

### Phase 3 — Extract the runtime from the game ✅ **done**

Introduce `GameModule`, register today's rules package as module `"zolik"`, and make
`Manager`/`Hub`/repository generic over the opaque state blob. Split `models.Game` into envelope
+ state and migrate existing documents. Retire `GameStateMsg` and the old
`toRulesState`/`fromRulesState` pair.

Test strategy: the 80 existing rules tests are untouched (they call the engine directly), and a
golden replay over the action log of a recorded match proves the runtime swap changed no
behaviour.

### Phase 4 — Prove it with a second module ✅ **done**

Implement a deliberately different, small game — Prší (Czech Mau-Mau) is the right choice:
shedding rather than melding, no draw/meld/discard phases, suit-and-rank matching, special-card
effects. If it needs no client change beyond assets, the abstraction holds. If it does, the gap
it exposes is cheap to fix at that size.

Move `Agent` under the module in the same phase, and take the low-cost AI wins from §5.6
(variable-length meld enumeration, deterministic lay-off ordering, joker swap).

### Sequencing note

Phases 1 and 2 deliver the stated goal — rules/UI separation and a swappable UI — and do not
require the module abstraction at all. Phase 3 only pays off if a genuinely different game is
actually coming; if the roadmap is "more rummy variants", stop after Phase 2 and spend the
remaining effort on `RulesConfig` knobs instead.

---

## 9. Risks & open decisions

| Risk | Shape of the problem | Mitigation |
|---|---|---|
| Offer explosion | Enumerating every legal meld from a 13-card hand is combinatorial — the AI's own search caps at 200k nodes for the same reason. | Offers describe *shapes* (selector predicates), not enumerated card sets. "Any 3+ cards from your hand forming a valid group" is one offer; the server validates the concrete submission. |
| Latency / feel | A drag that must round-trip to learn it is legal will feel worse than today's local check. | Offers arrive with each state push, before the gesture starts — the client answers locally from the offer list. No extra round trip. |
| Over-abstraction | A generic protocol built against one game usually fits exactly one game. | Phase 4 exists to falsify this, and is deliberately a very different game. Do not design Phase 3's interface without sketching Prší against it first. |
| Two clients, one migration | RN and TUI must both keep working through Phases 1–3. | Every phase ships the new shape *alongside* the old; the old is removed only after both clients have moved. |
| Replay compatibility | The action log is persisted history; changing the action vocabulary breaks old replays. | Version the log per module. Old entries stay readable by the module version that wrote them. |

**Open decisions**

1. ~~**How different is "completely different"?**~~ **Answered by building it.** Prší now runs
   behind the same module contract as Žolíky, and a driver that reads only the offer list plays
   both. The interface bent in exactly three places, all recorded in the plan's §4.x — an offer
   parameter that is not a card, an action naming its offer, and a variation that is a name
   rather than a number. None of them were visible from rummy alone, which is the argument for
   having written the second game before fixing the interface.
2. **Do modules load in-process or out?** In-process Go packages (proposed) are simplest. A
   plugin/WASM boundary would let third parties add games at a large cost in complexity.
   Recommend in-process until there is a second author.
3. **Does the client get any predictive rendering?** The offer list makes optimistic UI possible
   (show the meld on the table before the server confirms). Worth it for feel; adds rollback
   complexity. Recommend deferring past Phase 2.
4. **What happens to the standalone scoring module?** `internal/scoring` hardcodes 7 rounds and is
   unrelated to live games. Either generalise it alongside the module descriptor or split it out
   as its own small product.

---

## 10. Defects found — all fixed

Seven defects were found while reading. All are now fixed, each with a regression test that was
verified to fail against the old code before the fix went in.

| Was | Fix | Guarded by |
|---|---|---|
| **`rules/engine.go` hardcoded the final deal.** `ns.GameNumber == 7` where `cfg.IsFinalDeal(...)` was meant. Harmless for both shipped profiles by coincidence, but a profile with `FixedDealCount != 7` would let a player meld away their last card on the final deal without the deal ever ending — leaving the match wedged with a player holding no cards and no legal move. | Both go-out checks now ask the ruleset. | `rules/final_deal_test.go` — 4 tests, incl. a five-deal custom profile and the score-limited case that must *never* end this way |
| **The config split brain.** `toRulesState` re-derived `RulesConfig` from the profile *name* on every load, so per-game overrides were silently discarded, and editing a shipped profile constant retroactively changed in-flight matches. The engine also read the two duplicated knobs off `GameState`, not off the config. | The resolved config is frozen onto the document at creation (`models.Game.Rules`) and read back by `game.GameRules`. The `GameState` duplicates are deleted, so `RulesConfig` is the only home for a rule. Legacy documents migrate on load. | `game/rules_config_test.go` — 10 tests, incl. a full save/load cycle proving the *engine* enforces a house-rule floor, and a round-trip test that fails if a config field is added without a persisted mirror |
| **`draw_card` dropped its target card.** `toRulesAction` built the action without `Card`, so `ValidateDraw`'s `targetCard` was unreachable from any client and `zolik_classic`'s "take any card from the pile" rule was dead code. | The field is carried in both directions (client → engine, and the AI's action → wire). | `game/draw_target_test.go` — 5 tests, incl. an end-to-end deep pickup and a check that a top-only profile still can't be bypassed |
| **TUI header claimed a seven-deal match for every profile.** `"Game N of 7: <contract>"` printed unconditionally; under `zolik_classic` both halves were wrong, and past deal 7 it read "Game 9 of 7". | `dealHeaderLabel(profile, game)`, mirroring the RN client's existing helper. | `client-tui/ui/helpers_test.go` — 4 tests, incl. the past-deal-7 case |
| **`HeuristicAgent.rnd` was dead.** Seeded from the wall clock and never read, implying a variability that never existed. | Removed; the agent's determinism is now documented and asserted. | `ai/determinism_test.go` — `TestChooseAction_IsDeterministic` across all three difficulties |
| **AI meld search depended on Go map iteration order.** `findLayOff` and `extendsAnyLiveMeld` ranged a map, so the same position could produce different play on different runs — untestable at the margin and unreproducible from a bug report. | Owners are visited in sorted order. | `ai/determinism_test.go` — 200-iteration stability check plus a test pinning *which* order was chosen |
| **`startGame` duplicated the deal setup.** It hand-rolled what `rules.StartNextGame` already did, with a slightly different field set (`MeldMeta` built without `WildCount`) — two paths that had to be kept in sync by hand. | Both go through `rules.dealNewGame`, exposed as `rules.StartMatch` for the opening deal; the REST handler now just calls `fromRulesState`. | `rules/start_match_test.go` — 4 tests, incl. one asserting the opening deal and a later deal produce the same shape, and that stale undo snapshots don't leak across a deal boundary |

### What the fixes changed structurally

Three of the seven were not really bugs but *duplication* — the same rule expressed twice and
allowed to drift. Each fix deleted the second copy rather than syncing it, which is the same
move §7 makes at a larger scale:

- The rule value lived on both `RulesConfig` and `GameState` → one home.
- The deal setup lived in both the rules package and the REST handler → one implementation.
- The deal label lived in both clients (and disagreed) → still two, and will stay on §6's list
  until the module descriptor owns rule text.

### State of the tests

`go test ./...` is green across the server and TUI, and RN's Jest suite passes. Counts after
this work:

| Package | Tests | Change |
|---|---|---|
| `internal/rules` | 89 | +9 |
| `internal/game` | 24 | +15 |
| `internal/ai` | 16 | +4 |
| `client-tui/ui` | 4 | +4 (first tests in this package) |

Rules coverage was already good — ace placement, clean-run preservation, contract progression
and the undo windows all have dedicated cases — and the new tests deliberately went to the
*layers* that had none: the config persistence round trip, the wire translation, and the TUI's
label logic.

The remaining gap is unchanged: no test drives `Manager.HandleAction` end to end (it needs a
Mongo repository), and the RN client's coverage is still just `cards.test.ts` plus the Playwright
e2e suite. Phase 1's `LegalActions` is the natural place to add the first true cross-layer test.

---

## 11. Match records and lifetime statistics

Two collections, and the direction between them matters:

| Collection | Written | Shape |
|---|---|---|
| `match_results` | once, when a game reaches `completed` | immutable: final standings, roster, ruleset, composition |
| `player_stats` | on every match, per participant | lifetime aggregates, derived from the above |

The aggregates are a **cache** of the records, never the reverse. A wrong average can be rebuilt
by replaying `match_results`; a lost record cannot be recovered from an average. That is why
recording inserts the record first and only then folds it into the aggregates: whoever loses the
unique-`gameId` race stops before touching a counter, which is what makes a retried or
concurrently-observed completion idempotent instead of a double count.

### The subject — who a statistic belongs to

A seat is held by one of three things, and lifetime statistics need an identity that spans all
three. `stats.Subject` supplies it:

| Kind | Key | Durable? | Why |
|---|---|---|---|
| user | `user:<oid>` | yes | the account is the identity |
| ai | `ai:<difficulty>` | yes | bot *instance* IDs are minted per lobby, so aggregating on them would produce a new one-match "player" every game. Difficulty is what is stable, and what anyone actually wants a number for |
| guest | — | no | a guest name is claimed per session and two people can hold the same one; a durable record keyed on it would merge strangers |

A guest still appears in the match record and still counts as a human opponent in everyone
else's split — they simply carry no record of their own.

Bots keeping a record on the same footing as people is deliberate, and it is what makes the
feature symmetric. "You are 4–11 against hard" comes out of the same data as any player rivalry,
and the AI's own aggregate win rate becomes a tuning signal that costs nothing extra to collect.

### The splits

`vsHumans` and `vsAI` **overlap** rather than partition: a mixed table counts in both, because
the question worth answering is "was a person involved", not "was the table pure". Alongside
them the record keeps per-difficulty, per-profile and per-table-size buckets, plus a head-to-head
map decided *pairwise on final total* — so in a four-player match the players who came third and
fourth still have a meaningful record against each other.

Per-profile buckets exist because a Continental total and a Žolíky total are not the same
currency: one is seven deals of penalty, the other is a race to 200. Averaging them would produce
a number that means nothing.

### One scoring path, live and final

`BuildScoreboard` is pure, and serves both the running table and the permanent record. During
play it names a provisional leader with `rules.DetermineMatchWinner` — the very rule that will
settle the match — and once complete it takes the winner off the document rather than
recomputing it, so a record can never disagree with the match the players watched end. The
`rules` helpers it calls (`DealsWonByPlayer`, `DetermineMatchWinner`) are the raw forms of the
engine's own functions, taking just the fields the decision depends on, so there is no second
implementation of "who is winning" to drift.
