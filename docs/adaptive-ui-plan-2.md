# Adaptive match screen, part 2 — reaching a target, and putting things away

Follow-up to [`adaptive-ui-plan.md`](adaptive-ui-plan.md), which is implemented.
Three things came out of using it:

1. There is no usable way to play a card onto **another player's meld**.
2. **Table**, **hand** and **players** should be minimizable like everything else.
3. A minimized panel should carry a **super-condensed digest** on its collapsed
   rail, so putting something away doesn't mean losing sight of it.

Same discipline as part 1: `client-react-native/` only, no server change, no
protocol change, and no game's vocabulary in the shell (`src/lib/__tests__/shell.test.ts`
greps for it with comments stripped).

---

## 1. What is actually wrong with dropping on another player's meld

Diagnosed against the live match the report came from
(`/match/6a8e1c15f09ce6bf876621b7`, Žolíky, human + one bot).

**The panel renders correctly.** The board shows `Bot 95 / Melds / 9` with all
three of its groups (`4♥ 5♥ JKR`, `Q♥ K♥ A♥` badged *Clean run*, `6♣ 6♦ 6♠`).
Part 1's owner-named spread row is working.

**The server does offer the move.** `/matches/…?as=…` shows four `lay_off:*`
offers, each targeting a real group — three of them the bot's
(`target.ownerId = bot:7XJM66F8`, `meldId = meld_1…3`). Playing onto an
opponent's meld is a supported move in this module, and the offer names the
exact group it lands on.

**The mechanism works.** Dropping a card on a `group-<meldId>` inside a spread
that isn't yours is already proven by a passing test —
`drag-and-drop.spec.ts:230`, which drags a Canasta card onto a partnership meld
(an unowned spread) and asserts that *that* meld grew and every other one on the
table was untouched. Žolíky's lay-off travels the identical code path.

**So what went wrong is two things, neither of them the drop itself:**

### 1a. Every one of those offers was disabled — and the control refused to say why

All four `lay_off:*` offers came back `enabled: false`, `whyNot:
ROUND_REQ_NOT_MET` — "Lay your own initial meld first". A legitimate rule; the
player simply hadn't met the deal's contract yet.

The bug is that they were never told. Several offers that share a label and each
name a target of their own are folded into one control (`FoldedOffer` in
`OfferBar.tsx`) — and **`FoldedOffer` has no `whyNot` branch at all**. An
ordinary offer renders `{!offer.enabled && offer.whyNot ? <Text testID={`why-${offer.id}`}>…}`;
the folded one renders the button and, at most, a "more than one place this
could go" hint that is itself gated on `!disabled`.

The result is visible in the page text of that very match — every disabled
control explains itself except the two folded ones:

```
Lay meld     It's not your turn
Lay off                            ← nothing
Swap joker                         ← nothing
Discard      It's not your turn
Undo draw    It's not your turn
```

A greyed-out button with no reason, next to seven greyed-out buttons that all
have one, reads as broken rather than as illegal. That is the whole report.

### 1b. A drop target you can only find by dragging at it

Even once the move is legal, nothing on a meld says it will accept a card.
Targets light up only *during* a drag (`activeDrops`, computed from
`dropSpotsFor(state.legalActions, drag.cards)`), so the affordance is invisible
until you are already committed to the gesture — and if the offer happens to be
disabled, the drag lights nothing up and silently does nothing, which is
indistinguishable from the drag being broken.

On a phone this is the difference between playable and not: the opponents' melds
are below your hand on a page that scrolls, so a drag onto one means picking a
card up and dragging it across a scrolling viewport. The fix is not a better
drag — it is to stop requiring one.

---

## 2. Work items

### W1 — A folded control says why it cannot be pressed

**`src/components/match/OfferBar.tsx`**, in `FoldedOffer`.

The group's members can in principle be disabled for different reasons, so pick
deliberately rather than showing the first:

