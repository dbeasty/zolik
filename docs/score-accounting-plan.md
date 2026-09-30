# Score accounting — "where did my points come from?" (shared sheet + Canasta)

**Problem.** In Canasta the score jumps by hundreds at the end of a deal and
nothing on the table says why. The server already knows. `scoreDeal`
(`server/internal/canasta/scoring.go:84`) splits every deal into melded cards,
canastas, red threes, going out and cards caught in hand. It keeps every deal
in `GameState.Deals`, and `Rounds()` sends each deal's parts to the client as
`RoundScore.Facts` on every `match_state`. But no client component renders
those facts, and no score on the table can be tapped.

**Goal.** Tapping a score (your own, your partner's or an opponent's) opens a
pop-out **Score sheet**. The sheet shows a full accounting for that
side/player:

- every finished deal, broken into line items that **add up on screen** to
  the deal's total;
- the running total after each deal;
- the deal in progress, as far as it is public;
- what the score means next: the target, and in Canasta the next initial-meld
  minimum.

This plan covers the **shared sheet** and **Canasta**. The other games that
keep accounts (Žolíky, Gin Rummy, Rummy Tiles, Blackjack, Hold'em) each have
their own section in
[score-accounting-games-plan.md](score-accounting-games-plan.md). They all
build on Phase 1 below.

---

## Principles

1. **The accounting balances.** For every deal the line items must sum to
   `RoundScore.Delta` (or to `Shown`, where the game prints penalties). The
   server tests this for every game. A sheet that doesn't add up is worse
   than no sheet.
2. **The server does the arithmetic; the client only prints it.** The client
   stays game-agnostic (`matchTypes.ts` has no meld, suit or canasta nouns).
   Line items are keys + params + points, and the words live in the locales.
3. **Only public information.** `RoundLog` goes to every viewer, including
   spectators and replays. Anything about a deal in progress that is not
   already visible on the table goes in the per-viewer view, never in the
   `RoundLog`.
4. **Old matches still open.** New fields are `omitempty`. A deal stored
   before this change falls back to today's five summed lines.

---

## Phase 1 — shared: the line-item shape and the Score sheet

### 1a. Server: `module.ScoreLine`

`RoundScore.Facts` is a list of labels. It carries no points the client can
add up or indent. Add a typed, still game-agnostic, line:

```go
// server/internal/module/rounds.go
// ScoreLine is one row of a round's accounting. Points are signed and in the
// same orientation as RoundScore.Shown/Delta, so that for every RoundScore
// sum(Lines[i].Points) == shown delta. Sub-lines break a line down further
// (each meld, each player's hand) and must themselves sum to the parent.
type ScoreLine struct {
    LabelKey string         `json:"labelKey"`
    Params   map[string]any `json:"params,omitempty"`
    Points   int            `json:"points"`
    Sub      []ScoreLine    `json:"sub,omitempty"`
}

type RoundScore struct {
    ...
    Lines []ScoreLine `json:"lines,omitempty"` // NEW
}
```

- Keep `Facts` for the compact one-line summary (ResultsFlash, the
  RoundResults cell). `Lines` is the full accounting.
- Add `module.CheckLines(rs RoundScore) error` for the balancing rule. Every
  module test calls it on every round of a played-out match.
- Extend the key scanner (`module/emittable.go`) so that `ScoreLine{LabelKey:
  "…", Params: map[string]any{…}}` literals are collected like `Fact`s. Write
  every line as a literal: a key passed through a variable drops out of
  `serverKeys.json` (see [[message-keys-must-be-static-literals]]).

### 1b. Server: the live projection (per viewer)

Between deals, the `RoundLog` already answers everything. **During** a deal,
the sheet should also show the deal in progress: "on the table so far". Add
an optional interface:

```go
// module/rounds.go
type Projected interface {
    // Projection is the in-progress round for one seat as `viewer` may see it:
    // table-visible parts for anyone, the viewer's own hand only for themself.
    Projection(state json.RawMessage, viewer, seat string) (*RoundScore, error)
}
```

This goes on the state message as `projection map[seatId]RoundScore`. It is
built in `match/state_msg.go` next to `Rounds`, per recipient, the same way
`View` is. Projection lines are labelled *provisional*. They never include a
partner's or opponent's hand.

### 1c. Client: `ScoreSheet` pop-out

- **New:** `src/components/match/ScoreSheet.tsx`, a pop-out built on the
  `WhySheet.tsx` pattern (RN `Modal transparent`, backdrop to dismiss,
  `ScrollView` body, `useMetrics`/`useSkin`). It opens full-screen on phone
  widths and as a side panel on wide screens. It is its own component with
  its own layout, not a section of the match screen.
- **Sheet layout, top to bottom:**
  1. **Header:** the seat or partnership name, the running total, and "Target
     5000 · 1 240 to go".
  2. **This deal (provisional):** the projection lines, if the match is live.
  3. **Deal history:** newest first, one collapsible card per deal. The
     header row shows `Deal 3 · +1 185 → 2 940`. Expanded, it lists the line
     items with right-aligned signed points, sub-lines indented, and a rule
     under them with the total. The deal's facts (who went out, concealed,
     stock exhausted) sit under the header.
  4. **Footer:** what the score implies next, from the module's
     `Status`/standing facts (e.g. "Next deal: your first meld needs 90").
