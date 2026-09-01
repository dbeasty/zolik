import { Fragment, memo, useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Pressable, StyleSheet, Text, View } from 'react-native';
import { Gesture, GestureDetector } from 'react-native-gesture-handler';
import { runOnJS } from 'react-native-reanimated';

import type { Zone } from '@/src/api/matchTypes';
import { CardGlance } from '@/src/components/match/CardGlance';
import { CardView } from '@/src/components/CardView';
import { Panel } from '@/src/components/match/Panel';
import { SettleIn } from '@/src/components/match/SettleIn';
import { useMetrics } from '@/src/hooks/useMetrics';
import { useSkin } from '@/src/hooks/useSkin';
import { zoneElementId } from '@/src/lib/drops';
import { insertionAtPoint, moveTargetFor, type Rect, type Slot } from '@/src/lib/hand';
import type { Metrics } from '@/src/lib/layout';
import { label } from '@/src/lib/labels';
import { ms } from '@/src/lib/motion';
import type { Skin } from '@/src/skins/types';
import { dragLayer } from '@/src/theme';

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
 * where the card is, and the screen — which can see the whole board and the
 * offers — says whether it was let go of somewhere that wanted it. Only if
 * nothing did does the drag mean "move this card along the fan".
 *
 * ## The row never changes shape while a card is being dragged
 *
 * A picked-up card leaves the layout entirely — it is drawn floating, at an
 * offset read once when it was picked up — and the gap showing where it will
 * land takes its place. So the row always holds exactly as many boxes as there
 * are cards, all the same size, in the same positions as before the drag
 * started. Moving the gap changes which box holds which card and nothing else.
 *
 * That is what makes it safe to hit-test against positions measured once, at
 * pick-up, and never re-read: the positions genuinely have not changed.
 *
 * ## The card is moved by React state, not by an animated style
 *
 * The obvious way to carry a dragged card is a Reanimated shared value driving
 * a transform, which keeps it off the render path entirely. It does not work
 * here: on this web build the animated style is applied once, with its initial
 * value, and never updated again. Measured directly in a browser — the shared
 * value changed, and the element's transform stayed `translateX(0px)` for the
 * whole drag, even when set from outside the gesture.
 *
 * The symptom was a card that never moved while the gap went on tracking the
 * pointer, so the two grew further apart the further the drag went. It reads
 * as the gap drifting; it was the card standing still.
 *
 * So the offset is ordinary state. The hand re-renders as it moves, which for
 * a dozen cards is nothing, and it demonstrably works.
 */

type Props = {
  zone: Zone;
  slots: Slot[];
  selected: ReadonlySet<string>;
  onToggle: (slotId: string) => void;
  onMove: (from: number, to: number) => void;
  /** Tidy the hand into rank order, likely melds held together. */
  onAutoArrange?: () => void;
  /** A drag began on the card at this index. */
  onDragStart?: (index: number) => void;
  /** The card moved, in window coordinates. */
  onDragMove?: (x: number, y: number) => void;
  /** Let go of. Returns true if something on the board took the card. */
  onDragEnd?: (x: number, y: number) => boolean;
  /** Set while the card is over a drop target outside the hand. */
  externalTarget?: string | null;
  /**
   * Marks the module put on particular cards, by card value — see
   * `CardView.badgeKeys`. Long-pressing a marked card asks what the mark
   * means; a tap still selects it for play.
   */
  badges?: ReadonlyMap<string, string[]>;
  onPressBadge?: (card: string, badgeKeys: string[]) => void;
  /** Stable id for remembering whether this panel is put away. Omit for a panel that may not be minimized. */
  panelId?: string;
  minimized?: boolean;
  onToggleMinimized?: () => void;
  /**
   * Publishes where the fan is, under the zone's element id — so a card in
   * flight (see `FlightLayer`) can aim at the hand the way it aims at any
   * zone. The registry's, not this component's: same contract as
   * `ZoneView`'s `registerDrop`.
   */
  registerSpot?: (elementId: string, node: Measurable | null) => void;
  /**
   * How long a card mounting now should hold its entrance — set while a
   * flight is landing here, so the card doesn't greet its own arrival.
   * Never touches the deal's own stagger.
   */
  entranceDelay?: number;
};

type Measurable = {
  measureInWindow: (cb: (x: number, y: number, w: number, h: number) => void) => void;
};

type Offset = { dx: number; dy: number };

