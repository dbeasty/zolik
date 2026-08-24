# Canasta — implementation plan

Third game module for the runtime built in `extensibility-plan.md` Phase 3.

- **Baseline:** `main` @ `62f9c61`
- **Deliverable:** `server/internal/canasta` implementing `module.GameModule`, registered
  alongside `zolik` and `prsi`, with a full test pyramid ending in a browser-run e2e that
  plays a whole match over real HTTP + WebSocket + Mongo.

---

## 1. Why a module and not a `RulesConfig` profile

`architecture.md` §1 says the "slightly different rummy" case — Canasta is named there — is a
*configuration* problem. Reading the engine, it is not, and the reason is worth recording
because it is the first time that claim has been tested against a real second rummy.

| What Canasta needs | What `internal/rules` has |
|---|---|
| Melds owned by a **partnership**, extendable by either partner | melds owned by a player |
| Capture of the **whole discard pile**, gated on the top card | draw one card from the pile |
| A pile that can be **frozen**, permanently, by a wild discard | no pile state at all |
| **Red threes** — cards that leave your hand on sight and score by themselves | no card with an out-of-turn effect |
| **Canastas** (7-card melds) as the unit of progress and the licence to go out | melds are just melds |
| Wild-card limits *inside* a meld (≤3 wilds, ≥2 naturals) | jokers count toward run/set shape |
| **Multi-deal** scoring to a target, with the initial-meld floor rising with your own score | a fixed contract per deal number |
| Going out, concealed-hand bonus, stock exhaustion ending the deal with no winner | go out by melding and discarding |

Every row is a rule with no knob behind it. Bolting them onto `rules` would put a second
game's clauses inside the engine that Žolíky depends on — the exact coupling Phase 3 removed.
So: a module, using the seam that already exists.

It is also the more interesting test of the abstraction. Prší falsified it by sharing *no*
vocabulary. Canasta probes the opposite failure: a game close enough to rummy that a
rummy-shaped interface might quietly fit it badly.

## 2. Rules implemented

Classic (American) Canasta, the ruleset in Hoyle and on pagat.com.

**Deck.** 108 cards: two standard decks plus four jokers. Notation matches the rest of the
server (`AS`, `TD`, `2C`, `JOKER1`…`JOKER4`), with duplicates disambiguated the way the
existing 2-deck rummy deck already does.

**Card values.** Joker 50 · 2 20 · A 20 · K Q J T 9 8 10 · 7 6 5 4 5 · black 3 5 · red 3 100.

**Wilds.** Jokers and 2s. A meld may hold at most 3 wilds and must hold at least 2 naturals.

**Melds.** Sets of equal rank, 3–7 cards. No runs. Melds belong to the partnership; either
partner may lay off onto them. A rank may have only one meld per partnership. A meld of 7 is
a **canasta** — *natural* (no wilds, 500) or *mixed* (300) — and is closed to further cards.

**Red threes.** Never melded or discarded. Turned up on deal, or drawn, they go straight to
the partnership's red-three row and the player draws a replacement. 100 each, 800 for all
four, and **negative** if the partnership finished the deal with no canasta.

**Black threes.** Discarding one blocks the pile for the next player only. They may be melded
only as part of going out, and never with a wild.

**The discard pile.** A player may take the entire pile instead of drawing, if they can use the
top card immediately:

- *Unfrozen* — the top card plus two naturals of its rank from hand, **or** laid off onto an
  existing partnership meld of that rank.
- *Frozen* — two naturals from hand only. The pile is frozen by any wild discarded onto it, by
  a red three turned up at the start of a deal, and (for a partnership that has not yet made
  its initial meld) for that partnership specifically.
- Taking the pile to make the **initial meld** must reach the minimum using the top card and
  cards from hand — never the rest of the pile.
- The rest of the pile then goes to the taker's hand.

**Initial meld minimum**, from the partnership's accumulated score: `< 0` → 15 · `0–1495` → 50
· `1500–2995` → 90 · `3000+` → 120. Counted on card values of everything laid in that turn,
excluding bonuses.

**Turn.** Draw one (or take the pile) → meld and lay off freely → discard one. The turn ends
on the discard.