```ts
// Every member disabled for the same reason (the common case: a rule that
// gates the verb, not the target) — say it once, exactly as a lone offer of
// the same shape would.
const reasons = new Set(group.filter((o) => !o.enabled && o.whyNot).map((o) => o.whyNot!));
const sharedReason = reasons.size === 1 ? [...reasons][0] : undefined;
```

When `disabled`:
- `sharedReason` → render it, `testID={`why-group:${groupKey}`}`, same
  `styles.why` and `numberOfLines={2}` an ordinary offer uses.
- reasons differ → render the most frequent one. Different targets refusing for
  different reasons is still a real answer to "why is this greyed out", and
  picking the modal one beats inventing a summary sentence the locale bundle
  has no key for.
- no member carries a `whyNot` at all → render nothing, as today.

**Acceptance:** in the reported match's state, `Lay off` and `Swap joker` read
"Lay your own initial meld first" / "No joker in this meld" instead of nothing.

**Test:** extend `e2e/tests/offer-labels.spec.ts` (it already owns "telling the
controls apart"): drive a Žolíky deal to a board with melds on it, then assert
that **no** disabled control anywhere on screen is missing a reason —
`for each [data-testid^="offer-"][aria-disabled="true"], a why-* sibling exists`.
That phrasing catches the next control that forgets, not just this one.

### W2 — A selected card lights up everywhere it may go, and a tap sends it

This is the real fix for the report, and it is mostly *deleting a condition*
rather than adding machinery: the screen already computes pressable targets and
already resolves a press into a submission (`pendingSpots` / `pressDrop`), but
only ever populates them after a folded control has been pressed and found
ambiguous.

**`app/match/[matchId].tsx`.** Today:

```ts
const pendingOffers = pendingGroupKey
  ? state.legalActions.filter((o) => o.enabled && o.target?.meldId && offerGroupKey(o) === pendingGroupKey)
  : [];
```

Change the resting state from "nothing" to "everything this selection could
do". Keep `pendingGroupKey` as a *narrowing* filter, not as the on-switch:

```ts
// A card in hand is a card looking for somewhere to go. Every enabled offer
// that would take the current selection is a live target — the same set a
// drag would light up, offered to a tap as well, because a drag across a
// scrolling board is a poor way to reach a target on a phone.
//
// `pendingGroupKey`, when set, narrows this to one folded control's own
// targets: pressing it said which *kind* of move was meant, and only the
// target is still open.
const candidates = state.legalActions.filter(
  (o) => o.enabled && (!pendingGroupKey || offerGroupKey(o) === pendingGroupKey),
);
const pendingSpots: DropSpot[] =
  selectedCards.length > 0
    ? dropSpotsFor(candidates, selectedCards)
    : pendingGroupKey
      ? /* unchanged: a folded control pressed with nothing selected */
      : [];
```

Note `dropSpotsFor` already excludes offers that take no cards (`minCards === 0`),
so pressing a pile that is only ever a button cannot happen by accident.

Three consequences to get right:

- **Drop the `target?.meldId` restriction.** It is why only groups, never whole
  zones, were ever pressable. `dropSpotsFor` already resolves `target.zoneId` to
  `zone-<id>`, and `ZoneView` already registers that id and already renders a
  press overlay for a group — the same overlay has to exist at zone level. Add
  it to the zone box in `ZoneView`, guarded by `pressableDrops.has(zoneId)`,
  `testID={`zone-press-${zone.id}`}`.
- **The overlay must not eat a card's own tap.** A zone-level press overlay
  drawn over a hand would swallow selection taps. Render it only when the zone
  is genuinely pressable, and never for the viewer's own hand — the zone a
  selection is *made in* is not a zone it can be sent to. (`dropSpotsFor` will
  not produce it anyway; belt and braces.)
- **Clearing.** `toggleSlot` already resets `pendingGroupKey`; the highlight now
  follows the selection, so it clears itself when the selection empties.

**Acceptance:** with one card selected in Žolíky and the contract met, every
meld on the table that would take it is outlined, yours and the opponents'
alike, and tapping one plays the card there.

**Tests** (new `e2e/tests/tap-to-play.spec.ts`, phone viewport):
1. *A selected card shows where it can go* — select a card the server says a
   lay-off accepts; assert `group-<meldId>` gains a live outline and a
   `group-press-<meldId>` overlay exists.
2. *Tapping a lit target plays the card there* — the Canasta shape from
   `drag-and-drop.spec.ts:230`, tapped instead of dragged: that meld grew by
   one, every other meld unchanged.
3. *Nothing selected lights nothing up* — no `*-press-*` overlay anywhere.
4. *A tap on a target that is not lit does nothing* — no action reaches the
   server (poll the state; it is unchanged).

### W3 — Table, hand and players all minimize

Part 1 gave a minimize control to every *zone* panel and to the seat strip, but
two of the three regions a player actually wants out of the way are not zones:

| Region | Today | Wanted |
|---|---|---|
| Players (`SeatStrip`) | ✅ minimizable (`panelId: "zone:seats"`) | — |
| Table (`Section title="Table"`) | ❌ a bare `<Text>` title over a row of panels | one panel, minimizable |
| Your hand (`HandZone`) | ❌ draws its own box | a `Panel`, minimizable |

**`Section` → a real panel.** Wrap its body in `Panel` with
`panelId={`section:${title.toLowerCase()}`}` and `title`. The zones inside stay
their own panels, which nests a panel in a panel — so add a `nested` prop to
`Panel` that drops the background fill and softens the border, and pass it to
any `ZoneView` rendered inside a `Section`. Without that the table reads as a
box in a box in a box.

**`HandZone` → a `Panel`.** It already has exactly the pieces `Panel` takes:
- `title` ← `label(zone.labelKey)`, `count` ← `zone.count`,
  `countTestID={`zone-count-${zone.id}`}` (keep — `hand-order.spec.ts` reads it),
- `accessory` ← the existing Auto-arrange `Pressable`
  (`hand-auto-arrange-<id>` must keep its testID),
- `testID={`zone-${zone.id}`}`, `panelId`, `minimized`, `onToggleMinimized`
  threaded from the screen the same way `zonePanelProps` already does it.

Two hazards, both real:

- **Do not force-open the hand for a drag.** `ZoneView` sets `forceOpen` when it
  is a live drop target; the hand is where drags *start*, so it needs no such
  rule — but it must also never be force-opened by one, or minimizing it would
  spring back open the moment anything is dragged.
- **A minimized hand must not strand a drag.** Minimizing is only reachable
  between gestures (the control is in the header, a drag holds the pointer), so
  no in-flight drag can be interrupted — but `heldRef`/`insertRef` should be
  cleared on unmount so a hand minimized during an aborted gesture does not come
  back holding a card. Add a `useEffect` cleanup in `HandZone`.

### W4 — The condensed rail

A minimized panel currently shows title + count + `▸`. That is enough for a
spread and not enough for anything else: a put-away table tells you nothing
about what is on it, and a put-away hand tells you nothing at all.

**`Panel` gains `summary?: ReactNode`**, rendered in the header row — after the
titles, before the count — **only when collapsed**. Nothing else about `Panel`
changes.

```tsx
{collapsed && summary ? <View style={styles.summary}>{summary}</View> : null}
```

**New: `src/components/match/CardChips.tsx`** — the digest itself, and
deliberately game-blind: it is handed card strings and renders rank+suit as
tiny coloured text, capped, with a `+n` tail.

```tsx
export function CardChips({ cards, max = 6 }: { cards: string[]; max?: number })
```

Uses `parseCard` (already shared with `CardView`) for rank, suit glyph and
red/black, so a chip and a card can never disagree about what a string means.
`testID="card-chips"`, each chip `testID={`chip-${card}-${i}`}`.

What each panel puts in its summary — all of it derived from generic fields
(`cards`, `groups`, `count`, seats), none of it naming a game:

| Panel | Collapsed rail shows |
|---|---|
| Hand | `<CardChips cards={slots.map(s => s.card)} />` — your own cards, your own screen |
| A spread | `n groups` + `<CardChips>` of each group's **last** card (the one drawn in full when stacked) |
| A pile | `<CardChips>` of the top card — the only card a pile is ever about |
| A stack | its count, which it already shows and is all a face-down pile has |
| Table (`Section`) | one `label · count` per zone inside, e.g. `Draw pile 77 · Discard pile 4` |
| Players (`SeatStrip`) | each seat's name, the active one marked `●`, `numberOfLines={1}` |
| Controls | `n available` — how many offers are enabled, so a put-away control bar still says whether it is your move |

`ZoneView` can build the first four itself from `zone.kind` — the same
kind-driven switch it already uses to lay a zone out, so a game added tomorrow
gets a rail without this file being edited.

**Acceptance:** minimize everything on a Žolíky board and the screen still says
whose turn it is, what is on top of the pile, how many cards you hold and what
is melded — in well under a viewport.

**Tests** (`e2e/tests/adaptive-layout.spec.ts`, alongside the existing five):
- *A minimized panel still says what is in it* — collapse the hand; assert
  `card-chips` is present inside `zone-hand:<id>` and that it names at least one
  card the server says is in hand.
- *Minimizing everything fits the board on a phone* — collapse table, players,
  hand and every spread; assert `match-screen` `scrollHeight <= clientHeight`.
- *Every region can be put away* — assert a `panel-toggle-*` exists for the
  hand, the table section, the players strip and the controls.

### W5 — Two chevrons side by side

A discard pile now renders `4 ▾` (its own show-all control, from part 1's
`accessory`) immediately followed by `▾` (minimize). Two adjacent triangles
meaning different things is a coin flip for the player.

Keep both controls — they do genuinely different jobs — but stop them looking
alike: give the pile's control the count and a distinct glyph pair (`⌄`/`⌃`
outline, or the count in a bordered chip), and leave the solid `▾`/`▸` to
minimize alone. Purely presentational; `zone-toggle-<id>` and its
`accessibilityLabel` stay exactly as they are, so `board-layout.spec.ts` is
unaffected.

