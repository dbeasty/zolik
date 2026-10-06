import { useEffect, useMemo, useRef } from 'react';
import { Animated, Easing, StyleSheet, Text, useWindowDimensions } from 'react-native';

import { loopWithHeadStart } from '@/src/components/fun/loopWithHeadStart';
import { useReducedMotion } from '@/src/hooks/useReducedMotion';
import { ms } from '@/src/lib/motion';

/**
 * Suit symbols tumbling down the screen — the confetti of a card room.
 *
 * Positions, sizes and timings come from the piece's index rather than from
 * `Math.random()`, so a given screen always rains the same way: nothing flickers
 * on a re-render and a screenshot of it is reproducible. Not drawn at all under
 * reduce-motion: confetti is nothing but motion.
 */

const GLYPHS = ['♠', '♥', '♦', '♣'];
const RED = '#ef5350';
const PALE = '#f3efe2';

/** A cheap deterministic 0..1 from an index and a salt. */
const unit = (i: number, salt: number) => {
  const x = Math.sin(i * 12.9898 + salt * 78.233) * 43758.5453;
  return x - Math.floor(x);
};

export function CardRain({ pieces = 22 }: { pieces?: number }) {
  const stillness = useReducedMotion();
  const { height } = useWindowDimensions();
  if (stillness) return null;
  return (
    <>
      {Array.from({ length: pieces }, (_, i) => (
        <Piece key={i} i={i} total={pieces} fall={height + 80} />
      ))}
    </>
  );
}

function Piece({ i, total, fall }: { i: number; total: number; fall: number }) {
  const t = useRef(new Animated.Value(0)).current;
  const spec = useMemo(
    () => ({
      left: `${Math.round(((i + unit(i, 1) * 0.8) / total) * 100)}%` as `${number}%`,
      size: 18 + Math.round(unit(i, 2) * 22),
      duration: ms(1500 + Math.round(unit(i, 3) * 1400)),
      delay: Math.round(unit(i, 4) * ms(1100)),
      spin: (unit(i, 5) > 0.5 ? 1 : -1) * (180 + Math.round(unit(i, 6) * 360)),
      sway: (unit(i, 7) - 0.5) * 80,
      glyph: GLYPHS[i % GLYPHS.length]!,
    }),
    [i, total],
  );

  useEffect(() => {
    // The delay is the piece's one-off head start, not a pause before every fall.
    return loopWithHeadStart(
      t,
      { duration: spec.duration, easing: Easing.in(Easing.quad), useNativeDriver: true },
      spec.delay,
    );
  }, [t, spec]);

  const red = spec.glyph === '♥' || spec.glyph === '♦';
  return (
    <Animated.View
      pointerEvents="none"
      style={[
        styles.piece,
        {
          left: spec.left,
          opacity: t.interpolate({ inputRange: [0, 0.05, 0.9, 1], outputRange: [0, 1, 1, 0] }),
          transform: [
            { translateY: t.interpolate({ inputRange: [0, 1], outputRange: [-60, fall] }) },
            { translateX: t.interpolate({ inputRange: [0, 0.5, 1], outputRange: [0, spec.sway, 0] }) },
            { rotateZ: t.interpolate({ inputRange: [0, 1], outputRange: ['0deg', `${spec.spin}deg`] }) },
          ],
        },
      ]}
    >
      <Text style={{ fontSize: spec.size, color: red ? RED : PALE }}>{spec.glyph}</Text>
    </Animated.View>
  );
}

const styles = StyleSheet.create({
  piece: { position: 'absolute', top: 0 },
});
