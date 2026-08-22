import { StyleSheet, View } from 'react-native';
import { Gesture, GestureDetector, ScrollView } from 'react-native-gesture-handler';
import Animated, {
  runOnJS,
  useAnimatedStyle,
  useSharedValue,
  type SharedValue,
} from 'react-native-reanimated';

import { CardView } from '@/src/components/CardView';
import { moveCardToIndex } from '@/src/lib/cards';

export type DropZone = { x: number; y: number; width: number; height: number };

export type MeldZone = { meldId: string; zone: DropZone; type?: 'run' | 'set' };

export type MeldHoverTarget = { meldId: string; position: 'front' | 'end' } | null;

// Shared drag-preview state, owned by the screen that renders HandRow so it
// can also render the floating overlay outside HandRow's own ScrollView
// (which would otherwise clip the dragged card instead of letting it float
// over other content like the discard pile).
export type DragPreview = {
  x: SharedValue<number>;
  y: SharedValue<number>;
  // Where within the card it was grabbed, so the overlay stays glued to the
  // finger/cursor at that same point instead of re-centering itself on it.
  offsetX: SharedValue<number>;
  offsetY: SharedValue<number>;
  active: SharedValue<boolean>;
  draggingIndex: SharedValue<number>;
};

export function useDragPreview(): DragPreview {
  const x = useSharedValue(0);
  const y = useSharedValue(0);
  const offsetX = useSharedValue(0);
  const offsetY = useSharedValue(0);
  const active = useSharedValue(false);
  const draggingIndex = useSharedValue(-1);
  return { x, y, offsetX, offsetY, active, draggingIndex };
}

type Props = {
  cards: string[];
  selected: Set<number>;
  // Every real action (discard, stage into a new meld, lay off, swap a
  // joker) happens by dragging a card onto its target — see the various
  // onDropOn* callbacks below. A plain tap is the one non-drag gesture left:
  // it just toggles the card's gold-ring selection, used to build a
  // multi-card lay-off/swap-joker before dragging one of the selected cards
  // onto a table meld (see MeldTable).
  onTapCard: (index: number) => void;
  onReorder: (newOrder: string[]) => void;
  // Measures the discard pile's current screen-space rect and reports it
  // via callback. Measured live at drag-end time (rather than read from a
  // value cached on scroll/layout) so it reflects the real current layout.
  // Reporting null disables drop detection.
  measureDropZone?: (cb: (zone: DropZone | null) => void) => void;
  onDropOnZone?: (index: number) => void;
  // Same idea for table melds a dragged card can be laid off onto.
  measureMeldZones?: (cb: (zones: MeldZone[]) => void) => void;
  onDropOnMeld?: (index: number, meldId: string, position: 'front' | 'end') => void;
  // Same idea again for the "build a new meld" staging area. absoluteX/Y
  // are passed through (rather than resolved to a position here) so the
  // caller can work out which group and which position within it the card
  // was actually dropped on, instead of always appending to the end.
  measureStagingZone?: (cb: (zone: DropZone | null) => void) => void;
  onDropOnStaging?: (index: number, absoluteX: number, absoluteY: number) => void;
  onDragCardChange?: (card: string | null) => void;
  // Fires (throttled, on the JS thread) as the finger moves during a drag,
  // so the screen can live-highlight whichever meld — and which end of it —
  // is currently under the finger. Absent means "no live highlight", not an
  // error; dropping still works purely from handleDragEnd's own zone check.
  onDragHover?: (absoluteX: number, absoluteY: number) => void;
  dragPreview: DragPreview;
  compact?: boolean;
  // Card value just picked up from the deck/discard pile this turn — every
  // card matching it gets the "just drawn" ring (see CardView).
  justDrawnCard?: string | null;
};

const CARD_WIDTH = 52;
const CARD_WIDTH_COMPACT = 44;
const CARD_MARGIN = 6;

export function pointInZone(x: number, y: number, zone: DropZone): boolean {
  return x >= zone.x && x <= zone.x + zone.width && y >= zone.y && y <= zone.y + zone.height;
}

