# Blackjack — implementation notes

- **Deliverable:** `server/internal/blackjack`, implementing `module.GameModule` like the six
  games before it, plus the two registration points, the shared contract test's game table, and
  wording in both locales.
- **Non-goal:** a card counter, a side-bet market, or a client screen. The shell already renders
  this game; see the verification section.

Written after the fact rather than before it, because there was nothing to argue about first:
the module interface is now six games old and blackjack needed no change to it. What is worth
recording is where the game sits *differently* to the other six, and what that turned out to
cost.

---

## 1. Why this game was worth adding

Every module in this codebase is players against each other. Blackjack is players against the
house, and the house is not a seat — it is a hand the engine plays by a rule with no choices in
it. That makes it the sharpest available test of a claim the runtime has been making since Phase
3: that it hosts a game without knowing what the game is.

It also has two decisions the other games take one at a time and a real table takes all at once —
the stake, and insurance. Modelling them the way a table does means the module waits on *every*
seat, twice a round, for the whole match.

## 2. What it needed from the protocol

Nothing. That is the result, and it is worth stating plainly since the last four games each bent
the interface somewhere:

| What blackjack needed | Where it landed |
|---|---|
| An opponent that is not a player | A `Zone` with no `OwnerID`. Nothing else knows the dealer exists. |
| Two phases that wait on several seats at once | `Seat.Active` is already plural — the intermission needed that first (`module.AwaitedSeats`), and a simultaneous phase is the same shape without the special case. |
| A stake, which is a number in a range | `ParamKindInt`, which no-limit poker asked for. |
| More than one winner | `Finished` already returns a list; a fixed number of rounds can end level. |
| Chips per seat, a stake in front of it, a hand total | `Seat.Facts`, which poker added for exactly this. |
| Several hands in one box, after a split | `Zone.Groups`, plus `BadgeKeys` to mark the one in play. |

The one thing it inverts is hidden information. Every other module hides *each player's own*
cards from the others; here every player card is face up (which is how a shoe game is dealt) and
the only secret on the table is the dealer's hole card — hidden from everybody, including the
seat on turn. `View` filters it the ordinary way: the card is not sent, and `Zone.Count` still
says it is there.

## 3. House rules are options, not constants

Which rules a table uses is what distinguishes one casino's blackjack from another's, so every
one of them is a declared option rather than a constant in the engine: deck count, table minimum,
dealer on soft 17, blackjack payout (3:2, 6:5, even money), splitting limit, double after split,
late surrender, insurance, starting chips, rounds.

Three shipped variations set them to rulesets somebody actually deals — Vegas Strip, Atlantic
City, and the modern single-decker whose one deck is paid for with a 6:5 blackjack.

`tableOf` resolves a lobby's choices into a table in one place, and `NewMatch`, `Rules` and
`ExplainRefusal` all call it. That is deliberate: resolving the options separately in each is how
a rules screen ends up describing a table nobody is sitting at, which is the split brain Phase 0
closed for the first game.

Rules that are *off* are written out too — "pairs are not split at this table" — because a rule
not in force is still something a player has to be told. A refusal points at whichever of the two
sentences this table actually states (`offers.go`'s `ruleIDsFor`, checked against `Rules()` by a
test that walks every option value).

## 4. The bot

`module.OfferBot` cannot play this game at all: hit and stand are offered on every hand, so
reading the offers alone gets you a player that either draws until it busts or stands on four.
Blackjack's entire content is *which* of two always-legal moves to make.

So the module ships its own, playing published basic strategy, and the skill dial is unusually
honest as a result — the strong setting is not a better guesser, it is the correct play, and the
weak ones are named mistakes:

| | plays |
|---|---|
| easy | mimics the dealer, never doubles or splits, takes insurance |
| medium | the hit/stand chart, tens and elevens doubled, aces and eights split |
| hard | the whole chart: soft doubles, every pair, late surrender, insurance declined |

Measured rather than asserted: `TestBot_TheSkillsAreOrderedByWhatTheyWin` seats all three at one
table — a paired comparison, since they share a dealer — over 120 matches, and requires the
ordering to hold. It does, by about 7% of a bankroll between hard and easy.

## 5. Verification

| Level | What it proves | Where |
|---|---|---|
| Unit | every clause: naturals and their payout rates, split aces taking one card, twenty-one after a split not being a blackjack, bust losing to a dealer who then busts too, the dealer's soft-17 rule both ways, surrender's odd chip, insurance at 2:1 | `blackjack/engine_test.go` |
| Unit | offers never disagree with `Apply`, the board's awaited seats equal the enabled offers, no offer names a card, every refusal's rule id resolves | `blackjack/agreement_test.go` |
| Unit | whole matches played from the offer list alone, across three variations, four playing styles and every optional move; chips conserved against the round table; no seat ever left with nothing to press | `blackjack/conformance_test.go` |
| Contract | the nineteen terms every hosted game is held to | `module/allmodules_test.go` |
| Bench | `LegalActions` against a one-deck and an eight-deck shoe | `blackjack/bench_test.go` |
| E2E | real HTTP, real sockets, real Mongo: the table asks every seat for a stake at once, the hole card never reaches a viewer, a whole match plays to a winner, and the persisted round table's arithmetic still holds | `e2e/tests/blackjack.spec.ts` |
| E2E | the unchanged generic shell renders and plays it in a browser, and the lobby lists it | `e2e/tests/generic-shell.spec.ts` |

No file outside `internal/blackjack` learned a blackjack rule; the only edits elsewhere are the
two registration points, the contract test's game table, the generated key manifest and the two
locale bundles.
