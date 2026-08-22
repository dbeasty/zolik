# Extensibility Plan — task-level execution

Companion to [`architecture.md`](./architecture.md). That document argues *what* should change
and *why*; this one is the build order — the concrete tasks, the file-level surface each touches,
the tests that prove it, and the exit criterion that says a phase is done.

- **Baseline:** `main` @ `8cb3ab5`
- **Goal:** the same engine runs a different card game behind a different UI, and rules stop
  being re-implemented in three clients.
- **Non-goal (for now):** a plugin/WASM module boundary, third-party game authors, predictive
  client rendering. See `architecture.md` §9 open decisions.

---

## 0. The one rule this plan enforces

> **Every rule-derived fact is computed once, on the server, inside the ruleset — and shipped.
> A client that has to *derive* a rule is a bug.**

Everything below is an application of that sentence. The measure of progress is not how much new
abstraction exists; it is how many rows disappear from `architecture.md` §6's leak table.

Three distinct kinds of leak, which need three different fixes:

| Leak class | Example today | Fix | Phase |
|---|---|---|---|
| **Legality** — "may I?" re-derived in UI | `canLayOff = isMyTurn && phase==='meld' && roundReqMet[me]` | ship the legal-action list | 1 |
| **Configuration** — a profile's constants re-typed in the client | `rulesSummaryLines()`'s `isZolik ? '13 cards' : '12 cards'` | ship the resolved ruleset | 1 |
| **Presentation** — rule *text*, labels, i18n | `roundRequirementLabel()`'s 7-deal contract table | ship message keys from a module descriptor | 2 |

---

## Phase 0 — Repair the config split brain ✅ done

Shipped in `8cb3ab5`. See `architecture.md` §8 and §10. Summary: the resolved `RulesConfig` is
frozen onto the match document and is the single home for a rule value.

---

## Phase 1 — The server answers "what may I do?" ✅ done

**Thesis.** `GameStateMsg` today ships *facts* and no *affordances*, so each client re-derives
legality. Phase 1 makes affordances a server output. It touches no persistence, no action
vocabulary, and no turn model, so it is safe to ship on its own and is independently valuable
even if Phases 2–4 never happen.

### 1.1 `rules.LegalActions` — new file `server/internal/rules/offers.go`

```go
func LegalActions(state GameState, playerID string) []ActionOffer
```

Pure, no I/O, same package as the validators. Returns the **complete** offer set every time —
disabled offers included, each carrying a `WhyNot` error code — because "greyed out with a
reason" is a UI requirement, and an omitted offer is indistinguishable from a client bug.

Offer identity is stable and content-addressable so a client can diff across pushes:

| Offer ID | Verb | Source | Target |
|---|---|---|---|
| `draw:deck` | `draw` | `deck` | — |
| `draw:discard` | `draw` | `discard_pile` (eligible pile cards) | — |
| `lay_meld` | `lay_meld` | `hand`, `minCards`…`maxCards` | table |
| `lay_off:<meldId>` | `lay_off` | `hand` (eligible cards + per-card run ends) | that meld |
| `swap_joker:<meldId>` | `swap_joker` | `hand` (cards that fit the joker's slot) | that meld |
| `discard` | `discard` | `hand` (legally discardable cards) | `discard_pile` |
| `undo:draw_discard` `undo:lay_off` `undo:lay_meld` `undo:turn` | `undo` | — | — |

**Anti-drift construction — the load-bearing design decision.** Coarse gating (is this verb
available at all, and if not, why) is produced by *probing the real validator* against a cloned
state and reading back its `RulesError`. `LegalActions` therefore cannot disagree with
`ApplyAction`: it *is* `ApplyAction`'s answer. It never re-states a rule in a second place, which
is precisely the failure mode this whole plan exists to end.

Fine-grained per-card eligibility (which hand cards may extend *this* meld) uses the pure,
state-free helpers the AI already uses — `ValidateMeld`, `LayOffBreaksCleanRun`, `IsJoker` — so
no state clone is needed per card.

**Cost control.** Per-card eligibility is computed only for the player whose turn it is; every
other viewer gets the offer set with `Enabled: false` / `WhyNot: NOT_YOUR_TURN` and no card
lists. Worst case per broadcast is therefore one player × (hand × table melds) pure meld
validations, and coarse probes are bounded at roughly a dozen state clones. Guarded by a
benchmark (`BenchmarkLegalActions`), not by hope.

**Deliberate limit.** `placements` lists cards legal *on their own*. A multi-card lay-off or a
meld shape is not enumerated — `architecture.md` §9's offer-explosion risk — so offers describe
*shapes*, and the server still validates the concrete submission on arrival. Authority stays
with `ApplyAction`; offers are a rendering input, never a permission grant.

### 1.2 Share the run-end resolution

`ValidateLayOff` computes whether a card grows a run's front or end. The offer's `positions`
hint needs the same answer. Extract `runGrowthSides(prevCards, newCards)` and call it from
both — one implementation, not two that agree today and drift tomorrow.

### 1.3 Ship the resolved ruleset and the current contract

Add to `GameStateMsg`:

- `rules` — the resolved `RulesConfig` (public; contains no hidden information).
- `contract` — `cfg.ContractFor(gameNumber)`, i.e. the sets/runs/clean-run requirement *for this
  deal*, resolved. Kills the client-side contract tables.
- `legalActions` — the offer list from 1.1.

The legacy scalars (`initialMeldMinimum`, `discardDrawMinRound`) and the four `canUndo*` booleans
stay on the wire for compatibility, but are **derived from the new fields** rather than computed
independently, and are marked deprecated. One implementation, two spellings.

### 1.4 Client adoption — delete, don't sync

| Client site | Deleted | Replaced by |
|---|---|---|
| `app/game/[gameId].tsx` `canLayOff` | legality expression | `offer('lay_off:*').enabled` |
| `app/game/[gameId].tsx` `discardLocked` | legality expression | `offer('draw:discard').whyNot === 'DISCARD_LOCKED'` |
| `app/game/[gameId].tsx` `canTakeDiscard` / `canDrawDeck` | legality expressions | the two `draw:*` offers |
| `app/game/[gameId].tsx` `canDiscardNow` | legality expression | `offer('discard').enabled` |
| the four `state.canUndo*` reads | ad-hoc flags | `offer('undo:*').enabled` |
| `MeldTable.tsx` `startsWith('JOKER')` | legality guess | `offer('swap_joker:<id>')` presence |
| `cards.ts` `rulesSummaryLines()` profile table | duplicated profiles | `state.rules` field reads |
| `cards.ts` / TUI `roundRequirementLabel()` | duplicated contract table | `state.contract` |

New client module `src/lib/offers.ts` (RN) and `ui/offers.go` (TUI): pure lookup helpers over the
offer list. They contain **no rule knowledge** — that is the acceptance test for the file.

`approximateNaturalValue()` (TUI) is a live readout of what the *currently selected* cards are
worth, i.e. a preview of a submission the server has not seen. It cannot be a pushed fact and is
explicitly deferred to Phase 2 (see 2.3).

### 1.5 Tests

| Level | What it proves | Where |
|---|---|---|
| Unit — rules | Every offer's `enabled`/`whyNot` matches what `ApplyAction` actually does, across both shipped profiles and a custom third | `rules/offers_test.go` |
| Unit — **agreement (the important one)** | For a corpus of states × players × candidate actions, `LegalActions` and `ApplyAction` never disagree. This is the test that makes drift impossible rather than merely unlikely | `rules/offers_agreement_test.go` |
| Unit — wire | `BuildGameStateMsg` emits offers per viewer; hidden information does not leak through them; legacy flags equal their offer counterparts | `game/legal_actions_test.go` |
| Unit — RN | `offers.ts` lookups; `rulesSummaryLines` reads config, not a profile name | `src/lib/offers.test.ts`, `cards.test.ts` |
| Unit — TUI | offer-driven gating helpers | `client-tui/ui/offers_test.go` |
| Bench | offer computation stays cheap enough for every broadcast | `rules/offers_bench_test.go` |
| **E2E** | the real browser UI enables/disables the real controls from real server offers | `e2e/tests/legal-actions.spec.ts` |

### Exit criterion

`grep` for a rule expression in either client returns nothing outside the offer-lookup modules;
both clients pass their suites; the e2e suite drives lay-off, discard-lock and undo affordances
purely from offers. Eight rows leave §6's leak table.

### 1.6 Outcome

Delivered as planned, with three deviations worth recording.

**The offer list turned out to be *sufficient*, not just correct — and that is testable.**
`game/offer_driven_play_test.go` plays whole games with a client that may read the offer list and
nothing else, through the real wire path, and fails the moment an offered action is refused or the
offers leave the player on turn with nothing to do. Under `zolik_classic` it plays matches to
completion. Under `continental` no player ever gets down: clearing a 35-point floor with two melds
laid in a single turn is beyond a 40-line shape-matcher, and the test asserts that honestly
(`wantGoDown` is false there) rather than hiding it behind a loose bound. That is a limitation of
the test client, not of the offers — `internal/ai` is what plays Continental well, via a real
search. What Continental does contribute is the harder offer surface: it is the profile that
produces the incomplete-initial-meld dead end below.

That test also found the one branch worth calling out: under a rotating contract, laying one set of
a required two leaves a player unable to discard. The client is not told that rule and does not
need to be — it can see `discard` is off and `undo:turn` is on, which is enough to recover. The
offer list is not only describing what is legal, it is describing the way out of a dead end.

**Two things were split out that the plan had lumped together.** The error-code *wording* went to
`src/lib/messages.ts` / `reasonMessages` rather than living in the offer-lookup modules — caught by
the no-rule-knowledge test, which is exactly the sort of thing it exists to catch, and it leaves
Phase 2's locale bundle a single file to replace. And `runGrowthSides` returns a `known` flag, not
just a side list: "cannot determine which end" and "grows both ends" are different answers, and
collapsing them would have made the client send a position the validator rejects.

**A hazard this work created, now guarded.** `toRulesState` hands the engine the match document's
own maps and slices by reference, and the validators mutate in place. Computing offers means
dry-running actions, so `BuildGameStateMsg` became the one read-only-looking function on the read
path that could corrupt a document just by rendering it. The probes clone;
`TestBuildGameStateMsg_DoesNotMutateTheGame` is what keeps them cloning. (This aliasing is
pre-existing and is what Phase 3's envelope/state split removes properly.)

**Two behaviour fixes fell out** rather than being coded: lay-off now honours the per-card run end
the server will accept (dropping on the other end used to bounce with `WRONG_RUN_END` even when the
move was legal), and "Swap joker here" is offered only where a card in hand actually takes that
joker's place.

**Left on §6's leak table**, deliberately: `approximateNaturalValue` (TUI) and the RN staging
area's local validity guess. Both price a submission the server has not seen, so they need Phase
2's preview round-trip (2.3), not the offer list. Both clients still hold their own card-string
parsers and rank ordering — presentation-adjacent, and Phase 2/§7.1's job.

---

## Phase 2 — The module descriptor and the view model ✅ done (2.1–2.3), 2.4 folded into Phase 3

**Thesis.** Phase 1 removes derived *legality*. Phase 2 removes derived *text and shape*.

### 2.1 `ModuleDescriptor`

`id`, `displayKey`, player range, and the **option schema** (name, type, allowed values, default,
label key). The create-lobby screen renders whatever the descriptor declares, so
`MELD_MINS = [0,35,50,70]` and `DISCARD_LOCK_ROUNDS` stop being client constants and adding a
knob becomes a server-only change.

### 2.2 Message keys, then a locale bundle

Every rule string the server ships becomes a key plus params (`contract.two_sets_of_3`,
`{n: 3}`), never a rendered English sentence. Clients own a bundle; Czech becomes possible in the
same move. `WhyNot` already ships as a `RulesErrorCode` in Phase 1, which is the same shape — so
Phase 2 is extending an established pattern, not inventing one.

### 2.3 Submission previews

Generalises the TUI's `approximateNaturalValue` and the RN staging area's local validity guess:
the client sends a *candidate* (`preview` frame), the server answers with validity, natural
value, and the meld type it would resolve to. Removes the last duplicated scoring table. Cheap
because the engine's validators are already pure.

### 2.4 `ViewModel` alongside `GameStateMsg`

Zones / groups / prompts / header facts, per `architecture.md` §7.3, emitted from the same state.
Migrate the RN client zone by zone with the old message still flowing; the TUI follows. Hidden-
information filtering moves out of `BuildGameStateMsg` and becomes a module responsibility —
the only layer that knows what is secret *in that game*.

### Exit criterion

§6's leak table is empty. A Czech locale bundle renders the whole game. `GameStateMsg` has no
readers left in the RN client.

### 2.x Outcome

**2.1 (descriptor) and 2.2 (message keys) shipped.** `GET /module` declares the module and the
lobby renders from it; `rules.ValidateOptions` makes the declaration authoritative rather than
decorative, so a value the schema does not list is a 400 on both create and settings. Message
keys arrived with a working Czech bundle — the seam is real, not theoretical: a whole second
language turned out to be a client-only change with no server edit.

Two things the translation forced, which are worth keeping in mind for any further locale:
counts are whole phrases per count rather than a number glued to a pluralised noun (Czech
inflects the noun: *Jedna skupina* / *Dvě skupiny* / *Tři skupiny*), and lookup degrades
locale → English → caller fallback → key, because an untranslated string is a bad day and a
blank control is a bug report.

**2.3 (submission previews) shipped** and removed the last duplicated scoring table. It reports
`valid` and `playable` separately on purpose — a valid set is unplayable on someone else's turn,
and saying which is more useful than one greyed-out control.

**2.4 (ViewModel) was folded into Phase 3** rather than built twice. The module contract needed a
`View` method anyway, and building a second view shape against `GameStateMsg` first would have
been throwaway work. `module.ViewModel` is that deliverable.

---

## Phase 3 — Extract the runtime from the game ✅ done

**Thesis.** Everything above works with one hardcoded game. Phase 3 makes the runtime not know
which game it is running.

1. Define `GameModule` (`architecture.md` §7.2). Register today's `rules` package as module
   `"zolik"`. **Sketch Prší against the interface before writing it** — an interface designed
   against one game fits exactly one game.
2. Split `models.Game` into a generic `Match` envelope + an opaque module-owned `state`
   sub-document. Deletes `toRulesState`/`fromRulesState` and its whole forgot-to-map-a-field bug
   class. One-shot migration of existing documents.
3. Make `Manager`/`Hub`/repository generic over the opaque blob.
4. Move `ai.Agent` under the module; the runtime only knows how to *drive* an agent.
5. Version the action log per module so old replays stay readable.

**Test strategy:** the rules tests are untouched (they call the engine directly). A golden replay
over a recorded match's action log proves the runtime swap changed no behaviour — this is the
acceptance test for the whole phase.

### 3.x Outcome

`internal/module` defines the contract; `internal/zolikmod` adapts the existing rules engine to
it as an **adapter, not a rewrite** — `internal/rules` is the mature, heavily-tested part of this
codebase and reshaping it to fit a new interface would have risked exactly the behaviour the
interface exists to preserve.

`models.Match` is the generic envelope and `internal/match` is the runtime that hosts it, mounted
at `/modules`, `/matches` and `/ws/matches`. It mirrors `internal/game`'s shape deliberately —
load, apply, persist under a version check, broadcast per viewer — because those are *runtime*
properties, not rummy ones. What changed is the middle: the game-specific step is a call into
whichever module owns the match, and per-viewer filtering moved into the module, since only the
module knows what is secret in its own game.

**Shipped alongside, not instead of.** The existing Žolíky routes, documents and clients are
untouched, and `matches` is a separate collection from `games`. Migrating the rummy documents is
a one-shot script worth running once the module path is the live one — see the remaining work
below.

**A property that came for free.** Behind an opaque blob, `Apply` decodes a fresh value, so it
cannot mutate its caller's state. The rummy engine mutates in place and needed a regression test
to stop a read-only-looking function corrupting a document; that hazard cannot exist through the
module interface, and both modules are now pinned to it.

**Proven end to end**, not just in memory: `e2e/tests/match-runtime.spec.ts` seats two human
players on two real WebSockets and plays a whole Prší match to a winner, choosing every move from
the offer list alone, then checks the database agrees with what the sockets reported. It also
asserts through the real serialisation that a viewer never receives another player's cards.

### What is left

| Remaining | Why it was not done here |
|---|---|
| Migrate `games` documents into `matches` | A one-shot script, only worth running once the module path is the live one. Nothing depends on it yet. |
| Retire `GameStateMsg` and the `game` package | It still serves the shipped RN and TUI clients. Both would need to move to the generic `match_state` shape first. |
| A generic client shell | The RN client renders Žolíky specifically. A shell that renders `ViewModel` zones and offer buttons would let it play Prší with no new screen — the last claim in `architecture.md` §7.7 still untested in a browser. |
| Module-scoped agents | `ai.HeuristicAgent` is a rummy player behind a rummy interface. `/matches/{id}/add-bot` seats a body but does not drive it. |

---

## Phase 4 — Falsify the abstraction with a second module ✅ done

Implement **Prší** (Czech Mau-Mau): shedding, not melding; no draw/meld/discard phase triple;
suit-and-rank matching; special-card effects. Chosen because it shares almost no vocabulary with
rummy — which is the point. If it needs no client change beyond assets, the abstraction holds.

Take the cheap AI wins from `architecture.md` §5.6 in the same phase (variable-length meld
enumeration, joker swap), now that `Agent` is module-scoped.

### 4.x Outcome — the abstraction held, and bent in three places

`internal/prsi` implements Prší: 32 cards, no melds, no phase triple, no contract, and cards with
effects (a 7 makes the next player draw two and stacks; an Ace skips; a Queen is wild and names
the suit that follows).

**The proof** is `module.PlayWithOffers`: a driver that may read a module's offer list and nothing
else — it never decodes `State`, never names a rank, a suit, a meld or a phase — handed both
games. It plays Prší to a winner on every seed tried, exercising all three verbs, and takes real
turns in Žolíky without ever being refused an action it was offered.

It cannot go *out* in Žolíky, and that is the documented limit rather than a surprise: going out
needs a contract assembled from a meld *shape*, which the offer protocol deliberately does not
enumerate (see §1.1's offer-explosion note). A UI shell has a human supplying that shape; the
driver does not.

**Where the interface bent** — each of these is a thing rummy alone would never have asked for,
which is why the second game had to exist before the interface was fixed:

| What Prší needed | Why rummy never asked | Result |
|---|---|---|
| An offer parameter that is not a card | Playing a Queen names the suit that follows — a choice with no card to drag and no zone to drop on. Every rummy action's input is cards. | `module.ParamSpec` |
| An action naming the offer it came from | Žolíky has two draws sharing the verb `draw`; the verb alone cannot say which. Surfaced building the *adapter*, not Prší. | `Action.OfferID` |
| A variation that is a name, not a number | An int-keyed options map has nowhere to put a profile name. | `MatchConfig{Variation, Options}` |

**Where it did not bend:** zones/groups described a rummy table and a shedding pile without
change; hidden-information filtering moved into `View` cleanly for both; and the descriptor shape
expressed a game with *none* of Žolíky's knobs simply by declaring none.

---

## Sequencing and the stop point

Phases 1 and 2 deliver the stated goal — genuine rules/UI separation and a swappable UI — and
need no module abstraction at all. **Phase 3 only pays off if a genuinely different game is
actually coming.** If the roadmap is "more rummy variants", stop after Phase 2 and spend the
remaining effort on `RulesConfig` knobs and profiles, which is already cheap.

The decision point is the end of Phase 2, and it is the same open decision as
`architecture.md` §9.1. Nothing in Phases 1–2 is wasted either way.
