import { useEffect, useRef } from 'react';
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
  // Plain tap: stages the card straight into the meld-building pane, same
  // destination a drag-drop onto the staging area reaches.
  onTapCard: (index: number) => void;
  // Long-press: toggles the card's gold-ring selection used for the
  // lay-off / swap-joker flow (see MeldTable) — a separate gesture from tap
  // since tap is now an immediate action, not a selection.
  onLongPress?: (index: number) => void;
  onReorder: (newOrder: string[]) => void;
  onDoubleTap?: (index: number) => void;
  // Measures the discard pile's current screen-space rect and reports it
  // via callback. Measured live at drag-end time (rather than read from a
  // value cached on scroll/layout) so it reflects the real current layout.
  // Reporting null disables drop detection.
  measureDropZone?: (cb: (zone: DropZone | null) => void) => void;
  onDropOnZone?: (index: number) => void;
  // Same idea for table melds a dragged card can be laid off onto.
  measureMeldZones?: (cb: (zones: { meldId: string; zone: DropZone }[]) => void) => void;
  onDropOnMeld?: (index: number, meldId: string) => void;
  // Same idea again for the "build a new meld" staging area — dropping a
  // card there just selects it (same effect as tapping it).
  measureStagingZone?: (cb: (zone: DropZone | null) => void) => void;
  onDropOnStaging?: (index: number) => void;
  onDragCardChange?: (card: string | null) => void;
  dragPreview: DragPreview;
  compact?: boolean;
  // When true, a single tap discards the card directly instead of toggling
  // selection — used during the discard phase, where selecting a card only
  // ever leads to discarding it anyway.
  tapToDiscard?: boolean;
  // Card value just picked up from the deck/discard pile this turn — every
  // card matching it gets the "just drawn" ring (see CardView).
  justDrawnCard?: string | null;
};

const CARD_WIDTH = 52;
const CARD_WIDTH_COMPACT = 44;
const CARD_MARGIN = 6;
const DOUBLE_TAP_MS = 400;

function pointInZone(x: number, y: number, zone: DropZone): boolean {
  return x >= zone.x && x <= zone.x + zone.width && y >= zone.y && y <= zone.y + zone.height;
}

