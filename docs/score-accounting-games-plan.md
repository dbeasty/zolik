# Score accounting — the other games (Žolíky, Gin Rummy, Rummy Tiles, Blackjack, Hold'em)

This plan is a companion to
[score-accounting-plan.md](score-accounting-plan.md). That plan builds the
shared pieces, and every section here depends on them:

- the `ScoreSheet` pop-out;
- `module.ScoreLine` / `RoundScore.Lines`;
- `CheckLines`;
- the optional `Projected` interface;
- the shareable score page for finished matches.

Each game below is a **separate PR** with its own tests. Once Phase 1 of the
shared plan has landed, the games can be done in any order.

The same rules apply to every game:

- The lines sum to the delta the player sees (`Shown` when the game prints
  penalties).
- `RoundLog` holds only public information.
- Every key is a literal, and the manifest is regenerated.
- Old stored rounds fall back to the existing facts.

**Today, per game:**

| Game | Accumulates? | Breakdown computed | Breakdown sent | Work |
|---|---|---|---|---|
| Žolíky | penalty per deal, lowest wins | summed only | none | **large**: tally in `ScoreDeal` |
| Gin Rummy | points to target | deadwood stored; bonuses folded into `Deltas` | kind only | medium |
| Rummy Tiles | points, winner collects | summed only | kind only | medium |
| Blackjack | chip stack | outcome keys only | outcome facts | medium |
| Hold'em | chip stack | pots in `LastHand` only | pot fact | medium–large |
| Prší | no points | — | — | none: the score stays not tappable |

---

## 1. Žolíky (the joker game) — do this one first

**Why first.** Žolíky has the steepest per-card arithmetic in the app:

- a joker left in hand costs 50;
- an ace costs 25, **or 1** when it sits in a natural run fragment in hand or
  could extend a run on the table;
- T/J/Q/K cost 10 each, and number cards cost their face value.

Today the player only sees "+87". The ace-at-1 rule is invisible and is
certain to look like a bug.

**Where the score is computed.** `rules/round.go:49` `ScoreDeal` sums each
player's penalty with `HandPenaltyTotalWithMelds`
(`rules/round_requirement.go:172`). The state keeps only the per-deal totals:
`GameScores`, `TotalScores` and `DealWinners` (`rules/types.go:319-330`).

**Server work**

1. Add `PenaltyTally` to `internal/rules`:

   ```go
   type PenaltyTally struct {
       Cards      int  `json:"cards"`
       Jokers     int  `json:"jokers,omitempty"`      // × 50
       AcesHigh   int  `json:"acesHigh,omitempty"`    // × 25
       AcesLow    int  `json:"acesLow,omitempty"`     // × 1, with reason
       AceLowWhy  []string `json:"aceLowWhy,omitempty"` // "run-in-hand" | "extends-table"
       Faces      int  `json:"faces,omitempty"`       // T–K × 10
       Numbers    int  `json:"numbers,omitempty"`     // face-value sum
       WentOut    bool `json:"wentOut,omitempty"`
       Total      int  `json:"total"`
   }
   ```

   - Implement it as `HandPenaltyTallyWithMelds`, and make
     `HandPenaltyTotalWithMelds` return `.Total`, so there is one code path
     and the two cannot drift.
   - `ScoreDeal` stores `DealTallies []map[string]PenaltyTally` alongside
     `GameScores`, with `omitempty`.
2. In `zolikmod/rounds.go`, emit `Lines` per seat. The points are penalties,
   so they are shown positive, in the `Shown` orientation that
   `RoundScore.Shown` already uses:

   ```
   Went out                      0
   — or —
   Jokers × 1 (50)              50
   Aces × 1 (25)                25
   Ace counted as 1 (run A-2-3)  1
   Tens–Kings × 1 (10)          10
   Number cards                  7
   ───────────────────────────────
   Deal penalty                 93   → 141 total (200 ends the match)
   ```

   The footer comes from the variation: the Classic target (200), or the
   Continental deal count ("deal 4 of 7") plus that deal's contract. The
   contract is already in `contractFacts`.
3. **Privacy.** During the intermission the opponents' hands are hidden
   today. `reveal` only applies to `OpenView` at match end. The tally gives
   categories and counts, not card identities, which is the same compromise
   as Canasta. **Decision needed:** should the finished deal's leftover cards
   be shown face up in the sheet, as they would be at a real table? The
   proposal is categories only in v1.
4. **Projection.** For the viewer's own seat, "if the deal ended now:
   N", using the live tally. The code already computes this for
   `zolik.standing.inHand` via `handPenalty` (`zolikmod/module.go:698`). For
   other seats, "N cards", with no points.

