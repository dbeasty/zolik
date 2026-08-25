import { Fragment, useCallback, useRef, useState, type ReactNode } from 'react';
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
 *
 * ## The row never changes shape while a card is being dragged
 *
 * This is the invariant everything else here depends on, and getting it wrong
 * is what made an earlier version drift away from the pointer.
 *
 * A picked-up card leaves the layout entirely — it is drawn floating, at an
 * offset read once when it was picked up — and the gap showing where it will
 * land takes its place. So the row always holds exactly as many boxes as there
 * are cards, all the same size, in the same positions as before the drag
 * started. Moving the gap changes which box holds which card and nothing else.
 *
 * That is what makes it safe to hit-test the pointer against positions
 * measured once, at pick-up, and never re-read: the positions genuinely have
 * not changed. The version before this one left the dragged card in the layout
 * *and* inserted a gap, so the row grew by a card's width and everything to the
 * right of the gap shifted — while the hit test went on using the old
 * positions. The gap ended up a card away from the pointer, and the further
 * you dragged the worse it read.
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
  const rowRef = useRef<Measurable | null>(null);
  const cardRefs = useRef<(Measurable | null)[]>([]);

  // Where the row and each card are *on screen*, which is the same space a
  // gesture reports its pointer in — and the reason these are measured rather
  // than taken from `onLayout`. The two platforms disagree about what
  // `onLayout` means: on iOS and Android it is relative to the parent, and on
  // react-native-web it comes back in page coordinates. Comparing either one
  // against a pointer position happens to work on exactly one of them, which
  // is a bug that only shows up on the platform you did not test.
  //
  // Refs, not state: these are read during a gesture and must never render.
  const rects = useRef<Rect[]>([]);
  const rowRect = useRef<Rect | null>(null);
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
   * Reads where the row and its cards are, at touch-down.
   *
   * Touch-down rather than when the drag activates, because `measureInWindow`
   * answers on a later tick and by the time a pointer has moved far enough to
   * start a drag the answers are in. Until they are, `measured` is false and
   * the fan refuses to guess.
   *
   * Never *during* a drag, which is what the guard is for — see the note on
   * the component above. The row keeps its shape for the whole gesture, so
   * there is nothing to re-read; taking a second look could only introduce
   * error.
   */
  const measure = useCallback(() => {
    if (heldRef.current !== null) return;
    rowRef.current?.measureInWindow((x, y, width, height) => {
      rowRect.current = { x, y, width, height };
    });
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
      // the fan, so the gap goes back to where the card came from — two
      // answers to "where does this land?" on screen at once is one too many.
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
   * Whether a point counts as "in the hand".
   *
   * Measured against the row rather than against the cards in it, which is a
   * correction rather than a detail: the row runs the whole width of the
   * screen while the cards may fill a third of it, and the empty space to the
   * right of the last card looks exactly like part of your hand. Judging by
   * the cards meant that dropping a card into that space — the obvious way to
   * say "put this at the end" — fell outside the hand and was silently
   * refused, so the card sprang back and the end of the fan was unreachable in
   * practice however well the gaps worked.
   */
  const withinFan = useCallback(
    (x: number, y: number) => {
      const slop = 32;
      const row = rowRect.current;
      if (row) {
        return (
          x >= row.x - slop &&
          x <= row.x + row.width + slop &&
          y >= row.y - slop &&
          y <= row.y + row.height + slop
        );
      }
      const live = rects.current.slice(0, slots.length).filter(Boolean);
      if (!live.length) return false;
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

  // A card can only be lifted clear of the layout if we know where it was, and
  // until then it stays in the flow and no gap is drawn — a drag that started
  // before the measurements landed still works, it just has no preview.
  const lifted = held !== null && !!rects.current[held] && !!rowRect.current;
  // Which gap is open, counted the way `insertionAtPoint` counts: gap i sits
  // before card i, and gap n is past the last card. While the pointer is out
  // over the board it rests at the card's own position, so the row keeps its
  // shape there too.
  const gapIndex = lifted && held !== null ? insertion ?? held : null;

  const renderCard = (slot: Slot, index: number, floating: boolean): ReactNode => (
    <DraggableCard
      key={slot.id}
      index={index}
      slot={slot}
      count={slots.length}
      selected={selected.has(slot.id)}
      held={held === index}
      floatingAt={
        floating && rowRect.current && rects.current[index]
          ? {
              left: rects.current[index]!.x - rowRect.current.x,
              top: rects.current[index]!.y - rowRect.current.y,
            }
          : null
      }
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
  );

  return (
    <View style={styles.zone} testID={`zone-${zone.id}`}>
      <View style={styles.headerRow}>
        <Text style={styles.title}>{title}</Text>
        <Text style={styles.count} testID={`zone-count-${zone.id}`}>
          {zone.count}
        </Text>
      </View>

      <View
        ref={(n) => {
          rowRef.current = n as unknown as Measurable | null;
        }}
        style={styles.cards}
        testID={`hand-${zone.id}`}
        onLayout={measure}
      >
        {/* Every element here keeps its place in the tree for the whole life
            of the hand: a gap before each card and one after the last, all but
            one of them taking up no room. Nothing is ever inserted, removed or
            reordered while a card is being dragged.

            That is not tidiness, it is the difference between working and not.
            An earlier version moved the dragged card to the end of the list so
            it would draw on top — which moves its node — and gesture-handler
            lost the pointer it had captured, so drags silently did nothing at
            all. Turning boxes on and off changes only their style. */}
        {slots.map((slot, index) => (
          <Fragment key={slot.id}>
            <DropGap active={gapIndex === index} />
            {renderCard(slot, index, lifted && index === held)}
          </Fragment>
        ))}
        <DropGap active={gapIndex === slots.length} />
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
 * cards it stands among sit where they always sat.
 *
 * There is one of these before every card and one after the last, and all but
 * at most one are `display: 'none'` — which takes them out of the layout
 * entirely, gaps between flex children included, so an inactive one costs
 * nothing. They exist all the time so that opening a gap never adds or moves
 * an element.
 */
function DropGap({ active }: { active: boolean }) {
  return (
    <View style={[styles.slot, !active && styles.gapHidden]}>
      {/* The test id goes on the ring rather than the outer box, because that
          is where CardView puts a card's — so a gap and a card can be measured
          against each other and come out the same size. */}
      <View style={styles.gapRing} testID={active ? 'hand-drop-gap' : undefined}>
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
  /** Where to pin it once it has left the layout, relative to the row. */
  floatingAt: { left: number; top: number } | null;
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
  floatingAt,
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
    // what gets the row and its cards measured in time to be useful.
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
        style={[
          style,
          styles.slot,
          // Pinned where it was picked up, then carried by the transform. The
          // offset is the one read at pick-up, so the card does not jump at
          // the moment it leaves the layout.
          floatingAt ? { position: 'absolute', left: floatingAt.left, top: floatingAt.top } : null,
          held && styles.lifted,
        ]}
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
  gapHidden: { display: 'none' },
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
