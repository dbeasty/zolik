# Adaptive match screen — implementation plan

The web/mobile client's match screen was laid out for a desktop window and
degrades badly below it. This plan makes it adaptive and compact, without the
shell learning a single game's rule or vocabulary.

Everything below is a change to `client-react-native/`. **The server is not
touched**, `client-tui/` is not touched, and no message key or protocol field is
added.

---

## 1. What is wrong today, measured

Measured in a real browser against a live 4-player Žolíky match at a 375×812
viewport (see §7 for the recipe to reproduce it):

| Symptom | Measurement |
|---|---|
| Controls are unreachable | `action-bar` is **135 px wide holding 1003 px** of buttons. 7 of 8 controls (`Take from discard`, `Lay meld`, `Discard`, 4× `Undo …`) are off-screen in a horizontal scroller with no scrollbar and no affordance. |
| Opponent hands are noise | 3 full-width panels, 57 px each, reading `Opponent hand 13 / 13 hidden` — while the seat strip directly above already says `Cards 13` for each of them. ~190 px spent on nothing. |
| Meld panels are anonymous and greedy | Each spread is a **343 px full-width row titled just "Melds"**, one per player, with no owner name. |
| Nothing can be put away | No panel on the screen can be minimized. |
| The 4th seat is invisible | The seat strip scrolls horizontally; `Bot HT` is clipped at the right edge with no indication it exists. |
| The hand dominates | 13 cards at 52×72 = **407 px of a 716 px viewport**, 4 per row, 4 rows. |
| Total | page `scrollHeight` 1034 px against a 716 px viewport. At 1280 px everything fits on one line and the layout is fine — the defect is entirely at narrow widths. |

Relevant source: [`app/match/[matchId].tsx`](../client-react-native/app/match/%5BmatchId%5D.tsx),
[`ZoneView.tsx`](../client-react-native/src/components/match/ZoneView.tsx),
[`OfferBar.tsx`](../client-react-native/src/components/match/OfferBar.tsx),
[`HandZone.tsx`](../client-react-native/src/components/match/HandZone.tsx),
[`SeatStrip.tsx`](../client-react-native/src/components/match/SeatStrip.tsx),
[`CardView.tsx`](../client-react-native/src/components/CardView.tsx).

---

## 2. Decisions already taken

These were asked and answered; do not re-open them.

1. **Owner naming is client-only.** Name a panel from `zone.ownerId` +
   `state.players` where the server sends an owner. Canasta's team spreads carry
   no `ownerId`; they keep their existing label ("Team melds"). No protocol
   change.
2. **A hidden hand is not drawn at all.** A zone that is not the viewer's, has
   no cards and no groups, is not rendered. Nothing is lost — the seat strip
   already carries the count. If a game ever reveals those cards (a poker
   showdown), the cards arrive and the panel appears on its own.
3. **Minimized state is remembered per match on the device**, using the same
   mechanism hand order already uses (`src/lib/handOrderStore.ts`): local
   storage, keyed by match, bounded to a handful of matches.
4. **Cards shrink only on small/low-resolution screens.** A desktop window keeps
   today's card size exactly. The scale is driven by viewport width, not by
   platform.

---

## 3. Invariants the implementation must not break

Read these before writing a line. Each has a test that will fail loudly, or a
bug that will not.

1. **The shell names no game.** `src/lib/__tests__/shell.test.ts` greps the
   shell files for game vocabulary (`meld`, `discard`, `draw`, `suit`, `run`,
   `pot`, …) *with comments stripped* — so prose may say "meld", identifiers may
   not. Any new shell file must be added to `SHELL_FILES` there, and the
   `expect(SHELL_FILES.length).toBeGreaterThanOrEqual(8)` guard bumped.
   Name things `panel`, `zone`, `group`, `owner`, `minimized` — never
   `collapsedMelds`.
2. **A drop region must not change size while a drag is in flight.**
   `ZoneView`'s `live`/`hovered` styles deliberately change colour only, never
   border width. Keep that. A minimized panel is a *smaller* region, which is
   why §4.4 forces a panel open while it is a live target — and why the measure
   pass has to happen after that expansion has laid out (§6, R1).
