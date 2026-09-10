# Samba — implementation plan

Samba shipped as a third **variation of the Canasta module**, alongside `classic` and
`modern_american`.

- **Baseline:** `main` @ `88feae2`, rebased onto `6396cac` when the meld-zone label fix landed
  mid-flight.
- **Deliverable:** `variation: "samba"` on module `canasta` — 162 cards, sequence melds, a
  permanently frozen pile, a 10,000-point target, **two to six seats** — with the two existing
  variations playing **byte-identically** to how they play today, and the conformance driver
  finishing whole Samba matches from offers alone.
- **Source of rules:** pagat.com's Samba page, cross-read with Hoyle. Every number in §2 is from
  there; the four places the source is silent are called out in §9 as decisions rather than left
  to be discovered in a bug report.

---

## 1. Why a variation and not a seventh module

`docs/canasta-plan.md` §2 put "the Samba / Bolivia variant families" out of scope with the note
that each is *additive*. That was half right, and the half that was wrong is the interesting part.

Additive is true of the numbers: three decks instead of two, fifteen cards instead of eleven,
10,000 instead of 5,000, a 150-point initial-meld band on top of the existing four. Those are
already the shape `variationDefaults` has — a struct of ints resolved once in `NewMatch`.

What is *not* additive is that a Samba meld can be a **sequence**, and three of this module's
load-bearing assumptions are written against "a meld is n cards of one rank":

| Assumption | Where it lives | What Samba does to it |
|---|---|---|
| A meld is identified by its rank | `meldID(teamID, rank)`, `Team.meld(rank)` | two groups of one rank are legal, and a sequence has no single rank |
| A meld is at most seven cards and then closed | `Meld.closed()`, `canastaSize` | a group canasta stays open; a samba closes at seven |
| The pile is frozen *sometimes* | `GameState.Frozen`, the personal freeze | the pile is frozen *always*, and there is a second way to touch it |

None of that is a reason to fork the package. Partnership-owned melds, whole-pile capture, red
and black threes, the initial-meld floor as a property of a *turn*, deal rollover, intermissions
and the six-part settlement are all the same game, and a second copy of them is a second place
for them to drift. The work is to widen the meld model *inside* `internal/canasta` so that "a
group of a rank" is one kind of meld rather than the only kind — which is the same shape
`ginrummy.Meld` already has (`Kind: "set" | "run"`), so it is a shape this codebase has agreed on
once already.

The test that this was the right call is §7's exit criterion 3: `classic` and `modern_american`
must come out the other side identical, deal for deal, on fixed seeds.

---

## 2. Rules implemented

Everything below is Samba's; the middle column is what the module does today, kept so that the
diff between the two is the whole specification.

| | Classic / Modern American | **Samba** |
|---|---|---|
| Seats | 2–4 | **2–6** — partnerships at 4 and 6, individuals at 2, 3 and 5 |
| Deck | 108 — two decks, four jokers | **162 — three decks, six jokers** |
| Dealt | 11 (13 modern) | **15, and 13 at six seats** |
| Draw | one card | **two cards** |
| Melds | groups only | **groups and sequences** |
| Sequence | — | **3–7 natural cards, one suit, consecutive; no wilds, no threes; ace high, so 4-10 up to 8-A** |
| Wilds in a group | ≤ 3, and ≥ 2 naturals | **≤ 2, and naturals ≥ 2 × wilds** |
| Second group of a rank | refused (`RANK_ALREADY_MELDED`) | **allowed, kept separate** |
| Seven cards | canasta, closed | **group canasta stays open; a seven-card sequence is a _samba_ and closes** |
| Taking the pile | top + two naturals, or top + natural + wild, or lay the top card onto a meld — the last two only while unfrozen | **top + two naturals from hand, only. The pile is frozen all the time** |
| Second way in | — | **if an open sequence on the table takes the top card, you may take _that one card_ instead of drawing two — and the pile stays** |
| Initial meld floor | 15 / 50 / 90 / 120 at 0 / 1500 / 3000 | **15 / 50 / 90 / 120 / 150 at 0 / 1500 / 3000 / 7000** |
| Red threes | 100 each, 800 for all four; negative with no canasta | **100 each, 1000 for all six; negative without the canastas to go out** |
| Bonuses | natural 500, mixed 300, out 100, concealed 200 | **natural 500, mixed 300, samba 1500, out 200, no concealed bonus** |
| Going out | 1 canasta (2 modern) | **two, in any mix of canastas and sambas** |
| Target | 5000 | **10,000** |