---

## 3. Risks

**R1 — the press overlay and the card underneath.** W2's zone-level overlay is
`StyleSheet.absoluteFill` over a whole zone. Get its condition wrong and it
swallows every tap in that zone, including card selection, and the board goes
dead in a way no test currently watches. Test 4 in W2 exists for exactly this;
add a case that selects a card in the hand *while* another zone is lit and
confirms the selection still toggles.

**R2 — nesting panels changes measured rects.** `Section` becoming a panel adds
padding around the piles, which moves every drop region below it. Harmless in
itself (regions are re-measured at the start of each drag) but it will shift
`board-layout.spec.ts`'s "a stack and a pile share a line" geometry — re-run it,
do not assume.

**R3 — the shell vocabulary grep.** `CardChips`, `summary`, `nested`, `rail` are
all safe words; `meld`, `discard`, `draw` in an identifier are not. New shell
files (`CardChips.tsx`) must be added to `SHELL_FILES` in
`src/lib/__tests__/shell.test.ts` and the count guard raised — the guard on the
guard exists so a new file cannot be silently exempt.

**R4 — a rail that is longer than the panel.** `CardChips` of a 13-card hand
must not wrap the collapsed header into three lines, which would defeat the
point. Cap it (`max`), give the container `flexShrink: 1` and
`numberOfLines={1}`, and let the `+n` tail carry the remainder.