3. **A gap in the hand must be exactly a card wide.** `HandZone`'s `DropGap` is
   built from `CARD_METRICS`; if cards scale and the gap does not, the fan
   visibly shuffles mid-drag. Both must read the *same* metrics object.
4. **Existing testIDs stay.** e2e depends on `match-screen`, `action-bar`,
   `zone-<id>`, `zone-count-<id>`, `zone-toggle-<id>` (the pile's show-all
   control, which is **not** the new minimize control), `group-<id>`,
   `offer-<id>`, `card-<zone>-<i>`, `hand-<zone>`, `hand-drop-gap`,
   `seat-<playerId>`. `zone-<id>` must remain on the element that registers as
   a drop region.
5. **The shell derives no rule.** Layout may branch on `zone.kind`, on
   `ownerId === viewerId`, and on viewport width. Never on a verb, a label key's
   contents, or a card.

---

## 4. Work items

### W0 — One place that knows how big things are

**New: `src/lib/layout.ts`** (pure, unit-tested).

```ts
export type Metrics = {
  /** 1 on a comfortable screen, less on a small one. */
  scale: number;
  /** A one-column screen: panels stack instead of sitting side by side. */
  narrow: boolean;
  card: {
    width; height; compactWidth; compactHeight; gap;
    ringPadding; ringBorder;      // hairlines: never scaled
    rankFont; suitFont;
  };
  panel: { padding; gap; radius; titleFont; bodyFont };
  /** Visible corner height of an overlapped card in a group. */
  stackedCorner: number;
  /** Smallest a control may be before it wraps to the next line. */
  buttonMinWidth: number;
};

export function metricsFor(width: number): Metrics;
```

Breakpoints (dp), chosen to match the measurements in §1:

| width | scale | narrow |
|---|---|---|
| ≥ 768 | 1.0 | false |
| 480–767 | 0.88 | true |
| 380–479 | 0.78 | true |
| < 380 | 0.70 | true |

Rules: card box dimensions and gaps are `Math.round(base * scale)`; fonts are
`Math.max(9, Math.round(base * scale))`; `ringPadding`/`ringBorder` are constant
(a scaled hairline renders as a smudge); `stackedCorner = Math.max(20,
Math.round(26 * scale))`. Base values are today's `CARD_METRICS`, which stays
exported as the scale-1 baseline so nothing else has to move at once.

**New: `src/hooks/useMetrics.tsx`** — `MetricsProvider` (calls
`useWindowDimensions()`, memoises `metricsFor(width)`) and `useMetrics()`.
Mount the provider once, in `app/match/[matchId].tsx`, around the screen body.
`useMetrics()` outside a provider returns `metricsFor(Dimensions.get('window').width)`
so no consumer can crash.

### W1 — A panel is a thing, not a copy-pasted `View`

**New: `src/components/match/Panel.tsx`** — the bordered rectangle every region
of the board is drawn in, with the minimize control built in.

```ts
type PanelProps = {
  /** Stable id for remembering whether this panel is put away. */
  panelId: string;
  title: string;
  /** Secondary line — the owner's name, or what kind of place this is. */
  subtitle?: string;
  count?: number;
  /** Rendered in the header, left of the minimize control (e.g. a pile's show-all). */
  accessory?: ReactNode;
  minimized?: boolean;
  onToggleMinimized?: () => void;
  /** Held open regardless of `minimized` — see §4.4. */
  forceOpen?: boolean;
  /** Sized to its contents rather than filling the row. */
  inline?: boolean;
  live?: boolean;      // would accept the card in flight
  hovered?: boolean;   // the pointer is over it right now
  testID?: string;     // goes on the outer box, so `zone-<id>` still lands here
  innerRef?: (node: Measurable | null) => void;
  children?: ReactNode;
};
```