Unchanged and worth saying so: card point values, black threes (melded only on the way out,
blocking the pile for one turn while they are the top card), red threes laying themselves down
and being replaced from the stock, the initial-meld minimum being a property of a whole turn,
stock exhaustion ending the deal with no going-out bonus, and the deal→match rollover.

---

## 3. Design

### 3.1 `variationDefaults` becomes a ruleset

Today it is three ints. It becomes the one place a variation's *shape* is written, read once in
`NewMatch` and frozen into `GameState` so a match in flight cannot change rules underneath a
deal — the property `Pause` already relies on:

```go
type ruleset struct {
    handSize, targetScore, canastasToGoOut   int
    decks, jokersPerDeck, drawCount          int
    sequences                                bool // sequence melds legal at all
    maxWildsPerGroup                         int  // 3 classic, 2 samba
    naturalsPerWild                          int  // 1 classic (">= 2 naturals"), 2 samba
    groupsPerRank                            int  // 1 classic, 0 = unlimited in samba
    groupCanastaCloses                       bool // true classic, false samba
    pileAlwaysFrozen                         bool
    meldFloors                               []floor // {score, minimum} bands
    sambaBonus, goingOutBonus, concealedBonus int
    handSizeAt                               map[int]int // seat count → hand size, where it differs
}
```

`redThreeAll` — the bonus for holding every red three — is deliberately *not* in there: with two
decks there are four of them and it is 800, with three there are six and it is 1000, so it is a
function of `decks` and computing it from the deck is one fact rather than two that can disagree.

`classic` and `modern_american` are populated with today's constants, so Phase 0 is a refactor
with no behaviour change and no test edits — which is exactly what makes it safe to land first.

The scalars a table may genuinely want to set stay **declared options**, not constants:
`handSize`, `targetScore` and `canastasToGoOut` already are, and `targetScore` gains a `10000`
choice so Samba's default is a value the descriptor allows (`conformance_test.go` enforces that,
which is how we find out if we forget).

### 3.2 Seating and partnerships

Partnerships are already built and are **not** variation-specific: `NewMatch` computes them once
from seat parity and stores them in `TeamOf`/`Teams`, so nothing else in the package re-derives
who is on whose side, and `View`, `scoreDeal` and `Standings` all loop over `s.Teams` rather than
counting to two. Samba inherits all of it unchanged — "your side needs two canastas to go out" is
already a team question.

What has to grow is the seat count. Today's rule (`teams := 2; if n == 3 { teams = 3 }`) is a
special case of a general one, and it generalises without a new concept:

```
teams := n                                  // odd, or heads-up: everyone is their own side
if n%2 == 0 && n >= 4 { teams = n / 2 }     // partnerships, partner i and i+teams
```

So 4 seats stay 0+2 vs 1+3, and 6 seats become three partnerships sitting opposite —
0+3, 1+4, 2+5. Five seats play as five individuals, which is exactly how three plays today; see
§9.4. The values at 2, 3 and 4 are unchanged, which the golden fixture in §6 enforces.

**Per-variation seat ranges — the one change outside the module.** `MinPlayers`/`MaxPlayers` live
on `ModuleDescriptor`, not on `VariationSpec`, and `manager.go` gates joins and starts on them.
Raising canasta's maximum to six would therefore raise it for `classic` and `modern_american`
too — and classic on 108 cards at six seats deals 66 and leaves a 42-card stock. Real 5–6 player
canasta uses three decks, which changes the meld arithmetic `cards.go` says is a property of the
deck; reconstructing that ruleset is not something this plan should do on the way past.

So `VariationSpec` gains optional `MinPlayers`/`MaxPlayers`, zero meaning "inherit the module's",
and the two gates in `match/manager.go` prefer the variation's range when the match has one:

| | `MinPlayers` | `MaxPlayers` |
|---|---|---|
| module `canasta` | 2 | **6** |
| `classic`, `modern_american` | — | **4** |
| `samba` | — | — (inherits 6) |

Roughly fifteen lines of shared code, and it is a gap the seam has anyway: Hold'em and Blackjack
both ship variations whose real seat ranges differ from their module's. It also means a fifth
player is refused at the lobby with `MATCH_FULL` rather than at `Start` — a table that fills and
then cannot deal is the dead position this codebase already refuses to create elsewhere.

