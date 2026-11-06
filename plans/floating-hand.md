# Floating hand (dockable hand + action bar)

## The problem

In CanastaX the board is laid out top to bottom as: table → move announcements →
**hand → action bar** → your side's melds → opponents
(`BoardLayout.tsx:253-256`, the `holdsHand ? spreadsRow` branch). Once a side has
a dozen melds, looking them over means scrolling down, and the hand scrolls off
the top. You then can't drag a card onto the meld you're looking at, so you
scroll back up, pick, and scroll down again while carrying it.

## What we build

A **Float hand** toggle. When it's on, the hand and the action bar leave the
page flow and dock as a compact strip pinned to the top of the match screen.
The board scrolls underneath. Everything the in-flow hand can do still works
from the dock:

- tap to select, drag to rearrange;
- drag onto a meld to add to it;
- drag a natural onto a meld that holds its wild to poach;
- every action-bar button.

**Only one copy exists.** The hand and the bar are rendered once. Floating
moves that single element from the `hand`/`controls` slots of `BoardLayout`
into an overlay. While it floats, those slots render nothing, so the melds move
up into the space it left. Nothing is hidden-but-mounted, so testIDs and focus
targets stay unique (see the "stack screens stay mounted" memory for why this
matters).

## Design

### 1. One element, two places (`app/match/[matchId].tsx`)

- Build `handDock = <>{handPanel}{controlsPanel}</>` once, as today.
- State: `floating` (boolean). Persist it per module through the existing
  settings store, the same way Auto-arrange is persisted, defaulting to off.
- When `floating` is false: pass it to `BoardLayout` as now.
- When `floating` is true: pass `hand={null} controls={null}` to `BoardLayout`,
  and render `handDock` as a sibling **after** the `ScrollView` inside
  `screen`, in an absolutely positioned `<View style={styles.dock}>` pinned to
  `top: 0` (below `TableBanner`/`GuideCard`), at the board's `maxWidth`, with
  the felt colour behind it and a bottom shadow.
- Toggling remounts `HandZone`. That's fine because everything worth keeping
  lives in the parent: `useHandOrder` slots, `selected`, `offerParams`, and
  the armed meld. Turn the toggle off while a drag is live so it can't
  remount mid-gesture.
- When the dock first appears, give the `ScrollView` a `paddingTop` (via
  `contentContainerStyle`) equal to the dock's measured height. That way the
  top of the board can still be scrolled out from under it. Read the height
  from the dock's `onLayout`.

### 2. The toggle

- A small pin/unpin icon button at the right end of the hand's header row,
  next to Auto-arrange. It's the same control in both states, so it is always
  where the hand is. Label: "Float hand" / "Put hand back". It gets a11y
  tooltips plus a keyboard shortcut (`F`) in `offerShortcuts`' neighbourhood.
- Not offered when the viewer holds no hand (blackjack, spectators, replays).
  Not offered between rounds either (`paused`), when the controls panel moves
  up beside the settlement.
- Optional later step: **auto-float**. If the toggle is on but the in-flow
  hand is still on screen, it stays in flow, and it docks only once it scrolls
  out (sticky behaviour). Start with the explicit button; add auto-float only
  if the button feels like a chore.

### 3. Compact presentation while docked

The dock must stay short, or it just takes the space the melds needed.

- **Hand.** `HandZone` gets a `compact` prop: a smaller card scale (one notch
  down the existing metrics, not a skin, because sizes are layout), a single
  overlapping row, and horizontal scrolling if it overflows. Badges, the
  carry/hold-to-drag behaviour and keyboard focus are unchanged.
- **Action bar.** `OfferBar` gets a `compact` prop:
  - one row;
  - enabled primary offers first, at full name;
  - undo offers collapsed to a single "Undo" with a menu;
  - disabled offers folded into a "More" menu that still shows their reasons
    (offers never vanish, they just move);
  - params steppers inline only when the lead offer needs them.
- **Text.** The hint, error and "waiting for" lines become a single clipped
  status line in the dock. Tapping the error still opens `WhySheet`.
- **Collapse.** A grab handle collapses the dock to just the action bar,
  roughly 44 px. Tapping the handle brings the cards back.
- **Height cap.** At most ~40% of the window height. On a phone in landscape,
  the hand row and the bar sit side by side.

### 4. Drag and drop from the dock

The drop machinery already works in window coordinates
(`useDropRegistry.measureInWindow`, gesture `x/y`) and re-measures at every
drag start. So a card dragged out of an overlay lands on melds with no
coordinate translation. What needs adding:

1. **Occlusion.** A meld scrolled *under* the dock still has a window rect.
   `hit()` would accept a drop there, and the auto-scroll would treat it as "on
   screen". Give `useDropRegistry` an `occluder` rect (the dock's window rect,
   updated from `onLayout` + `measureInWindow`). Clip every target rect against
   it before hit-testing, and drop targets that are fully covered. Unit-test
   this in `useDropRegistry` with a target that is partly and fully covered.