export function HandZone({
  zone,
  slots,
  selected,
  onToggle,
  onMove,
  onAutoArrange,
  onDragStart,
  onDragMove,
  onDragEnd,
  externalTarget,
  badges,
  onPressBadge,
  panelId,
  minimized,
  onToggleMinimized,
  registerSpot,
  entranceDelay,
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
  // Where the finger went down. Not the gesture's own `translation`, which is
  // measured from where it *activated* — and activation is held back until the
  // pointer has moved far enough to prove it is a drag and not a tap, so every
  // one of those threshold pixels is missing from it.
  const startPoint = useRef({ x: 0, y: 0 });
  const draggedRef = useRef(false);

  const [held, setHeld] = useState<number | null>(null);
  const [insertion, setInsertion] = useState<number | null>(null);
  const [offset, setOffset] = useState<Offset | null>(null);

  // Minimizing unmounts the fan (see the `Panel` this returns), which takes
  // every `DraggableCard` and its gesture with it — the minimize control
  // lives in the header, not the fan, so there is no real way to reach it
  // mid-gesture, but a held card left in this state would come back stuck to
  // the pointer if that ever changed. Belt and braces, cheaply.
  useEffect(() => {
    if (!minimized) return;
    heldRef.current = null;
    insertRef.current = null;
    setHeld(null);
    setInsertion(null);
    setOffset(null);
  }, [minimized]);

  // Mirrors of things that change while a drag is in flight. The gesture's
  // callbacks are handed over once; reading these through a ref rather than
  // closing over the props keeps a mid-drag re-render — and the parent
  // re-renders on every move, to light up targets — from leaving the gesture
  // holding a stale copy.
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
   * Never *during* a drag, which is what the guard is for. The row keeps its
   * shape for the whole gesture, so there is nothing to re-read; taking a
   * second look could only introduce error.
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

  const touchDown = useCallback(
    (x: number, y: number) => {
      startPoint.current = { x, y };
      measure();
    },
    [measure],
  );

  const pickUp = useCallback((index: number) => {
    heldRef.current = index;
    draggedRef.current = true;
    setHeld(index);
    setOffset({ dx: 0, dy: 0 });
    startRef.current?.(index);
  }, []);

  /**
   * The pointer moved. Everything downstream is told where the *card* is, not
   * where the pointer is.
   *
   * Those are not the same place. A card is picked up wherever it happened to
   * be touched and keeps that grip, so one taken near its left edge is drawn
   * most of a card-width to the right of the pointer. Opening the gap under
   * the pointer put the card and the space it was about to drop into visibly
   * apart. Since the thing being moved is the card, the card decides where it
   * lands.
   */
  const hover = useCallback(
    (absoluteX: number, absoluteY: number) => {
      const index = heldRef.current;
      const dx = absoluteX - startPoint.current.x;
      const dy = absoluteY - startPoint.current.y;
      if (index !== null) setOffset({ dx, dy });

      const base = index === null ? undefined : rects.current[index];
      const point = base
        ? { x: base.x + base.width / 2 + dx, y: base.y + base.height / 2 + dy }
        : { x: absoluteX, y: absoluteY };

      lastPoint.current = point;
      moveRef.current?.(point.x, point.y);
      if (index === null || !measured.current) return;

      // Over the board, the card is going somewhere rather than moving along
      // the fan, so the gap goes back to where the card came from — two
      // answers to "where does this land?" on screen at once is one too many.
      const at = outsideRef.current
        ? null
        : insertionAtPoint(rects.current.slice(0, slots.length), point);
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
   * Measured against the row rather than against the cards in it: the row runs
   * the whole width of the screen while the cards may fill a third of it, and
   * the empty space to the right of the last card looks exactly like part of
   * your hand. Judging by the cards meant that dropping a card there — the
   * obvious way to say "put this at the end" — fell outside and was silently
   * refused.
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
    setOffset(null);

    // The board gets first refusal. Only a card nothing wanted is a card being
    // moved along the fan — and only if it was let go of over the fan.
    const taken = endRef.current?.(x, y) ?? false;
    if (taken) return;
    if (!withinFan(x, y)) return;
    if (from === null || gap === null) return;

    const to = moveTargetFor(from, gap);
    if (to !== from) onMove(from, to);
  }, [onMove, withinFan]);

  /** Was that press the end of a drag? If so it is not also a tap. */
  const consumedByDrag = useCallback(() => {
    if (!draggedRef.current) return false;
    draggedRef.current = false;
    return true;
  }, []);

  // Stable handles for everything a card is handed, so that `DraggableCard`'s
  // memo can actually hold. The dragged card's offset changes on every pointer
  // move and the whole hand re-renders with it; without this, every other card
  // re-renders too, for a dozen or more cards, mid-gesture. A callback rebuilt
  // each render is enough on its own to defeat the memo, which is why these go
  // through refs rather than being passed straight down.
  const toggleRef = useRef(onToggle);
  toggleRef.current = onToggle;
  const moveHandlerRef = useRef(onMove);
  moveHandlerRef.current = onMove;
  const stableToggle = useCallback((slotId: string) => toggleRef.current(slotId), []);
  const stableMove = useCallback((from: number, to: number) => moveHandlerRef.current(from, to), []);

  const binders = useRef(new Map<number, (node: Measurable | null) => void>());
  const binderFor = useCallback((index: number) => {
    let fn = binders.current.get(index);
    if (!fn) {
      fn = (node: Measurable | null) => {
        cardRefs.current[index] = node;
      };
      binders.current.set(index, fn);
    }
    return fn;
  }, []);

  const title = label(zone.labelKey) || zone.id;
  const metrics = useMetrics();
  const skin = useSkin();
  // Recomputed only when the card's own size or the skin changes — a resize,
  // a rotation, a look switched — never on a pointer move, which is what
  // keeps `DraggableCard`'s memo intact through a drag (see the comment on
  // it below).
  const styles = useMemo(() => handStyles(metrics, skin), [metrics, skin]);

  // Cards mounting in the hand's opening moments are the deal, and enter as
  // one — staggered left to right. A card mounting later arrived alone (a
  // draw) and enters at once — unless a flight is carrying it here, in which
  // case it waits for the landing (`entranceDelay`). Read at render, used
  // only at each card's own mount, so the distinction costs nothing after
  // the deal.
  const openedAt = useRef(Date.now());
  const dealDelayFor = (index: number) =>
    Date.now() - openedAt.current < 700 ? Math.min(index, 12) * ms(40) : entranceDelay ?? 0;

  // A card can only be lifted clear of the layout if we know where it was, and
  // until then it stays in the flow and no gap is drawn — a drag that started
  // before the measurements landed still works, it just has no preview.
  const lifted = held !== null && !!rects.current[held] && !!rowRect.current;
  // Which gap is open, counted the way `insertionAtPoint` counts: gap i sits
  // before card i, and gap n is past the last card. While the card is out over
  // the board it rests at the card's own position, so the row keeps its shape
  // there too.
  const gapIndex = lifted && held !== null ? insertion ?? held : null;

  // The hand and the row inside it ride up onto the drag layer with the card
  // they are carrying (see `dragLayer`), so a card taken down onto a meld is
  // drawn over the meld rather than behind it. Only while a card is actually
  // held: a hand permanently above the board would put its own edge over
  // every panel below it for no reason.
  const carrying = held !== null;

  return (
    <Panel
      style={carrying && dragLayer}
      panelId={panelId}
      title={title}
      minimized={minimized}
      onToggleMinimized={onToggleMinimized}
      testID={`zone-${zone.id}`}
      count={zone.count}
      countTestID={`zone-count-${zone.id}`}
      // Capped only on a narrow screen, where the header has no room to
      // spare. On a wide one the collapsed hand sits in a full-width row with
      // space to say the whole thing, so there's no reason to fold most of a
      // 13-card hand behind a "+5" that a wider screen never needed.
      summary={
        <CardGlance
          cards={slots.map((s) => s.card)}
          max={metrics.narrow ? 8 : slots.length}
          testID={`zone-summary-${zone.id}`}
        />
      }
      accessory={
        onAutoArrange && slots.length > 1 ? (
          <Pressable onPress={onAutoArrange} hitSlop={8}>
            <Text style={styles.autoArrange} testID={`hand-auto-arrange-${zone.id}`}>
              Auto-arrange
            </Text>
          </Pressable>
        ) : undefined
      }
    >
      <View
        ref={(n) => {
          rowRef.current = n as unknown as Measurable | null;
          // The fan doubles as a flight destination — the row rather than
          // the whole panel, so a landing card aims at the cards, not the
          // header above them.
          registerSpot?.(zoneElementId(zone.id), n as unknown as Measurable | null);
        }}
        style={[styles.cards, carrying && dragLayer]}
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
            <DropGap active={gapIndex === index} styles={styles} />
            <DraggableCard
              index={index}
              slot={slot}
              count={slots.length}
              selected={selected.has(slot.id)}
              held={held === index}
              offset={held === index ? offset : null}
              floatingAt={
                lifted && index === held && rowRect.current && rects.current[index]
                  ? {
                      left: rects.current[index]!.x - rowRect.current.x,
                      top: rects.current[index]!.y - rowRect.current.y,
                    }
                  : null
              }
              testID={`card-${zone.id}-${index}`}
              dealDelay={dealDelayFor(index)}
              bindRef={binderFor(index)}
              onToggle={stableToggle}
              onTouch={touchDown}
              onPickUp={pickUp}
              onHover={hover}
              onRelease={release}
              onMove={stableMove}
              consumedByDrag={consumedByDrag}
              badgeKeys={badges?.get(slot.card)}
              onPressBadge={onPressBadge}
              styles={styles}
            />
          </Fragment>
        ))}
        <DropGap active={gapIndex === slots.length} styles={styles} />
      </View>

      {slots.length > 1 ? (
        <Text style={styles.hint} testID={`hand-hint-${zone.id}`}>
          Drag a card along the fan to rearrange it, or onto the board to play it
        </Text>
      ) : null}
    </Panel>
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
function DropGap({ active, styles }: { active: boolean; styles: HandStyles }) {
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
  /** How far this card has been carried, while it is being carried. */
  offset: Offset | null;
  /** Where to pin it once it has left the layout, relative to the row. */
  floatingAt: { left: number; top: number } | null;
  testID: string;
  /** Milliseconds to hold before this card's entrance — the deal's stagger. */
  dealDelay: number;
  bindRef: (node: Measurable | null) => void;
  onToggle: (slotId: string) => void;
  /** Touch-down, before the drag has activated — early enough to measure. */
  onTouch: (x: number, y: number) => void;
  onPickUp: (index: number) => void;
  onHover: (absoluteX: number, absoluteY: number) => void;
  onRelease: () => void;
  onMove: (from: number, to: number) => void;
  consumedByDrag: () => boolean;
  /** Marks the module put on this card, if any — see `HandZone.badges`. */
  badgeKeys?: string[];
  onPressBadge?: (card: string, badgeKeys: string[]) => void;
  styles: HandStyles;
};

