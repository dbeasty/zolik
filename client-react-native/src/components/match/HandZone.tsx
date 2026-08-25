import { useCallback, useRef, useState } from 'react';
import { StyleSheet, Text, View } from 'react-native';
import { Gesture, GestureDetector } from 'react-native-gesture-handler';
import Animated, { runOnJS, useAnimatedStyle, useSharedValue } from 'react-native-reanimated';

import type { Zone } from '@/src/api/matchTypes';
import { CardView } from '@/src/components/CardView';
import { slotAtPoint, type Rect, type Slot } from '@/src/lib/hand';
import { label } from '@/src/lib/labels';
import { colors } from '@/src/theme';

/**
 * The viewer's own hand: the one zone on the board they may rearrange.
 *
 * Everything else the shell draws is the server's business — a discard pile is
 * in the order it was discarded and a meld is in the order it was laid. The
 * order of the cards *in your hand*, though, is nobody's business but yours,
 * which is why this is the only zone that is interactive in this way, and why
 * no module was changed to add it. Every game gets it because no game owns it.
 *
 * It knows no more about the cards than `ZoneView` does. It cannot sort by
 * rank or group by suit, because doing either would mean deciding what a rank
 * or a suit is, and that is the line this client does not cross. It moves the
 * card a person picked up to the place they dropped it.
 */

type Props = {
  zone: Zone;
  slots: Slot[];
  selected: ReadonlySet<string>;
  onToggle: (slotId: string) => void;
  onMove: (from: number, to: number) => void;
};

type Measurable = { measureInWindow: (cb: (x: number, y: number, w: number, h: number) => void) => void };

export function HandZone({ zone, slots, selected, onToggle, onMove }: Props) {
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
  const targetRef = useRef<number | null>(null);
  const [held, setHeld] = useState<number | null>(null);
  const [target, setTarget] = useState<number | null>(null);

  // Called at touch-down rather than when a drag activates: `measureInWindow`
  // answers on a later tick, and by the time a pointer has moved far enough to
  // start a drag the answers are in. Until they are, `measured` is false and
  // the fan refuses to guess — a drag too fast to measure leaves the card
  // where it was instead of flinging it somewhere arbitrary.
  const measure = useCallback(() => {
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
  }, []);

  const hover = useCallback(
    (absoluteX: number, absoluteY: number) => {
      if (heldRef.current === null || !measured.current) return;
      const at = slotAtPoint(rects.current.slice(0, slots.length), { x: absoluteX, y: absoluteY });
      // Only a change is worth a render; a pan reports every pointer move.
      if (at !== targetRef.current) {
        targetRef.current = at;
        setTarget(at);
      }
    },
    [slots.length],
  );

  const release = useCallback(() => {
    const from = heldRef.current;
    const to = targetRef.current;
    heldRef.current = null;
    targetRef.current = null;
    setHeld(null);
    setTarget(null);
    if (from !== null && to !== null && from !== to) onMove(from, to);
  }, [onMove]);

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
          <DraggableCard
            key={slot.id}
            index={index}
            slot={slot}
            count={slots.length}
            selected={selected.has(slot.id)}
            held={held === index}
            isTarget={held !== null && held !== index && target === index}
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
        ))}
      </View>

      {slots.length > 1 ? (
        <Text style={styles.hint} testID={`hand-hint-${zone.id}`}>
          Drag a card sideways to rearrange your hand
        </Text>
      ) : null}
    </View>
  );
}

type CardProps = {
  index: number;
  slot: Slot;
  count: number;
  selected: boolean;
  held: boolean;
  isTarget: boolean;
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
  isTarget,
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
    // Sideways starts a drag, downwards scrolls the page. The hand fans
    // horizontally, so a sideways drag cannot have been meant for the
    // vertical scroll view this sits inside, and a vertical one almost
    // certainly was — handing that case straight to the scroll view is what
    // keeps the board reachable from a hand that fills the bottom of a phone.
    //
    // Once a drag has started neither threshold applies any more, so moving a
    // card down onto the second row of a wrapped fan works: go sideways
    // first, then wherever you like.
    //
    // Deliberately *not* `activateAfterLongPress`. It reads like it would add
    // a press-and-hold way in alongside these, and it does the opposite — it
    // gates activation on the hold, so the sideways threshold stops working
    // and a normal quick drag does nothing at all, silently.
    .activeOffsetX([-12, 12])
    .failOffsetY([-24, 24])
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
        style={[style, styles.slot, held && styles.lifted, isTarget && styles.target]}
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
  // The border is always here and always this wide, and only its colour
  // changes — the same discipline CardView's own ring keeps, for the same
  // reason. Adding a border on hover instead would resize that card and shift
  // every card after it *during* the drag, moving the very measurements the
  // drop is tested against: the fan squirms away from the pointer and the
  // card lands somewhere nobody aimed at.
  slot: {
    borderRadius: 8,
    borderWidth: 2,
    borderColor: 'transparent',
  },
  // Where the held card will land. Drawn on the card being displaced rather
  // than as a bar between cards, because the fan wraps and a gap between two
  // cards on different rows has no obvious place to put a bar.
  target: {
    borderColor: colors.gold,
    borderStyle: 'dashed',
  },
});
