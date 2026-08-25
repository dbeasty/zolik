import { Fragment, useCallback, useRef, useState } from 'react';
import { StyleSheet, Text, View } from 'react-native';
import { Gesture, GestureDetector } from 'react-native-gesture-handler';
import Animated, { runOnJS, useAnimatedStyle, useSharedValue } from 'react-native-reanimated';

import type { Zone } from '@/src/api/matchTypes';
import { CARD_METRICS, CardView } from '@/src/components/CardView';
import { insertionAtPoint, moveTargetFor, type Rect, type Slot } from '@/src/lib/hand';
import { label } from '@/src/lib/labels';
import { colors } from '@/src/theme';

/**
 * The viewer's own hand: the one zone on the board they may rearrange, and the
 * one a drag starts from.
 *
 * The order of the cards *in your hand* is nobody's business but yours, which
 * is why this is the only zone that can be rearranged, and why no module was
 * changed to add it. Every game gets it because no game owns it.
 *
 * A drag out of the hand is a different thing from a drag within it, and this
 * component deliberately does not know which one is happening: it reports
 * where the pointer is, and the screen — which can see the whole board and the
 * offers — says whether the card was let go of somewhere that wanted it. Only
 * if nothing did does the drag mean "move this card along the fan".
 *
 * It knows no more about the cards than `ZoneView` does. It cannot sort by
 * rank or group by suit, because doing either would mean deciding what a rank
 * or a suit is, and that is the line this client does not cross.
 */

type Props = {
  zone: Zone;
  slots: Slot[];
  selected: ReadonlySet<string>;
  onToggle: (slotId: string) => void;
  onMove: (from: number, to: number) => void;
  /** A drag began on the card at this index. */
  onDragStart?: (index: number) => void;
  /** The pointer moved, in window coordinates. */
  onDragMove?: (x: number, y: number) => void;
  /** Let go of. Returns true if something on the board took the card. */
  onDragEnd?: (x: number, y: number) => boolean;
  /** Set while the pointer is over a drop target outside the hand. */
  externalTarget?: string | null;
};

type Measurable = {
  measureInWindow: (cb: (x: number, y: number, w: number, h: number) => void) => void;
};