/**
 * One card in the fan.
 *
 * Memoised, because the hand re-renders on every pointer move to carry the one
 * card being dragged, and the other twelve have not changed. Everything it is
 * given is either a primitive or held stable by the hand above.
 */
const DraggableCard = memo(function DraggableCard({
  index,
  slot,
  count,
  selected,
  held,
  offset,
  floatingAt,
  testID,
  dealDelay,
  bindRef,
  onToggle,
  onTouch,
  onPickUp,
  onHover,
  onRelease,
  onMove,
  consumedByDrag,
  badgeKeys,
  onPressBadge,
  styles,
}: CardProps) {
  // A long-press that just opened the badge explanation must not also toggle
  // selection when the finger comes up — Pressable has no long-press of its
  // own here, so this is the only guard.
  const suppressTapRef = useRef(false);

  const explainBadge = useCallback(() => {
    if (!badgeKeys?.length || !onPressBadge) return;
    suppressTapRef.current = true;
    onPressBadge(slot.card, badgeKeys);
  }, [badgeKeys, onPressBadge, slot.card]);

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
    // scrolling silently cancelled every drag aimed at the table.
    //
    // Deliberately *not* `activateAfterLongPress`. It reads like it would add
    // a press-and-hold way in alongside a movement threshold, and it does the
    // opposite — it gates activation on the hold, so the movement threshold
    // stops working and a normal quick drag does nothing at all, silently.
    .minDistance(10)
    // Fires on touch-down whether or not this ever becomes a drag, which is
    // what gets the row and its cards measured in time to be useful, and what
    // records where the finger actually went down.
    .onBegin((e) => {
      runOnJS(onTouch)(e.absoluteX, e.absoluteY);
    })
    .onStart(() => {
      runOnJS(onPickUp)(index);
    })
    .onUpdate((e) => {
      runOnJS(onHover)(e.absoluteX, e.absoluteY);
    })
    // onFinalize rather than onEnd: onEnd does not fire when a gesture is
    // cancelled, and a drag abandoned mid-flight must still put the card down
    // rather than leave it stuck to the pointer.
    .onFinalize(() => {
      runOnJS(onRelease)();
    });

  // Web only, and the reason a phone couldn't be scrolled past the hand.
  // Attaching *any* Pan gesture makes this library set the card's own
  // `touch-action` to `none` — every touch starting on a card is claimed for
  // the gesture before the browser gets a say, vertical swipe included, so
  // the scroll view underneath never saw one. `pan-y` hands vertical panning
  // back to the browser's own touch scrolling, which is safe now in a way it
  // was not when the comment above was written: reaching a meld above the
  // hand no longer depends on a drag at all, tap-to-play does it (see
  // `app/match/[matchId].tsx`), so a phone's vertical swipe is free to mean
  // "scroll" without taking anything away. A horizontal drag is untouched —
  // `touch-action` only ever hands off the axis the browser is told about,
  // and rearranging the fan stays a sideways gesture. There is no chained
  // setter for this on `Gesture.Pan()`; `config` is the same public,
  // typed field every chained setter writes into.
  pan.config.touchAction = 'pan-y';

  const longPress = Gesture.LongPress()
    .minDuration(400)
    .onStart(() => {
      runOnJS(explainBadge)();
    });
  const gesture =
    badgeKeys?.length && onPressBadge ? Gesture.Simultaneous(pan, longPress) : pan;

  const carried = useMemo(
    () => (offset ? { transform: [{ translateX: offset.dx }, { translateY: offset.dy }] } : null),
    [offset?.dx, offset?.dy],
  );
  const pinned = useMemo(
    () =>
      floatingAt
        ? { position: 'absolute' as const, left: floatingAt.left, top: floatingAt.top }
        : null,
    [floatingAt?.left, floatingAt?.top],
  );

  return (
    <GestureDetector gesture={gesture}>
      <View
        ref={(n) => bindRef(n as unknown as Measurable | null)}
        style={[styles.slot, pinned, held && styles.lifted, selected && !held && styles.raised, carried]}
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
        <SettleIn kind="deal" delay={dealDelay}>
          <CardView
            card={slot.card}
            selected={selected}
            dragging={held}
            badged={Boolean(badgeKeys?.length)}
            testID={testID}
            onPress={() => {
              // A press that ended a drag is not also a tap. Gesture-handler
              // normally cancels the child responder for us; this is the belt to
              // those braces, because a drag that silently toggled selection
              // would be maddening.
              if (consumedByDrag()) return;
              if (suppressTapRef.current) {
                suppressTapRef.current = false;
                return;
              }
              onToggle(slot.id);
            }}
          />
        </SettleIn>
      </View>
    </GestureDetector>
  );
});