- Header: `title` (+ `subtitle` under it, `numberOfLines={1}`), spacer,
  `accessory`, `count`, then the minimize control:
  `testID={`panel-toggle-${panelId}`}`, `accessibilityRole="button"`,
  `accessibilityState={{ expanded }}`, label `Minimize <title>` / `Show <title>`,
  glyph `▾` / `▸`, `hitSlop={8}`.
- Minimized: header only, children unmounted. The header keeps the count, so a
  put-away panel still says how much is in it.
- `live`/`hovered` change border **colour** only (invariant 2).
- All spacing/fonts from `useMetrics()`.

### W2 — Controls get their own rectangle, and wrap

**`src/components/match/OfferBar.tsx`**

- Replace the horizontal `ScrollView` with a plain `View`:
  `{ flexDirection: 'row', flexWrap: 'wrap', alignItems: 'flex-start', gap }`.
  Keep `testID="action-bar"` on it.
- Each control slot: `minWidth: metrics.buttonMinWidth` (92 × scale),
  `maxWidth: '100%'`, `flexShrink: 1`. A control never overflows the row; it
  moves to the next line.
- The `whyNot` line under a disabled control keeps `maxWidth` but must be
  allowed to wrap (it is what makes each slot tall); cap at two lines with
  `numberOfLines={2}`.
- Nothing else in this file changes. It still decides nothing.

**`app/match/[matchId].tsx`**

- Wrap `<OfferBar>` in `<Panel panelId="controls" title="Controls">`, together
  with the error text and the "Waiting for another player…" line that already
  live beside it.
- The `tableRow` becomes adaptive: at `metrics.narrow` it is a **column** (piles
  panel, then the controls panel, each full width); at ≥ 768 it stays a row with
  the controls panel as `flex: 1, minWidth: 0` exactly as today.
- Delete `styles.offerBarWrap`'s `marginTop: 18` hack — the panel header lines
  the two up now.

Acceptance: at 375 px, every `offer-*` button's `right` is ≤ viewport width, and
the buttons occupy more than one row.

### W3 — Every panel can be put away, and stays that way

**New: `src/lib/panelStore.ts`** — a near-copy of `handOrderStore.ts`, same
shape and the same reasons in its doc comment.

```ts
const KEY = 'zolik_panels';
const KEEP = 5;
type Entry = { matchId: string; minimized: string[] };
export function loadMinimized(matchId: string): Promise<string[]>;
export function saveMinimized(matchId: string, ids: string[]): Promise<void>;
```

**New: `src/hooks/usePanelState.ts`**

```ts
export function usePanelState(matchId?: string): {
  isMinimized: (panelId: string) => boolean;
  toggle: (panelId: string) => void;
};
```

Loads once per match, writes on every toggle, swallows storage errors (a lost
preference must never interrupt a game). Panel ids: `zone:<zoneId>`,
`controls`, `table`, `seats`.

Wire it in the match screen and pass `minimized`/`onToggleMinimized` to every
`Panel`.

### W4 — Spreads are named by their owner and share a row

**`src/components/match/ZoneView.tsx`**

- Render through `Panel` instead of its own `View`. `testID={`zone-${zone.id}`}`
  and the drop-registration ref both go to the panel's outer box (invariant 4).