- **Fallback:** a `RoundScore` without `lines` renders its `facts` as a plain
  list (old matches, and games not yet converted).
- **Entry points** (all open the same sheet on the tapped seat):
  - `SeatStrip.tsx`: the standing score (:124) and the collapsed summary pill
    (:197) become `Pressable`, each with an `accessibilityHint` of "Show score
    breakdown";
  - `BoardLayout.tsx:192`: the `status.teamScore` line;
  - `RoundResults.tsx`: tapping a cell opens that deal already expanded;
  - `ResultsFlash.tsx`: a "Details" affordance that stays while the
    intermission lasts.
- **Also:** `RoundResults.tsx` finally renders `RoundScore.facts` under each
  cell. That is a one-line summary; the sheet holds the full version.
- **Shareable page:** `app/match/[matchId]/score.tsx`, a full route that
  renders the same `ScoreSheet` body for completed matches. It fetches the
  replay payload, which already carries `rounds`, so it needs no socket. Link
  to it from the end-of-match banner and the replay screen. Live matches use
  the modal only, because their state comes over the socket.
- Add the types (`ScoreLine`, `lines?`, `projection?`) to
  `src/api/matchTypes.ts`. They contain no game nouns.

### 1e. Meld values: tap a meld's value to see what it is worth

The second half of the same question: "what is *this* meld worth?". Today no
game sends a per-meld value. Canasta sends only a badge, and only once the
meld is already a canasta.

**Server: `Group.Value`.** Add a field to `module.Group`
(`protocol.go:169`):

```go
Value *ScoreLine `json:"value,omitempty"` // what this group is worth, broken down
```

It reuses `ScoreLine`, so the same line that appears in the meld's popover
also appears as a sub-line of "Melded cards" in the deal accounting. The
arithmetic is written once. A test asserts that the sum of `Group.Value`
over a team's spread equals `meldCardScore` plus the canasta bonuses the
deal will pay.

**Client: the tap target.** A plain tap on a meld is already taken: it
expands or stacks the cards and arms the meld as a lay-off target
(`ZoneView.tsx:382`). While a card is being dropped, a press-overlay covers
the meld (`:435`). So the value gets its own target instead of overloading
that tap:

- **A value chip** sits in the meld's existing badge row (`ZoneView.tsx:428`),
  e.g. `70 · 5/7`. It is always visible, so for most questions no tap is
  needed. It is a `Pressable` with `hitSlop` and an `accessibilityLabel` of
  "Meld value, 70 points, 2 cards to a canasta". Tapping it opens the
  popover.
- **Long-press on the meld**, which no gesture uses today, opens the same
  popover. It is a shortcut, not the only way in, since long-press can't be
  discovered and is awkward with a mouse.
- **Hidden while dragging:** while the meld is a live drop target, the chip
  is hidden, so a drop can never hit it by mistake.
- **New component:** `src/components/match/MeldValuePopover.tsx`. It is
  anchored to the meld on wide screens and is a small bottom sheet on
  phones, using the `WhySheet` `Modal` pattern. It renders the `ScoreLine`
  and its sub-lines, the same way `ScoreSheet` renders them.
- **Setting:** add a "Show meld values" toggle, on by default. Players who
  count for themselves can turn the chip off, and the popover stays
  reachable by long-press.
- **Row height:** the chip lives in the badge row, which already takes its
  height, so a meld with no badge would otherwise grow when the chip
  appears. Reserve that row's height for every group so the layout does not
  jump as melds change.