**Going out.** Requires the partnership to hold the required number of canastas (1 in
`classic`, 2 in `modern_american`). A player goes out by shedding their last card, with or
without a final discard. Bonus 100, or **200 concealed** — melding a whole hand including a
canasta in one turn, having laid nothing before.

**Stock exhaustion.** If the stock runs out and the player to move cannot take the pile, the
deal ends with nobody going out and no going-out bonus.

**Deal scoring.** Canastas + going-out + red threes + the card value of every melded card,
minus the card value of every card left in hand. Deals repeat until a partnership passes the
target score; highest total wins.

**Deliberately out of scope**, stated so it is a decision and not an oversight: asking a
partner's permission to go out (a social convention with no state), the "seven canastas /
Samba / Bolivia" variant families, and the special-hands variants. Each is additive and none
changes a type.

## 3. Design

### 3.1 Partnerships behind a player-shaped interface

`module.GameModule` speaks players; Canasta scores teams. Teams are seat parity — seats 0,2 vs
1,3 at four, and one player per team at two — computed once in `NewMatch` and stored, so no
other file re-derives it.

`Finished` returns one `winnerID` because the runtime's envelope has one field. It returns the
winning partnership's first seat, and the ViewModel carries the full per-team scoreboard, which
is where a UI reads it. This is the one place the interface fits Canasta imperfectly, and it is
worth writing down rather than papering over: a `winners []string` would be the honest shape.

### 3.2 Verbs and offers

| Offer ID | Verb | Input |
|---|---|---|
| `draw:deck` | `draw` | — |
| `take_pile:<rank>` | `take_pile` | the naturals from hand that receive the top card |
| `take_pile:meld:<meldId>` | `take_pile` | — (lay the top card off onto that meld) |
| `lay_meld:<rank>` | `lay_meld` | the exact cards of a candidate meld |
| `lay_off:<meldId>` | `lay_off` | eligible cards from hand |
| `discard` | `discard` | the legally discardable cards |

Every enabled/disabled decision is produced by **probing the real engine** against the state,
exactly as `prsi/offers.go` does — the offer list is `Apply`'s own answer asked in advance, so
it cannot drift. Free because state is an opaque blob: `Apply` decodes a fresh value, so a dry
run cannot touch the caller's copy.

**Canasta melds are enumerable, and that matters.** A meld here is "3+ of one rank", so the
candidate set is at most 13 shapes — unlike a rummy run, whose shapes explode (see
`extensibility-plan.md` §1.1). So `lay_meld` ships **concrete card lists**, one offer per rank,
and Canasta becomes the first module a generic offer-only driver can play *to completion*,
including going out. That closes the limitation §4.x documents for Žolíky rather than
inheriting it.

To let the shared driver use them, `module.chooseAction` must send `MinCards` cards instead of
always one. Prší's offers set `MinCards: 1`, so its behaviour is unchanged; the change is
verified by running both existing modules' suites.

### 3.3 Files

```
server/internal/canasta/
  cards.go       deck, notation, wild/red-three/black-three predicates, point values
  state.go       GameState, teams, meld model, error codes, verbs
  engine.go      NewMatch, Apply and the five verbs, deal rollover, match end
  scoring.go     initial-meld minimum, deal scoring, canasta bonuses
  offers.go      LegalActions, built by probing the engine
  view.go        Descriptor, View (per-viewer filtering)
```

Registered in `internal/app/app.go` and `internal/match/state_msg_test.go` next to the other
two.

## 4. Tests

| Level | What it proves | Where |
|---|---|---|
| Unit — cards/scoring | point values, wild limits, canasta classification, red-three sign flip, the four initial-meld floors | `canasta/scoring_test.go` |
| Unit — engine | each rule as a table: frozen pile, black-three block, taking the pile both ways, initial-meld shortfall, going out without the required canastas, stock exhaustion | `canasta/engine_test.go` |
| Unit — **agreement** | for a corpus of states × players × candidate actions, `LegalActions` and `Apply` never disagree in either direction | `canasta/agreement_test.go` |
| Unit — hidden info | `View` never shows a hand, the stock, or the buried pile to the wrong viewer | `canasta/view_test.go` |
| Conformance | `module.PlayWithOffers` — the driver that may read offers and nothing else — plays whole Canasta matches to a winner across many seeds, at 2 and 4 players, both variations | `canasta/conformance_test.go` |
| Regression | the other two modules still pass with the driver change | existing suites |
| **E2E** | two real players on two real WebSockets play a whole match to a winner, every move chosen from the offer list; Mongo agrees with what the sockets reported; a viewer never receives another player's cards | `e2e/tests/canasta.spec.ts` |