**Hand size at six seats.** Samba deals 15 at two to five seats and 13 at six. `Defaults` is a
static map and cannot say "13 at six", so the seat-dependent default resolves in `NewMatch`;
`handSize` stays a declared option, so a table that wants 15 at six seats can say so and the
lobby's default is a default rather than a constant.

### 3.3 The meld model

```go
type Meld struct {
    ID     string
    TeamID int
    Kind   string // "set" | "run"           (new)
    Rank   string // set: the rank; run: ""
    Suit   string // run: the suit; set: ""  (new)
    Cards  []string
}
```

**Identity.** `meldID(teamID, rank)` stays as the id of a team's *first* group of a rank, so
every existing test and every `t0-K` on the wire is unchanged. A second group of that rank gets
`t0-K-2`; a sequence gets `t0-seq<n>` from a per-team counter on `GameState`, because a run grows
at both ends and an id derived from its low card would not survive the growth — and offer ids
have to be stable enough for a client to diff across pushes.

**Lookup.** `Team.meld(rank)` — which today means "the group of this rank, if any" — becomes
`Team.openGroup(rank)`, returning the first group of that rank that can still take cards. Under
`classic` there is at most one, so every existing call site keeps its meaning.

**Validation.** `validateMeld` splits into `validateGroup` (today's function, with the wild
limits read from the ruleset) and `validateRun`: one suit, distinct consecutive ranks, no wilds,
no threes, 3–7 cards, ace high only. `validateMeld` becomes the dispatcher that decides which one
a card list is asking for — a list of one rank is a group, anything else is tried as a run — so
there is still exactly one entry point that every path putting cards on the table goes through.

**Closing.** `Meld.closed()` becomes ruleset-aware: a run closes at seven (it is a samba), a
group closes at seven only when `groupCanastaCloses` is set.

### 3.4 Sequences are still enumerable, and that is the property to protect

The strongest claim this module makes — `conformance_test.go`, and `canasta-plan.md` §7 — is that
`module.PlayWithOffers`, a driver that reads the offer list and nothing else, plays whole matches
*to a winner*. It holds because a Canasta meld is "n of one rank", so the candidates are at most
one per rank and the server can ship exact cards. `extensibility-plan.md` §1.1 records why a
rummy run cannot be enumerated that way, and Žolíky still cannot be driven from offers.

**Samba runs are the exception, and for a specific reason: they may not contain wilds.** A
candidate sequence is therefore fully determined by the hand — the maximal blocks of consecutive
distinct ranks within one suit — and there are at most four suits × a handful of blocks, in
practice nought to four. There is no shape for a human to compose and nothing to solve.

So `newMeldCandidates` gains a second half that walks each suit, finds each maximal block of
length ≥ 3, caps it at seven and emits it as a concrete card list, exactly as the group
candidates are emitted. The driver property survives, and §7's exit criterion 2 makes it a test
rather than a claim.

### 3.5 Verbs and offers

One new verb. The rest are the existing ones with wider card lists.

| Offer ID | Verb | Input | New? |
|---|---|---|---|
| `draw:deck` | `draw` | — | draws `drawCount` cards |
| `take_pile:naturals` | `take_pile` | the two naturals that receive the top card | the only capture in Samba |
| `take_pile:meld:<meldId>` | `take_pile` | — | **not offered in Samba** — the pile is always frozen |
| `take_top:<meldId>` | **`take_top`** | — | **new**: the top card onto an open sequence, instead of drawing |
| `lay_meld:<rank>` | `lay_meld` | a group's exact cards | unchanged |
| `lay_meld:run:<suit><low>` | `lay_meld` | a sequence's exact cards | **new** |
| `lay_off:<meldId>` | `lay_off` | eligible cards — for a run, the cards that extend either end | widened |
| `discard` | `discard` | the legally discardable cards | unchanged |

`take_top` is a separate verb rather than a flavour of `take_pile` because it does something
different: it takes one card and leaves the pile standing. Labelling that "take the pile" would
be a lie on a button, and the offer list is the only rulebook a client gets.

Every enabled/disabled decision keeps coming from probing the real engine (`offers.go`'s
`probe`), so nothing here restates a rule and the offer list cannot drift from `Apply`. `Bot()`
and `driverPrefer` gain `VerbTakeTop`.

### 3.6 Files

No new package. Inside `server/internal/canasta/`:

```
cards.go      buildDeck(decks, jokers); rank ordering for sequences; sequence predicates
meld.go       validateGroup / validateRun / validateMeld; run candidates; run lay-off
state.go      Meld.Kind + Suit, per-team meld counter, the new error codes
engine.go     ruleset resolution, seating for 2–6, draw-two, take_top, frozen-always pile,
              second group of a rank
scoring.go    floor bands from the ruleset, samba bonus, red threes from the deck size,
              no concealed bonus
offers.go     run candidates and take_top offers
view.go       the samba VariationSpec and its seat range, run groups and the samba badge
rules.go      Rules() sections for a sequence game
```

**One change outside the module, and it is not a rule:** `module/descriptor.go` gains
`MinPlayers`/`MaxPlayers` on `VariationSpec` with an accessor that falls back to the module's,
and `match/manager.go`'s join and start gates call it (§3.2). No module that does not set the
fields behaves differently.

Then three files that are words rather than rules: `scripts/check-server-labels.js` (add `Samba`
to the proper-noun set; it is a name), the 24 locale bundles, and
`client-react-native/src/lib/serverKeys.json`. `app.go` needs nothing — the module is already
registered.

**No client work.** `ZoneView` renders a spread from its groups and never reads `group.kind` —
`ginrummy` and `rummytiles` already ship `"run"` — so a sequence renders today. The only client
change is words.

---

## 4. i18n

New server-side message keys, all static literals (a key passed through a variable drops silently
out of `serverKeys.json`):

- `variation.canasta.samba` — the variation's label.
- `canasta.rules.*` — sequences, the samba bonus, draw two, the always-frozen pile, the 150 floor.
- `badge.samba` — on a seven-card sequence, beside the existing natural/mixed canasta badges.
- `verb.takeTopForSequence` — the new offer's label.
- `err.*` for the new refusals: `MELD_NOT_A_SEQUENCE`, `SEQUENCE_NEEDS_ONE_SUIT`,
  `SEQUENCE_NO_WILDS`, `RUN_NOT_CONSECUTIVE`, `SAMBA_CLOSED`, `PILE_ALWAYS_FROZEN`.

Then `cd server && go run ./cmd/dump-keys > ../client-react-native/src/lib/serverKeys.json`, and
English plus 23 translations, checked with `npm run i18n`. `cmd/dump-keys`' golden test fails CI
if the manifest is stale, which is the intended reminder.

---

## 5. Phases

Each phase is independently green.

**Phase 0 — the ruleset, no behaviour change.** `variationDefaults` → `ruleset`; deck size, draw
count, wild limits, floors and bonuses read from it; `classic` and `modern_american` populated
with today's constants. No test touched, all tests pass. Plus the golden fixture of exit
criterion 3, recorded here so it is recorded *before* anything can change.

**Phase 1 — seating, and the seat range on the seam.** The generalised partnership rule (§3.2),
`MinPlayers`/`MaxPlayers` on `VariationSpec` with the module-level fallback, and the two gates in
`match/manager.go`. Canasta's module maximum goes to 6 and both existing variations pin
themselves at 4 in the same commit, so no shipped table changes size. Lands separately from
everything else because it is the only change to shared code, and a bisect should be able to
land on it alone.

**Phase 2 — sequences in the model, reachable by nothing.** `Meld.Kind`/`Suit`, the id scheme,
`validateRun`, run candidates, run lay-off, multiple groups per rank — all gated on ruleset
fields no shipped variation sets. New unit tests only; `classic` provably unmoved.

**Phase 3 — the variation.** The `samba` ruleset and `VariationSpec`, its 2–6 seat range and the
seat-dependent hand size, draw-two, the always-frozen pile, `take_top`, samba scoring, `Rules()`,
the `10000` target choice. Conformance seeds.