**Meld values (shared plan Phase 1e).** Žolíky melds score nothing at the
end of a deal. Their value is what counts toward the **opening
requirement** (`initialMeldMinimum`), which is the number players argue
about. `zolikmod/module.go:495–518` sets `Group.Value` on every meld:

```
Run ♥ 9-10-J-Q (with joker)       39   chip: "39"
    9 + 10 + J 10 + Q 10          39
    Joker stands for J
Set of Aces × 3                    75   (25 each)
```

- Values come from `rules/meld.go:145 runRankValue` / `:167 runValue` and
  `scoring.go:65 NaturalSetCardValue`.
- Say what the joker stands for, and flag a joker that can be reclaimed
  ("Swap in the J♥ to take the joker"). That reuses the existing
  `zolik.badge.jokerOwed` logic.
- **Your own opening.** Show a running total on your own spread, e.g. "This
  turn: 39 of 51 needed to open". It comes from `rules/preview.go`.
- The existing `badge.cleanRun` stays next to the chip.

**Tests**

- Table tests for each category, and for both ace-at-1 routes.
- Meld value: runs with jokers at either end, aces low and high, and sets.
  Each must equal what the opening check counts, so the chip and the
  refusal can never disagree.
- `CheckLines` over full matches of both profiles (Continental and Classic).
- The tally total must equal the existing `HandPenaltyTotalWithMelds` for
  every hand seen in the sim sweep, run with `-short` (see
  [[go-test-flakes-on-main]]).

**Separately:** the manual Žolíky scorepad (`internal/scoring`,
`app/scoring`) is not part of this. It records totals typed in by hand, so
there is nothing to break down.

---

## 2. Gin Rummy

**Where the score is computed.** `ginrummy/scoring.go:20` `scoreHand`, plus
`applyLineBonuses` (:55) and `deadHand` (:133). `HandResult`
(`state.go:80-89`) already keeps `Kind`, `KnockerDeadwood` and
`DefenderDeadwood`. The line bonus and the game/shutout bonus are folded into
`Deltas`, so they cannot be shown separately.

**Server work**

1. Add `LineBonus`, `GameBonus`, `Shutout` and `BigGin` to `HandResult`
   (`omitempty`), filled by `applyLineBonuses` and `knockOut`
   (`engine.go:252`).
2. `ginrummy/rounds.go` emits, for the scoring seat:

   ```
   Opponent's deadwood          +34
   Your deadwood                −6      (knock only)
   Gin bonus                    +25     (or Big gin / Undercut bonus)
   ───────────────────────────────
   Hand                         +53
   — on the match-ending hand, additionally —
   Line bonus: 4 hands × 25    +100
   Game bonus                  +100     (+200 shutout)
   ```

   - The other seat gets "Knocked on / caught: 0". Undercut flips the sides.
   - A dead hand gets one line: "Dead hand: no score".
   - Deadwood becomes public once the hand is laid down.
3. **Rule check to raise, not change:** big gin currently scores exactly
   like gin (+25). Many tables pay 31. If that is a house rule, it belongs
   as a declared option (see [[house-rules-are-options-not-constants]]),
   not a silent change. Flag it in the PR.
4. **Projection:** for the viewer only, "your deadwood now: N (knock limit
   10)". Oklahoma sets the limit from the upcard. That is the most-asked
   question in Gin.

**Meld values.** Gin melds score nothing; only deadwood does. The table
melds exist only in the lay-off window after a knock (`s.KnockerMelds`,
`engine.go:283`). So the chip there is not a meld value. It sets:

- the **knocker's deadwood** on the `ginrummy.zone.knockerHand` zone: "Deadwood
  6";
- on each knocker meld, what laying off onto it saves the defender: "Lay off
  here: −N deadwood". The number comes from `handValue` (`cards.go:36`).

`meldCandidate.Value` (`meld.go:56`) is already computed. Carry it through
to `Meld`. The popover explains the undercut maths live: "Your deadwood 9
vs knocker 6: knock stands" or "…undercut +25".

**Tests:** gin, big gin, knock, undercut, dead hand, the match-ending hand
with line bonuses and a shutout, and both variations and all
`targetScore`/`lineBonuses` options, each passing through `CheckLines`.

---

## 3. Rummy Tiles

**Where the score is computed.** `rummytiles/scoring.go`: `endRoundOut` (:8)
and `endRoundPoolExhausted` (:28). A tile's value is its face number, and a
joker is worth 30 (`tiles.go:80`). `RoundResult` keeps only `Deltas`.

**Server work**

1. Store `Racks map[pid]RackTally{Tiles, Jokers, FaceSum, Total}` on
   `RoundResult`.
