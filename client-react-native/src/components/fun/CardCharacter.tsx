import { useEffect, useRef } from 'react';
import { Animated, Easing, Platform, StyleSheet, Text, View, type ViewStyle } from 'react-native';

import { loopWithHeadStart } from '@/src/components/fun/loopWithHeadStart';
import { useReducedMotion } from '@/src/hooks/useReducedMotion';
import { cssLoop } from '@/src/lib/cssLoop';
import { ms } from '@/src/lib/motion';

/**
 * A playing card with legs.
 *
 * Pure decoration for the moments the product is allowed to be playful — the
 * start page and the end of a match. It never appears on the board itself and
 * it never catches a touch (callers put it in `pointerEvents="none"` layers).
 *
 * Same house rules as `SettleIn`: React Native's own `Animated`, native
 * driver, identical in a browser. Asked for stillness it is drawn once, mid
 * stride, and does not move.
 */

export type Suit = 'S' | 'H' | 'D' | 'C';

const GLYPH: Record<Suit, string> = { S: '♠', H: '♥', D: '♦', C: '♣' };
const RED = '#c62828';
const INK = '#1b1f27';

type Props = {
  suit: Suit;
  /** Card width in px; everything else scales from it. */
  size?: number;
  /**
   * 'walk' strides in place (the caller moves it); 'dance' hops and waves — the
   * end of a match; 'twirl' spins on the spot with arms out — the end of a round,
   * so the two endings never look alike.
   */
  mode?: 'walk' | 'dance' | 'twirl';
  /** Milliseconds before the first step, so a row doesn't move in lockstep. */
  delay?: number;
};