### 1d. Tests for Phase 1

- Go: `CheckLines` unit tests; a scanner test that `ScoreLine` literal keys
  and params land in the manifest.
- Jest: `ScoreSheet` renders lines and sub-lines, shows the running total,
  and falls back to facts.
- Playwright: tap the score, the sheet opens, and **the visible line items
  sum to the visible deal total**. That is an assertion on what the player
  sees (see [[assert-what-the-user-sees]]).

---

## Phase 2 — Canasta

### 2a. Store the parts that are currently only summed

Extend `TeamResult` (`canasta/state.go:560`) with `omitempty` fields that
`scoreDeal` fills in:

| New field | What it holds | Why |
|---|---|---|
| `Melds []MeldTally{Rank, Kind, Cards, Wilds, Points}` | each meld on the table at the end of the deal | "Kings ×7 (2 wild) = 70" instead of "Cards laid 410" |
| `Naturals, Mixed, Runs int` | canasta counts by kind | "2 natural × 500, 1 mixed × 300" instead of "Canastas 1300" |
| `RedThreeCount int`, `RedThreeAll bool`, `RedThreePenalty bool` | how the signed red-three value was reached | the negative case needs its reason: "only 0 canastas, 1 needed" |
| `GoingOutKind string` (`""`/`plain`/`concealed`) + `WentOut` player | who earned the bonus and which one | Samba pays plain for a concealed hand, and that should be visible |
| `Hands []HandTally{PlayerID, Cards, Points, BlackThrees, Wilds}` | each partner's leftover hand | "caught in hand" is currently one team figure, so nobody can tell whose hand it was |

- Once the deal is scored, the hands are public: they are counted face up at
  the table, and the next deal replaces them. List them in the `RoundLog` as
  **category counts and points**, not card identities. That is enough to
  argue about and consistent with Žolíky (see the games plan).
- Black threes go in their own sub-line: "2 black threes × 100". Wilds and
  jokers get theirs too.
- Old `DealResult`s with no tallies fall back to the five summed lines.

### 2b. Emit `Lines` from `rounds.go`

Replace `teamFacts` with `teamLines(r ruleset, d DealResult, t TeamResult)
[]module.ScoreLine`, written as literals:

```
Melded cards                     +410
    Kings ×7 (2 wild)             +70
    Aces ×5 (1 wild)             +100 …
Canastas                        +1300
    Natural × 2 (500 each)       +1000
    Mixed × 1 (300)               +300
Red threes × 2                   +200     (or: −200 "only 0 canastas, 1 needed")
Going out — concealed (Anna)     +200
Caught in hand                   −115
    Bob: 9 cards                  −115
      2 black threes             −100
      other                        −15
────────────────────────────────────
Deal total                      +1995  → 4 210
```

- Every key is a literal `module.ScoreLine{LabelKey: "canasta.line.…"}`.
  This fixes the existing gap where `canasta.round.meldCards` and friends
  never reached `serverKeys.json`, because `teamFacts` builds its keys in an
  anonymous struct.
- Keep `teamFacts` for the compact summary, rewritten as literals too.
- Samba: sequence canastas (1500), 200 for going out, the flat red-three
  penalty. The ruleset supplies the values, and the lines carry the value as
  a param so the sheet never hardcodes 500/300.

### 2c. Projection for the deal in progress

`Projection(state, viewer, seat)` for the seat's partnership:

- melded cards, canastas and red threes, as currently on the table (public);
- "going out: not yet";
- "in hand": the viewer's own hand value when `seat == viewer`, otherwise
  "N cards" with no points;
- a provisional total from the public parts only, labelled as such.

It also shows the **initial meld requirement** for the current deal, derived
from the team's score. It is the most-asked "why" question in Canasta, and
the ruleset already knows the thresholds.

### 2f. Canasta meld values

`canasta/view.go:203–247` sets `Group.Value` on every meld in both
partnerships' spreads. They are all public.

```
Kings                               60   chip: "60 · 5/7"
    Naturals: K × 4 (10 each)       40
    Wilds: 2 × 1 (20)               20
    2 more cards to a canasta
    1 wild — will be a mixed canasta (300); 2 more wilds allowed
```

Values come from `cardValue` (`cards.go:166`): jokers are 50, twos and aces
20, 8–K 10, 4–7 5, and black threes 100.