export function HandZone({
  zone,
  slots,
  selected,
  onToggle,
  onMove,
  onDragStart,
  onDragMove,
  onDragEnd,
  externalTarget,
}: Props) {
  const cardRefs = useRef<(Measurable | null)[]>([]);

  // Where each card is *on screen*, which is the same space a gesture reports
  // its pointer in — and the reason these are measured rather than taken from
  // `onLayout`. The two platforms disagree about what `onLayout` means: on
  // iOS and Android it is relative to the parent, and on react-native-web it
  // comes back in page coordinates. Comparing either one against a pointer
  // position happens to work on exactly one of them, which is a bug that only
  // shows up on the platform you did not test.
  //
  // A ref, not state: this is read during a gesture and must never render.
  const rects = useRef<Rect[]>([]);
  const measured = useRef(false);

  const heldRef = useRef<number | null>(null);
  const insertRef = useRef<number | null>(null);
  const lastPoint = useRef({ x: 0, y: 0 });
  const [held, setHeld] = useState<number | null>(null);
  const [insertion, setInsertion] = useState<number | null>(null);

  // Mirrors of things that change while a drag is in flight. The gesture's
  // callbacks are handed to the UI thread once; reading these through a ref
  // rather than closing over the props keeps a mid-drag re-render — and the
  // parent re-renders on every pointer move, to light up targets — from
  // leaving the gesture holding a stale copy.
  const outsideRef = useRef<string | null>(null);
  outsideRef.current = externalTarget ?? null;
  const moveRef = useRef(onDragMove);
  moveRef.current = onDragMove;
  const endRef = useRef(onDragEnd);
  endRef.current = onDragEnd;
  const startRef = useRef(onDragStart);
  startRef.current = onDragStart;

  /**
   * Reads where the cards are, at touch-down.
   *
   * Touch-down rather than when the drag activates, because
   * `measureInWindow` answers on a later tick and by the time a pointer has
   * moved far enough to start a drag the answers are in. Until they are,
   * `measured` is false and the fan refuses to guess.
   *
   * Never *during* a drag, which is what the guard is for. The gap that opens
   * up to show where the card will land is itself a layout change, so it
   * retriggers this — and re-measuring mid-drag would mean testing the pointer
   * against positions the gap had just moved, so opening a gap would change
   * where the gap belongs. The fan would squirm away from the pointer. Holding
   * the pre-drag positions for the whole gesture is what makes it steady: the
   * card lands where it looked like it would when you picked it up.
   */
  const measure = useCallback(() => {
    if (heldRef.current !== null) return;
    cardRefs.current.slice(0, slots.length).forEach((ref, i) => {
      ref?.measureInWindow((x, y, width, height) => {
        rects.current[i] = { x, y, width, height };
        measured.current = true;
      });
    });
  }, [slots.length]);

  const pickUp = useCallback((index: number) => {
    heldRef.current = index;
    setHeld(index);
    startRef.current?.(index);
  }, []);

  const hover = useCallback(
    (absoluteX: number, absoluteY: number) => {
      lastPoint.current = { x: absoluteX, y: absoluteY };
      moveRef.current?.(absoluteX, absoluteY);
      if (heldRef.current === null || !measured.current) return;

      // Over the board, the card is going somewhere rather than moving along
      // the fan, so the gap closes — two answers to "where does this land?" on
      // screen at once is one too many.
      const at = outsideRef.current
        ? null
        : insertionAtPoint(rects.current.slice(0, slots.length), { x: absoluteX, y: absoluteY });
      // Only a change is worth a render; a pan reports every pointer move.
      if (at !== insertRef.current) {
        insertRef.current = at;
        setInsertion(at);
      }
    },
    [slots.length],
  );

  /**
   * Whether a point is over the fan at all.
   *
   * What this prevents: a card dragged onto the table and refused — because
   * the engine will not take it yet — falling through to "move it along the
   * fan" and rearranging the hand as a consolation prize. A card let go of
   * over nothing belongs back where it came from, which is what a real table
   * does too.
   */
  const withinFan = useCallback(
    (x: number, y: number) => {
      const live = rects.current.slice(0, slots.length).filter(Boolean);
      if (!live.length) return false;
      const slop = 40;
      const left = Math.min(...live.map((r) => r.x)) - slop;
      const right = Math.max(...live.map((r) => r.x + r.width)) + slop;
      const top = Math.min(...live.map((r) => r.y)) - slop;
      const bottom = Math.max(...live.map((r) => r.y + r.height)) + slop;
      return x >= left && x <= right && y >= top && y <= bottom;
    },
    [slots.length],
  );

  const release = useCallback(() => {
    const from = heldRef.current;
    const gap = insertRef.current;
    const { x, y } = lastPoint.current;
    heldRef.current = null;
    insertRef.current = null;
    setHeld(null);
    setInsertion(null);

    // The board gets first refusal. Only a card nothing wanted is a card being
    // moved along the fan — and only if it was let go of over the fan.
    const taken = endRef.current?.(x, y) ?? false;
    if (taken) return;
    if (!withinFan(x, y)) return;
    if (from === null || gap === null) return;

    const to = moveTargetFor(from, gap);
    if (to !== from) onMove(from, to);
  }, [onMove, withinFan]);

  const title = label(zone.labelKey) || zone.id;

  return (
    <View style={styles.zone} testID={`zone-${zone.id}`}>
      <View style={styles.headerRow}>
        <Text style={styles.title}>{title}</Text>
        <Text style={styles.count} testID={`zone-count-${zone.id}`}>
          {zone.count}
        </Text>
      </View>

      <View style={styles.cards} testID={`hand-${zone.id}`} onLayout={measure}>
        {slots.map((slot, index) => (
          <Fragment key={slot.id}>
            {insertion === index ? <DropGap /> : null}
            <DraggableCard
              index={index}
              slot={slot}
              count={slots.length}
              selected={selected.has(slot.id)}
              held={held === index}
              testID={`card-${zone.id}-${index}`}
              bindRef={(node) => {
                cardRefs.current[index] = node;
              }}
              onToggle={onToggle}
              onTouch={measure}
              onPickUp={pickUp}
              onHover={hover}
              onRelease={release}
              onMove={onMove}
            />
          </Fragment>
        ))}
        {/* The gap past the last card. It is a separate line rather than part
            of the loop because there is one more gap than there are cards, and
            that last one is the only way to say "put it at the end". */}
        {insertion === slots.length ? <DropGap /> : null}
      </View>

      {slots.length > 1 ? (
        <Text style={styles.hint} testID={`hand-hint-${zone.id}`}>
          Drag a card along the fan to rearrange it, or onto the board to play it
        </Text>
      ) : null}
    </View>
  );
}

