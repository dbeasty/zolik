import { useEffect } from 'react';
import { Easing, useAnimatedStyle, useSharedValue, withRepeat, withTiming } from 'react-native-reanimated';

import { colors } from '@/src/theme';

// Gentle opacity pulse on a gold border — the shared "you can drop it here"
// cue used by every legal drop target (discard pile, meld-staging area,
// table melds) while a hand card is being dragged. Kept in one place so all
// three read as the same visual language instead of three slightly
// different highlights.
//
// Callers must include the returned style unconditionally (never gate it
// behind `active && pulseStyle`). react-native-web's Reanimated binding
// writes opacity straight onto the DOM node outside of React's own diffing;
// dropping the style from the array mid-fade freezes that inline opacity at
// whatever the last frame happened to be (e.g. a permanent 0.5) instead of
// clearing it, which reads as the target staying dim forever. Gating on
// `active` here, inside the worklet, avoids that: the style stays bound and
// resolves to a plain `{ opacity: 1 }` (no `borderColor`, so the caller's own
// base border shows through) once the fade-out finishes.
export function useDropPulseStyle(active: boolean) {
  const pulse = useSharedValue(0);
  useEffect(() => {
    if (active) {
      pulse.value = withRepeat(withTiming(1, { duration: 550, easing: Easing.inOut(Easing.ease) }), -1, true);
    } else {
      pulse.value = withTiming(0, { duration: 150 });
    }
  }, [active, pulse]);
  return useAnimatedStyle(() => {
    if (!active && pulse.value === 0) {
      return { opacity: 1 };
    }
    return {
      borderColor: colors.gold,
      opacity: 0.35 + pulse.value * 0.5,
    };
  });
}
