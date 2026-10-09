import { useEffect, useState } from 'react';
import { AccessibilityInfo, Platform } from 'react-native';

import { useA11yPrefs } from '@/src/a11y/prefs';

/**
 * Whether the person has asked their system for less movement.
 *
 * Extracted from `useArrival` so every animation in the client honors the
 * same setting the same way: someone who asked for stillness is told about
 * something appearing by its being there, not by its travel.
 *
 * On web this reads the `prefers-reduced-motion` media query directly rather
 * than trusting `AccessibilityInfo` to — which also gives the e2e suite a
 * deterministic board: Playwright emulates `reduce`, and every entrance
 * snaps to its final frame instead of racing the test's first measurement.
 *
 * The in-app choice (Settings → Accessibility → Motion) layers over the
 * system's: `auto` follows it, `on` asks for stillness on a device whose
 * system setting is buried or missing, `off` keeps the board moving.
 */
export function useReducedMotion(): boolean {
  const { motion } = useA11yPrefs();
  const [stillness, setStillness] = useState(() => {
    if (Platform.OS === 'web' && typeof window !== 'undefined' && window.matchMedia) {
      return window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    }
    return false;
  });

  useEffect(() => {
    if (Platform.OS === 'web' && typeof window !== 'undefined' && window.matchMedia) {
      const mq = window.matchMedia('(prefers-reduced-motion: reduce)');
      const onChange = () => setStillness(mq.matches);
      onChange();
      mq.addEventListener?.('change', onChange);
      return () => mq.removeEventListener?.('change', onChange);
    }

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

  if (motion === 'on') return true;
  if (motion === 'off') return false;
  return stillness;
}
