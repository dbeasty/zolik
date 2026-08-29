import { useEffect, useRef, useState } from 'react';
import { Animated, Easing, StyleSheet, View } from 'react-native';

import { CardBack } from '@/src/components/CardBack';
import { CardView } from '@/src/components/CardView';
import { useMetrics } from '@/src/hooks/useMetrics';
import { FLIGHT_MS, FLIGHT_STALE_MS, type Flight } from '@/src/lib/flights';
import type { Rect } from '@/src/lib/hand';

/**
 * The air above the board: every card currently travelling between zones.
 *
 * Drawn once, over everything, and touchable by nothing — a flight is pure
 * narration of a state the board already shows, so this layer must never
 * catch a pointer or move a measurement. Positions come from the same drop
 * registry a drag hit-tests against, read fresh per flight because the board
 * scrolls; a flight whose endpoints can't be measured simply doesn't fly,
 * and the board still snaps to the truth.
 *
 * Same house rules as `SettleIn`: React Native's own `Animated`, native
 * driver, works identically in a browser. The caller keeps this layer empty
 * under reduced motion by never planning flights there.
 */

/** A planned flight plus when it was put in the air — see FLIGHT_STALE_MS. */
export type QueuedFlight = Flight & { bornAt: number };

type Props = {
  flights: readonly QueuedFlight[];
  /** Window-coordinate rect for a registered element id. */
  rectFor: (id: string) => Rect | undefined;
  /** Re-reads every registered rect — the board may have scrolled. */
  measure: () => void;
  onDone: (id: string) => void;
};

type Measurable = { measureInWindow: (cb: (x: number, y: number, w: number, h: number) => void) => void };

export function FlightLayer({ flights, rectFor, measure, onDone }: Props) {
  // Rects are in window coordinates but this layer starts below the
  // navigation header, so everything it draws is offset by its own origin.
  const origin = useRef({ x: 0, y: 0 });
  const selfRef = useRef<Measurable | null>(null);
  const remeasureOrigin = () => {
    selfRef.current?.measureInWindow((x, y) => {
      origin.current = { x, y };
    });
  };

  if (!flights.length) return null;

  return (
    <View
      ref={(n) => {
        selfRef.current = n as unknown as Measurable | null;
      }}
      pointerEvents="none"
      style={styles.layer}
      onLayout={remeasureOrigin}
      testID="flight-layer"
    >
      {flights.map((f) => (
        <FlightCard key={f.id} flight={f} rectFor={rectFor} measure={measure} origin={origin} onDone={onDone} />
      ))}
    </View>
  );
}

function FlightCard({
  flight,
  rectFor,
  measure,
  origin,
  onDone,
}: {
  flight: QueuedFlight;
  rectFor: (id: string) => Rect | undefined;
  measure: () => void;
  origin: { current: { x: number; y: number } };
  onDone: (id: string) => void;
}) {
  const metrics = useMetrics();
  const progress = useRef(new Animated.Value(0)).current;
  const [path, setPath] = useState<{ from: Rect; to: Rect } | null>(null);
  const doneRef = useRef(onDone);
  doneRef.current = onDone;

  useEffect(() => {
    // The rects were last read at the previous gesture — or never, on the
    // session's very first flight. Re-measure, then try each frame until
    // both endpoints have answered; `measureInWindow` replies on its own
    // tick, and the first pass of a session can take a few frames to land.
    measure();
    let alive = true;
    const raf =
      typeof requestAnimationFrame === 'function'
        ? requestAnimationFrame
        : (cb: () => void) => setTimeout(cb, 16);
    const attempt = () => {
      if (!alive) return;
      // Also the bound on the retries above — and the reason a background
      // tab, whose frames the browser throttles, never releases a backlog
      // of stale cards when it comes back: too old never takes off.
      if (Date.now() - flight.bornAt > FLIGHT_STALE_MS) {
        doneRef.current(flight.id);
        return;
      }
      const from = rectFor(flight.fromId);
      const to = rectFor(flight.toId);
      if (!from || !to) {
        raf(attempt);
        return;
      }
      setPath({ from, to });
      Animated.timing(progress, {
        toValue: 1,
        duration: FLIGHT_MS,
        easing: Easing.inOut(Easing.cubic),
        useNativeDriver: true,
      }).start(() => doneRef.current(flight.id));
    };
    raf(attempt);
    return () => {
      alive = false;
    };
    // A flight is one journey: its endpoints are fixed at take-off.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  if (!path) return null;

  const w = metrics.card.width;
  const h = metrics.card.height;
  const centre = (r: Rect) => ({ x: r.x + r.width / 2, y: r.y + r.height / 2 });
  const from = centre(path.from);
  const to = centre(path.to);
  const dx = to.x - from.x;
  const dy = to.y - from.y;

  return (
    <Animated.View
      style={{
        position: 'absolute',
        left: from.x - w / 2 - origin.current.x,
        top: from.y - h / 2 - origin.current.y,
        // Fades in over the first stretch and out on landing, so a card
        // neither pops into the air nor doubles the one the board shows.
        opacity: progress.interpolate({ inputRange: [0, 0.12, 0.85, 1], outputRange: [0, 1, 1, 0] }),
        transform: [
          { perspective: 800 },
          { translateX: progress.interpolate({ inputRange: [0, 1], outputRange: [0, dx] }) },
          { translateY: progress.interpolate({ inputRange: [0, 1], outputRange: [0, dy] }) },
          // Lifts off the felt mid-journey and settles back down — the same
          // reads-as-height trick the dragged card uses, animated.
          { scale: progress.interpolate({ inputRange: [0, 0.5, 1], outputRange: [1, 1.14, 1] }) },
          { rotateZ: progress.interpolate({ inputRange: [0, 1], outputRange: [dx >= 0 ? '-4deg' : '4deg', '2deg'] }) },
          // A slight turn around the vertical edge, the peel of a card
          // leaving one place for another.
          { rotateY: progress.interpolate({ inputRange: [0, 1], outputRange: ['18deg', '0deg'] }) },
        ],
      }}
    >
      <View style={styles.shadow}>
        {flight.card && !flight.faceDown ? (
          <CardView card={flight.card} />
        ) : (
          <CardBack width={w} height={h} />
        )}
      </View>
    </Animated.View>
  );
}

const styles = StyleSheet.create({
  // Above the drag layer's raised chain (zIndex 100): a flight narrates the
  // newest state and nothing on the board should slice it in half.
  layer: { position: 'absolute', top: 0, bottom: 0, left: 0, right: 0, zIndex: 200 },
  shadow: {
    shadowColor: '#000',
    shadowOffset: { width: 0, height: 10 },
    shadowOpacity: 0.4,
    shadowRadius: 14,
    elevation: 10,
  },
});