2. **Auto-scroll bands.** The edge auto-scroll (`[matchId].tsx` ~700-748)
   measures its top band from the window top. While docked, the top band
   starts at the dock's bottom edge, so dragging up past the dock's edge
   scrolls the board up toward the table. The bottom band is unchanged.
3. **Re-measure on dock changes.** When the dock collapses, expands or changes
   height, call `drops.measure()`, the same as after a scroll.
4. **No zIndex flip mid-gesture.** Today the hand gets `dragLayer`
   (`zIndex: 100`) only while dragging. On iOS a zIndex change mid-pan cancels
   the gesture (see `meldCarryStacking.test.ts`). The dock sits at a fixed
   zIndex above the `ScrollView` the whole time it is shown. So don't apply
   `dragLayer` to it at all, and make sure the carried card's flight layer
   still draws above the dock.
5. **Poaching.** CanastaX's poach is an ordinary targeted offer
   (`house.go:1034-1043`: `poach:<meldId>`, the hand card goes onto
   `Target: meldId`), so `spotsFor` already lights the meld for a dragged
   natural. Nothing new is needed beyond items 1-4. Cover it with a test
   anyway, because it is the case the user named.
6. **Press path.** Tap a card in the dock, then tap a lit meld (`pressDrop`).
   This already works because it doesn't depend on position. Check that the
   `pageY` → position choice still reads correctly with the dock's padding
   offset.

### 5. "Lay off" → "Add" (wording)

"Lay off" is rummy jargon, and new players don't parse it. Proposal:

| key | now | proposed |
|---|---|---|
| `verb.lay_off` | Lay off | **Add to meld** |
| `verb.undoLayOff` | Undo lay off | **Undo add** |
| `verb.finish_layoff` | Done laying off | **Done adding** |
| `verb.poach` | Poach a wild | **Swap for wild** (tooltip explains) |
| `a11y.board.tip.playTo` (used when a lay-off has a target) | Plays the selected cards to {target}. | **Adds the selected cards to {target}.** for add offers; a new key, `a11y.board.tip.addTo` |
| new `a11y.board.tip.poach` | — | **Swap a natural card from your hand for the wild in {target}, and take the wild.** |

- Canasta's rules and remedy strings (`canasta.rules.layOffAfterOpening`,
  `canasta.remedy.*`) change from "lay off" to "add" in step.
- **Gin Rummy keeps "Lay off"** (`ginrummy.offer.layOff`), because there it's
  the game's own term. Its tooltip gains a plain explanation instead: "Add your
  unmatched cards to your opponent's melds."
- Mechanics: 23 locale files. For the server-side keys, regenerate
  `serverKeys.json`, then run `go generate ./internal/gamemcp` (en.ts feeds
  `messages_en.json`). Keys stay static literals.
- This is independent of the dock and small. It ships first, as its own PR.

## Phases and PRs

1. **Wording PR**, section 5. Small, needs no UI work, and is easy to review.
2. **Dock PR**, sections 1, 2, 4. The full-size hand and bar move into the
   dock unchanged. This proves single-instance rendering, occlusion,
   auto-scroll and poach-by-drag before any visual work.
3. **Compact PR**, section 3. Compact `HandZone`/`OfferBar`, the collapse
   handle and the height cap.
4. ~~Auto-float~~: dropped, button only.

## Verification

- **Unit:** `useDropRegistry` occlusion clipping, and the `OfferBar` compact
  grouping (no offer lost, disabled reasons reachable).
- **e2e (web, Playwright), CanastaX, several seeds** (melds only pile up in
  some shuffles):
  - toggle the dock: `[data-testid="hand-zone"]` count is exactly 1 in both
    states, and the action bar count is 1;
  - scroll to the bottom meld, drag a card from the docked hand onto it, and
    assert the meld grew in the DOM the user sees, not just the server state;
  - drop onto a meld that sits under the dock: refused, with no lay-off sent;
  - poach: drag a natural onto a meld holding its wild, and assert the wild
    is now in hand;
  - drag up past the dock edge: the board scrolls.
- **Touch:** a `touchCarry` spec for the dock drag (mouse specs missed the last
  phone drag bug), plus an iOS simulator pass on a real CanastaX table:
  hold-to-drag from the dock onto a meld, with no cancelled pan.
- **Phone width:** check at a plain custom width (~390), not the mobile preset.
- Measure in a live browser before calling any layout bug fixed.

## Decisions (2026-11-04)

1. **Button only.** The pin button sits at the right of the hand's header row.
   Press it and the hand floats; press it again and the hand goes back into
   the page. No auto-float. Phase 4 is dropped.
2. **Remembered per game** (per module id).
3. **Wording approved:** "Add to meld" and "Swap for wild", plus the related
   strings in section 5.