/**
 * The space a dragged card would drop into.
 *
 * Exactly a card wide, and built from the same metrics a card is, so that the
 * cards it pushes apart move by precisely one card and not by nearly one.
 */
function DropGap() {
  return (
    <View style={styles.slot} testID="hand-drop-gap">
      <View style={styles.gapRing}>
        <View style={styles.gapCard} />
      </View>
    </View>
  );
}

type CardProps = {
  index: number;
  slot: Slot;
  count: number;
  selected: boolean;
  held: boolean;
  testID: string;
  bindRef: (node: Measurable | null) => void;
  onToggle: (slotId: string) => void;
  /** Touch-down, before the drag has activated — early enough to measure. */
  onTouch: () => void;
  onPickUp: (index: number) => void;
  onHover: (absoluteX: number, absoluteY: number) => void;
  onRelease: () => void;
  onMove: (from: number, to: number) => void;
};

function DraggableCard({
  index,
  slot,
  count,
  selected,
  held,
  testID,
  bindRef,
  onToggle,
  onTouch,
  onPickUp,
  onHover,
  onRelease,
  onMove,
}: CardProps) {
  const tx = useSharedValue(0);
  const ty = useSharedValue(0);

  // Set the moment the pan activates, so the press that ends a drag cannot
  // also be read as a tap that selects the card. Gesture-handler normally
  // cancels the child responder for us; this is the belt to those braces,
  // because a drag that silently toggled selection would be maddening.
  //
  // A shared value rather than a ref because it is written from the gesture's
  // worklet, which runs on the UI thread and cannot touch React state; the
  // press handler reads it back on the JS thread, which is allowed.
  const dragged = useSharedValue(false);

  const pan = Gesture.Pan()
    // Any direction, once the pointer has genuinely moved. The threshold is
    // only there so that a tap with a shaky finger stays a tap.
    //
    // An earlier version activated on sideways movement and handed vertical
    // movement to the scroll view this sits inside, on the reasoning that the
    // fan is horizontal so a sideways drag must have meant the fan. That was
    // true while the only thing a drag could do was rearrange the hand. It
    // stopped being true the moment a card could be dropped on the board,
    // because the board is *above* the hand: reserving vertical movement for
    // scrolling silently cancelled every drag aimed at the table, which is the
    // main thing a person drags a card for.
    //
    // The cost is that a touch starting on a card no longer scrolls the page.
    // It is the right way round — a card let go of over nothing goes back
    // where it came from, so a scroll misread as a drag costs nothing, while a
    // drag misread as a scroll costs the whole feature.
    //
    // Deliberately *not* `activateAfterLongPress`. It reads like it would add
    // a press-and-hold way in alongside a movement threshold, and it does the
    // opposite — it gates activation on the hold, so the movement threshold
    // stops working and a normal quick drag does nothing at all, silently.
    .minDistance(10)
    // Fires on touch-down whether or not this ever becomes a drag, which is
    // what gets the cards measured in time to be useful.
    .onBegin(() => {
      runOnJS(onTouch)();
    })
    .onStart(() => {
      dragged.value = true;
      runOnJS(onPickUp)(index);
    })
    .onUpdate((e) => {
      tx.value = e.translationX;
      ty.value = e.translationY;
      runOnJS(onHover)(e.absoluteX, e.absoluteY);
    })
    // onFinalize rather than onEnd: onEnd does not fire when a gesture is
    // cancelled, and a drag abandoned mid-flight must still put the card down
    // rather than leave it stuck to the pointer.
    .onFinalize(() => {
      tx.value = 0;
      ty.value = 0;
      runOnJS(onRelease)();
    });

  const style = useAnimatedStyle(() => ({
    transform: [{ translateX: tx.value }, { translateY: ty.value }],
  }));

  return (
    <GestureDetector gesture={pan}>
      <Animated.View
        ref={bindRef as never}
        style={[style, styles.slot, held && styles.lifted]}
        // The same move, for anyone not using a pointer. A drag is not an
        // affordance a screen reader can offer, so the two directions are
        // published as actions instead.
        accessible
        accessibilityLabel={slot.card}
        accessibilityActions={[
          { name: 'moveLeft', label: 'Move left' },
          { name: 'moveRight', label: 'Move right' },
        ]}
        onAccessibilityAction={(e) => {
          if (e.nativeEvent.actionName === 'moveLeft') onMove(index, Math.max(0, index - 1));
          if (e.nativeEvent.actionName === 'moveRight') onMove(index, Math.min(count - 1, index + 1));
        }}
      >
        <CardView
          card={slot.card}
          selected={selected}
          dragging={held}
          testID={testID}
          onPress={() => {
            if (dragged.value) {
              dragged.value = false;
              return;
            }
            onToggle(slot.id);
          }}
        />
      </Animated.View>
    </GestureDetector>
  );
}

