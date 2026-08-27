import { useEffect, useRef, useState } from 'react';
import { AccessibilityInfo, Animated, Easing } from 'react-native';

/**
 * A short arrival for something that appears because a thing just happened.
 *
 * The board changes constantly and almost none of it is an event: a card moves,
 * a count ticks, a control greys out. The end of a round is different — the
 * table stops, and a panel that was not there is suddenly there — and it was
 * arriving with no more ceremony than a count changing, in the middle of a page
 * whose reader was probably looking at their own hand. People sat waiting for
 * something to happen after it already had.
 *
 * So this is about noticing, not decoration: fade up and rise a little, once,
 * and then be still. It re-plays when `key` changes, so each round's result
 * announces itself rather than only the first.
 *
 * Built on React Native's own Animated rather than Reanimated because it drives
 * two properties for a quarter of a second and has to work identically in a
 * browser, where the web client and its end-to-end tests actually run.
 */
export function useArrival(key: string | number) {
  const progress = useRef(new Animated.Value(0)).current;
  const [stillness, setStillness] = useState(false);

  // Someone who has asked their system for less movement is told about
  // something appearing by its being there, not by its travel.
  useEffect(() => {
    let live = true;
    AccessibilityInfo.isReduceMotionEnabled()
      .then((on) => live && setStillness(on))
      .catch(() => {});
    const sub = AccessibilityInfo.addEventListener('reduceMotionChanged', (on) =>
      setStillness(!!on),
    );
    return () => {
      live = false;
      sub?.remove?.();
    };
  }, []);

  useEffect(() => {
    if (stillness) {
      progress.setValue(1);
      return;
    }
    progress.setValue(0);
    const anim = Animated.timing(progress, {
      toValue: 1,
      duration: 260,
      easing: Easing.out(Easing.cubic),
      useNativeDriver: true,
    });
    anim.start();
    return () => anim.stop();
  }, [key, stillness, progress]);

  return {
    opacity: progress,
    transform: [
      {
        translateY: progress.interpolate({ inputRange: [0, 1], outputRange: [10, 0] }),
      },
    ],
  };
}
