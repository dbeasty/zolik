import { useEffect, useRef } from 'react';
import { Animated, Pressable, StyleSheet, Text } from 'react-native';

import { useReducedMotion } from '@/src/hooks/useReducedMotion';
import { t } from '@/src/lib/i18n';

/** Rules orange — the same on every skin, so the button reads as the one thing to press. */
const ORANGE = '#F28C28';

/**
 * The rules link in the board's header line, dressed as a button.
 *
 * Before the game starts (`pulse`) it breathes gently: a table in the lobby
 * is the moment someone who has never played this game wants the rules, and a
 * 12px blue link is easy to miss. Once play begins it holds still, and it
 * holds still for anyone who asked for less motion.
 */
export function RulesButton({ onPress, pulse }: { onPress: () => void; pulse: boolean }) {
  const reduced = useReducedMotion();
  const breath = useRef(new Animated.Value(0)).current;
  const animate = pulse && !reduced;

  useEffect(() => {
    if (!animate) {
      breath.setValue(0);
      return;
    }
    const loop = Animated.loop(
      Animated.sequence([
        Animated.timing(breath, { toValue: 1, duration: 900, useNativeDriver: true }),
        Animated.timing(breath, { toValue: 0, duration: 900, useNativeDriver: true }),
      ]),
    );
    loop.start();
    return () => loop.stop();
  }, [animate, breath]);

  const scale = breath.interpolate({ inputRange: [0, 1], outputRange: [1, 1.1] });

  return (
    <Animated.View style={{ transform: [{ scale }] }}>
      <Pressable
        testID="match-rules"
        accessibilityRole="link"
        onPress={onPress}
        hitSlop={8}
        style={styles.button}
      >
        <Text style={styles.label}>{t('nav.rules')}</Text>
      </Pressable>
    </Animated.View>
  );
}

const styles = StyleSheet.create({
  button: {
    minHeight: 24,
    justifyContent: 'center',
    paddingHorizontal: 10,
    borderRadius: 12,
    backgroundColor: ORANGE,
  },
  label: { color: '#1A1208', fontSize: 12, fontWeight: '800' },
});