- **Meld cards.** One sub-line per value class, e.g. "Naturals 4 × K (10)
  = 40" and "Wilds: joker × 1 (50), 2 × 1 (20) = 70". The parent line is
  their sum.
- **Canasta bonus, once complete** (`isCanasta`): a line of +500 natural,
  +300 mixed, or +1500 Samba sequence. The value comes from the ruleset
  (`NaturalCanastaBonus` etc.), so no number is hardcoded. The chip then
  reads `590 ★`.
- **Not yet complete.** The line is informational (0 points): "2 more
  cards to a canasta" from `canastaSize` − `len(Cards)`. There is also a
  status line from `naturals()`/`wilds()`:
  - "Natural so far", or
  - "1 wild — will be mixed (300)".
  It also shows how many more wilds may go on under the ruleset.
- **Closed or full melds** (`closed(r)`, `room(r)`): "Closed: no more
  lay-offs". This answers "why can't I lay off here" before the player
  tries (see [[refusals-point-at-written-rules]]).
- **Black-three meld** (the going-out meld): "Black threes × 3 (100)".
- **Red threes zone.** One group-level value on the zone: "Red threes × 2 =
  200". If the partnership has fewer canastas than `RedThreesNeed`, add a
  warning line: "counts −200 if the deal ended now".
- **The team spread label** also shows the sum, "Melded: 410 + canastas
  1300". That is the same number the projection (2c) uses, so the table and
  the Score sheet can never disagree.

Tests: a table test for each meld shape (natural and mixed, in the making
and complete, a Samba sequence, black threes, closed). The balance test
from 2e also asserts that the spread's summed `Group.Value` equals
`MeldCards + Canastas` for the deal at scoring time.

### 2d. Words

- Add every new `canasta.line.*` key to `en.ts` and to all 24 locale bundles
  (`serverKeys.test.ts` enforces that).
- Regenerate with `cd server && go run ./cmd/dump-keys >
  ../client-react-native/src/lib/serverKeys.json` (see
  [[server-keys-manifest-locks-wording]]).

### 2e. Tests for Canasta

- `keys_test.go`: also walk `Rounds()` output, which it currently skips, so
  line keys are checked.
- Add a new balance test that plays classic, modern_american and samba over
  **five seeds each** (group-canasta situations only persist in Samba; see
  [[canasta-bot-bugs-hide-in-samba]]). For every deal and team:
  - `CheckLines` passes;
  - the meld sub-lines sum to `MeldCards`;
  - the hand sub-lines sum to `InHand`.
- Add table tests for the edge cases:
  - red threes with too few canastas, in classic and in Samba (flat);
  - all four red threes;
  - a concealed go-out in Samba (paid as plain);
  - stock exhausted with no one out;
  - a deal decoded from a pre-change JSON with no tallies.
- Projection test: never contains another seat's hand points.
- e2e: in a two-deal Canasta match, open the sheet from the seat strip.
  Assert that the deal's lines sum to the header delta and the running total
  matches the seat score. Check in the browser against the live stack before
  calling it done (see [[verify-ui-bugs-in-the-browser]]).

---

## Order of work and PRs

1. **PR 1 (Phase 1a + 1c, facts only):**
   - `ScoreSheet` + tap targets + `RoundResults` rendering `facts`;
   - `ScoreLine` type, `CheckLines`, scanner support.
   - This is useful on its own: Canasta and Blackjack already send facts.
2. **PR 2 (Phase 2a/2b/2d/2e):** Canasta tallies and lines. This is the fix
   for the reported problem.
3. **PR 2b (Phase 1e + 2f):** `Group.Value`, the meld value chip and
   popover, and Canasta meld values. It can land before or after PR 2.
4. **PR 3 (Phase 1b + 2c):** the `Projected` interface and the Canasta live
   projection.
5. **PR 4:** the `app/match/[matchId]/score.tsx` shareable page for finished
   matches.
6. Then the per-game PRs from
   [score-accounting-games-plan.md](score-accounting-games-plan.md).

## Open questions

- Should the sheet open to **your** side by default when you tap an
  opponent's pill, with a toggle? The proposal is no: open the tapped side,
  with a segmented control to switch sides.
- `stats/standings.go:73` drops `Facts` when a match is persisted to stats.
  Replays recompute from state, so the sheet works there without changing
  it. Only extend stats if accounting should outlive the match state.