## 5. Exit criteria

1. `go test ./...` green in `server/`, including the two existing modules.
2. `module.PlayWithOffers` finishes a Canasta match — going out included — from offers alone,
   on every seed tried, at both table sizes.
3. `/modules` lists three games and the e2e spec plays Canasta end to end against a live
   server and database.
4. No file outside `internal/canasta` learns a Canasta rule.

## 6. Scope note

Server-side only. `extensibility-plan.md` §3.x records the generic client shell as unbuilt —
the RN client renders Žolíky specifically — so a Canasta *screen* would mean building that shell
first, which is its own project. Canasta is therefore delivered the way Prší was: playable
through the real API and socket, proven by an e2e that drives them, and ready for the shell when
it lands.

---

## 7. Outcome

Delivered as planned. Four things are worth recording.

**The offer list turned out to be sufficient to *finish* the game, not just to take turns.**
`module.PlayWithOffers` — the driver that may read a module's offer list and nothing else, and
which has never heard of a canasta, a red three or a frozen pile — plays whole Canasta matches
to a winner, at two and four seats, under both variations, on every seed tried. That closes the
limitation `extensibility-plan.md` §4.x records for Žolíky, where going out needs a meld shape
the offer protocol deliberately does not enumerate. The difference is the game, not the
protocol: a Canasta meld is n cards of one rank, so the candidate set is at most thirteen and
the server can ship exact cards. A rummy run cannot be enumerated that way, and still cannot.

The one change the shared driver needed was to send `MinCards` cards rather than always one.
Prší declares `MinCards: 1` throughout and is unaffected; both existing modules' suites pass
unchanged.

**The initial meld forced a rule the plan had not anticipated.** The minimum is a property of a
*turn*, not of a meld — real play reaches 50 with two melds — so a lay that falls short has to
be allowed. But nothing can take cards back off the table, so a player who lays 30 and then
cannot reach 50 is stuck with no legal move at all. The fix is `meld.go`'s `reachableValue`: a
lay that would put the minimum out of reach is refused *before* the cards leave the hand, using
a greedy, concretely achievable lower bound. Erring high would put the dead end back, so the
bound is deliberately conservative.

The same class of dead end appears once more, and gets the same treatment: a turn ends with a
discard and shedding your last card is going out, so a partnership that cannot go out must be
left holding two cards. `checkLeavesPlayable` enforces it, and `MUST_KEEP_A_CARD` is what the
offer list says when it bites.

**Where the module interface fits imperfectly.** `Finished` returns one `winnerID`, and Canasta
is won by a partnership. It returns the winning partnership's first seat and puts the full
per-team standings on the ViewModel, which is where a UI reads them — `winners []string` would
be the honest shape. This is the first time the interface has been visibly the wrong shape for a
game rather than merely unfamiliar with it, and it is recorded rather than papered over. Prší
and Žolíky are both won by one player, so neither would ever have surfaced it.

**What it cost the other two games:** nothing. `internal/rules`, `internal/zolikmod` and
`internal/prsi` are untouched apart from the four-line driver change above, and both their
suites and `e2e/tests/match-runtime.spec.ts` pass unchanged.

### Measured

| | |
|---|---|
| Offer-driven matches finished from offers alone | 36/36 (2p and 4p, both variations, 12 seeds each) |
| Enabled offers cross-checked against `Apply` | 2,598, none refused |
| Per-card discard / lay-off decisions cross-checked | 4,098 and 11,933, none disagreeing |
| `LegalActions` cost, four-handed mid-turn | ~380 µs (about forty engine probes, each a decode) |
| e2e | 8 specs, green on three consecutive runs |