---

## 4. Verifying

The stack is already up (API `:8090`, web `:8114`). The Žolíky board that
produced this report is `/match/6a8e1c15f09ce6bf876621b7` — a board with melds
on both sides is the one worth re-checking, since an empty table exercises none
of this.

```js
// No disabled control anywhere is missing its reason (W1).
[...document.querySelectorAll('[data-testid^="offer-"][aria-disabled="true"]')]
  .map(b => b.parentElement.innerText.replace(/\n/g, ' · '))
```

Then: select a card and confirm targets outline (W2); collapse each region and
read its rail (W3/W4); and re-run both suites — `npm test` in
`client-react-native/`, and `npx playwright test` from `e2e/` with
`ZOLIK_E2E_API_BASE=http://127.0.0.1:8090 ZOLIK_E2E_WEB_BASE=http://127.0.0.1:8114`.

> One pre-existing failure is expected and unrelated:
> `generic-shell.spec.ts › a finished match is recorded` fails on the
> match-recording timing, server-side, and fails the same way on an unmodified
> checkout.

---

## 5. Order

1. `fix(client): say why a folded control cannot be pressed` — W1. Smallest,
   and on its own it turns the reported symptom from "broken" into "not yet".
2. `feat(client): a selected card lights up everywhere it may go` — W2. The
   actual fix for reaching an opponent's meld.