// Table melds sit close together (especially once several are stacked
// under one owner), so their hit-tested zones — each padded out from its
// visual rect to make aiming forgiving — commonly overlap a neighbor's.
// Picking the *first* zone a point falls in (array order) then silently
// resolves a drop meant for meld B onto meld A whenever their padding
// overlaps near the boundary. Picking the one whose unpadded center is
// nearest the point instead disambiguates by "which meld is this actually
// closest to", which matches what the player sees, regardless of how much
// the padding overlaps.
export function closestZone<T extends { zone: DropZone }>(x: number, y: number, zones: T[]): T | null {
  let best: T | null = null;
  let bestDist = Infinity;
  for (const z of zones) {
    if (!pointInZone(x, y, z.zone)) continue;
    const cx = z.zone.x + z.zone.width / 2;
    const cy = z.zone.y + z.zone.height / 2;
    const dist = (x - cx) ** 2 + (y - cy) ** 2;
    if (dist < bestDist) {
      bestDist = dist;
      best = z;
    }
  }
  return best;
}

// Which half of a meld's rect a point falls in — the left half means
// "extend the front (low) end of the run", the right half "extend the end
// (high) end". Runs render their cards left-to-right in ascending rank (see
// OrderMeldForDisplay server-side), so this lines up with what's on screen.
export function zonePosition(x: number, zone: DropZone): 'front' | 'end' {
  return x < zone.x + zone.width / 2 ? 'front' : 'end';
}

export function HandRow({
  cards,
  selected,
  onTapCard,
  onReorder,
  measureDropZone,
  onDropOnZone,
  measureMeldZones,
  onDropOnMeld,
  measureStagingZone,
  onDropOnStaging,
  onDragCardChange,
  onDragHover,
  dragPreview,
  compact,
  justDrawnCard,
}: Props) {
  const slot = (compact ? CARD_WIDTH_COMPACT : CARD_WIDTH) + CARD_MARGIN;

  return (
    <ScrollView horizontal showsHorizontalScrollIndicator style={styles.row}>
      {cards.map((c, i) => (
        <DraggableCard
          key={`${c}-${i}`}
          card={c}
          index={i}
          count={cards.length}
          slot={slot}
          selected={selected.has(i)}
          justDrawn={c === justDrawnCard}
          compact={compact}
          onTapCard={onTapCard}
          measureDropZone={measureDropZone}
          onDropOnZone={onDropOnZone}
          measureMeldZones={measureMeldZones}
          onDropOnMeld={onDropOnMeld}
          measureStagingZone={measureStagingZone}
          onDropOnStaging={onDropOnStaging}
          onDragCardChange={onDragCardChange}
          onDragHover={onDragHover}
          dragPreview={dragPreview}
          onDrop={(from, to) => onReorder(moveCardToIndex(cards, from, to))}
        />
      ))}
      <View style={styles.spacer} />
    </ScrollView>
  );
}