export function CardCharacter({ suit, size = 34, mode = 'walk', delay = 0 }: Props) {
  const beat = useRef(new Animated.Value(0)).current;
  const stillness = useReducedMotion();

  useEffect(() => {
    if (stillness) {
      beat.setValue(0.25);
      return;
    }
    if (Platform.OS === 'web') return;
    beat.setValue(0);
    // The delay is a one-off head start, not a pause before every step.
    return loopWithHeadStart(
      beat,
      {
        duration: mode === 'dance' ? ms(520) : mode === 'twirl' ? ms(900) : ms(480),
        easing: Easing.linear,
        useNativeDriver: true,
      },
      delay,
    );
  }, [beat, mode, delay, stillness]);

  const h = Math.round(size * 1.4);
  const leg = Math.round(size * 0.34);
  const colour = suit === 'H' || suit === 'D' ? RED : INK;
  const swing = (a: string, b: string) =>
    beat.interpolate({ inputRange: [0, 0.5, 1], outputRange: [a, b, a] });
  // Two bounces per cycle: a walker steps on each leg, a dancer hops on each beat.
  const hop = beat.interpolate({
    inputRange: [0, 0.25, 0.5, 0.75, 1],
    outputRange: [0, -size * 0.12, 0, -size * 0.12, 0],
  });
  const twirl = mode === 'twirl';
  const tilt = twirl ? '0deg' : swing(mode === 'dance' ? '-10deg' : '-3deg', mode === 'dance' ? '10deg' : '3deg');
  // A turn on the spot, drawn as the card narrowing to an edge and widening
  // again — twice per cycle, so it reads as a full pirouette.
  const turn = beat.interpolate({
    inputRange: [0, 0.2, 0.4, 0.6, 0.8, 1],
    outputRange: [1, 0.08, 1, 0.08, 1, 1],
  });
  // One tall leap per cycle rather than a hop on every beat.
  const leap = beat.interpolate({
    inputRange: [0, 0.3, 0.6, 1],
    outputRange: [0, -size * 0.45, 0, 0],
  });
  const lift = mode === 'dance' ? ['-150deg', '-30deg'] : twirl ? ['-100deg', '-80deg'] : ['25deg', '-25deg'];
  const armA = swing(lift[0]!, lift[1]!);
  const armB = swing(lift[1]!, lift[0]!);

  // On the web the browser draws the steps (see `cssLoop`): the same curves,
  // sampled into keyframes, so a row of walkers costs the page nothing.
  const webDuration = mode === 'dance' ? ms(520) : mode === 'twirl' ? ms(900) : ms(480);
  const sample = (points: [number, number][], x: number) => {
    for (let k = 1; k < points.length; k++) {
      const [x0, y0] = points[k - 1]!;
      const [x1, y1] = points[k]!;
      if (x <= x1) return y0 + ((y1 - y0) * (x - x0)) / (x1 - x0 || 1);
    }
    return points[points.length - 1]![1];
  };
  const deg = (v: string) => parseFloat(v);
  const tiltPts: [number, number][] = twirl
    ? [[0, 0], [1, 0]]
    : [[0, deg(mode === 'dance' ? '-10deg' : '-3deg')], [0.5, deg(mode === 'dance' ? '10deg' : '3deg')], [1, deg(mode === 'dance' ? '-10deg' : '-3deg')]];
  const hopPts: [number, number][] = twirl
    ? [[0, 0], [0.3, -size * 0.45], [0.6, 0], [1, 0]]
    : [[0, 0], [0.25, -size * 0.12], [0.5, 0], [0.75, -size * 0.12], [1, 0]];
  const turnPts: [number, number][] = [[0, 1], [0.2, 0.08], [0.4, 1], [0.6, 0.08], [0.8, 1], [1, 1]];
  const stops = twirl ? [0, 0.2, 0.3, 0.4, 0.6, 0.8, 1] : [0, 0.25, 0.5, 0.75, 1];
  const bodyFrames: Record<string, ViewStyle> = {};
  for (const x of stops) {
    bodyFrames[`${Math.round(x * 100)}%`] = {
      transform: [
        { translateY: sample(hopPts, x) },
        { rotateZ: `${sample(tiltPts, x)}deg` },
        ...(twirl ? [{ scaleX: sample(turnPts, x) }] : []),
      ],
    };
  }
  const swingFrames = (a: string, b: string): Record<string, ViewStyle> => ({
    '0%': { transform: [{ rotateZ: a }] },
    '50%': { transform: [{ rotateZ: b }] },
    '100%': { transform: [{ rotateZ: a }] },
  });
  const webBody = cssLoop(bodyFrames, { duration: webDuration, delay });
  const webA = cssLoop(swingFrames(lift[0]!, lift[1]!), { duration: webDuration, delay });
  const webB = cssLoop(swingFrames(lift[1]!, lift[0]!), { duration: webDuration, delay });

  const limb = (width: number, length: number) => ({
    position: 'absolute' as const,
    width,
    height: length,
    borderRadius: width / 2,
    backgroundColor: INK,
    transformOrigin: ['50%', '0%', 0] as [string, string, number],
  });

  const face = (
    <View style={[styles.card, { width: size, height: h, borderRadius: size * 0.14 }]}>
      <Text style={[styles.corner, { color: colour, fontSize: size * 0.3 }]}>{GLYPH[suit]}</Text>
      <Text style={[styles.pip, { color: colour, fontSize: size * 0.62 }]}>{GLYPH[suit]}</Text>
      {/* Eyes — the difference between a card and a character. */}
      <View style={[styles.eyes, { top: h * 0.1, gap: size * 0.16 }]}>
        <View style={[styles.eye, { width: size * 0.12, height: size * 0.16 }]} />
        <View style={[styles.eye, { width: size * 0.12, height: size * 0.16 }]} />
      </View>
    </View>
  );
  if (webBody && webA && webB && !stillness) {
    return (
      <View
        aria-hidden
        pointerEvents="none"
        accessibilityElementsHidden
        importantForAccessibility="no-hide-descendants"
        style={[{ width: size + leg, height: h + leg, alignItems: 'center' }, webBody]}
      >
        {face}
        <View style={[limb(size * 0.1, leg), { top: h - 2, left: leg / 2 + size * 0.25 }, webA]} />
        <View style={[limb(size * 0.1, leg), { top: h - 2, left: leg / 2 + size * 0.65 }, webB]} />
        <View style={[limb(size * 0.08, leg * 0.8), { top: h * 0.4, left: leg / 2 - 2 }, webB]} />
        <View style={[limb(size * 0.08, leg * 0.8), { top: h * 0.4, left: leg / 2 + size - 2 }, webA]} />
      </View>
    );
  }

  return (
    <Animated.View
      aria-hidden
      pointerEvents="none"
      accessibilityElementsHidden
      importantForAccessibility="no-hide-descendants"
      style={{
        width: size + leg,
        height: h + leg,
        alignItems: 'center',
        transform: [{ translateY: twirl ? leap : hop }, { rotateZ: tilt }, ...(twirl ? [{ scaleX: turn }] : [])],
      }}
    >
      <View style={[styles.card, { width: size, height: h, borderRadius: size * 0.14 }]}>
        <Text style={[styles.corner, { color: colour, fontSize: size * 0.3 }]}>{GLYPH[suit]}</Text>
        <Text style={[styles.pip, { color: colour, fontSize: size * 0.62 }]}>{GLYPH[suit]}</Text>
        {/* Eyes — the difference between a card and a character. */}
        <View style={[styles.eyes, { top: h * 0.1, gap: size * 0.16 }]}>
          <View style={[styles.eye, { width: size * 0.12, height: size * 0.16 }]} />
          <View style={[styles.eye, { width: size * 0.12, height: size * 0.16 }]} />
        </View>
      </View>
      <Animated.View
        style={[limb(size * 0.1, leg), { top: h - 2, left: leg / 2 + size * 0.25, transform: [{ rotateZ: armA }] }]}
      />
      <Animated.View
        style={[limb(size * 0.1, leg), { top: h - 2, left: leg / 2 + size * 0.65, transform: [{ rotateZ: armB }] }]}
      />
      {/* Arms, on the card's flanks. */}
      <Animated.View
        style={[limb(size * 0.08, leg * 0.8), { top: h * 0.4, left: leg / 2 - 2, transform: [{ rotateZ: armB }] }]}
      />
      <Animated.View
        style={[
          limb(size * 0.08, leg * 0.8),
          { top: h * 0.4, left: leg / 2 + size - 2, transform: [{ rotateZ: armA }] },
        ]}
      />
    </Animated.View>
  );
}

const styles = StyleSheet.create({
  card: {
    backgroundColor: '#fbfaf5',
    borderWidth: 1,
    borderColor: 'rgba(0,0,0,0.25)',
    alignItems: 'center',
    justifyContent: 'center',
    shadowColor: '#000',
    shadowOffset: { width: 0, height: 3 },
    shadowOpacity: 0.3,
    shadowRadius: 4,
    elevation: 3,
  },
  corner: { position: 'absolute', top: 1, left: 3, fontWeight: '700' },
  pip: { marginTop: 8 },
  eyes: { position: 'absolute', flexDirection: 'row' },
  eye: { borderRadius: 4, backgroundColor: INK },
});