- New props: `title?: string`, `subtitle?: string` — when the caller supplies a
  title (the owner's name), it wins over `label(zone.labelKey)`, which becomes
  the subtitle. When it does not, today's behaviour is unchanged.
- The pile's existing show-all control (`zone-toggle-<id>`) moves into the
  panel's `accessory` slot. It is a different control from the minimize chevron
  and both are present on a pile.
- `forceOpen` is set by the caller when this zone or any group inside it is in
  `activeDrops` or `pressableDrops` — a target may never be hidden by a
  preference.
- The `stackedOverlap: -40` constant is replaced by a derived value:
  `-(metrics.card.compactHeight + ringOuterHeight - metrics.stackedCorner)`.

**`app/match/[matchId].tsx`**

- Collect **all** `kind === 'spread'` zones — the viewer's and everyone else's —
  into one wrapping row, ordered viewer-first then in `view.seats` order:
  ```
  container: { flexDirection: 'row', flexWrap: 'wrap', gap: metrics.panel.gap }
  each panel: { flexGrow: 1, flexBasis: metrics.narrow ? '48%' : 240,
                minWidth: 150, maxWidth: '100%' }
  ```
  Two per row on a phone, as many as fit on a desktop, each sized to the row
  rather than to the screen.
- Title for a spread: `zone.ownerId ? playerName(state.players, zone.ownerId) : label(zone.labelKey)`,
  with `(you)` appended when `zone.ownerId === viewerId`. Subtitle: the label
  key's own text (`Melds`, `Team melds`) when the title came from an owner.
- The old `Section title="Opponents"` disappears for spreads. The `Section`
  helper stays for shared table zones (piles/stacks).

Acceptance: with two spreads on screen at 375 px, their panels have the same
`y` and different `x`, and each panel's title is a player's name.

### W5 — An unseen hand is not drawn

**New: `src/lib/board.ts`** (pure, unit-tested):

```ts
/** A zone claiming to hold cards it isn't showing this viewer. */
export function isConcealed(zone: Zone): boolean;

/**
 * The zones worth drawing for this viewer: their own always, anything not
 * concealed, and anything the card in flight could land on. What is left is
 * a count with nothing shown for it, which the seats already report.
 */
export function drawableZones(
  zones: Zone[], viewerId: string, activeDrops: ReadonlySet<string>,
): Zone[];
```

**Concealment, not content, is the test — the two are not the same thing, and
conflating them was a real bug caught only by running a Canasta match through
this in a browser.** A zone with `count > 0` and no cards and no groups shown
is *hiding* something (that's the module's whole anti-cheat surface); a zone
with `count === 0` is simply *empty*, and emptiness on its own conceals
nothing. `isConcealed` is `zone.count > 0 && no cards && no groups && kind !==
'stack'` — exactly the inverse of the condition the old `{zone.count} hidden}`
text in `ZoneView` already used. An earlier draft of this plan called the
positive test `hasContent` and dropped every zone that failed it; that
silently deleted Canasta's own unowned, always-empty-at-the-start team-melds
zones (`zone.teamMelds`, no `ownerId`) the instant a deal began, because an
empty spread satisfies neither "is the viewer's own" nor "has content" — it
only stopped conflating them by testing against a live Canasta match, which is
the point of §7 below: verify in the browser, don't trust the plan.

Rules, in order: keep if `zone.ownerId === viewerId`; keep if
`activeDrops.has(zoneElementId(zone.id))`; keep if `!isConcealed(zone)`;
otherwise drop. The viewer's own empty spread is kept by the first rule
regardless — it is the target for their first group — but so is *any* empty
spread by the third, owned or not, which is what an unowned Canasta team melds
zone needs.

Apply it in the match screen where `mine`/`shared`/`others` are computed. Keep
the `{zone.count} hidden}` branch in `ZoneView` even though nothing reaches it
for the games in this repo any more — it is dead code only because upstream
filtering now catches every case it used to handle, and `ZoneView` may still
be exercised directly (a test, a future direct caller) without going through
`drawableZones` first.

Acceptance: the text `13 hidden` appears nowhere at 375 px in a Žolíky match,
`zone-hand:<opponentId>` is not in the DOM, and a fresh Canasta deal still
shows both teams' (empty) melds panels side by side.

**Watch for flex stretch when panels share a wrapping row.** A `flexDirection:
'row', flexWrap: 'wrap'` container defaults `alignItems` to `stretch`, which
sizes every panel in one row to match its *tallest* sibling — so a minimized
panel next to an open one silently renders at the open one's height, looking
exactly as if minimizing had done nothing. Every row of panels this plan adds
(the spreads row, the narrow seat strip) needs `alignItems: 'flex-start'`
explicitly; `OfferBar`'s wrapping row and `Section`'s existing `beside` row
already had it, which is why they didn't need a second look.

### W6 — Cards shrink on a small screen

Thread `useMetrics()` through the three files that size cards. Keep colours,
borders and radii in `StyleSheet.create`; compute only the size-dependent
properties inline, memoised on `metrics`.

- **`CardView.tsx`** — width/height/font sizes from `metrics.card`.
  `CARD_METRICS` stays exported as the baseline (other modules import it).
- **`ZoneView.tsx`** — `StackBack`'s `backCompact`/`backFull` and the
  `stackedOverlap` derivation (§4.4).
- **`HandZone.tsx`** — `DropGap`'s `gapCard` must use the identical scaled
  numbers (invariant 3). `DraggableCard`'s `memo` is unaffected — context
  consumers re-render on a context change regardless — but do **not** pass
  metrics as a prop, or the memo breaks on every render.

Expected effect at 375 px: 13 cards at ~36×50 → 6–7 per row, 2 rows, hand height
407 px → ~200 px.

### W7 — The seat strip wraps instead of hiding players

Not asked for explicitly, but it is the same defect: the 4th player is currently
scrolled off-screen with no affordance, on the one screen where knowing who is
at the table matters.

**`SeatStrip.tsx`** — at `metrics.narrow`, drop the horizontal `ScrollView` for a
wrapping row; seat tiles `flexGrow: 1, flexBasis: '48%', minWidth: 0`, with the
tile's own font sizes from `metrics`. At ≥ 768 keep today's scroller. Wrap the
whole strip in a `Panel` (`panelId="seats"`, title "Table") so it can be put
away like everything else.

---

## 5. Test plan

**Unit (`npm test` in `client-react-native/`, jest matches `src/**/*.test.ts`)**

- `src/lib/layout.test.ts` — every breakpoint returns the documented scale;
  fonts never fall below 9; hairlines never scale; `metricsFor` is pure and
  total for widths 200…2000.
- `src/lib/board.test.ts` — a viewer's own empty spread survives; an opponent's
  countable-but-cardless hand does not; a cardless zone that is a live drop
  target survives; a `stack` survives on its count alone; a revealed opponent
  hand (cards present) survives.
- `src/lib/panelStore.test.ts` — round-trips, evicts past `KEEP`, survives
  unreadable storage.
- `src/lib/__tests__/shell.test.ts` — add `src/components/match/Panel.tsx`,
  `src/lib/layout.ts`, `src/lib/board.ts` to `SHELL_FILES` and raise the count
  guard. Confirm the vocabulary grep still passes.

**e2e (`e2e/`, Playwright against the real stack)**

New `e2e/tests/adaptive-layout.spec.ts`, running at a phone viewport via
`test.use({ viewport: { width: 375, height: 812 } })`:

1. *Every control is reachable* — for each `[data-testid^="offer-"]`, its
   bounding box `x + width <= 375`; and the set of distinct `y` values is > 1
   (the bar genuinely wrapped).
2. *No hand is drawn that nobody can see* — `page.getByText('hidden')` has count
   0, and no `[data-testid^="zone-hand:"]` other than the viewer's own.
3. *Two spreads share a row* — seed **Canasta** (its team spreads are emitted
   from the deal, so this is deterministic), assert two spread panels with
   `|y1 - y2| < 8` and `x2 > x1 + w1 - 1`.
4. *A panel can be put away and stays away* — click `panel-toggle-zone:<id>`,
   assert the body is gone and the header remains; reload; assert it is still
   minimized.
5. *Minimize never hides a drop target* — with a panel minimized, start a drag
   from the hand onto that zone (reuse `helpers/drag.ts`) and assert it opens
   and accepts the drop.
6. *Desktop is unregressed* — the existing `board-layout.spec.ts` runs at the
   default desktop viewport and must pass untouched.

Run the lot with `./scripts/dev-stack.sh test`.

---

## 6. Risks, and what to do about them

**R1 — measuring a panel that is still collapsed.** `beginDrag` calls
`drops.measure()` immediately, but a panel that `forceOpen` is about to expand
has not laid out yet, so its rect is the collapsed header's. Fix in
`useDropRegistry.measure()`: defer the pass one frame
(`requestAnimationFrame`, falling back to `setTimeout(0)` where it is absent).
Re-measure is idempotent and cheap. Verify with e2e case 5 above — it is the one
test that catches this.

**R2 — a scaled gap that does not match a scaled card.** Symptom: cards jitter
by a few pixels as a drag moves along the fan. Both must come from the same
`metrics` object in the same render (invariant 3).

**R3 — the shell vocabulary grep.** It strips comments, so prose is free; it is
identifiers that fail. Run `npm test` before every commit, not at the end.

**R4 — width changes mid-drag** (a desktop window resized, or a phone rotated).
`metricsFor` changes, every card resizes, and the rects measured at pick-up are
stale. Acceptable — a resize mid-drag is not a real scenario — but do not add
animation to the scale change, which would make it a *continuous* one.

**R5 — `flexBasis: '48%'` with `gap`.** On react-native-web the gap is added
outside the basis, so two 48% panels plus a gap can exceed 100% and wrap to one
per row. If that shows up, use `flexBasis: 0, flexGrow: 1, minWidth: 150`
instead and let flex do the division.

---

## 7. Verifying it, for real

Do not infer from the code that it worked — measure it, the same way §1 was
measured.

```bash
./scripts/dev-stack.sh up    # API :8096, web :8114  (a plain dev server is :8090/:8081)
```

Seed a match against bots and open it as that guest:

```bash
API=http://127.0.0.1:8090
G=$(curl -fsS -X POST $API/auth/guest -H 'content-type: application/json' -d '{"guestName":"ui-check"}')
TOK=$(echo "$G" | python3 -c 'import sys,json;print(json.load(sys.stdin)["accessToken"])')
MID=$(curl -fsS -X POST $API/matches -H "Authorization: Bearer $TOK" -H 'content-type: application/json' -d '{"moduleId":"zolik","options":{}}' | python3 -c 'import sys,json;print(json.load(sys.stdin)["matchId"])')
for i in 1 2 3; do curl -fsS -X POST $API/matches/$MID/add-bot -H "Authorization: Bearer $TOK" -o /dev/null; done
curl -fsS -X POST $API/matches/$MID/start -H "Authorization: Bearer $TOK" -o /dev/null
echo "$G"; echo "match $MID"
```

Then, in the browser at the web port, put the guest's
`{accessToken, refreshToken, userId, username, isGuest}` into
`localStorage.zolik_session`, resize to **375×812**, and open `/match/<MID>`.
Note the web build's API base is baked in at build time — match the port the
build points at (a stock dev build points at `:8090`).

The numbers to re-read, against §1:

```js
const bar = document.querySelector('[data-testid="action-bar"]');
const scr = document.querySelector('[data-testid="match-screen"]');
({ barFits: bar.scrollWidth <= bar.clientWidth,      // was 1003 > 135
   pageHeight: scr.scrollHeight,                     // was 1034
   hiddenHands: document.body.innerText.match(/hidden/g)?.length ?? 0 })  // was 3
```

Targets: `barFits` true, `pageHeight` under ~700 (the board fits a phone
without scrolling past the hand), `hiddenHands` 0. Take a screenshot at 375 and
at 1280 and check the desktop one against the current build — the wide layout
should be visibly unchanged apart from the named spread panels and the minimize
chevrons.

---

## 8. Suggested commit sequence

Each step builds and tests green on its own.

1. `feat(client): one place that knows how big a card is` — W0 + `layout.test.ts`.
2. `feat(client): a panel every region of the board is drawn in` — W1, adopted
   by `ZoneView` and `HandZone` with no behaviour change yet.
3. `fix(client): keep every control inside its rectangle` — W2.
4. `feat(client): put a panel away, and find it that way` — W3.
5. `feat(client): name a spread by whose it is, and let them share a row` — W4.
6. `fix(client): stop drawing hands nobody can see` — W5 + `board.test.ts`.
7. `feat(client): size the cards to the screen they are on` — W6.
8. `fix(client): show every seat on a phone` — W7.
9. `test(e2e): the board on a phone` — the new spec, plus the shell-file list.
