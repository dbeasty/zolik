import { useEffect, useRef } from 'react';
import { Animated, StyleSheet, Text, View } from 'react-native';

import { useReducedMotion } from '@/src/hooks/useReducedMotion';
import { ARRIVAL_EASING, ms } from '@/src/lib/motion';
import { colors } from '@/src/theme';

const CARDS = [
  { rank: 'A', suit: '♠', red: false },
  { rank: 'K', suit: '♥', red: true },
  { rank: 'Q', suit: '♣', red: false },
  { rank: 'J', suit: '♦', red: true },
  { rank: '10', suit: '♠', red: false },
] as const;

const SPREAD_DEG = 14;

/**
 * Five cards dealt into a fan: the menu's one piece of decoration, and the
 * only thing on it that is not information.
 *
 * Each card turns about a point below itself, from a pile at the left of the
 * fan out to its place. Transform and opacity only, native-driven, and it
 * plays once per mount; asking the system for less motion shows the finished
 * fan. Decorative, so hidden from screen readers.
 */
export function CardFan({ size = 1 }: { size?: number }) {
  const still = useReducedMotion();
  const dealt = useRef(CARDS.map(() => new Animated.Value(still ? 1 : 0))).current;

  useEffect(() => {
    if (still) {
      dealt.forEach((v) => v.setValue(1));
      return undefined;
    }
    const run = Animated.stagger(
      ms(70),
      dealt.map((v) =>
        Animated.timing(v, {
          toValue: 1,
          duration: ms(380),
          easing: ARRIVAL_EASING,
          useNativeDriver: true,
        }),
      ),
    );
    run.start();
    return () => run.stop();
  }, [still, dealt]);

  const w = 44 * size;
  const h = 64 * size;
  return (
    <View
      aria-hidden
      importantForAccessibility="no-hide-descendants"
      style={{ width: w * 3.2, height: h * 1.15 }}
    >
      {CARDS.map((card, i) => {
        const angle = (i - (CARDS.length - 1) / 2) * SPREAD_DEG;
        const v = dealt[i];
        return (
          <Animated.View
            key={i}
            style={[
              styles.card,
              { width: w, height: h, left: w * 1.1 },
              {
                opacity: v,
                transform: [
                  { translateY: h * 0.45 },
                  { rotate: v.interpolate({ inputRange: [0, 1], outputRange: ['-80deg', `${angle}deg`] }) },
                  { translateY: -h * 0.45 },
                  { translateX: v.interpolate({ inputRange: [0, 1], outputRange: [-w * 1.4, 0] }) },
                ],
              },
            ]}
          >
            <Text style={[styles.rank, card.red && styles.red, { fontSize: 14 * size }]}>
              {card.rank}
            </Text>
            <Text style={[styles.rank, card.red && styles.red, { fontSize: 14 * size }]}>
              {card.suit}
            </Text>
          </Animated.View>
        );
      })}
    </View>
  );
}

const styles = StyleSheet.create({
  card: {
    position: 'absolute',
    top: 4,
    borderRadius: 6,
    borderWidth: 1,
    borderColor: colors.border,
    backgroundColor: '#f4f1e8',
    paddingHorizontal: 5,
    paddingTop: 3,
  },
  rank: { color: '#12151c', fontWeight: '700', lineHeight: 16 },
  red: { color: '#c0392b' },
});