function DraggableCard({
  card,
  index,
  count,
  slot,
  selected,
  justDrawn,
  compact,
  onTapCard,
  measureDropZone,
  onDropOnZone,
  measureMeldZones,
  onDropOnMeld,
  measureStagingZone,
  onDropOnStaging,
  onDragCardChange,
  onDragHover,
  dragPreview,
  onDrop,
}: {
  card: string;
  index: number;
  count: number;
  slot: number;
  selected: boolean;
  justDrawn?: boolean;
  compact?: boolean;
  onTapCard: (index: number) => void;
  measureDropZone?: (cb: (zone: DropZone | null) => void) => void;
  onDropOnZone?: (index: number) => void;
  measureMeldZones?: (cb: (zones: MeldZone[]) => void) => void;
  onDropOnMeld?: (index: number, meldId: string, position: 'front' | 'end') => void;
  measureStagingZone?: (cb: (zone: DropZone | null) => void) => void;
  onDropOnStaging?: (index: number, absoluteX: number, absoluteY: number) => void;
  onDragCardChange?: (card: string | null) => void;
  onDragHover?: (absoluteX: number, absoluteY: number) => void;
  dragPreview: DragPreview;
  onDrop: (from: number, to: number) => void;
}) {
  // A plain tap just toggles selection — no double-tap/long-press timing
  // games needed since nothing else competes with tap for this gesture
  // anymore (every action is a drag; see the module doc comment on Props).
  const tap = Gesture.Tap().onEnd((_e, success) => {
    if (success) runOnJS(onTapCard)(index);
  });

  function handleDragStart() {
    onDragCardChange?.(card);
  }

  function clearDragState() {
    onDragCardChange?.(null);
  }

  // Round-to-nearest-slot flips as soon as translationX crosses half a
  // slot's width (~29px) — barely past the pan gesture's 10px activation
  // distance. That gap was small enough that an ordinary tap with a little
  // hand tremor or trackpad drift would swap the card into the next slot.
  // Require clearing most of a slot before committing to a move; anything
  // short of that snaps back to where the card started.
  const REORDER_COMMIT_RATIO = 0.75;

  function reorder(translationX: number) {
    const slots = translationX / slot;
    const deltaSlots =
      Math.abs(slots) < REORDER_COMMIT_RATIO
        ? 0
        : Math.sign(slots) * Math.round(Math.abs(slots) - (REORDER_COMMIT_RATIO - 0.5));
    const target = Math.max(0, Math.min(count - 1, index + deltaSlots));
    if (target !== index) {
      onDrop(index, target);
    }
  }

  // A fast, short flick upward reads as "discard" even if the card never
  // reaches the discard pile's on-screen rect — asking players to drag all
  // the way there felt heavier than it needed to be. Only the *last*
  // resort, though: checked after every real drop zone (discard pile,
  // table melds, staging area) has already come up empty. The staging area
  // and table melds both sit above the hand row in this layout, so an
  // ordinary deliberate drag up into either of them is also, incidentally,
  // a fast upward motion — checking this first would silently reroute that
  // drag into a discard instead of the meld action the player was aiming
  // for, wherever their finger actually let go.
  const QUICK_SWIPE_UP_DISTANCE = 40;
  const QUICK_SWIPE_UP_VELOCITY = -600;

  function tryDropOnStaging(translationX: number, translationY: number, velocityY: number, absoluteX: number, absoluteY: number) {
    function fallback() {
      if (onDropOnZone && translationY < -QUICK_SWIPE_UP_DISTANCE && velocityY < QUICK_SWIPE_UP_VELOCITY) {
        onDropOnZone(index);
      } else {
        reorder(translationX);
      }
    }
    if (!measureStagingZone) {
      fallback();
      return;
    }
    measureStagingZone((zone) => {
      if (zone && onDropOnStaging && pointInZone(absoluteX, absoluteY, zone)) {
        onDropOnStaging(index, absoluteX, absoluteY);
      } else {
        fallback();
      }
    });
  }

  // Checked in order: an existing table meld (lay off — the higher-stakes
  // action once you're down) before the new-meld staging area (just
  // selecting a card, same effect as tapping it).
  function tryDropOnMeld(
    translationX: number,
    translationY: number,
    velocityY: number,
    absoluteX: number,
    absoluteY: number,
  ) {
    if (!measureMeldZones) {
      tryDropOnStaging(translationX, translationY, velocityY, absoluteX, absoluteY);
      return;
    }
    measureMeldZones((zones) => {
      const hit = closestZone(absoluteX, absoluteY, zones);
      if (hit && onDropOnMeld) {
        onDropOnMeld(index, hit.meldId, zonePosition(absoluteX, hit.zone));
      } else {
        tryDropOnStaging(translationX, translationY, velocityY, absoluteX, absoluteY);
      }
    });
  }

  // Zones are measured live (not read from a value cached on scroll/layout
  // events) so a drop always sees the current on-screen position, even if
  // the page scrolled or the layout shifted since the last such event. The
  // discard pile's own rect is still checked directly (rather than folded
  // into the quick-swipe fallback) since it's a real, aimable target same as
  // a meld or the staging area — the swipe-up shortcut only kicks in once
  // none of the real drop zones matched, see tryDropOnStaging.
  function handleDragEnd(
    translationX: number,
    translationY: number,
    velocityY: number,
    absoluteX: number,
    absoluteY: number,
  ) {
    onDragCardChange?.(null);
    if (!measureDropZone) {
      tryDropOnMeld(translationX, translationY, velocityY, absoluteX, absoluteY);
      return;
    }
    measureDropZone((zone) => {
      if (zone && onDropOnZone && pointInZone(absoluteX, absoluteY, zone)) {
        onDropOnZone(index);
      } else {
        tryDropOnMeld(translationX, translationY, velocityY, absoluteX, absoluteY);
      }
    });
  }

  // Set synchronously inside onStart/onEnd's worklets (not via runOnJS —
  // see onFinalize below for why the ordering matters) so onFinalize can
  // tell whether the gesture ever actually got moving, and whether onEnd
  // ran, before it fires.
  const dragStarted = useSharedValue(false);
  const dragEnded = useSharedValue(false);

  const pan = Gesture.Pan()
    .minDistance(10)
    .onStart((e) => {
      dragStarted.value = true;
      dragEnded.value = false;
      dragPreview.draggingIndex.value = index;
      dragPreview.active.value = true;
      // Keep the card glued to the exact point it was grabbed at, instead
      // of re-centering itself under the cursor — otherwise the card jumps
      // on pickup and the grab point drifts relative to the card as you drag.
      dragPreview.offsetX.value = e.x;
      dragPreview.offsetY.value = e.y;
      dragPreview.x.value = e.absoluteX;
      dragPreview.y.value = e.absoluteY;
      runOnJS(handleDragStart)();
    })
    .onUpdate((e) => {
      dragPreview.x.value = e.absoluteX;
      dragPreview.y.value = e.absoluteY;
      if (onDragHover) runOnJS(onDragHover)(e.absoluteX, e.absoluteY);
    })
    .onEnd((e) => {
      dragEnded.value = true;
      runOnJS(handleDragEnd)(e.translationX, e.translationY, e.velocityY, e.absoluteX, e.absoluteY);
    })
    .onFinalize(() => {
      dragPreview.active.value = false;
      dragPreview.draggingIndex.value = -1;
      // onEnd only fires when the pan gesture is recognized as a genuine
      // release — react-native-gesture-handler's web pointer manager can
      // fail to emit it and cancel the gesture instead if the pointer sits
      // still for a beat before lifting, which is completely ordinary human
      // behavior when carefully aiming a drag at a small target (confirmed
      // via e2e: a drag that pauses ~150ms before release over a table meld
      // silently never sends the lay_off). Cancelled means handleDragEnd
      // (the actual drop resolution — discard/meld/staging — and the
      // onDragCardChange(null) that clears the hover highlights) never
      // runs at all, so both the drop and the highlight get stranded.
      // onFinalize always runs, success or cancel, so recover here using
      // dragPreview.x/y — continuously updated on every onUpdate, so it
      // still holds wherever the finger last was — with translation/
      // velocity zeroed (only affects the reorder/quick-swipe fallback,
      // which a cancelled gesture has no reliable numbers for anyway).
      // Gated on dragStarted too: onFinalize also fires for a gesture that
      // never activated at all (lost the Race to tap, or never even
      // reached onStart) — dragPreview.x/y would still be whatever the
      // *previous* drag on this card left them at (or 0,0 if there's never
      // been one), so firing the fallback unconditionally could resolve a
      // drop against a stale or bogus position instead of just no-oping.
      if (dragStarted.value && !dragEnded.value) {
        runOnJS(handleDragEnd)(0, 0, 0, dragPreview.x.value, dragPreview.y.value);
      }
      dragStarted.value = false;
      runOnJS(clearDragState)();
    });

  const gesture = Gesture.Race(pan, tap);

  const animatedStyle = useAnimatedStyle(() => ({
    opacity: dragPreview.draggingIndex.value === index ? 0.3 : 1,
    cursor: (dragPreview.draggingIndex.value === index ? 'grabbing' : 'grab') as string,
  }));

  return (
    <GestureDetector gesture={gesture}>
      {/* userSelect: none stops a quick tap from being swallowed by the
          browser's native "select this text" behavior. */}
      <Animated.View style={[animatedStyle, { userSelect: 'none' } as object]}>
        <CardView
          card={card}
          selected={selected}
          justDrawn={justDrawn}
          compact={compact}
          testID={`hand-card-${index}`}
        />
      </Animated.View>
    </GestureDetector>
  );
}

const styles = StyleSheet.create({
  row: {
    flexGrow: 0,
    marginVertical: 8,
  },
  spacer: {
    width: 8,
  },
});