/**
 * Every size-dependent style the hand and its cards need, recomputed only
 * when `metrics` itself changes (see the `useMemo` in `HandZone`) — which is
 * what keeps `DraggableCard`'s memo intact through an entire drag: the same
 * `styles` object reference is handed to every card on every pointer move.
 */
function handStyles(m: Metrics, s: Skin) {
  const colors = s.colors;
  const dropArmed = s.dropArmed;
  return StyleSheet.create({
    autoArrange: { color: colors.accent, fontSize: m.panel.bodyFont, fontWeight: '600' },
    cards: { flexDirection: 'row', flexWrap: 'wrap', gap: 4, marginTop: 6 },
    hint: { color: colors.muted, fontSize: Math.max(9, m.panel.bodyFont - 2), marginTop: 6, fontStyle: 'italic' },
    lifted: { zIndex: 20, opacity: 0.92 },
    // Pulled up out of the fan rather than left flush with its neighbours, so
    // the card about to be played reads at a glance instead of needing its
    // border colour picked out from a row of a dozen others. `zIndex` keeps it
    // drawn over the cards it now overlaps at the top edge.
    raised: { transform: [{ translateY: -10 }], zIndex: 10 },
    // The wrapper every card and every gap sits in, so the two are the same size
    // to the pixel. Its border is always here and always this wide — only its
    // colour ever changes, the same discipline CardView's own ring keeps.
    slot: {
      borderRadius: 8,
      borderWidth: m.card.ringBorder,
      borderColor: 'transparent',
    },
    gapHidden: { display: 'none' },
    gapRing: {
      borderRadius: 8,
      borderWidth: m.card.ringBorder,
      borderColor: 'transparent',
      padding: m.card.ringPadding,
    },
    gapCard: {
      width: m.card.width,
      height: m.card.height,
      marginRight: m.card.gap,
      borderRadius: 6,
      borderWidth: 2,
      ...dropArmed,
    },
  });
}

type HandStyles = ReturnType<typeof handStyles>;