export function HandRow({
  cards,
  selected,
  onTapCard,
  onLongPress,
  onReorder,
  onDoubleTap,
  measureDropZone,
  onDropOnZone,
  measureMeldZones,
  onDropOnMeld,
  measureStagingZone,
  onDropOnStaging,
  onDragCardChange,
  dragPreview,
  compact,
  tapToDiscard,
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
          onLongPress={onLongPress}
          onDoubleTap={onDoubleTap}
          measureDropZone={measureDropZone}
          onDropOnZone={onDropOnZone}
          measureMeldZones={measureMeldZones}
          onDropOnMeld={onDropOnMeld}
          measureStagingZone={measureStagingZone}
          onDropOnStaging={onDropOnStaging}
          onDragCardChange={onDragCardChange}
          dragPreview={dragPreview}
          tapToDiscard={tapToDiscard}
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
  onLongPress,
  onDoubleTap,
  measureDropZone,
  onDropOnZone,
  measureMeldZones,
  onDropOnMeld,
  measureStagingZone,
  onDropOnStaging,
  onDragCardChange,
  dragPreview,
  tapToDiscard,
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
  onLongPress?: (index: number) => void;
  onDoubleTap?: (index: number) => void;
  measureDropZone?: (cb: (zone: DropZone | null) => void) => void;
  onDropOnZone?: (index: number) => void;
  measureMeldZones?: (cb: (zones: { meldId: string; zone: DropZone }[]) => void) => void;
  onDropOnMeld?: (index: number, meldId: string) => void;
  measureStagingZone?: (cb: (zone: DropZone | null) => void) => void;
  onDropOnStaging?: (index: number) => void;
  onDragCardChange?: (card: string | null) => void;
  dragPreview: DragPreview;
  tapToDiscard?: boolean;
  onDrop: (from: number, to: number) => void;
}) {
  const lastTapAt = useRef(0);
  // A staged card is pulled out of the hand array entirely (see
  // visibleHand in the game screen), so firing onTapCard immediately on the
  // first tap would unmount this exact component before a following second
  // tap could ever reach it — the double-tap-to-discard gesture would just
  // silently become "stage this card, then select whatever slid into its
  // place." Holding the stage until the double-tap window closes without a
  // second tap arriving keeps that gesture reliable regardless of what the
  // single-tap path does to the layout.
  const pendingToggle = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    return () => {
      if (pendingToggle.current) clearTimeout(pendingToggle.current);
    };
  }, []);

  // Manual JS timing on top of a single gesture-handler Tap (the same
  // gesture system as the pan below, which is proven to work — mixing it
  // with a plain RN Pressable underneath turned out not to be reliable).
  // Two taps within the window count as one discard, not one stage.
  function handleTap() {
    // During the discard phase a single tap discards outright — there's no
    // other reason to tap a card there, so making the player double-tap
    // was just extra friction.
    if (tapToDiscard) {
      if (onDoubleTap) onDoubleTap(index);
      return;
    }
    const now = Date.now();
    if (now - lastTapAt.current < DOUBLE_TAP_MS) {
      lastTapAt.current = 0;
      if (pendingToggle.current) {
        clearTimeout(pendingToggle.current);
        pendingToggle.current = null;
      }
      if (onDoubleTap) onDoubleTap(index);
      return;
    }
    lastTapAt.current = now;
    pendingToggle.current = setTimeout(() => {
      pendingToggle.current = null;
      onTapCard(index);
    }, DOUBLE_TAP_MS);
  }

  const tap = Gesture.Tap().onEnd((_e, success) => {
    if (success) runOnJS(handleTap)();
  });

  // Holding a card selects it (gold ring) for the lay-off/swap-joker flow
  // instead of staging it — a distinct, deliberate gesture from a plain
  // tap so the two don't collide. Cancels the pending tap-stage timer (and
  // any double-tap-discard window) so a long-press never also fires a stage
  // once the finger lifts.
  function handleLongPress() {
    if (!onLongPress) return;
    lastTapAt.current = 0;
    if (pendingToggle.current) {
      clearTimeout(pendingToggle.current);
      pendingToggle.current = null;
    }
    onLongPress(index);
  }

  const longPress = Gesture.LongPress()
    .minDuration(350)
    .maxDistance(10)
    .onStart(() => {
      if (tapToDiscard) return;
      runOnJS(handleLongPress)();
    });

  function handleDragStart() {
    onDragCardChange?.(card);
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

  function tryDropOnStaging(translationX: number, absoluteX: number, absoluteY: number) {
    if (!measureStagingZone) {
      reorder(translationX);
      return;
    }
    measureStagingZone((zone) => {
      if (zone && onDropOnStaging && pointInZone(absoluteX, absoluteY, zone)) {
        onDropOnStaging(index);
      } else {
        reorder(translationX);
      }
    });
  }

  // Checked in order: an existing table meld (lay off — the higher-stakes
  // action once you're down) before the new-meld staging area (just
  // selecting a card, same effect as tapping it).
  function tryDropOnMeld(translationX: number, absoluteX: number, absoluteY: number) {
    if (!measureMeldZones) {
      tryDropOnStaging(translationX, absoluteX, absoluteY);
      return;
    }
    measureMeldZones((zones) => {
      const hit = zones.find(({ zone }) => pointInZone(absoluteX, absoluteY, zone));
      if (hit && onDropOnMeld) {
        onDropOnMeld(index, hit.meldId);
      } else {
        tryDropOnStaging(translationX, absoluteX, absoluteY);
      }
    });
  }

  // A fast, short flick upward reads as "discard" even if the card never
  // reaches the discard pile's on-screen rect — asking players to drag all
  // the way there felt heavier than it needed to be.
  const QUICK_SWIPE_UP_DISTANCE = 40;
  const QUICK_SWIPE_UP_VELOCITY = -600;

  // Zones are measured live (not read from a value cached on scroll/layout
  // events) so a drop always sees the current on-screen position, even if
  // the page scrolled or the layout shifted since the last such event.
  function handleDragEnd(
    translationX: number,
    translationY: number,
    velocityY: number,
    absoluteX: number,
    absoluteY: number,
  ) {
    onDragCardChange?.(null);
    if (onDropOnZone && translationY < -QUICK_SWIPE_UP_DISTANCE && velocityY < QUICK_SWIPE_UP_VELOCITY) {
      onDropOnZone(index);
      return;
    }
    if (!measureDropZone) {
      tryDropOnMeld(translationX, absoluteX, absoluteY);
      return;
    }
    measureDropZone((zone) => {
      if (zone && onDropOnZone && pointInZone(absoluteX, absoluteY, zone)) {
        onDropOnZone(index);
      } else {
        tryDropOnMeld(translationX, absoluteX, absoluteY);
      }
    });
  }

  const pan = Gesture.Pan()
    .minDistance(10)
    .onStart((e) => {
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
    })
    .onEnd((e) => {
      runOnJS(handleDragEnd)(e.translationX, e.translationY, e.velocityY, e.absoluteX, e.absoluteY);
    })
    .onFinalize(() => {
      dragPreview.active.value = false;
      dragPreview.draggingIndex.value = -1;
    });

  const gesture = Gesture.Race(pan, longPress, tap);

  const animatedStyle = useAnimatedStyle(() => ({
    opacity: dragPreview.draggingIndex.value === index ? 0.3 : 1,
    cursor: (dragPreview.draggingIndex.value === index ? 'grabbing' : 'grab') as string,
  }));

  return (
    <GestureDetector gesture={gesture}>
      {/* userSelect: none stops a rapid double-tap from being swallowed by
          the browser's native "select this text" double-click behavior. */}
      <Animated.View style={[animatedStyle, { userSelect: 'none' } as object]}>
        <CardView card={card} selected={selected} justDrawn={justDrawn} compact={compact} />
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