2. Lines:
   - **Loser:** "Left on rack: 6 tiles", "Jokers × 1 (30)", "Numbers 41",
     giving −71.
   - **Winner:** "Collected from Anna 71", "Collected from Bob 23", giving
     +94. There is one sub-line per opponent, and they sum to the parent.
   - **Pool exhausted:** each player gets their own rack line, plus "Lowest
     rack: round winner" when the `poolExhaustion` option records one.
3. Footer: "Target 300 · 58 to go" or "Round 4 of 10", from `targetScore` /
   `roundLimit`.

**Set values.** Sets on the table don't score at round end either. Their
value is what counts toward the initial meld. `view.go:138–146` sets
`Group.Value` from `setValueOf` (`sets.go:139`): "Run 7-8-9 = 24", or
"Group 11 × 3 + joker (as 11) = 44".

- While your workspace is open, the sets shown are **uncommitted**.
  - The chip is labelled provisional.
  - The spread shows "New this turn: 24 of 30 to open", from `newSetsValue`
    (`remedy.go:112`).
  - An invalid set shows no value, only its existing
    `rummytiles.badge.invalid`.
- The chip only shows committed values for other players' sets. Those are
  already public.

**Tests:** both endings, with and without jokers, and every `poolExhaustion`
setting, through `CheckLines`.

---

## 4. Blackjack

**Where the score is computed.** The chip stack moves in `engine.go:601`
`settle` / `settleHand` (:663), plus insurance (:626). `RoundSummary`
(`state.go:215`) keeps outcome keys and deltas. The per-hand bet, doubling,
payout and insurance premium are not stored.

**Server work**

1. Add `Hands map[pid][]HandSettle{Bet, Doubled, Split, Outcome, Payout}` and
   `Insurance map[pid]{Premium, Paid}` to `RoundSummary`.
2. Lines per seat, one parent line per hand, so splits read naturally:

   ```
   Hand 1: blackjack, bet 10 (pays 3:2)   +15
   Hand 2: split, doubled 20, lost        −20
   Insurance: premium 5, paid 3:1         +10      (or −5)
   ───────────────────────────────────────
   Round                                    +5  → stack 215
   ```

   - Surrender: "Surrendered: half of 10 returned, −5".
   - The payout ratio comes from the `blackjackPays` option. It is not
     hardcoded.
   - The dealer total stays a round fact.
3. There is no projection beyond the current bets, which are already
   visible on the table.

**Tests:** every outcome key, split and resplit to `maxSplits`, double after
split, insurance won and lost, surrender, and 6:5 against 3:2.

---

## 5. Texas Hold'em

**Where the score is computed.** `engine.go:436` `endHand`. The rich
`LastHand` (pots, shown hands) is kept for the latest hand only.
`Hands []HandSummary` keeps winners, pot size and deltas.

**Server work**

1. Add `Put map[pid]StreetBets{Blind, Preflop, Flop, Turn, River}` and `Won
   map[pid][]PotShare{Pot: "main"|"side1"…, Amount, Split int}` to
   `HandSummary`.
2. Lines per seat:

   ```
   Put in: blind 2, preflop 8, flop 20     −30
   Won main pot (split 2 ways)             +45
   Won side pot 1                          +12
   ───────────────────────────────────────
   Hand                                    +27  → stack 1 027
   ```

   - An uncontested pot reads "Won uncontested".
   - **No cards**, per the existing `RoundLog` rule. The winning hand's
     *rank name* ("two pair") may be added only when it was shown at
     showdown under `showdownReveal`. Mucked hands never are.
3. Projection: "In this hand so far: −30". It is public, since bets are on
   the table.

**Tests:** side pots with three all-ins, split pots with an odd chip,
uncontested hands, and `CheckLines` over a sim of `handLimit` hands.

---

## 6. Prší

Prší has no points: it is a single deal, ranked by cards left, and it does
not implement `Rounded`. Its score pill stays not tappable, or at most shows
"Cards left: N". There is no work beyond making sure the shared sheet
**does not** offer itself when a module has no `RoundLog` and no
`Projected`.

---

## Rollout order

After shared PR 1 and Canasta PR 2:

1. Žolíky
2. Gin Rummy
3. Rummy Tiles
4. Blackjack
5. Hold'em

Blackjack, Hold'em and Prší have no melds, so they have no meld values.

Every PR also includes:

- new keys in `en.ts` and all 24 locales, with `serverKeys.json`
  regenerated;
- the module's `keys_test` extended to walk `Rounds()`, not just the view;
- one e2e: open the sheet, and check that the lines on screen sum to the
  delta on screen.
