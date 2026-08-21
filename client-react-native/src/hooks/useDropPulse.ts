import { useEffect } from 'react';
import { Easing, useAnimatedStyle, useSharedValue, withRepeat, withTiming } from 'react-native-reanimated';

import { colors } from '@/src/theme';

// Gentle opacity pulse on a gold border — the shared "you can drop it here"
// cue used by every legal drop target (discard pile, meld-staging area,
// table melds) while a hand card is being dragged. Kept in one place so all
// three read as the same visual language instead of three slightly
// different highlights.
export function useDropPulseStyle(active: boolean) {
  const pulse = useSharedValue(0);
  useEffect(() => {
    if (active) {
      pulse.value = withRepeat(withTiming(1, { duration: 550, easing: Easing.inOut(Easing.ease) }), -1, true);
    } else {
      pulse.value = withTiming(0, { duration: 150 });
    }
  }, [active, pulse]);
  return useAnimatedStyle(() => ({
    borderColor: colors.gold,
    opacity: 0.35 + pulse.value * 0.5,
  }));
}