3. `feat(client): put the table, the hand and the players away too` — W3.
4. `feat(client): say what is in a panel that has been put away` — W4.
5. `fix(client): stop two different controls looking like one` — W5.

---

## 6. Implemented — notes for whoever reads this after the fact

All five items shipped. `CardChips` became **`CardGlance`**: `chip` is itself a
banned word in the shell grep (poker chip), caught by the test the moment the
file was added to `SHELL_FILES` — same category of thing part 1's own retro
found, one level down.

**W2 turned out simpler than it reads above.** The existing group-level press
overlay (`group-press-<meldId>`) already handled "lay off onto an existing
meld" — dragging onto an opponent's meld was never actually broken; only
`pressableDrops` was never populated outside an ambiguous folded press, so the
overlay never had anything to light up. Broadening `pendingCandidates` in the
match screen was the whole fix for that half. The genuinely new piece was the
**zone-level** overlay (`zone-press-<id>`) for offers that target a whole zone
rather than one group in it — composing a brand-new meld, discarding onto a
pile. That one needed a real content wrapper in `ZoneView` (`styles.content`,
`position: 'relative'`) so the overlay fills the zone's body and not its
header — a version that skipped the wrapper would have buried the minimize
chevron under the overlay the first time a whole zone became pressable.

**Verified against the reported match** (`/match/6a8e1c15f09ce6bf876621b7`):
the disabled `Lay off`/`Swap joker` controls now show `ROUND_REQ_NOT_MET`'s
text instead of nothing (W1, confirmed live). The actual fix (W2) couldn't be
driven in *that* browser tab — it needs the reported session's own
access token, which this session never had — so it's verified end-to-end
instead with a fresh Canasta match reaching the identical shape: select a
card, an unowned partnership meld (the same "someone else's meld" case) lights
up with no drag, a tap plays it, every other meld on the table is untouched.
New spec: `e2e/tests/tap-to-play.spec.ts`, 3 cases, all green. Also confirmed
by hand in the browser on a fresh Žolíky match: selecting three 4s force-opens
the viewer's own *minimized* melds panel (a live target overriding a
preference — the same `forceOpen` mechanism part 1 built for drags, now
reacting to a tap-selection too) and tapping it submits the meld.

**W3/W4 confirmed live**, all five panel kinds (players, table, controls,
hand, spread) collapse to a one-line digest — `● you ·Bot A ·Bot B ·Bot C`,
`Draw pile 55 ·Discard pile 4`, `2 available`, a full `CardGlance` of the
hand, `1 [4♣]` for a spread — and the whole board fits in roughly a fifth of
the vertical space it took open. Re-expanding a minimized hand mid-arrangement
was checked and the cards come back in the same order (`useHandOrder`'s state
lives in the parent and never unmounts, only the fan underneath it does).

**Full suite status**: 175 client unit tests, TypeScript clean, 78 e2e tests
(3 new) — 77 pass. The one failure
(`generic-shell.spec.ts › a finished match is recorded`) is the same
pre-existing, server-side match-recording-timing flake noted in part 1's
implementation; it exercises no client code (raw WebSocket via
`page.evaluate`, never touches a rendered component) and fails identically on
an unmodified checkout.
