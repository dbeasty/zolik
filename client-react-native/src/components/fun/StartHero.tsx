import { useEffect, useRef, useState } from 'react';
import { Animated, Easing, StyleSheet, Text, View } from 'react-native';

import { CardCharacter, type Suit } from '@/src/components/fun/CardCharacter';
import { loopWithHeadStart } from '@/src/components/fun/loopWithHeadStart';
import { useReducedMotion } from '@/src/hooks/useReducedMotion';
import { cssLoop } from '@/src/lib/cssLoop';
import { ms } from '@/src/lib/motion';
import { colors } from '@/src/theme';

/**
 * The start page's little stage: four cards strolling across the felt in a
 * line, with suit symbols drifting up behind them.
 *
 * It is the first thing the product shows and the one place nothing is at
 * stake, so it can afford to be charming. It is also `pointerEvents="none"`,
 * a fixed height, and sits above the game list — nothing a tap is aimed at
 * ever moves because of it. Under reduce-motion the cards stand in a row.
 */

const SUITS: Suit[] = ['S', 'H', 'C', 'D'];
const DRIFT = ['♠', '♥', '♣', '♦', '♠', '♥'];
const STAGE_HEIGHT = 92;
const CARD = 34;
const SPACING = 52;

export function StartHero() {
  const stillness = useReducedMotion();
  const [width, setWidth] = useState(0);
  const march = useRef(new Animated.Value(0)).current;

  const train = SUITS.length * SPACING;
  const travel = width + train;
  // On the web the browser draws the march (see `cssLoop`).
  const web = cssLoop(
    { '0%': { transform: [{ translateX: 0 }] }, '100%': { transform: [{ translateX: travel }] } },
    { duration: ms(9000) },
  );
  // Walks off the right edge and comes back on at the left, forever.
  useEffect(() => {
    if (stillness || !width || web) return;
    march.setValue(0);
    const loop = Animated.loop(
      Animated.timing(march, {
        toValue: 1,
        duration: ms(9000),
        easing: Easing.linear,
        useNativeDriver: true,
      }),
    );
    loop.start();
    return () => loop.stop();
  }, [march, stillness, width, web]);

  return (
    <View
      testID="start-hero"
      pointerEvents="none"
      aria-hidden
      accessibilityElementsHidden
      importantForAccessibility="no-hide-descendants"
      style={styles.stage}
      onLayout={(e) => setWidth(e.nativeEvent.layout.width)}
    >
      {DRIFT.map((g, i) => (
        <Drifter key={i} glyph={g} index={i} still={stillness} />
      ))}
      <View style={styles.floor} />
      {width && web && !stillness ? (
        <View style={[styles.train, { left: -train }, web]}>
          {SUITS.map((s, i) => (
            <View key={s} style={{ width: SPACING }}>
              <CardCharacter suit={s} size={CARD} delay={i * 90} />
            </View>
          ))}
        </View>
      ) : width ? (
        <Animated.View
          style={[
            styles.train,
            {
              left: stillness ? width / 2 - train / 2 : -train,
              transform: [
                {
                  translateX: stillness
                    ? 0
                    : march.interpolate({ inputRange: [0, 1], outputRange: [0, travel] }),
                },
              ],
            },
          ]}
        >
          {SUITS.map((s, i) => (
            <View key={s} style={{ width: SPACING }}>
              <CardCharacter suit={s} size={CARD} delay={i * 90} />
            </View>
          ))}
        </Animated.View>
      ) : null}
    </View>
  );
}

function Drifter({ glyph, index, still }: { glyph: string; index: number; still: boolean }) {
  const t = useRef(new Animated.Value(0)).current;
  // On the web the browser draws the drift (see `cssLoop`).
  const web = cssLoop(
    {
      '0%': { opacity: 0.05, transform: [{ translateY: 14 }, { rotateZ: '-12deg' }] },
      '50%': { opacity: 0.28, transform: [{ translateY: 0 }, { rotateZ: '0deg' }] },
      '100%': { opacity: 0.05, transform: [{ translateY: -14 }, { rotateZ: '12deg' }] },
    },
    { duration: ms(4200 + index * 650), timing: 'ease-in-out', delay: index * ms(500) },
  );
  useEffect(() => {
    if (still || web) return;
    return loopWithHeadStart(
      t,
      {
        duration: ms(4200 + index * 650),
        easing: Easing.inOut(Easing.sin),
        useNativeDriver: true,
      },
      index * ms(500),
    );
  }, [t, still, index, web]);

  const red = glyph === '♥' || glyph === '♦';
  if (web && !still) {
    return (
      <View style={[{ position: 'absolute', left: `${8 + index * 16}%`, top: 6 + (index % 3) * 10 }, web]}>
        <Text style={{ fontSize: 26, color: red ? '#ef5350' : colors.text }}>{glyph}</Text>
      </View>
    );
  }
  return (
    <Animated.View
      style={{
        position: 'absolute',
        left: `${8 + index * 16}%`,
        top: 6 + (index % 3) * 10,
        opacity: still ? 0.18 : t.interpolate({ inputRange: [0, 0.5, 1], outputRange: [0.05, 0.28, 0.05] }),
        transform: [
          { translateY: t.interpolate({ inputRange: [0, 1], outputRange: [14, -14] }) },
          { rotateZ: t.interpolate({ inputRange: [0, 1], outputRange: ['-12deg', '12deg'] }) },
        ],
      }}
    >
      <Text style={{ fontSize: 26, color: red ? '#ef5350' : colors.text }}>{glyph}</Text>
    </Animated.View>
  );
}

const styles = StyleSheet.create({
  stage: {
    height: STAGE_HEIGHT,
    marginBottom: 8,
    overflow: 'hidden',
    justifyContent: 'flex-end',
  },
  floor: {
    position: 'absolute',
    left: 0,
    right: 0,
    bottom: 2,
    height: 2,
    borderRadius: 1,
    backgroundColor: colors.border,
  },
  train: {
    position: 'absolute',
    bottom: 2,
    flexDirection: 'row',
    alignItems: 'flex-end',
  },
});