**Phase 4 — words and the wire.** Locale keys, `serverKeys.json`, the label check's proper-noun
set, the e2e spec, and the two comments that currently say Samba is out of scope (`cards.go`'s "a
third deck is a different game", `canasta-plan.md` §2) rewritten to point here.

---

## 6. Tests

| Level | What it proves | Where |
|---|---|---|
| Unit — cards | a 162-card deck with six jokers and six red threes; sequence rank ordering, ace high, threes and twos excluded | `cards_test.go` |
| Unit — melds | `validateRun` as a table: mixed suits, a gap, a wild, a duplicate rank, eight cards, A-2-3; group wild limits under both rulesets | `meld_test.go` |
| Unit — seating | 2→2 sides, 3→3, 4→0+2 vs 1+3, 5→5, 6→0+3, 1+4, 2+5; a seventh seat refused; a fifth refused on `classic` but not on `samba`; 13 dealt at six seats and 15 at five | `seating_test.go` |
| Unit — descriptor | a variation with no seat range inherits the module's, one with a range narrows it, and `manager.go`'s gates read the resolved one | `module/descriptor_test.go`, `match/manager_test.go` |
| Unit — scoring | the five floor bands and their edges (1499/1500, 6999/7000), samba 1500, six red threes 1000, the sign flip against `canastasToGoOut`, no concealed bonus | `scoring_test.go` |
| Unit — engine | draw two; the stock emptying mid-draw; `take_top` leaves the pile standing; the pile refusing a natural+wild capture; a second group of a rank; a samba refusing an eighth card while a group canasta accepts one | `engine_test.go` |
| Unit — agreement | the existing corpus, re-run under `samba`: `LegalActions` and `Apply` never disagree in either direction | `agreement_test.go` |
| Unit — hidden info | `View` still shows no hand, no stock and no buried pile to the wrong viewer, with runs on the table | `view_test.go` |
| Conformance | `PlayWithOffers` finishes whole `samba` matches to a winner at **every seat count from 2 to 6**, across seeds — and scores at least one samba across the corpus, so sequences are not decorative | `conformance_test.go` |
| **Regression** | `classic` and `modern_american` play identically to `main` on fixed seeds — see below | `variation_golden_test.go` |
| E2E | two real players on two real sockets play a `samba` match to a winner over HTTP + WebSocket + Mongo, every move taken from the offer list; `/modules` lists three canasta variations | `e2e/tests/canasta.spec.ts` |

The regression test is the one worth spelling out, because it is what makes a refactor of this
size safe: before Phase 0 changes anything, record the final `GameState` of a `PlayWithOffers`
run for a fixed seed corpus under both existing variations, and assert against it from then on.
A refactor that alters `classic` by one card fails immediately and locally, instead of surfacing
as a scoring complaint from a real table.

---

## 7. Exit criteria

1. `go test ./...` green in `server/`, and `npm test` in `client-react-native/`.
2. `PlayWithOffers` finishes `samba` matches from offers alone, going out included, on every seed
   tried, at every seat count from two to six — and sambas actually get scored.
3. `classic` and `modern_american` produce byte-identical final state to `main` for the recorded
   seed corpus, and neither can be joined by a fifth player.
4. `/modules` lists `samba` among canasta's variations with its own seat range, and the e2e spec
   plays it end to end against a live server and database.
5. No file outside `internal/canasta` learns a Samba rule. The seat range on `VariationSpec`
   (§3.2) is the one deliberate exception, and it is a mechanism rather than a rule: every other
   module behaves identically after it. Words and the key manifest do not count as rules.

---

## 8. Deliberately out of scope

Stated so each is a decision and not an oversight:

- **Bolivia and the other Samba-family variants** (wild-card canastas, and the variants that
  change the canasta count). Additive to the ruleset struct now that it exists.
- **Five and six seats on `classic` and `modern_american`.** Real 5–6 player canasta uses three
  decks, which changes the meld arithmetic; that is a canasta ruleset to be read off a source and
  tested, not a thing to reconstruct while shipping Samba. The seat range on `VariationSpec` is
  what makes it a later commit rather than a blocked one.
- **Asking a partner's permission to go out.** A social convention with no state, out of scope for
  Canasta already.

## 9. Decisions where the source is silent

Four, each resolved the way the module already leans, and each cheap to revisit:

1. **A stock of one card when two must be drawn.** Draw the one and let the existing exhaustion
   path end the deal, rather than refusing the draw. It matches how the module already handles the
   stock emptying during a red-three replacement.
2. **Red threes and the 1000.** All six pays 1000 only when the side has its canastas; the penalty
   side is a flat 100 each, which is what pagat states explicitly and avoids a 1000-point swing
   against a side that never opened.
3. **How large a group canasta may grow.** Samba lets a canasta keep taking cards; the wild cap
   (≤ 2, and naturals ≥ 2 × wilds) already bounds it, so no separate maximum is imposed.
4. **Five seats.** Pagat gives the hand size at five (15) but not the seating, and five cannot be
   split into equal partnerships. Five plays as five individuals, which is what three already
   does — the module has never held that a Canasta table must have sides.

---

## 10. Outcome

Delivered as planned, in the five phases above. Seven things are worth recording — five of them
because the plan was wrong about them.

One caveat on the verification, stated rather than left implied: the browser-level checks were
not run against this branch. Metro resolves out of a symlinked `node_modules` into the *other*
worktrees under `.claude/worktrees/` and fails to bundle, so the Expo web client could not be
started from here. Everything below the UI was exercised against a real server on a real
database — the e2e suite runs green against a build of this branch, including a four-seat Samba
match played to a winner over real WebSockets — and the sweep in §10's last item is the
server-side answer to the question the untranslated-sweep spec asks in a browser. A Samba *screen*
has not been looked at.

**The offer list carries a sequence game.** `module.PlayWithOffers` — the driver that reads a
module's offer list and nothing else, and has never heard of a suit — finishes whole Samba
matches to a winner at every seat count from two to six, and the e2e plays a four-seat Samba
match to a winner over real sockets against real Mongo. That is the claim §3.3 rests on, and it
holds for the reason given: a Samba sequence takes no wilds, so a candidate is the maximal block
of consecutive ranks a hand actually holds in a suit. `extensibility-plan.md` §1.1's
offer-explosion limit is about shapes a *human* composes, and this is not one. Žolíky still
cannot be driven from offers, and still for the same reason.

**The driver found a dead position within eight seeds.** `take_top` is the only move in the game
that takes a card without putting one in the hand — it replaces the draw rather than following
it. So a player holding a single card could take the top card onto a sequence and then have no
legal way to end the turn: their discard would be their last card, which is going out, which
their side could not do. The plan did not see it, the rules do not mention it, and a human tester
would have had to be dealt it. It is refused up front now, beside the two dead ends
`checkLeavesPlayable` already prevented.

**Phase 0's "no test edits" was wrong.** The unit tests call `validateMeld`, `redThreeScore` and
`initialMeldMinimum` directly, so widening those signatures reached them. What survived intact is
the thing that mattered: every *assertion* is unchanged, and the tests that pinned Canasta's
numbers still pin Canasta's numbers — they now name the ruleset they are asserting about instead
of assuming there is only one.

**The golden fixture had to hash the play, not the state.** Hashing the encoded `GameState` went
red the moment the ruleset was stored on it, having changed no card — and Phase 2 would have done
it again by adding `Kind` to every meld. It hashes each deal's six-part settlement, the running
scores, the winner and the move tally instead. The recorded values came from a throwaway worktree
at the commit before the refactor, so "nothing moved" is measured against the old code rather than
asserted about the new.

**Two numbers nearly changed a shipped variation.** Red threes are the sign-flip case: Modern
American needs two canastas to go out but has always paid for red threes after *one*, so deriving
the threshold from `CanastasToGoOut` would have quietly rewritten it. And Samba's flat 100-a-three
penalty is Samba's — Canasta negates the all-of-them bonus instead. Both are ruleset fields for
that reason, and the golden fixture is what caught the first attempt at deriving them.

**Three error codes, not six.** A closed samba reports the existing `MELD_CLOSED` and a
permanently frozen pile the existing `PILE_FROZEN`: both already say exactly what happened, and a
second code per variation would have been a second sentence to translate for no new meaning.

**Adding a badge exposed a hole that had been open since Canasta shipped, and pulling on it
found two more.** `dump-keys` reads keys out of the source, so it only finds the spellings it was
taught — and it had not been taught `append`. Canasta badges its canastas with
`g.BadgeKeys = append(...)`, so `badge.naturalCanasta`, `badge.mixedCanasta` and Žolíky's
`badge.cleanRun` were in no bundle at all and had been rendering in English in all 24 locales.
`badge.samba` would have been the fourth.

Rather than teach the scanner one more spelling and hope, `canasta/keys_test.go` now plays real
matches under all three variations and checks the keys the module *actually emits* against the
committed manifest. It found the other two immediately: `seat.LabelKeys` is written with `append`
in five modules, leaving twelve seat labels off the manifest and five of them — Hold'em's
folded/all-in/out, Žolíky's contract-met and Canasta's own meld prompt — with no wording in any
locale, English included; and Canasta's turn prompt chose its key through a variable, which
`dump-keys` documents as invisible and which is now two literals. All of it is worded.

None of those five keys is Samba's. They are what a static scan cannot see, found because
something finally looked.