const styles = StyleSheet.create({
  zone: {
    backgroundColor: colors.surface,
    borderRadius: 10,
    borderWidth: 1,
    borderColor: colors.border,
    padding: 8,
    marginBottom: 8,
  },
  headerRow: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center' },
  title: { color: colors.muted, fontSize: 12, fontWeight: '700' },
  count: { color: colors.muted, fontSize: 12 },
  cards: { flexDirection: 'row', flexWrap: 'wrap', gap: 4, marginTop: 6 },
  hint: { color: colors.muted, fontSize: 10, marginTop: 6, fontStyle: 'italic' },
  lifted: { zIndex: 20, opacity: 0.92 },
  // The wrapper every card and every gap sits in, so the two are the same size
  // to the pixel. Its border is always here and always this wide — only its
  // colour ever changes, the same discipline CardView's own ring keeps.
  slot: {
    borderRadius: 8,
    borderWidth: CARD_METRICS.ringBorder,
    borderColor: 'transparent',
  },
  gapRing: {
    borderRadius: 8,
    borderWidth: CARD_METRICS.ringBorder,
    borderColor: 'transparent',
    padding: CARD_METRICS.ringPadding,
  },
  gapCard: {
    width: CARD_METRICS.width,
    height: CARD_METRICS.height,
    marginRight: CARD_METRICS.gap,
    borderRadius: 6,
    borderWidth: 2,
    borderStyle: 'dashed',
    borderColor: colors.gold,
    // A wash rather than a fill. Filled in solid it read as a *card* sitting
    // in the fan, which is the one thing it is not — it is the space the card
    // in your hand is about to occupy, and it should look like space.
    backgroundColor: 'rgba(251, 191, 36, 0.14)',
  },
});
